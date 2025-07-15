package handlers

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"sync"

	"github.com/Pjt727/classy/api/components"
	"github.com/Pjt727/classy/collection"
	test_banner "github.com/Pjt727/classy/collection/services/banner/test"
	"github.com/Pjt727/classy/data/db"
	dbhelpers "github.com/Pjt727/classy/data/testdb"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	log "github.com/sirupsen/logrus"
)

type Sanatized int

const (
	OrchestratorLabel Sanatized = iota
	UserCookie
)

type ManageHandler struct {
	DbPool     *pgxpool.Pool
	TestDbPool *pgxpool.Pool

	// not safe mutliple changes at the same time
	orchestrators         map[int]*sessionOrchestrator
	lastOrchestratorLabel int
}

// keeps track of all sessions that are on this orchestrator
// when an orchestrator gets a request it should notify all websocket connections
// this is a wrapper around ManagementOrchestrator because data is fields the templates need
//
//	and the rest is needed to for the websockets
//
// moving ManagementOrchestrator here would result in circular imports
type sessionOrchestrator struct {
	data        *components.ManagementOrchestrator
	connections []*WebSocketConnection
	isTest      bool
	mu          sync.Mutex
}

func GetManageHandler(pool *pgxpool.Pool, testPool *pgxpool.Pool) *ManageHandler {

	testingSerivce, err := test_banner.GetTestingService()
	if err != nil {
		panic(err)
	}

	h := &ManageHandler{
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
		[]collection.Service{testingSerivce},
		nil,
		testPool,
	)
	if err != nil {
		log.Warn("Testing orchestrator could not be made: ", err)
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

func Notify(
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

func (h *ManageHandler) DashboardHome(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	// servicesForSchools := h.Orchestrator.GetSchoolsWithService()

	managementOrchs := make([]*components.ManagementOrchestrator, len(h.orchestrators))
	i := 0
	for _, o := range h.orchestrators {
		managementOrchs[i] = o.data
		i++
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	err := components.Dashboard(managementOrchs).Render(r.Context(), w)

	if err != nil {
		log.Error("Could not render template: ", err)
		Notify(w, r, components.NotifyError, "Dashboard could not be rendered")
		return
	}

}

func (h *ManageHandler) NewOrchestrator(w http.ResponseWriter, r *http.Request) {
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
		log.Error("Error decoding post: ", err)
		Notify(w, r, components.NotifyError, "Invlaid parameters")
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
		log.Error("Could not render template: ", err)
		Notify(w, r, components.NotifyError, "Could not render orchestrator")
		return
	}

	Notify(w, r, components.NotifySuccess, fmt.Sprintf("Succesfully added `%s`", name))

	err = components.NewOrchestrator().Render(r.Context(), w)
	if err != nil {
		log.Error("Could not render template: ", err)
		Notify(w, r, components.NotifyError, "Could not render new form orchestrator")
		return
	}
}

func (h *ManageHandler) ValidateOrchestrator(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		label, err := strconv.Atoi(chi.URLParam(r, "orchestratorLabel"))
		fmt.Println(label)
		_, orchExists := h.orchestrators[label]
		if err != nil || !orchExists {
			if !orchExists {
				log.Error("Orchestrator does not exists:", label)
			} else {
				log.Error("Invalid Orchestrator value", label)
			}
			http.Redirect(w, r, "/manage", http.StatusSeeOther)
			return
		}

		ctx = context.WithValue(ctx, OrchestratorLabel, label)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// this will be changed to actual auth to allow allow users
//
//	with a given key to manage
func EnsureCookie(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		var cookie *http.Cookie
		var err error
		cookie, err = r.Cookie("user_id")
		if err != nil {
			id := uuid.New().String()
			cookie = &http.Cookie{
				Name:     "user_id",
				Value:    id,
				Path:     "/manage",
				Secure:   true,
				HttpOnly: true,
				SameSite: http.SameSiteStrictMode,
			}
			http.SetCookie(w, cookie)
		}

		ctx = context.WithValue(ctx, UserCookie, cookie.String())

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (h *ManageHandler) OrchestratorHome(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	ctx := r.Context()
	index := ctx.Value(OrchestratorLabel).(int)
	fmt.Println(index)

	orchestrator := h.orchestrators[index]
	orchestrator.data.O.GetSchoolsWithService()
	err := components.OrchestratorDashboard(orchestrator.data, orchestrator.data.O.ListRunningCollections()).
		Render(r.Context(), w)

	if err != nil {
		log.Error("Orchestrator dashboard error: ", err)
		Notify(w, r, components.NotifyError, "Orchestrator rendering failed")
		return
	}
}

func (h *ManageHandler) OrchestratorGetTerms(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	ctx := r.Context()
	index := ctx.Value(OrchestratorLabel).(int)
	serviceName := r.FormValue("serviceName")
	schoolID := r.FormValue("schoolID")

	orchestrator := h.orchestrators[index]
	terms, err := orchestrator.data.O.GetTerms(ctx, serviceName, schoolID)
	if err != nil {
		badValues := fmt.Sprintf("service name: `%s`, school ID: `%s`", serviceName, schoolID)
		log.Error(fmt.Sprintf("Could not get terms for %s: ", badValues), err)
		Notify(w, r, components.NotifyError, fmt.Sprintf("Failed to get terms for %s", badValues))
		return
	}

	err = components.TermCollections(orchestrator.data, terms, serviceName).Render(ctx, w)

	if err != nil {
		log.Error("Term collections failed to render: ", err)
		Notify(w, r, components.NotifyError, "Orchestrator rendering failed")
		return
	}
}

func (h *ManageHandler) CollectTerm(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	ctx := r.Context()
	label := ctx.Value(OrchestratorLabel).(int)
	serviceName := r.FormValue("serviceName")
	schoolID := r.FormValue("schoolID")
	termID := r.FormValue("termID")
	isFullCollection := r.FormValue("isFullCollection") == "on"
	orchestrator := h.orchestrators[label]

	school, ok := orchestrator.data.O.GetSchoolById(schoolID)
	if !ok {
		log.Error(fmt.Sprintf("Could not find school `%s`: ", schoolID))
		Notify(
			w,
			r,
			components.NotifyError,
			fmt.Sprintf("Could not find the school `%s`", schoolID),
		)
		return
	}

	//  creating the logger and kicking off the process
	go func() {
		ctx := context.Background()
		customLogger := log.New()
		if orchestrator.isTest {
			customLogger.SetReportCaller(true)
		}

		oneOffLogger := customLogger.WithFields(log.Fields{
			"job":    "User driven",
			"termID": termID,
			"school": school,
		})
		oneOffLogger = customLogger.WithContext(ctx)
		hook := WebsocketLoggingHook{
			orchestratorLabel: label,
			termCollection: db.TermCollection{
				ID:              termID,
				SchoolID:        schoolID,
				Year:            0,
				Season:          "",
				Name:            pgtype.Text{String: "", Valid: false},
				StillCollecting: false,
			},
			serviceName: serviceName,
			h:           h,
		}
		oneOffLogger.Logger.AddHook(&hook)

		hook.start(ctx)
		// flush all terms
		err := orchestrator.data.O.UpsertSchoolTermsWithService(
			ctx,
			oneOffLogger,
			school,
			serviceName,
		)
		if err != nil {
			oneOffLogger.Error("upsert schools terms failed", err)
			hook.finish(ctx, components.JobError)
			return
		}

		// get the rest of the termCollection information because it is needed for this part

		var q *db.Queries
		if orchestrator.isTest {
			q = db.New(h.TestDbPool)
		} else {
			q = db.New(h.DbPool)
		}
		termCollection, err := q.GetTermCollection(ctx, db.GetTermCollectionParams{
			ID:       termID,
			SchoolID: schoolID,
		})

		if err != nil {
			oneOffLogger.Error("Could not get term collection: ", err)
			hook.finish(ctx, components.JobError)
			return
		}

		err = orchestrator.data.O.UpdateAllSectionsOfSchoolWithService(
			ctx,
			termCollection,
			oneOffLogger,
			serviceName,
			isFullCollection,
		)
		if err != nil {
			hook.finish(ctx, components.JobError)
			return
		}
		hook.finish(ctx, components.JobSuccess)
	}()
	Notify(
		w,
		r,
		components.NotifyProgress,
		fmt.Sprintf("Starting collection for `%s` `%s`", schoolID, termID),
	)
}

// runs all up and down migrations of the database...
//
//	ALL data is lost when doing this
func (h *ManageHandler) ResetDatabase(w http.ResponseWriter, r *http.Request) {
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
	Notify(
		w,
		r,
		status,
		message,
	)
}
