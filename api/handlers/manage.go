package handlers

import (
	"context"
	"fmt"
	"net/http"
	"strconv"

	"github.com/Pjt727/classy/api/components"
	"github.com/Pjt727/classy/collection"
	"github.com/Pjt727/classy/data/db"
	"github.com/go-chi/chi/v5"
	"github.com/gorilla/websocket"

	"github.com/jackc/pgx/v5/pgxpool"
	log "github.com/sirupsen/logrus"
)

type ManageUrlParams int

const (
	OrchestratorIndex ManageUrlParams = iota
)

// not safe mutliple changes at the same time
var lastOrchestratorLabel = 0
var orchestrators map[int]components.ManagementOrchestrator = make(map[int]components.ManagementOrchestrator, 0)

func init() {
	newOrchestrator, err := collection.CreateOrchestrator(collection.DefaultEnabledServices, nil)
	if err != nil {
		panic(err)
	}
	managementOrchestrator := components.ManagementOrchestrator{
		O:    newOrchestrator,
		Name: "Default Orch",
	}
	orchestrators[lastOrchestratorLabel] = managementOrchestrator
	lastOrchestratorLabel++
}

type ManageHandler struct {
	DbPool *pgxpool.Pool
}

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
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

func (h ManageHandler) DashboardHome(w http.ResponseWriter, r *http.Request) {
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

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	err = components.Dashboard(orchestrators).Render(r.Context(), w)

	if err != nil {
		log.Error("Could not render template: ", err)
		Notify(w, r, components.NotifyError, "Dashboard could not be rendered")
		return
	}

}

func (h ManageHandler) NewOrchestrator(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	name := r.FormValue("name")

	newOrchestrator, err := collection.CreateOrchestrator(collection.DefaultEnabledServices, nil)
	managementOrchestrator := components.ManagementOrchestrator{
		O:    newOrchestrator,
		Name: name,
	}
	orchestrators[lastOrchestratorLabel] = managementOrchestrator
	lastOrchestratorLabel++

	if err != nil {
		log.Error("Error decoding post: ", err)
		Notify(w, r, components.NotifyError, "Invlaid parameters")
		return
	}

	err = components.ManageOrchestrators(orchestrators).Render(r.Context(), w)

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

		ctx = context.WithValue(ctx, OrchestratorIndex, label)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (h ManageHandler) OrchestratorHome(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	ctx := r.Context()
	index := ctx.Value(OrchestratorIndex).(int)
	fmt.Println(index)

	orchestrator := orchestrators[index]
	orchestrator.O.GetSchoolsWithService()
	err := components.OrchestratorDashboard(orchestrator, orchestrator.O.ListRunningCollections()).Render(r.Context(), w)

	if err != nil {
		log.Error("Orchestrator dashboard error: ", err)
		Notify(w, r, components.NotifyError, "Orchestrator rendering failed")
		return
	}
}

func (h ManageHandler) OrchestratorGetTerms(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	ctx := r.Context()
	index := ctx.Value(OrchestratorIndex).(int)
	serviceName := r.FormValue("serviceName")
	schoolID := r.FormValue("schoolID")

	orchestrator := orchestrators[index]
	terms, err := orchestrator.O.GetTerms(ctx, serviceName, schoolID)
	if err != nil {
		badValues := fmt.Sprintf("service name: `%s`, school ID: `%s`", serviceName, schoolID)
		log.Error(fmt.Sprintf("Could not get terms for %s: ", badValues), err)
		Notify(w, r, components.NotifyError, fmt.Sprintf("Failed to get terms for %s", badValues))
		return
	}

	err = components.TermCollections(orchestrator, terms, serviceName).Render(ctx, w)

	if err != nil {
		log.Error("Term collections failed to render: ", err)
		Notify(w, r, components.NotifyError, "Orchestrator rendering failed")
		return
	}
}

func (h ManageHandler) CollectTerm(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	ctx := r.Context()
	index := ctx.Value(OrchestratorIndex).(int)
	serviceName := r.FormValue("serviceName")
	schoolID := r.FormValue("schoolID")
	termID := r.FormValue("termID")
	orchestrator := orchestrators[index]

	school, ok := orchestrator.O.GetSchoolById(schoolID)
	if !ok {
		log.Error(fmt.Sprintf("Could not find school `%s`: ", schoolID))
		Notify(w, r, components.NotifyError, fmt.Sprintf("Could not find the school `%s`", schoolID))
		return
	}

	oneOffLogger := log.WithFields(log.Fields{
		"job":    "User driven",
		"termID": termID,
		"school": school,
	})

	go func() {
		orchestrator.O.UpsertSchoolTermsWithService(ctx)
	}()

	orchestrator.O.UpsertSchoolTermsWithService(ctx, *oneOffLogger, school, serviceName)

	terms, err := orchestrator.O.GetTerms(ctx, serviceName, schoolID)
	if err != nil {
		badValues := fmt.Sprintf("service name: `%s`, school ID: `%s`", serviceName, schoolID)
		log.Error(fmt.Sprintf("Could not get terms for %s: ", badValues), err)
		Notify(w, r, components.NotifyError, fmt.Sprintf("Failed to get terms for %s", badValues))
		return
	}

	err = components.TermCollections(orchestrator, terms).Render(ctx, w)

	if err != nil {
		log.Error("Term collections failed to render: ", err)
		Notify(w, r, components.NotifyError, "Orchestrator rendering failed")
		return
	}
}

func loggingWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Error(err)
		return
	}

}
