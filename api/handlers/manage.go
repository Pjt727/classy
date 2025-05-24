package handlers

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"sync"

	"github.com/Pjt727/classy/api/components"
	"github.com/Pjt727/classy/collection"
	"github.com/Pjt727/classy/data/db"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	log "github.com/sirupsen/logrus"
)

type Sanatized int

const (
	OrchestratorLabel Sanatized = iota
	UserCookie
)

// not safe mutliple changes at the same time
var lastOrchestratorLabel = 0

var orchestrators map[int]*sessionOrchestrator = make(
	map[int]*sessionOrchestrator,
	0,
)

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
	mu          sync.Mutex
}

func init() {
	newOrchestrator, err := collection.CreateOrchestrator(collection.DefaultEnabledServices, nil)
	if err != nil {
		panic(err)
	}
	managementOrchestrator := components.ManagementOrchestrator{
		O:     &newOrchestrator,
		Name:  "Default Orch",
		Label: lastOrchestratorLabel,
	}
	orchestrator := sessionOrchestrator{
		data:        &managementOrchestrator,
		connections: []*WebSocketConnection{},
		mu:          sync.Mutex{},
	}
	orchestrators[lastOrchestratorLabel] = &orchestrator
	lastOrchestratorLabel++
}

type ManageHandler struct {
	DbPool *pgxpool.Pool
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

	ctx := r.Context()
	q := db.New(h.DbPool)
	err := q.GetPreviousCollections(ctx)
	if err != nil {
		log.Error("Could not get school rows: ", err)
		Notify(w, r, components.NotifyError, "Database not working")
		return
	}
	managementOrchs := make([]*components.ManagementOrchestrator, len(orchestrators))
	i := 0
	for _, o := range orchestrators {
		managementOrchs[i] = o.data
		i++
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	err = components.Dashboard(managementOrchs).Render(r.Context(), w)

	if err != nil {
		log.Error("Could not render template: ", err)
		Notify(w, r, components.NotifyError, "Dashboard could not be rendered")
		return
	}

}

func (h *ManageHandler) NewOrchestrator(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	name := r.FormValue("name")

	newOrchestrator, err := collection.CreateOrchestrator(collection.DefaultEnabledServices, nil)
	managementOrchestrator := components.ManagementOrchestrator{
		O:    &newOrchestrator,
		Name: name,
	}
	sessionOrchestrator := sessionOrchestrator{
		data:        &managementOrchestrator,
		connections: []*WebSocketConnection{},
		mu:          sync.Mutex{},
	}
	orchestrators[lastOrchestratorLabel] = &sessionOrchestrator
	lastOrchestratorLabel++

	if err != nil {
		log.Error("Error decoding post: ", err)
		Notify(w, r, components.NotifyError, "Invlaid parameters")
		return
	}

	managementOrchs := make([]*components.ManagementOrchestrator, len(orchestrators))
	i := 0
	for _, o := range orchestrators {
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

func ValidateOrchestrator(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		label, err := strconv.Atoi(chi.URLParam(r, "orchestratorLabel"))
		fmt.Println(label)
		_, orchExists := orchestrators[label]
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

	orchestrator := orchestrators[index]
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

	orchestrator := orchestrators[index]
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
	orchestrator := orchestrators[label]

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

	oneOffLogger := log.WithFields(log.Fields{
		"job":    "User driven",
		"termID": termID,
		"school": school,
	})

	hook := WebsocketLoggingHook{
		orchestratorLabel: label,
		termID:            termID,
		schoolID:          schoolID,
	}

	oneOffLogger.Logger.AddHook(&hook)

	go func() {
		orchestrator.data.O.UpsertSchoolTermsWithService(ctx, *oneOffLogger, school, serviceName)
	}()
	Notify(
		w,
		r,
		components.NotifyProgress,
		fmt.Sprintf("Starting collection for `%s` `%s`", schoolID, termID),
	)
}
