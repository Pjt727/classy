package servermanage

import (
	"context"
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

type manageHandler struct {
	DbPool     *pgxpool.Pool
	TestDbPool *pgxpool.Pool
	// not safe mutliple changes at the same time
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

		ctx = context.WithValue(ctx, UserCookie, cookie.String())

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (h *manageHandler) dashboardHome(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	managementOrchs := make([]*components.ManagementOrchestrator, len(h.orchestrators))
	i := 0
	for _, o := range h.orchestrators {
		managementOrchs[i] = o.data
		i++
	}

	err := components.Dashboard(managementOrchs).Render(r.Context(), w)

	if err != nil {
		h.baseLogger.ErrorContext(r.Context(), "Could not render dashboard home", "error", err)
		http.Error(w, http.StatusText(500), 500)
		return
	}

}

func (h *manageHandler) newOrchestrator(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	name := r.FormValue("name")
	isTest := r.FormValue("isTest") == "true"

	var newOrchestrator collection.Orchestrator
	var err error
	if isTest {
		newOrchestrator, err = collection.CreateOrchestrator(
			collection.DefaultEnabledServices,
			nil,
			h.TestDbPool,
		)
	} else {
		newOrchestrator, err = collection.CreateOrchestrator(collection.DefaultEnabledServices, nil, h.DbPool)
	}

	managementOrchestrator := components.ManagementOrchestrator{
		O:    &newOrchestrator,
		Name: name,
	}
	sessionOrchestrator := sessionOrchestrator{
		data:        &managementOrchestrator,
		connections: []*WebSocketConnection{},
		mu:          sync.Mutex{},
	}
	h.orchestrators[h.lastOrchestratorLabel] = &sessionOrchestrator
	h.lastOrchestratorLabel++

	if err != nil {
		h.baseLogger.ErrorContext(r.Context(), "Error decoding post", "error", err)
		notify(w, r, components.NotifyError, "Invlaid parameters")
		return
	}

	managementOrchs := make([]*components.ManagementOrchestrator, len(h.orchestrators))
	i := 0
	for _, o := range h.orchestrators {
		managementOrchs[i] = o.data
		i++
	}

	err = components.ManageOrchestrators(managementOrchs).Render(r.Context(), w)

	if err != nil {
		h.baseLogger.ErrorContext(r.Context(), "Could not render template", "error", err)
		notify(w, r, components.NotifyError, "Could not render orchestrator")
		return
	}

	notify(w, r, components.NotifySuccess, fmt.Sprintf("Succesfully added `%s`", name))

	err = components.NewOrchestrator().Render(r.Context(), w)
	if err != nil {
		h.baseLogger.ErrorContext(r.Context(), "Could not render template", "error", err)
		return
	}
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
			"deleted",
			results.Deleted,
			"duration",
			results.Duration,
		)
		webWriter.finish(ctx, components.JobSuccess)
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
