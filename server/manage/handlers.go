package servermanage

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"

	"log/slog"

	"github.com/Pjt727/classy/collection"
	test_banner "github.com/Pjt727/classy/collection/services/banner/testbanner"
	"github.com/Pjt727/classy/data/db"
	logginghelpers "github.com/Pjt727/classy/data/logging-helpers"
	dbhelpers "github.com/Pjt727/classy/data/testdb"
	"github.com/Pjt727/classy/server/components"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"
)

type Sanatized int

const (
	OrchestratorLabel Sanatized = iota
	UserCookie
)

const DEFAULT_SCHEDULING_LIMIT = 15
const DEFAULT_TOKEN_EXPIRY = 5 * time.Minute

// auth for management is in memory as the expected number of users authenticated
// is tiny
const UserCookieName = "user_token"

type managementUser struct {
	username   string
	expireTime time.Time
}

type tokenStore struct {
	tokenToUser   map[string]*managementUser
	tokenDuration time.Duration
	mu            sync.RWMutex
}

func (t *tokenStore) getToken(token string) (managementUser, bool) {
	t.refreshTokens()
	t.mu.Lock()
	defer t.mu.Unlock()
	user, ok := t.tokenToUser[token]
	if ok {
		user.expireTime = time.Now().Add(t.tokenDuration)
		return *user, ok
	} else {
		return managementUser{}, ok
	}

}

func (t *tokenStore) addToken(token string, username string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.tokenToUser[token] = &managementUser{
		username:   username,
		expireTime: time.Now().Add(t.tokenDuration),
	}
}

// could also use goroutines but this should be fine
// bc of the low number of expected users for management auth
func (t *tokenStore) refreshTokens() {
	currentTime := time.Now()
	t.mu.Lock()
	defer t.mu.Unlock()
	for token, user := range t.tokenToUser {
		if currentTime.After(user.expireTime) {
			delete(t.tokenToUser, token)
		}
	}
}

var memoryTokenStore tokenStore = tokenStore{
	tokenToUser:   map[string]*managementUser{},
	tokenDuration: DEFAULT_TOKEN_EXPIRY,
	mu:            sync.RWMutex{},
}

type manageHandler struct {
	DbPool     *pgxpool.Pool
	TestDbPool *pgxpool.Pool
	// not for safe mutliple changes at the same time
	orchestrators         map[int]*sessionOrchestrator
	lastOrchestratorLabel int

	baseLogger     *slog.Logger
	testBaseLogger *slog.Logger
}

// keeps track of all sessions that are on this orchestrator
// when an orchestrator gets a request it should notify all websocket connections
// this is a wrapper around ManagementOrchestrator because data contains fields the templates need
// and the rest is needed to for the websockets
//
// moving ManagementOrchestrator here would result in circular imports
type sessionOrchestrator struct {
	data        *components.ManagementOrchestrator
	connections []*WebSocketConnection
	isTest      bool
	mu          sync.Mutex
}

// TODO: change this config somewhere
var testingLoggingOptions = &logginghelpers.Options{
	AddSource: true,
	Level:     logginghelpers.LevelReportIO,
	NoColor:   false,
}

var loggingOptions = &logginghelpers.Options{
	AddSource: false,
	Level:     slog.LevelInfo,
	NoColor:   false,
}

func getManageHandler(pool *pgxpool.Pool, testPool *pgxpool.Pool, logger *slog.Logger) *manageHandler {

	testServices := make([]collection.Service, 0)

	testingFileService, err := test_banner.GetFileTestingService()
	if err != nil {
		logger.Warn("testing file service not online", "err", err)
	} else {
		testServices = append(testServices, testingFileService)
	}
	frameLogger := logginghelpers.NewHandler(os.Stdout, &logginghelpers.Options{
		AddSource: true,
		Level:     slog.LevelInfo,
		NoColor:   false,
	})
	testingMockService, err := test_banner.GetMockTestingService(*slog.New(frameLogger), context.Background())
	if err != nil {
		logger.Warn("testing mock service not online", "err", err)
	} else {
		testServices = append(testServices, testingMockService)
	}

	baseLogger := slog.New(logginghelpers.NewMultiHandler(logginghelpers.NewHandler(os.Stdout, &logginghelpers.Options{
		AddSource: false,
		Level:     logginghelpers.LevelReportIO,
		NoColor:   false,
	})))
	testBaseLogger := slog.New(logginghelpers.NewMultiHandler(logginghelpers.NewHandler(os.Stdout, &logginghelpers.Options{
		AddSource: true,
		Level:     logginghelpers.LevelReportIO,
		NoColor:   false,
	})))

	h := &manageHandler{
		DbPool:                pool,
		TestDbPool:            testPool,
		orchestrators:         map[int]*sessionOrchestrator{},
		lastOrchestratorLabel: 0,
		baseLogger:            baseLogger,
		testBaseLogger:        testBaseLogger,
	}
	defaultOrchestrator := collection.GetDefaultOrchestrator(pool)

	testOrchestrator, err := collection.CreateOrchestrator(
		testServices,
		slog.New(frameLogger),
		testPool,
	)
	if err != nil {
		slog.Warn("Testing orchestrator could not be made", "err", err)
	}
	managementOrchestrator := &components.ManagementOrchestrator{
		O:     &defaultOrchestrator,
		Name:  "Default Orch",
		Label: h.lastOrchestratorLabel,
	}
	orchestrator := sessionOrchestrator{
		data:        managementOrchestrator,
		connections: []*WebSocketConnection{},
		isTest:      false,
		mu:          sync.Mutex{},
	}
	h.orchestrators[h.lastOrchestratorLabel] = &orchestrator
	h.lastOrchestratorLabel++

	testingManagementOrchestrator := &components.ManagementOrchestrator{
		O:     &testOrchestrator,
		Name:  "Testing Orch",
		Label: h.lastOrchestratorLabel,
	}
	testingOrchestrator := sessionOrchestrator{
		data:        testingManagementOrchestrator,
		connections: []*WebSocketConnection{},
		isTest:      true,
		mu:          sync.Mutex{},
	}
	h.orchestrators[h.lastOrchestratorLabel] = &testingOrchestrator
	h.lastOrchestratorLabel++

	return h
}

func notify(
	w http.ResponseWriter,
	r *http.Request,
	notificationType components.NotificationType,
	message string,
) {
	err := components.Notification(notificationType, message).Render(r.Context(), w)
	if err != nil {
		http.Error(w, "Failed to render notification", http.StatusInternalServerError)
		return
	}
}

func getJobCollections(ctx context.Context, q *db.Queries) ([]*components.QueueCollectionMessage, error) {
	rows, err := q.ViewQueue(ctx, DEFAULT_SCHEDULING_LIMIT)
	queueMessages := make([]*components.QueueCollectionMessage, len(rows))
	if err != nil {
		return queueMessages, err
	}
	for i, row := range rows {
		var message collection.CollectionMessage
		err := json.Unmarshal(row.Message, &message)
		if err != nil {
			return queueMessages, err
		}

		queueMessages[i] = &components.QueueCollectionMessage{
			JobCollectionID:  row.MessageID,
			TermCollectionID: message.TermCollectionID,
			SchoolID:         message.SchoolID,
			Debug:            message.Debug,
			ServiceName:      message.ServiceName,
			IsFullCollection: message.IsFullCollection,
			TimeActive:       row.VisibleAt,
		}
	}

	return queueMessages, nil
}

func (h *manageHandler) loginView(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	err := components.Login().Render(r.Context(), w)

	if err != nil {
		h.baseLogger.ErrorContext(r.Context(), "Could not render login view", "error", err)
		http.Error(w, http.StatusText(500), 500)
		return
	}
}

func (h *manageHandler) login(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	username := r.FormValue("username")
	password := r.FormValue("password")
	// users only need to exist on the main database to have access to both of them
	q := db.New(h.DbPool)
	user, err := q.AuthGetUser(r.Context(), username)

	// need to ensure the server does not accidentally give information about
	//    what users exist
	errText := "Invalid log in - verify a user has been added to the database"
	if err != nil {
		h.baseLogger.ErrorContext(r.Context(), "could not get user", "error", err)
		notify(w, r, components.NotifyError, errText)
		return
	}
	err = bcrypt.CompareHashAndPassword([]byte(user.EncryptedPassword), []byte(password))
	if err != nil {
		h.baseLogger.ErrorContext(r.Context(), "password is not correct", "username", username)
		notify(w, r, components.NotifyError, errText)
		return
	}

	token := uuid.New().String()
	cookie := &http.Cookie{
		Name:     UserCookieName,
		Value:    token,
		Path:     "/manage",
		Secure:   true,
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
	}
	memoryTokenStore.addToken(token, username)
	http.SetCookie(w, cookie)
	w.Header().Add("HX-Redirect", "/manage")
}

func ensureLoggedIn(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		var cookie *http.Cookie
		var err error
		cookie, err = r.Cookie(UserCookieName)
		if err != nil {
			http.Redirect(w, r, "/manage/login", http.StatusSeeOther)
			return
		}

		_, doesExist := memoryTokenStore.getToken(cookie.Value)
		if !doesExist {
			http.Redirect(w, r, "/manage/login", http.StatusSeeOther)
			return
		}

		ctx = context.WithValue(ctx, UserCookie, cookie.Value)

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (h *manageHandler) dashboardHome(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	ctx := r.Context()

	managementOrchs := make([]*components.ManagementOrchestrator, len(h.orchestrators))
	i := 0
	for _, o := range h.orchestrators {
		managementOrchs[i] = o.data
		i++
	}

	q := db.New(h.DbPool)
	queueMessages, err := getJobCollections(ctx, q)

	if err != nil {
		h.baseLogger.ErrorContext(ctx, "Could not get collection job queue", "error", err)
		http.Error(w, http.StatusText(500), 500)
		return
	}

	err = components.Dashboard(managementOrchs, queueMessages).Render(ctx, w)

	if err != nil {
		h.baseLogger.ErrorContext(ctx, "Could not render dashboard home component err", "error", err)
		http.Error(w, http.StatusText(500), 500)
		return
	}

}

func (h *manageHandler) getScheduleCollectionForm(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	ctx := r.Context()

	// just using the first orchestrator maybe change
	o, ok := h.orchestrators[0]
	if !ok {
		h.baseLogger.ErrorContext(ctx, "Orch label 0 does not exist")
		http.Error(w, http.StatusText(500), 500)
		return
	}

	queryParams := r.URL.Query()
	serviceName := queryParams.Get("serviceName")
	schoolID := queryParams.Get("schoolId")
	var schools []db.School
	var termCollections []db.TermCollection
	var service collection.Service
	var err error
	if serviceName != "" {
		service, ok = o.data.O.GetService(serviceName)
		if ok {
			schools, _ = service.ListValidSchools(*h.baseLogger, ctx)
		}
	}

	if schoolID != "" && service != nil {
		q := db.New(h.DbPool)
		termCollections, err = q.GetTermCollectionsForSchool(ctx, db.GetTermCollectionsForSchoolParams{
			SchoolID:    schoolID,
			Offsetvalue: 0,
			Limitvalue:  1_000,
		})
		if err != nil {
			h.baseLogger.ErrorContext(ctx, "Could not render dashboard home component err", "error", err)
			http.Error(w, http.StatusText(500), 500)
			return
		}
	}
	var secondsTillConsumed uint
	secondsTillConsumed = 0
	s, err := strconv.Atoi(queryParams.Get("secondsTillConsumed"))
	if err == nil {
		secondsTillConsumed = uint(s)
	}
	err = components.NewScheduledCollection(components.ScheduleCollectionFormInfo{
		ServiceName:         serviceName,
		ServiceNames:        o.data.O.GetServices(),
		SchoolID:            schoolID,
		Schools:             schools,
		TermCollections:     termCollections,
		TermCollectionID:    queryParams.Get("termCollectionId"),
		Debug:               queryParams.Get("debug") == "on",
		IsFullCollection:    queryParams.Get("isFullCollection") == "on",
		SecondsTillConsumed: secondsTillConsumed,
	}).Render(ctx, w)

	if err != nil {
		h.baseLogger.ErrorContext(ctx, "Could not render dashboard home component err", "error", err)
		http.Error(w, http.StatusText(500), 500)
		return
	}
}

func (h *manageHandler) scheduleCollectionForm(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	ctx := r.Context()

	err := r.ParseForm()
	if err != nil {
		notify(w, r, components.NotifyError, "Could not parse form: "+err.Error())
		return
	}

	var serviceName pgtype.Text
	sName := r.PostForm.Get("serviceName")
	if sName == "" {
		serviceName.Valid = false
	} else {
		serviceName.Valid = true
		serviceName.String = sName
	}

	message := collection.CollectionMessage{
		TermCollectionID: r.PostForm.Get("termCollectionId"),
		SchoolID:         r.PostForm.Get("schoolId"),
		Debug:            r.PostForm.Get("debug") == "on",
		ServiceName:      serviceName,
		IsFullCollection: pgtype.Bool{
			Bool:  r.PostForm.Get("isFullCollection") == "on",
			Valid: true,
		},
	}

	q := db.New(h.DbPool)
	messageBytes, err := json.Marshal(message)
	if err != nil {
		notify(w, r, components.NotifyError, "Could not marshall message bytes: "+err.Error())
		return
	}

	secondTillAvailable, err := strconv.Atoi(r.PostForm.Get("secondsTillConsumed"))
	if err != nil || secondTillAvailable < 0 {
		notify(w, r, components.NotifyError, "Invalid seconds till consumed"+err.Error())
		return
	}

	err = q.AddToQueue(ctx, db.AddToQueueParams{
		QueueName:             collection.SECTIONS_OF_TERM_COLLECTIONS,
		Message:               messageBytes,
		SecondsUntilAvailable: secondTillAvailable,
	})

	if err != nil {
		h.baseLogger.ErrorContext(ctx, "Could not add collection message", "error", err)
		http.Error(w, http.StatusText(500), 500)
		return
	}

	queueMessages, err := getJobCollections(ctx, q)

	if err != nil {
		h.baseLogger.ErrorContext(ctx, "Could not get collection job queue", "error", err)
		http.Error(w, http.StatusText(500), 500)
		return
	}

	err = components.ManageScheduling(queueMessages).Render(ctx, w)
	if err != nil {
		notify(w, r, components.NotifyError, "Could not render scheduling component"+err.Error())
		return
	}

	notify(w, r, components.NotifySuccess, fmt.Sprintf("Scheduling collection for %s %s ", message.SchoolID, message.TermCollectionID))
}

func (h *manageHandler) deleteCollectionJob(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	ctx := r.Context()

	err := r.ParseForm()
	if err != nil {
		notify(w, r, components.NotifyError, "Could not parse form: "+err.Error())
		return
	}

	collectionJobIdStr := r.Form.Get("collectionJobId")
	collectionJobId, err := strconv.Atoi(collectionJobIdStr)

	if err != nil {
		notify(w, r, components.NotifyError, "Collection job Id is not an integer: "+err.Error())
		return
	}

	q := db.New(h.DbPool)
	err = q.DeleteFromQueue(ctx, db.DeleteFromQueueParams{
		QueueName: collection.SECTIONS_OF_TERM_COLLECTIONS,
		MessageID: int32(collectionJobId),
	})

	if err != nil {
		h.baseLogger.ErrorContext(ctx, "Could not cancel collection message", "error", err)
		http.Error(w, http.StatusText(500), 500)
		return
	}

	queueMessages, err := getJobCollections(ctx, q)

	if err != nil {
		h.baseLogger.ErrorContext(ctx, "Could not get collection job queue", "error", err)
		http.Error(w, http.StatusText(500), 500)
		return
	}

	err = components.ManageScheduling(queueMessages).Render(ctx, w)
	if err != nil {
		notify(w, r, components.NotifyError, "Could not render scheduling component"+err.Error())
		return
	}

	notify(w, r, components.NotifySuccess, fmt.Sprintf("Canceled colection job: %s", collectionJobIdStr))
}

func (h *manageHandler) validateOrchestrator(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		label, err := strconv.Atoi(chi.URLParam(r, "orchestratorLabel"))
		_, orchExists := h.orchestrators[label]
		if err != nil || !orchExists {
			if !orchExists {
				h.baseLogger.ErrorContext(r.Context(), "Orchestrator does not exists", "label", label)
			} else {
				h.baseLogger.ErrorContext(r.Context(), "Invalid Orchestrator value", "label", label)
			}
			http.Redirect(w, r, "/manage", http.StatusSeeOther)
			return
		}

		ctx = context.WithValue(ctx, OrchestratorLabel, label)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (h *manageHandler) orchestratorHome(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	ctx := r.Context()
	index := ctx.Value(OrchestratorLabel).(int)

	orchestrator := h.orchestrators[index]
	orchestrator.data.O.GetSchoolsWithService()
	termCollections, err := orchestrator.data.O.ListRunningCollections(ctx)
	if err != nil {
		h.baseLogger.ErrorContext(r.Context(), "Orchestrator term collections error", "error", err)
		http.Error(w, http.StatusText(500), 500)
		return
	}

	err = components.OrchestratorDashboard(orchestrator.data, termCollections).Render(r.Context(), w)

	if err != nil {
		h.baseLogger.ErrorContext(r.Context(), "Orchestrator dashboard error", "error", err)
		http.Error(w, http.StatusText(500), 500)
		return
	}
}

func (h *manageHandler) orchestratorGetTerms(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	ctx := r.Context()
	index := ctx.Value(OrchestratorLabel).(int)
	serviceName := r.FormValue("serviceName")
	schoolID := r.FormValue("schoolID")

	orchestrator := h.orchestrators[index]

	terms, err := orchestrator.data.O.GetTerms(ctx, *h.baseLogger, serviceName, schoolID)
	if err != nil {
		badValues := fmt.Sprintf("service name: `%s`, school ID: `%s`", serviceName, schoolID)
		h.baseLogger.ErrorContext(ctx, "Could not get terms", "serviceName", serviceName, "schoolID", schoolID, "error", err)
		notify(w, r, components.NotifyError, fmt.Sprintf("Failed to get terms for %s", badValues))
		return
	}

	err = components.TermCollections(orchestrator.data, terms, serviceName).Render(ctx, w)

	if err != nil {
		h.baseLogger.ErrorContext(ctx, "Term collections failed to render", "error", err)
		notify(w, r, components.NotifyError, "Orchestrator rendering failed")
		return
	}
}

func (h *manageHandler) collectTerm(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	ctx := r.Context()
	label := ctx.Value(OrchestratorLabel).(int)
	serviceName := r.FormValue("serviceName")
	schoolID := r.FormValue("schoolID")
	termID := r.FormValue("termID")
	isFullCollection := r.FormValue("isFullCollection") == "on"
	h.baseLogger.InfoContext(ctx, "Is full collection: ", "isFullCollection", isFullCollection)
	orchestrator := h.orchestrators[label]

	school, ok := orchestrator.data.O.GetSchoolById(schoolID)
	if !ok {
		h.baseLogger.ErrorContext(ctx, "Could not find school", "schoolID", schoolID)
		notify(
			w,
			r,
			components.NotifyError,
			fmt.Sprintf("Could not find the school `%s`", schoolID),
		)
		return
	}

	//  creating the slogger and kicking off the process
	go func() {
		ctx := context.Background()
		termCollection := db.TermCollection{
			ID:              termID,
			SchoolID:        schoolID,
			Year:            0,
			Season:          "",
			Name:            pgtype.Text{String: "", Valid: false},
			StillCollecting: false,
		}
		webWriter := newWebSocketWriter(ctx, label, termCollection, serviceName, h)
		var oneOffLogger *slog.Logger
		if orchestrator.isTest {
			// TODO: Change where some of these options get decided
			webHandler := logginghelpers.NewHandler(webWriter, &logginghelpers.Options{
				AddSource: true,
				Level:     logginghelpers.LevelReportIO,
				NoColor:   false,
			})
			oneOffLogger = logginghelpers.WithHandler(h.testBaseLogger, webHandler)
		} else {
			webHandler := logginghelpers.NewHandler(webWriter, &logginghelpers.Options{
				AddSource: false,
				Level:     slog.LevelInfo,
				NoColor:   false,
			})
			oneOffLogger = logginghelpers.WithHandler(h.baseLogger, webHandler)
		}

		oneOffLogger = oneOffLogger.With(
			slog.String("job", "User driven"),
			slog.String("termID", termID),
			slog.String("school", schoolID),
		)

		webWriter.start(ctx)
		// flush all terms
		err := orchestrator.data.O.UpsertSchoolTermsWithService(
			ctx,
			oneOffLogger,
			school,
			serviceName,
		)
		if err != nil {
			h.baseLogger.ErrorContext(ctx, "upsert schools terms failed", "error", err)
			webWriter.finish(ctx, components.JobError)
			return
		}

		// get the rest of the termCollection information because it is needed for this part

		var q *db.Queries
		if orchestrator.isTest {
			q = db.New(h.TestDbPool)
		} else {
			q = db.New(h.DbPool)
		}
		termCollection, err = q.GetTermCollection(ctx, db.GetTermCollectionParams{
			ID:       termID,
			SchoolID: schoolID,
		})

		if err != nil {
			h.baseLogger.ErrorContext(ctx, "Could not get term collection", "error", err)
			webWriter.finish(ctx, components.JobError)
			return
		}

		results, err := orchestrator.data.O.UpdateAllSectionsOfSchool(
			ctx,
			termCollection,
			collection.DefualtUpdateSectionsConfig().
				SetLogger(oneOffLogger).
				SetServiceName(serviceName).
				SetFullCollection(isFullCollection),
		)
		if err != nil {
			oneOffLogger.ErrorContext(ctx, "Failed collection", "error", err)
			webWriter.finish(ctx, components.JobError)
			return
		}
		oneOffLogger.Info(
			"Collection results",
			"inserted",
			results.Inserted,
			"updated",
			results.Updated,
			"deleted",
			results.Deleted,
			"duration",
			results.Duration,
		)

		err = webWriter.finish(ctx, components.JobSuccess)
		if err != nil {
			slog.Info("error finsihing sending the finish message", "err", err)
		}
	}()

	notify(
		w,
		r,
		components.NotifyProgress,
		fmt.Sprintf("Starting collection for `%s` `%s`", schoolID, termID),
	)
}

// runs all up and down migrations of the database...
//
//	ALL data is lost when doing this
func (h *manageHandler) resetDatabase(w http.ResponseWriter, r *http.Request) {
	isMainDb := r.FormValue("db") == "main"
	var message string
	var status components.NotificationType
	if isMainDb {
		err := dbhelpers.ReloadDb()
		if err != nil {
			message = "Could not reload main database: " + err.Error()
			status = components.NotifyError
		} else {
			message = "Reloaded main database"
			status = components.NotifySuccess
		}
	} else {
		err := dbhelpers.SetupTestDb()
		if err != nil {
			message = "Could not reload test database: " + err.Error()
			status = components.NotifyError
		} else {
			message = "Reloaded test database"
			status = components.NotifySuccess
		}
	}
	notify(
		w,
		r,
		status,
		message,
	)
}
