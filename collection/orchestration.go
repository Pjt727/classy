package collection

import (
	"context"
	"errors"
	"fmt"
	"golang.org/x/sync/errgroup"

	"github.com/Pjt727/classy/collection/services/banner"

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

// object responsible for which service should be used for a collection if not explicity stated
type ServiceManager struct {
	services []*Service
}

func NewServiceManager(services []*Service) (*ServiceManager, error) {
	if len(services) <= 0 {
		return nil, errors.New("Must have at least one service")
	}

	return &ServiceManager{
		services: services,
	}, nil
}

// eventually might be more sohpicated and ingest data about how collections went and whether the were sucessfull or not
func (s *ServiceManager) GetService() *Service {
	return s.services[0]
}

func (s *ServiceManager) GetServices() []*Service {
	return s.services
}

func (s *ServiceManager) AddSerivce(service *Service) {
	s.services = append(s.services, service)
}

type Orchestrator struct {
	serviceEntries map[string]*Service
	// it would not be good if the same term collection was being
	//    collected by multiple workers at the same time
	termCollectionStagingLocks map[db.TermCollection]bool
	schoolIdToServiceManager   map[string]*ServiceManager
	schoolIdToSchool           map[string]db.School
	orchestrationLogger        *log.Entry
	dbPool                     *pgxpool.Pool
}

var DefaultEnabledServices []Service

func init() {
	// might change to be determined by env variables or accesible resources
	DefaultEnabledServices = []Service{banner.Service}
}

func GetDefaultOrchestrator(pool *pgxpool.Pool) (Orchestrator, error) {
	ctx := context.Background()

	serviceEntries := make(map[string]*Service, 0)
	for _, service := range DefaultEnabledServices {
		serviceEntries[service.GetName()] = &service
	}

	orchestrator := Orchestrator{
		serviceEntries:             serviceEntries,
		termCollectionStagingLocks: map[db.TermCollection]bool{},
		schoolIdToServiceManager:   make(map[string]*ServiceManager),
		schoolIdToSchool:           make(map[string]db.School),
		orchestrationLogger:        log.WithFields(log.Fields{"job": "orchestration"}),
		dbPool:                     pool,
	}

	orchestrator.initMappings(ctx)

	return orchestrator, nil
}

func CreateOrchestrator(
	services []Service,
	logger *log.Entry,
	pool *pgxpool.Pool,
) (Orchestrator, error) {
	if logger == nil {
		logger = log.WithFields(log.Fields{"job": "orchestration"})
	}
	ctx := context.Background()

	serviceEntries := make(map[string]*Service, 0)
	for _, service := range services {
		serviceEntries[service.GetName()] = &service
	}

	orchestrator := Orchestrator{
		serviceEntries:             serviceEntries,
		termCollectionStagingLocks: map[db.TermCollection]bool{},
		schoolIdToServiceManager:   make(map[string]*ServiceManager),
		schoolIdToSchool:           make(map[string]db.School),
		orchestrationLogger:        logger,
		dbPool:                     pool,
	}

	orchestrator.initMappings(ctx)

	return orchestrator, nil
}

func (o Orchestrator) initMappings(ctx context.Context) {
	for _, service := range o.serviceEntries {
		serviceLogger := o.orchestrationLogger.WithField("service", (*service).GetName())
		schools, err := (*service).ListValidSchools(*serviceLogger, ctx)
		if err != nil {
			serviceLogger.Warn(
				fmt.Sprintf(
					"Skipping shool  to service mappings of `%s` because error: %s",
					(*service).GetName(),
					err,
				),
			)
			continue
		}

		for _, school := range schools {
			serviceManager, ok := o.schoolIdToServiceManager[school.ID]
			if ok {
				serviceManager.AddSerivce(service)
			} else {
				serviceManager, err := NewServiceManager([]*Service{service})
				if err != nil {
					serviceLogger.Warn("Skipping school to service mapping because error: ", err)
					continue
				}
				o.schoolIdToServiceManager[school.ID] = serviceManager
			}
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
	for schoolId, serviceManager := range o.schoolIdToServiceManager {
		for _, service := range serviceManager.GetServices() {
			schools = append(schools, SchoolWithService{
				School:      o.schoolIdToSchool[schoolId],
				ServiceName: (*service).GetName(),
			})
		}
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
	logger *log.Entry,
	school db.School,
) error {
	serviceManager, ok := o.schoolIdToServiceManager[school.ID]
	if !ok {
		return errors.New(
			fmt.Sprintf("Do not know how to scrape %s. No service was found.", school.ID),
		)
	}
	service := serviceManager.GetService()
	err := o.UpsertSchoolTermsWithService(ctx, logger, school, (*service).GetName())
	if err != nil {
		return err
	}
	return nil
}

// uses the specified service for the job
func (o Orchestrator) UpsertSchoolTermsWithService(
	ctx context.Context,
	logger *log.Entry,
	school db.School,
	serviceName string,
) error {
	logger.Info("starting collection and db addition of collection terms")
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
	termCollections, err := (*service).GetTermCollections(*logger, ctx, school)
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
	var outerErr error = nil
	dt.Exec(func(i int, e error) {
		if e != nil {
			tx.Rollback(ctx)
			outerErr = e
			return
		}
	})
	if outerErr != nil {
		return outerErr
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
			outerErr = e
			return
		}
	})
	if outerErr != nil {
		return outerErr
	}

	logger.Infof(`finished adding %d collection terms to db`, len(termCollections))
	tx.Commit(ctx)
	return nil
}

func (o Orchestrator) UpsertAllTerms(ctx context.Context) error {
	var eg errgroup.Group
	numberOfWorkers := len(o.schoolIdToServiceManager)
	o.orchestrationLogger.Infof(`Starting to add %d school's terms`, numberOfWorkers)

	for schoolID, serviceManager := range o.schoolIdToServiceManager {
		schoolID := schoolID             // Capture loop variable
		serviceManager := serviceManager // Capture loop variable
		eg.Go(func() error {
			s := serviceManager.GetService()
			school := o.schoolIdToSchool[schoolID]
			termLogger := o.orchestrationLogger.WithFields(log.Fields{
				"school_id": schoolID,
				"service":   (*s).GetName(),
			})
			if err := o.UpsertSchoolTerms(ctx, termLogger, school); err != nil {
				termLogger.Error("There was an error collecting terms: ", err)
				return errors.New(
					fmt.Sprintf("error upserting terms for school %d: %s", schoolID, err),
				)
			}
			termLogger.Info("Successfully upserted terms")
			return nil
		})
	}

	if err := eg.Wait(); err != nil {
		o.orchestrationLogger.Errorf("One or more schools failed to upsert terms: %v", err)
		return fmt.Errorf("one or more schools failed to upsert terms: %w", err)
	}

	o.orchestrationLogger.Infof(`Added %d school's terms successfully`, numberOfWorkers)
	return nil
}

func (o Orchestrator) UpdateAllSectionsOfSchool(
	ctx context.Context,
	termCollection db.TermCollection,
) error {
	serviceManager, ok := o.schoolIdToServiceManager[termCollection.SchoolID]
	updateLogger := o.orchestrationLogger.WithFields(log.Fields{
		"school_id": termCollection.SchoolID,
		"season":    termCollection.Season,
		"year":      termCollection.Year,
	})
	if !ok {
		errorTxt := fmt.Sprintf(
			"Could not find service manager for shool id %s",
			termCollection.SchoolID,
		)
		updateLogger.Error(errorTxt)
		return errors.New(errorTxt)
	}
	service := serviceManager.GetService()
	return o.UpdateAllSectionsOfSchoolWithService(
		ctx,
		termCollection,
		updateLogger,
		(*service).GetName(),
	)
}

func (o *Orchestrator) UpdateAllSectionsOfSchoolWithService(
	ctx context.Context,
	termCollection db.TermCollection,
	logger *log.Entry,
	serviceName string,
) error {
	service, ok := o.serviceEntries[serviceName]
	if !ok {
		return errors.New(fmt.Sprintf("The service `%s` was not found.", serviceName))
	}
	// take care of locking until this is done
	if _, ok := o.termCollectionStagingLocks[termCollection]; ok {
		return errors.New(
			fmt.Sprint("Already updating sections for this term collection ", termCollection),
		)
	}
	o.termCollectionStagingLocks[termCollection] = true
	defer delete(o.termCollectionStagingLocks, termCollection)

	school := o.schoolIdToSchool[termCollection.SchoolID]
	updateLogger := logger.WithFields(log.Fields{
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
) ([]db.TermCollection, error) {
	service, ok := o.serviceEntries[serviceName]
	if !ok {
		return []db.TermCollection{}, errors.New("Service not found")
	}
	school, ok := o.GetSchoolById(schoolID)
	if !ok {
		return []db.TermCollection{}, errors.New("School ID not found")
	}
	entryTermCollections, err := (*service).GetTermCollections(*o.orchestrationLogger, ctx, school)

	if err != nil {
		o.orchestrationLogger.Error("There")
		return []db.TermCollection{}, err
	}

	dbTermCollections := make([]db.TermCollection, len(entryTermCollections))
	for i, t := range entryTermCollections {
		dbTermCollections[i] = db.TermCollection{
			ID:              t.ID,
			SchoolID:        school.ID,
			Year:            t.Term.Year,
			Season:          t.Term.Season,
			Name:            t.Name,
			StillCollecting: t.StillCollecting,
		}
	}

	return dbTermCollections, nil
}
