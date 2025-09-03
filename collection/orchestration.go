package collection

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/Pjt727/classy/collection/services/banner"

	classentry "github.com/Pjt727/classy/data/class-entry"
	"github.com/Pjt727/classy/data/db"
	"github.com/jackc/pgx/v5/pgxpool"
)

// TODO: restructure printing error logs and returning them to ensure limited duplication

// note logging is generally made with the api server in mind so the semantics
//    might be a little confusing when running these commands from CMD commands

type Service interface {

	// get the name of the service
	GetName() string

	// get the schools for this service (only called once at the start of the program)
	ListValidSchools(
		logger slog.Logger,
		ctx context.Context,
	) ([]classentry.School, error)

	// Should put ALL sections / meetings in the staging table as well as make
	//     the needed professors / courses for the sections and meeting times
	// ALL sections and meeting times must be put because they are diffed with the
	//     current state of the database to make deletions
	StageAllClasses(
		logger slog.Logger,
		ctx context.Context,
		q *classentry.EntryQueries,
		schoolID string,
		termCollection classentry.TermCollection,
		fullCollection bool,
	) error

	// get the terms that school (does NOT upsert them to the db)
	GetTermCollections(
		logger slog.Logger,
		ctx context.Context,
		school classentry.School,
	) ([]classentry.TermCollection, error)
}

// object responsible for which service should be used for a collection if not explicity stated
type SchoolsServiceManager struct {
	services []*Service
}

func NewServiceManager(services []*Service) (*SchoolsServiceManager, error) {
	if len(services) <= 0 {
		return nil, errors.New("Must have at least one service")
	}

	return &SchoolsServiceManager{
		services: services,
	}, nil
}

// eventually might be more sohpicated and ingest data about how collections went and whether the were sucessfull or not
func (s *SchoolsServiceManager) GetService() *Service {
	return s.services[0]
}

func (s *SchoolsServiceManager) GetServices() []*Service {
	return s.services
}

func (s *SchoolsServiceManager) AddSerivce(service *Service) {
	s.services = append(s.services, service)
}

type Orchestrator struct {
	serviceEntries map[string]*Service
	// it would not be good if the same term collection was being
	//    collected by multiple workers at the same time
	schoolIdToServiceManager map[string]*SchoolsServiceManager
	schoolIdToSchool         map[string]db.School
	orchestrationLogger      slog.Logger
	dbPool                   *pgxpool.Pool
}

var DefaultEnabledServices []Service

func init() {
	// might change to be determined by env variables or accesible resources
	DefaultEnabledServices = []Service{banner.GetDefaultService()}
}

func GetDefaultOrchestrator(pool *pgxpool.Pool) Orchestrator {
	ctx := context.Background()

	serviceEntries := make(map[string]*Service, 0)
	for _, service := range DefaultEnabledServices {
		serviceEntries[service.GetName()] = &service
	}

	logger := slog.Default().With(slog.String("job", "orchestration"))
	orchestrator := Orchestrator{
		serviceEntries:           serviceEntries,
		schoolIdToServiceManager: make(map[string]*SchoolsServiceManager),
		schoolIdToSchool:         make(map[string]db.School),
		orchestrationLogger:      *logger,
		dbPool:                   pool,
	}

	orchestrator.initMappings(ctx)

	return orchestrator
}

func CreateOrchestrator(
	services []Service,
	logger *slog.Logger,
	pool *pgxpool.Pool,
) (Orchestrator, error) {
	if logger == nil {
		defLogger := slog.Default().With(slog.String("job", "orchestration"))
		logger = defLogger
	}
	ctx := context.Background()

	serviceEntries := make(map[string]*Service, 0)
	for _, service := range services {
		serviceEntries[service.GetName()] = &service
	}

	orchestrator := Orchestrator{
		serviceEntries:           serviceEntries,
		schoolIdToServiceManager: make(map[string]*SchoolsServiceManager),
		schoolIdToSchool:         make(map[string]db.School),
		orchestrationLogger:      *logger,
		dbPool:                   pool,
	}

	orchestrator.initMappings(ctx)

	return orchestrator, nil
}

func (o Orchestrator) initMappings(ctx context.Context) {
	for _, service := range o.serviceEntries {
		serviceLogger := o.orchestrationLogger.With(slog.String("service", (*service).GetName()))
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
					serviceLogger.Warn("Skipping school to service mapping", "error", err)
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
		o.orchestrationLogger.Error("Couldn't begin transaction", "error", err)
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
			o.orchestrationLogger.Error("Couldn't add school", "error", err)
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
	logger *slog.Logger,
	school db.School,
) error {
	serviceManager, ok := o.schoolIdToServiceManager[school.ID]
	if !ok {
		return fmt.Errorf("Do not know how to scrape %s. No service was found.", school.ID)
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
	logger *slog.Logger,
	school db.School,
	serviceName string,
) error {
	logger.Info("starting collection and db addition of collection terms")
	service, ok := o.serviceEntries[serviceName]
	if !ok {
		return fmt.Errorf("The service `%s` was not found.", serviceName)
	}
	tx, err := o.dbPool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	q := db.New(tx)
	termCollections, err := (*service).GetTermCollections(*logger, ctx, school)
	if err != nil {
		return fmt.Errorf("Failed getting term collections %w", err)
	}

	logger.Debug("starting adding collection terms to db")
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
		return fmt.Errorf("Failed upserting schools %w", err)
	}

	dt := q.UpsertTerm(ctx, terms)
	var outerErr error = nil
	dt.Exec(func(i int, e error) {
		if e != nil {
			outerErr = e
			return
		}
	})
	if outerErr != nil {
		return fmt.Errorf("Failed upserting terms %w", err)
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
			outerErr = e
			return
		}
	})
	if outerErr != nil {
		return fmt.Errorf("Failed upserting term collections %w", err)
	}

	tx.Commit(ctx)
	logger.Info("finished adding collection terms to db", slog.Int("terms", len(termCollections)))
	return nil
}

func (o Orchestrator) UpsertAllTerms(ctx context.Context) error {
	var eg errgroup.Group
	numberOfWorkers := len(o.schoolIdToServiceManager)
	o.orchestrationLogger.Info("Starting to add school's terms", slog.Int("schools", numberOfWorkers))

	for schoolID, serviceManager := range o.schoolIdToServiceManager {
		eg.Go(func() error {
			s := serviceManager.GetService()
			school := o.schoolIdToSchool[schoolID]
			termLogger := o.orchestrationLogger.With(
				slog.String("school_id", schoolID),
				slog.String("service", (*s).GetName()),
			)
			if err := o.UpsertSchoolTerms(ctx, termLogger, school); err != nil {
				termLogger.Error("There was an error collecting terms", "error", err)
				return fmt.Errorf("error upserting terms for school %s: %s", schoolID, err)
			}
			termLogger.Info("Successfully upserted terms")
			return nil
		})
	}

	if err := eg.Wait(); err != nil {
		o.orchestrationLogger.Error("One or more schools failed to upsert terms", "error", err)
		return fmt.Errorf("one or more schools failed to upsert terms: %s", err)
	}

	o.orchestrationLogger.Info("Added school's terms successfully", slog.Int("schools", numberOfWorkers))
	return nil
}

type UpdateSectionsConfig struct {
	isFullCollection bool
	serviceName      string
	service          *Service
	logger           *slog.Logger
	callback         func(*db.Queries) error
}

func DefualtUpdateSectionsConfig() UpdateSectionsConfig {
	return UpdateSectionsConfig{
		isFullCollection: false,
		serviceName:      "",
		callback:         nil,
		service:          nil,
	}
}

func (u UpdateSectionsConfig) SetFullCollection(isFullCollection bool) UpdateSectionsConfig {
	u.isFullCollection = isFullCollection
	return u
}

func (u UpdateSectionsConfig) SetServiceName(serviceName string) UpdateSectionsConfig {
	u.serviceName = serviceName
	return u
}

func (u UpdateSectionsConfig) SetLogger(logger *slog.Logger) UpdateSectionsConfig {
	u.logger = logger
	return u
}

// change values so they can be used in a collection
// e.i. "" serviceName to the default service
func (u *UpdateSectionsConfig) normalize(termCollection db.TermCollection, o *Orchestrator) error {
	if u.serviceName != "" {
		service, ok := o.serviceEntries[u.serviceName]
		if !ok {
			return fmt.Errorf("Could not find service %s", u.serviceName)
		}
		u.service = service
		return nil
	}
	serviceManager, ok := o.schoolIdToServiceManager[termCollection.SchoolID]
	if !ok {
		return fmt.Errorf("Could not find service manager for shool id %s", termCollection.SchoolID)
	}
	service := serviceManager.GetService()
	u.serviceName = (*service).GetName()
	u.service = service

	if u.logger == nil {
		u.logger = slog.Default()
	}

	if u.callback == nil {
		u.callback = func(_ *db.Queries) error { return nil }
	}

	return nil
}

func (o *Orchestrator) UpdateAllSectionsOfSchool(
	ctx context.Context,
	termCollection db.TermCollection,
	config UpdateSectionsConfig,
) error {
	config.normalize(termCollection, o)
	updateLogger := config.logger.With(
		slog.String("school_id", termCollection.SchoolID),
		slog.String("season", string(termCollection.Season)),
		slog.Int64("year", int64(termCollection.Year)),
		slog.String("service", config.serviceName),
		slog.String("termCollectionID", termCollection.ID),
		slog.Bool("isFullCollection", config.isFullCollection),
	)

	school := o.schoolIdToSchool[termCollection.SchoolID]

	// inserting the new term collection attempt in the history
	priviledgedQueryObject := db.New(o.dbPool)
	termCollectionHistoryID, err := priviledgedQueryObject.InsertTermCollectionHistory(ctx, db.InsertTermCollectionHistoryParams{
		TermCollectionID: termCollection.ID,
		SchoolID:         termCollection.SchoolID,
		IsFull:           config.isFullCollection,
	})

	if err != nil {
		return fmt.Errorf("Could not start collection %w", err)
	}
	collectionStatus := db.TermCollectionStatusEnumFailure
	defer func() {
		priviledgedQueryObject.FinishTermCollectionHistory(ctx, db.FinishTermCollectionHistoryParams{
			NewFinishedStatus:       collectionStatus,
			TermCollectionHistoryID: termCollectionHistoryID,
		})
		// now that the transacation is committed the historic data table should be populated by the AFTER
		// triggers telling us what information was changed
		// knowing the updated, inserted, deleted data numbers might be helpful to inform automatic scheduling
		//    of collections and it is also nice to have for logging
		changeInformation, err := priviledgedQueryObject.GetChangesFromMoveTermCollection(ctx, termCollectionHistoryID)
		if err != nil {
			updateLogger.Error("Could not query changed data", "error", err)
			return
		}
		duration := time.Duration(changeInformation.ElapsedTime.Microseconds) * time.Microsecond
		updateLogger.Info(
			"Inserted, updated, deleted records in ",
			slog.Int64("inserted", changeInformation.InsertRecords),
			slog.Int64("updated", changeInformation.UpdatedRecords),
			slog.Int64("deleted", changeInformation.DeletedRecords),
			slog.Duration("duration", duration),
		)
		err = cleanupStagingTables(ctx, priviledgedQueryObject, termCollectionHistoryID)
		if err != nil {
			updateLogger.Error(
				"!!! Could not clean up tables for collection !!!",
				slog.Int64("termCollectionHistoryID", int64(termCollectionHistoryID)),
				"error", err,
			)
			return
		}
	}()

	classEntryTermCollection := classentry.TermCollection{
		ID: termCollection.ID,
		Term: classentry.Term{
			Year:   termCollection.Year,
			Season: termCollection.Season,
		},
		Name:            termCollection.Name,
		StillCollecting: termCollection.StillCollecting,
	}

	// prepare the staging area and use the service to get the class information
	q := db.New(o.dbPool)
	if err := cleanupStagingTables(ctx, q, termCollectionHistoryID); err != nil {
		updateLogger.Error("Could not ready staging tables", "error", err)
		return err
	}
	entryQ := classentry.NewEntryQuery(o.dbPool, termCollection.SchoolID, &termCollection.ID, &termCollectionHistoryID)
	if err := (*config.service).StageAllClasses(
		*updateLogger,
		ctx,
		entryQ,
		school.ID,
		classEntryTermCollection,
		config.isFullCollection,
	); err != nil {
		updateLogger.Error("Update sections failed aborting any staged sections/ meetings", "error", err)
		return err
	}

	tx, err := o.dbPool.Begin(ctx)
	if err != nil {
		updateLogger.Error("couldn't begin transaction", "error", err)
		return err
	}
	defer tx.Rollback(ctx)
	// setting this variable so triggers are aware of the collection class information is coming from
	// this did not work using sqlc for some reason
	if _, err = tx.Exec(ctx, fmt.Sprintf("SET LOCAL app.term_collection_history_id = '%d';", termCollectionHistoryID)); err != nil {
		updateLogger.Error("Could set app term_collection_history_id variable", "error", err)
		return err
	}

	q = q.WithTx(tx)
	err = moveStagedTables(ctx, q, termCollection, termCollectionHistoryID)
	if err != nil {
		updateLogger.Error("Failed moving courses", "error", err)
		return err
	}

	err = config.callback(q)
	if err != nil {
		updateLogger.Error("Failed executing callback", "error", err)
		return err
	}

	err = tx.Commit(ctx)
	if err != nil {
		updateLogger.Error("Failed commiting move class data transacation", "error", err)
		return err
	}

	collectionStatus = db.TermCollectionStatusEnumSuccess

	return nil
}

// these are all active collections not just ones for this orchestrator
func (o *Orchestrator) ListRunningCollections(ctx context.Context) ([]db.TermCollection, error) {
	q := db.New(o.dbPool)
	rows, err := q.GetActiveTermCollections(ctx)
	if err != nil {
		return []db.TermCollection{}, err
	}

	termCollections := make([]db.TermCollection, len(rows))

	for i, row := range rows {
		termCollections[i] = row.TermCollection
	}
	return termCollections, nil
}

func (o *Orchestrator) GetTerms(
	ctx context.Context,
	logger slog.Logger,
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
	entryTermCollections, err := (*service).GetTermCollections(logger, ctx, school)

	if err != nil {
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

func cleanupStagingTables(
	ctx context.Context,
	q *db.Queries,
	termCollectionHistoryID int32,
) error {
	var eg errgroup.Group

	eg.Go(func() error {
		return q.DeleteStagingCourses(ctx, termCollectionHistoryID)
	})

	eg.Go(func() error {
		return q.DeleteStagingProfessors(ctx, termCollectionHistoryID)
	})

	eg.Go(func() error {
		return q.DeleteStagingSections(ctx, termCollectionHistoryID)
	})

	eg.Go(func() error {
		return q.DeleteStagingMeetingTimes(ctx, termCollectionHistoryID)
	})

	if err := eg.Wait(); err != nil {
		return fmt.Errorf("one or more deletions failed: %s", err)
	}

	return nil
}

// moves all staged
//
// MUST be done in a transacation with the term collection history id set
func moveStagedTables(
	ctx context.Context,
	q *db.Queries,
	termCollection db.TermCollection,
	termCollectionHistoryID int32,
) error {
	err := q.RemoveUnstagedMeetings(ctx, db.RemoveUnstagedMeetingsParams{
		TermCollectionID: termCollection.ID,
		SchoolID:         termCollection.SchoolID,
	})
	if err != nil {
		return fmt.Errorf("error unstaging meeting %v", err)
	}
	err = q.RemoveUnstagedSections(ctx, db.RemoveUnstagedSectionsParams{
		TermCollectionID: termCollection.ID,
		SchoolID:         termCollection.SchoolID,
	})
	if err != nil {
		return fmt.Errorf("error unstaging sections %v", err)
	}
	if err = q.MoveCourses(ctx, termCollectionHistoryID); err != nil {
		return fmt.Errorf("error moving staged courses %v", err)
	}
	if err = q.MoveProfessors(ctx, termCollectionHistoryID); err != nil {
		return fmt.Errorf("error moving staged professors %v", err)
	}
	if err = q.MoveStagedSections(ctx, termCollectionHistoryID); err != nil {
		return fmt.Errorf("error moving staged sections %v", err)
	}
	if err = q.MoveStagedMeetingTimes(ctx, termCollectionHistoryID); err != nil {
		return fmt.Errorf("error moving staged meetings %v", err)
	}
	return nil
}
