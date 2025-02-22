package collection

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/Pjt727/classy/collection/services"
	"github.com/Pjt727/classy/data"
	"github.com/Pjt727/classy/data/db"
	"github.com/jackc/pgx/v5/pgxpool"
	log "github.com/sirupsen/logrus"
)

// note logging is generally made with the api server in mind so the semantics
//    might be a little confusing when running these commands from CMD commands

type Service interface {

	// get the name of the service
	GetName() string

	// get the schools for this service (only called once at the start of the program)
	ListValidSchools(logger log.Entry, ctx context.Context, q *db.Queries) ([]db.School, error)

	// Should put ALL sections / meetings in the staging table as well as make
	//     the needed professors / courses for the sections and meeting times
	// ALL sections and meeting times must be put because they are diffed with the
	//     current state of the database to make deletions
	StageAllClasses(
		logger log.Entry,
		ctx context.Context,
		q *db.Queries,
		term db.TermCollection,
		fullCollection bool,
	) error

	// get the terms that school (does NOT upsert them to the db)
	GetTermCollections(
		logger log.Entry,
		ctx context.Context,
		school db.School,
	) ([]db.UpsertTermCollectionParams, error)
}

type Orchestrator struct {
	serviceEntries      []Service
	schoolIdToService   map[string]*Service
	schoolIdToSchool    map[string]db.School
	orchestrationLogger *log.Entry
	dbPool              *pgxpool.Pool
}

func GetDefaultOrchestrator() (Orchestrator, error) {
	ctx := context.Background()
	dbPool, err := data.NewPool(ctx)
	orchestrator := Orchestrator{
		serviceEntries:      []Service{services.Banner},
		schoolIdToService:   make(map[string]*Service),
		schoolIdToSchool:    make(map[string]db.School),
		orchestrationLogger: log.WithFields(log.Fields{"job": "orchestration"}),
		dbPool:              dbPool,
	}
	if err != nil {
		return orchestrator, err
	}

	orchestrator.initMappings(ctx)

	return orchestrator, nil
}

func (o Orchestrator) initMappings(ctx context.Context) {
	for _, service := range o.serviceEntries {
		serviceLogger := o.orchestrationLogger.WithField("service", service.GetName())
		q := db.New(o.dbPool)
		schools, err := service.ListValidSchools(*serviceLogger, ctx, q)
		if err != nil {
			serviceLogger.Warn("Skipping school to service mapping because error: ", err)
			continue
		}

		for _, school := range schools {
			o.schoolIdToService[school.ID] = &service
			o.schoolIdToSchool[school.ID] = school
		}
	}
}

func (o Orchestrator) GetSchoolById(schoolId string) (db.School, bool) {
	school, ok := o.schoolIdToSchool[schoolId]
	return school, ok
}

func (o Orchestrator) UpsertAllSchools(ctx context.Context) {
	tx, err := o.dbPool.Begin(ctx)
	if err != nil {
		o.orchestrationLogger.Error("Couldn't begin transaction", err)
		return
	}
	q := db.New(o.dbPool).WithTx(tx)
	for _, school := range o.schoolIdToSchool {
		err = q.UpsertSchool(
			ctx,
			db.UpsertSchoolParams{
				ID:   school.ID,
				Name: school.Name,
			})
		if err != nil {
			o.orchestrationLogger.Error("Couldn't add school", err)
			tx.Rollback(ctx)
			return
		}
	}
	tx.Commit(ctx)
}

func (o Orchestrator) UpsertSchoolTerms(ctx context.Context, logger log.Entry, school db.School) error {
	service, ok := o.schoolIdToService[school.ID]
	if !ok {
		return errors.New(fmt.Sprintf("Do not know how to scrape %. No service was found.", school.ID))
	}
	tx, err := o.dbPool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Commit(ctx)
	q := db.New(tx)
	logger.Tracef(`starting collection terms`)
	termCollections, err := (*service).GetTermCollections(logger, ctx, school)
	logger.Tracef(`finished collection terms`)
	if err != nil {
		logger.Trace(`propagating commit err: `, err)
		tx.Rollback(ctx)
		return err
	}

	logger.Tracef(`starting adding collection terms to db`)
	terms := make([]db.UpsertTermParams, len(termCollections))
	for i, termCollection := range termCollections {
		terms[i] = db.UpsertTermParams{
			Year:   termCollection.Year,
			Season: termCollection.Season,
		}
	}

	if err := q.UpsertSchool(ctx, db.UpsertSchoolParams{
		ID:   school.ID,
		Name: school.Name,
	}); err != nil {
		return err
	}

	dt := q.UpsertTerm(ctx, terms)
	dt.Exec(func(i int, e error) {
		if e != nil {
			tx.Rollback(ctx)
			err = e
			return
		}
	})
	if err != nil {
		return err
	}

	dtc := q.UpsertTermCollection(ctx, termCollections)
	dtc.Exec(func(i int, e error) {
		if e != nil {
			tx.Rollback(ctx)
			err = e
			return
		}
	})
	if err != nil {
		return err
	}
	logger.Tracef(`finished adding %d collection terms to db`, len(termCollections))
	tx.Commit(ctx)
	return nil
}

func (o Orchestrator) UpsertAllTerms(ctx context.Context) {
	var wg sync.WaitGroup
	errCh := make(chan error)
	numberOfWorkers := len(o.schoolIdToService)
	o.orchestrationLogger.Infof(`Starting to add %d school's terms`, numberOfWorkers)
	wg.Add(numberOfWorkers)
	for school_id, s := range o.schoolIdToService {
		school := o.schoolIdToSchool[school_id]
		termLogger := o.orchestrationLogger.WithFields(log.Fields{
			"school_id": school_id,
			"service":   (*s).GetName(),
		})
		go func() {
			defer wg.Done()
			if err := o.UpsertSchoolTerms(ctx, *termLogger, school); err != nil {
				errCh <- err
			}
		}()
	}

	go func() {
		wg.Wait()
		close(errCh)
	}()

	errorCount := 0
	for err := range errCh {
		o.orchestrationLogger.Error("There was an error collecting terms: ", err)
		errorCount++
	}

	o.orchestrationLogger.Infof(`Added %d school's terms successfully`, numberOfWorkers-errorCount)

}

func (o Orchestrator) UpdateAllSectionsOfSchool(ctx context.Context, schoolId string, termCollection db.TermCollection) {
	// there might be a good way to easily sandbox schoools to what they should change
	//    could make a wrapping q client with the state of the school and then write wrappers
	//    for each of the functions
	service, ok := o.schoolIdToService[schoolId]
	updateLogger := o.orchestrationLogger.WithFields(log.Fields{
		"school_id": schoolId,
		"season":    termCollection.Season,
		"year":      termCollection.Year,
	})
	if !ok {
		updateLogger.Error("Skipping update school... school not found")
		return
	}
	school := o.schoolIdToSchool[schoolId]
	updateLogger = o.orchestrationLogger.WithFields(log.Fields{
		"service":        (*service).GetName(),
		"school":         school,
		"termCollection": termCollection,
	})
	q := db.New(o.dbPool)
	if err := q.DeleteCoursesMeetingsStaging(ctx, termCollection); err != nil {
		updateLogger.Error("Could not ready staging tables", err)
		return
	}
	// defer q.CleanupCoursesMeetingsStaging(ctx)
	if err := (*service).StageAllClasses(*updateLogger, ctx, q, termCollection, false); err != nil {
		updateLogger.Error("Update sections aborting any staged sections/ meetings", updateLogger)
		return
	}
	tx, err := o.dbPool.Begin(ctx)

	if err != nil {
		updateLogger.Error("couldn't begin transaction: ", err)
		return
	}
	defer tx.Commit(ctx)
	q = db.New(o.dbPool).WithTx(tx)
	changesCount, err := q.MoveStagedCoursesAndMeetings(ctx, termCollection)
	if err != nil {
		updateLogger.Error("Failed moving courses: ", err)
		tx.Rollback(ctx)
		return
	}
	updateLogger.Infof("updated %d meetings and sections", changesCount)
}
