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
)

type Sanatized int

const (
	OrchestratorLabel Sanatized = iota
	UserCookie
)

// auth for management is in memory as the expected number of users authenticated
// is tiny
const UserCookieName = "user_token"

type tokenStore struct {
	tokenToUsername   map[string]string
	tokenToExpireTime map[string]time.Time
	tokenDuration     time.Duration
	mu                sync.RWMutex
}

func (t *tokenStore) getToken(token string) (string, bool) {
	t.refreshToken()
	t.mu.RLock()
	defer t.mu.RUnlock()
	username, ok := t.tokenToUsername[token]
	if ok {
		t.mu.Lock()
		t.tokenToExpireTime[token] = time.Now().Add(t.tokenDuration)
		t.mu.Unlock()
	}
	return username, ok
}

func (t *tokenStore) addToken(token string, username string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.tokenToUsername[token] = username
	t.tokenToExpireTime[token] = time.Now().Add(t.tokenDuration)
}

// could also use goroutines but this should be fine
// bc of the low number of expected users for management auth
func (t *tokenStore) refreshToken() {
	currentTime := time.Now()
	t.mu.Lock()
	defer t.mu.Unlock()
	for token, expiredTime := range t.tokenToExpireTime {
		if currentTime.After(expiredTime) {
			delete(t.tokenToUsername, token)
			delete(t.tokenToExpireTime, token)
		}
	}
}

type manageHandler struct {
	DbPool     *pgxpool.Pool
	TestDbPool *pgxpool.Pool

	// not safe mutliple changes at the same time
	orchestrators         map[int]*sessionOrchestrator
	lastOrchestratorLabel int
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

func getManageHandler(pool *pgxpool.Pool, testPool *pgxpool.Pool) *manageHandler {

	testingFileService, err := test_banner.GetFileTestingService()
	if err != nil {
		panic(err)
	}
	frameLogger := logginghelpers.NewHandler(os.Stdout, &logginghelpers.Options{
		AddSource: true,
		Level:     slog.LevelInfo,
		NoColor:   false,
	})
	testingMockService, err := test_banner.GetMockTestingService(*slog.New(frameLogger), context.Background())
	if err != nil {
		panic(err)
	}

	h := &manageHandler{
		DbPool:                pool,
		TestDbPool:            testPool,
		orchestrators:         map[int]*sessionOrchestrator{},
		lastOrchestratorLabel: 0,
	}
	defaultOrchestrator, err := collection.GetDefaultOrchestrator(pool)
	if err != nil {
		panic(err)
	}
	testOrchestrator, err := collection.CreateOrchestrator(
		[]collection.Service{testingFileService, testingMockService},
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
		slog.ErrorContext(r.Context(), "Could not render login view", "error", err)
		http.Error(w, http.StatusText(500), 500)
		return
	}
}

func (h *manageHandler) login(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	err := components.Login().Render(r.Context(), w)

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
	if err != nil {
		slog.ErrorContext(r.Context(), "Could not render login view", "error", err)
		http.Error(w, http.StatusText(500), 500)
		return
	}
}

func (h *manageHandler) dashboardHome(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	// servicesForSchools := h.Orchestrator.GetSchoolsWithService()

	managementOrchs := make([]*components.ManagementOrchestrator, len(h.orchestrators))
	i := 0
	for _, o := range h.orchestrators {
		managementOrchs[i] = o.data
		i++
	}

	err := components.Dashboard(managementOrchs).Render(r.Context(), w)

	if err != nil {
		slog.ErrorContext(r.Context(), "Could not render dashboard home", "error", err)
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
		slog.ErrorContext(r.Context(), "Error decoding post", "error", err)
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
		slog.ErrorContext(r.Context(), "Could not render template", "error", err)
		notify(w, r, components.NotifyError, "Could not render orchestrator")
		return
	}

	notify(w, r, components.NotifySuccess, fmt.Sprintf("Succesfully added `%s`", name))

	err = components.NewOrchestrator().Render(r.Context(), w)
	if err != nil {
		slog.ErrorContext(r.Context(), "Could not render template", "error", err)
		notify(w, r, components.NotifyError, "Could not render new form orchestrator")
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
				slog.ErrorContext(r.Context(), "Orchestrator does not exists", "label", label)
			} else {
				slog.ErrorContext(r.Context(), "Invalid Orchestrator value", "label", label)
			}
			http.Redirect(w, r, "/manage", http.StatusSeeOther)
			return
		}

		ctx = context.WithValue(ctx, OrchestratorLabel, label)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func ensureCookie(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		var cookie *http.Cookie
		var err error
		cookie, err = r.Cookie(UserCookieName)
		if err != nil {
		}

		ctx = context.WithValue(ctx, UserCookie, cookie.String())

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (h *manageHandler) orchestratorHome(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	ctx := r.Context()
	index := ctx.Value(OrchestratorLabel).(int)

	orchestrator := h.orchestrators[index]
	orchestrator.data.O.GetSchoolsWithService()
	err := components.OrchestratorDashboard(orchestrator.data, orchestrator.data.O.ListRunningCollections()).
		Render(r.Context(), w)

	if err != nil {
		slog.ErrorContext(r.Context(), "Orchestrator dashboard error", "error", err)
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
	terms, err := orchestrator.data.O.GetTerms(ctx, serviceName, schoolID)
	if err != nil {
		badValues := fmt.Sprintf("service name: `%s`, school ID: `%s`", serviceName, schoolID)
		slog.ErrorContext(ctx, "Could not get terms", "serviceName", serviceName, "schoolID", schoolID, "error", err)
		notify(w, r, components.NotifyError, fmt.Sprintf("Failed to get terms for %s", badValues))
		return
	}

	err = components.TermCollections(orchestrator.data, terms, serviceName).Render(ctx, w)

	if err != nil {
		slog.ErrorContext(ctx, "Term collections failed to render", "error", err)
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
	slog.InfoContext(ctx, "Is full collection: ", "isFullCollection", isFullCollection)
	orchestrator := h.orchestrators[label]

	school, ok := orchestrator.data.O.GetSchoolById(schoolID)
	if !ok {
		slog.ErrorContext(ctx, "Could not find school", "schoolID", schoolID)
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
		var options logginghelpers.Options
		if orchestrator.isTest {
			options = logginghelpers.Options{
				AddSource: true,
				Level:     slog.LevelInfo,
				NoColor:   false,
			}
		} else {
			options = logginghelpers.Options{
				AddSource: false,
				Level:     slog.LevelInfo,
				NoColor:   false,
			}
		}
		webWriter := newWebSocketWriter(ctx, label, termCollection, serviceName, h)
		stdHandler := logginghelpers.NewHandler(os.Stdout, &options)
		webHandler := logginghelpers.NewHandler(webWriter, &options)

		handler := logginghelpers.NewMultiHandler(stdHandler, webHandler)

		oneOffLogger := slog.New(handler).With(
			slog.String("job", "User driven"),
			slog.String("termID", termID),
			slog.String("school", schoolID),
		)

		webWriter.start(ctx)
		// flush all terms
		err := orchestrator.data.O.UpsertSchoolTermsWithService(
			ctx,
			*oneOffLogger,
			school,
			serviceName,
		)
		if err != nil {
			slog.ErrorContext(ctx, "upsert schools terms failed", "error", err)
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
			slog.ErrorContext(ctx, "Could not get term collection", "error", err)
			webWriter.finish(ctx, components.JobError)
			return
		}

		err = orchestrator.data.O.UpdateAllSectionsOfSchoolWithService(
			ctx,
			termCollection,
			*oneOffLogger,
			serviceName,
			isFullCollection,
		)
		if err != nil {
			webWriter.finish(ctx, components.JobError)
			return
		}
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
