package collection

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/Pjt727/classy/collection/services/banner"

	"github.com/Pjt727/classy/data"
	classentry "github.com/Pjt727/classy/data/class-entry"
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
	ListValidSchools(
		logger log.Entry,
		ctx context.Context,
	) ([]classentry.School, error)

	// Should put ALL sections / meetings in the staging table as well as make
	//     the needed professors / courses for the sections and meeting times
	// ALL sections and meeting times must be put because they are diffed with the
	//     current state of the database to make deletions
	StageAllClasses(
		logger log.Entry,
		ctx context.Context,
		q *classentry.EntryQueries,
		schoolID string,
		termCollection classentry.TermCollection,
		fullCollection bool,
	) error

	// get the terms that school (does NOT upsert them to the db)
	GetTermCollections(
		logger log.Entry,
		ctx context.Context,
		school classentry.School,
	) ([]classentry.TermCollection, error)
}

type Orchestrator struct {
	serviceEntries map[string]Service
	// it would not be good if the same term collection was being
	//    collected by multiple workers at the same time
	termCollectionStagingLocks map[db.TermCollection]bool
	schoolIdToService          map[string]*Service
	schoolIdToSchool           map[string]db.School
	orchestrationLogger        *log.Entry
	dbPool                     *pgxpool.Pool
}

var DefaultEnabledServices []Service

func init() {
	// might change to be determined by env variables or accesible resources
	DefaultEnabledServices = []Service{banner.Service}
}

func GetDefaultOrchestrator() (Orchestrator, error) {
	ctx := context.Background()
	dbPool, err := data.NewPool(ctx)

	serviceEntries := make(map[string]Service, 0)
	for _, service := range DefaultEnabledServices {
		serviceEntries[service.GetName()] = service
	}

	orchestrator := Orchestrator{
		serviceEntries:             serviceEntries,
		termCollectionStagingLocks: map[db.TermCollection]bool{},
		schoolIdToService:          make(map[string]*Service),
		schoolIdToSchool:           make(map[string]db.School),
		orchestrationLogger:        log.WithFields(log.Fields{"job": "orchestration"}),
		dbPool:                     dbPool,
	}
	if err != nil {
		return orchestrator, err
	}

	orchestrator.initMappings(ctx)

	return orchestrator, nil
}

func CreateOrchestrator(services []Service, logger *log.Entry) (Orchestrator, error) {
	if logger == nil {
		logger = log.WithFields(log.Fields{"job": "orchestration"})
	}
	ctx := context.Background()
	dbPool, err := data.NewPool(ctx)

	serviceEntries := make(map[string]Service, 0)
	for _, service := range services {
		serviceEntries[service.GetName()] = service
	}

	orchestrator := Orchestrator{
		serviceEntries:             serviceEntries,
		termCollectionStagingLocks: map[db.TermCollection]bool{},
		schoolIdToService:          make(map[string]*Service),
		schoolIdToSchool:           make(map[string]db.School),
		orchestrationLogger:        logger,
		dbPool:                     dbPool,
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
		schools, err := service.ListValidSchools(*serviceLogger, ctx)
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

type SchoolWithService struct {
	School      db.School
	ServiceName string
}

func (o Orchestrator) GetSchoolsWithService() []SchoolWithService {
	schools := make([]SchoolWithService, 0)
	for schoolId, service := range o.schoolIdToService {
		schools = append(schools, SchoolWithService{
			School:      o.schoolIdToSchool[schoolId],
			ServiceName: (*service).GetName(),
		})
	}
	return schools
}

func (o Orchestrator) GetSchoolById(schoolId string) (db.School, bool) {
	school, ok := o.schoolIdToSchool[schoolId]
	return school, ok
}

func (o Orchestrator) UpsertAllSchools(ctx context.Context) error {
	tx, err := o.dbPool.Begin(ctx)
	if err != nil {
		o.orchestrationLogger.Error("Couldn't begin transaction", err)
		return err
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
			return err
		}
	}
	tx.Commit(ctx)
	return nil
}

// uses the "best" service for the job
func (o Orchestrator) UpsertSchoolTerms(
	ctx context.Context,
	logger log.Entry,
	school db.School,
) error {
	logger.Info("starting collection and db addition of colleciton terms")
	service, ok := o.schoolIdToService[school.ID]
	if !ok {
		return errors.New(
			fmt.Sprintf("Do not know how to scrape %s. No service was found.", school.ID),
		)
	}
	o.UpsertSchoolTermsWithService(ctx, logger, school, (*service).GetName())
	return nil
}

// uses the specified service for the job
func (o Orchestrator) UpsertSchoolTermsWithService(
	ctx context.Context,
	logger log.Entry,
	school db.School,
	serviceName string,
) error {
	logger.Info("starting collection and db addition of colleciton terms")
	service, ok := o.serviceEntries[serviceName]
	if !ok {
		return errors.New(fmt.Sprintf("The service `%s` was not found.", serviceName))
	}
	tx, err := o.dbPool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Commit(ctx)
	q := db.New(tx)
	logger.Tracef(`starting collection from service of terms`)
	termCollections, err := service.GetTermCollections(logger, ctx, school)
	logger.Tracef(`finished collecting %d collection terms`, len(termCollections))
	if err != nil {
		logger.Trace(`propagating commit err: `, err)
		tx.Rollback(ctx)
		return err
	}

	logger.Tracef(`starting adding collection terms to db`)
	terms := make([]db.UpsertTermParams, len(termCollections))
	for i, termCollection := range termCollections {
		terms[i] = db.UpsertTermParams{
			Year:   termCollection.Term.Year,
			Season: termCollection.Term.Season,
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

	dbTermCollections := make([]db.UpsertTermCollectionParams, len(termCollections))
	for i, t := range termCollections {
		dbTermCollections[i] = db.UpsertTermCollectionParams{
			ID:              t.ID,
			SchoolID:        school.ID,
			Year:            t.Term.Year,
			Season:          t.Term.Season,
			Name:            t.Name,
			StillCollecting: t.StillCollecting,
		}
	}

	dtc := q.UpsertTermCollection(ctx, dbTermCollections)
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
	logger.Infof(`finished adding %d collection terms to db`, len(termCollections))
	tx.Commit(ctx)
	return nil
}

func (o Orchestrator) UpsertSchoolServiceTerms(
	ctx context.Context,
	logger log.Entry,
	school db.School,
) error {
	logger.Info("starting collection and db addition of colleciton terms")
	service, ok := o.schoolIdToService[school.ID]
	if !ok {
		return errors.New(
			fmt.Sprintf("Do not know how to scrape %s. No service was found.", school.ID),
		)
	}
	tx, err := o.dbPool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Commit(ctx)
	q := db.New(tx)
	logger.Tracef(`starting collection from service of terms`)
	termCollections, err := (*service).GetTermCollections(logger, ctx, school)
	logger.Tracef(`finished collecting %d collection terms`, len(termCollections))
	if err != nil {
		logger.Trace(`propagating commit err: `, err)
		tx.Rollback(ctx)
		return err
	}

	logger.Tracef(`starting adding collection terms to db`)
	terms := make([]db.UpsertTermParams, len(termCollections))
	for i, termCollection := range termCollections {
		terms[i] = db.UpsertTermParams{
			Year:   termCollection.Term.Year,
			Season: termCollection.Term.Season,
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

	dbTermCollections := make([]db.UpsertTermCollectionParams, len(termCollections))
	for i, t := range termCollections {
		dbTermCollections[i] = db.UpsertTermCollectionParams{
			ID:              t.ID,
			SchoolID:        school.ID,
			Year:            t.Term.Year,
			Season:          t.Term.Season,
			Name:            t.Name,
			StillCollecting: t.StillCollecting,
		}
	}

	dtc := q.UpsertTermCollection(ctx, dbTermCollections)
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
	logger.Infof(`finished adding %d collection terms to db`, len(termCollections))
	tx.Commit(ctx)
	return nil
}

func (o Orchestrator) UpsertAllTerms(ctx context.Context) error {
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
	if errorCount > 0 {
		return errors.New(fmt.Sprintf("There were %d school errors", errorCount))
	}
	return nil
}

func (o Orchestrator) UpdateAllSectionsOfSchool(
	ctx context.Context,
	termCollection db.TermCollection,
) error {
	// take care of locking until this is done
	if _, ok := o.termCollectionStagingLocks[termCollection]; ok {
		return errors.New(
			fmt.Sprint("Already updating a section for this term collection ", termCollection),
		)
	}
	o.termCollectionStagingLocks[termCollection] = true
	defer delete(o.termCollectionStagingLocks, termCollection)

	// there might be a good way to easily sandbox schoools to what they should change
	//    could make a wrapping q client with the state of the school and then write wrappers
	//    for each of the functions
	service, ok := o.schoolIdToService[termCollection.SchoolID]
	updateLogger := o.orchestrationLogger.WithFields(log.Fields{
		"school_id": termCollection.SchoolID,
		"season":    termCollection.Season,
		"year":      termCollection.Year,
	})
	if !ok {
		updateLogger.Error("Skipping update school... school not found")
		return errors.New(
			fmt.Sprintf("Could not find service for shool id %s", termCollection.SchoolID),
		)
	}
	school := o.schoolIdToSchool[termCollection.SchoolID]
	updateLogger = o.orchestrationLogger.WithFields(log.Fields{
		"service":        (*service).GetName(),
		"school":         school,
		"termCollection": termCollection,
	})
	q := classentry.NewEntryQuery(o.dbPool, termCollection.SchoolID, &termCollection.ID)
	classEntryTermCollection := classentry.TermCollection{
		ID: termCollection.ID,
		Term: classentry.Term{
			Year:   termCollection.Year,
			Season: termCollection.Season,
		},
		Name:            termCollection.Name,
		StillCollecting: termCollection.StillCollecting,
	}
	if err := q.DeleteSectionsMeetingsStaging(ctx, classEntryTermCollection); err != nil {
		updateLogger.Error("Could not ready staging tables", err)
		return err
	}
	// defer q.CleanupCoursesMeetingsStaging(ctx)
	if err := (*service).StageAllClasses(*updateLogger, ctx, q, school.ID, classEntryTermCollection, false); err != nil {
		updateLogger.Error("Update sections failed aborting any staged sections/ meetings", err)
		return err
	}
	tx, err := o.dbPool.Begin(ctx)

	if err != nil {
		updateLogger.Error("couldn't begin transaction: ", err)
		return err
	}
	defer tx.Commit(ctx)

	q = q.WithTx(tx)
	changesCount, err := q.MoveStagedCoursesAndMeetings(ctx, classEntryTermCollection)
	if err != nil {
		updateLogger.Error("Failed moving courses: ", err)
		tx.Rollback(ctx)
		return err
	}
	updateLogger.Infof("updated %d meetings and sections", changesCount)
	return nil
}

func (o Orchestrator) ListRunningCollections() []db.TermCollection {
	collections := make([]db.TermCollection, 0)

	for collection, isValid := range o.termCollectionStagingLocks {
		// the hashmap is used as a set but check anyways I guees
		if !isValid {
			continue
		}
		collections = append(collections, collection)
	}
	return collections
}

func (o Orchestrator) GetTerms(
	ctx context.Context,
	serviceName string,
	schoolID string,
) ([]classentry.TermCollection, error) {
	service, ok := o.serviceEntries[serviceName]
	if !ok {
		return []classentry.TermCollection{}, errors.New("Service not found")
	}
	school, ok := o.GetSchoolById(schoolID)
	if !ok {
		return []classentry.TermCollection{}, errors.New("School ID not found")
	}
	termCollections, err := service.GetTermCollections(*o.orchestrationLogger, ctx, school)

	if err != nil {
		o.orchestrationLogger.Error("There")
		return []classentry.TermCollection{}, err
	}

	return termCollections, nil
}
