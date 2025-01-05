package collection

import (
	"context"
	"sync"

	"github.com/Pjt727/classy/collection/services"
	"github.com/Pjt727/classy/data"
	"github.com/Pjt727/classy/data/db"
	"github.com/jackc/pgx/v5/pgxpool"
	log "github.com/sirupsen/logrus"
)

type Service interface {

	// get the name of the service
	GetName() string

	// get the schools for this service (only called once at the start of the program)
	ListValidSchools(logger log.Entry, ctx context.Context, q *db.Queries) ([]db.School, error)

	// adds every section to database and returns the amount changed
	//     because different services may have different adding procedures
	//     it is on the implementer
	StageAllClasses(
		logger log.Entry,
		ctx context.Context,
		q *db.Queries,
		school db.School,
		term db.Term,
	) error

	// get the terms that school (does NOT upsert them to the db)
	GetTermCollections(
		logger log.Entry,
		ctx context.Context,
		school db.School,
	) ([]db.UpsertTermCollectionParams, error)
}

var serviceEntries []Service
var schoolIdToService map[string]*Service
var schoolIdToSchool map[string]db.School
var orchestrationLogger *log.Entry
var dbPool *pgxpool.Pool

func init() {
	schoolIdToService = make(map[string]*Service)
	schoolIdToSchool = make(map[string]db.School)

	orchestrationLogger = log.WithFields(log.Fields{"job": "orchestration"})
	serviceEntries = []Service{services.Banner}
	ctx := context.Background()
	poolMaybe, err := data.NewQueries(ctx)
	if err != nil {
		// this package doesn't work if it can't access the database
		panic("Failed to init orchestration database connection")
	}
	dbPool = poolMaybe
	for _, service := range serviceEntries {
		serviceLogger := orchestrationLogger.WithField("service", service.GetName())
		q := db.New(dbPool)
		schools, err := service.ListValidSchools(*serviceLogger, ctx, q)
		if err != nil {
			serviceLogger.Error("Skipping school to service mapping because error: ", err)
			continue
		}

		for _, school := range schools {
			schoolIdToService[school.ID] = &service
			schoolIdToSchool[school.ID] = school
		}
	}
}

func UpsertAllSchools(ctx context.Context) {
	tx, err := dbPool.Begin(ctx)
	if err != nil {
		orchestrationLogger.Error("Couldn't begin transaction", err)
		return
	}
	q := db.New(dbPool).WithTx(tx)
	for _, school := range schoolIdToSchool {
		err = q.UpsertSchools(
			ctx,
			db.UpsertSchoolsParams{
				ID:   school.ID,
				Name: school.Name,
			})
		if err != nil {
			orchestrationLogger.Error("Couldn't add school", err)
			tx.Rollback(ctx)
			return
		}
	}
	tx.Commit(ctx)
}

func UpsertAllTerms(ctx context.Context) {
	var wg sync.WaitGroup
	errCh := make(chan error)
	numberOfWorkers := len(schoolIdToService)
	orchestrationLogger.Infof(`Starting to add %d school's terms`, numberOfWorkers)
	wg.Add(numberOfWorkers)
	for school_id, s := range schoolIdToService {
		school := schoolIdToSchool[school_id]
		termLogger := orchestrationLogger.WithFields(log.Fields{
			"school_id": school_id,
			"service":   (*s).GetName(),
		})
		go func(school db.School, s *Service, logger log.Entry) {
			defer wg.Done()
			tx, err := dbPool.Begin(ctx)
			if err != nil {
				errCh <- err
				return
			}
			defer tx.Commit(ctx)
			q := db.New(tx)
			logger.Tracef(`starting collection terms`)
			termCollections, err := (*s).GetTermCollections(logger, ctx, school)
			logger.Tracef(`finished collection terms`)
			if err != nil {
				errCh <- err
				logger.Trace(`propagating commit err: `, err)
				tx.Rollback(ctx)
				return
			}

			logger.Tracef(`starting adding collection terms to db`)
			terms := make([]db.UpsertTermParams, len(termCollections))
			for i, termCollection := range termCollections {
				terms[i] = db.UpsertTermParams{
					Year:   termCollection.Year,
					Season: termCollection.Season,
				}
			}
			dt := q.UpsertTerm(ctx, terms)
			dt.Exec(func(i int, err error) {
				if err != nil {
					errCh <- err
					tx.Rollback(ctx)
					return
				}
			})
			dtc := q.UpsertTermCollection(ctx, termCollections)
			dtc.Exec(func(i int, err error) {
				if err != nil {
					errCh <- err
					tx.Rollback(ctx)
					return
				}
			})

			logger.Tracef(`finished adding collection terms to db`)
		}(school, s, *termLogger)
	}

	go func() {
		wg.Wait()
		close(errCh)
	}()

	errorCount := 0
	for err := range errCh {
		orchestrationLogger.Error("There was an error collecting terms: ", err)
		errorCount++
	}

	orchestrationLogger.Infof(`Added %d school's terms successfully`, numberOfWorkers-errorCount)

}

func UpdateAllSectionsOfSchool(ctx context.Context, school_id string, term db.Term) {
	// there might be a good way to easily sandbox schoools to what they should change
	s, ok := schoolIdToService[school_id]
	updateLogger := orchestrationLogger.WithFields(log.Fields{
		"school_id": school_id,
		"season":    term.Season,
		"year":      term.Year,
	})
	if !ok {
		updateLogger.Error("Skipping update school... school not found")
		return
	}
	school := schoolIdToSchool[school_id]
	updateLogger = orchestrationLogger.WithFields(log.Fields{
		"service": (*s).GetName(),
		"school":  school,
		"term":    term,
	})
	q := db.New(dbPool)
	if err := q.ReadyCoursesMeetingsStaging(ctx); err != nil {
		updateLogger.Error("Could not ready staging tables", updateLogger)
		return
	}
	defer q.CleanupCoursesMeetingsStaging(ctx)
	if err := (*s).StageAllClasses(*updateLogger, ctx, q, school, term); err != nil {
		updateLogger.Error("Update sections aborting any staged sections/ meetings", updateLogger)
		return
	}
	tx, err := dbPool.Begin(ctx)
	if err != nil {
		updateLogger.Error("couldn't begin transaction", updateLogger)
		return
	}
	defer tx.Commit(ctx)
	q = db.New(dbPool).WithTx(tx)
	changesCount, err := q.MoveStagedCoursesAndMeetings(ctx, school_id, term)
	if err != nil {
		tx.Rollback(ctx)
		return
	}
	updateLogger.Infof("updated %d meetings and sections", changesCount)
}
