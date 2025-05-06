package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/Pjt727/classy/api/components"
	"github.com/Pjt727/classy/collection"
	"github.com/Pjt727/classy/data/db"

	"github.com/jackc/pgx/v5/pgxpool"
	log "github.com/sirupsen/logrus"
)

var orchestrators []components.ManagementOrchestrator = make([]components.ManagementOrchestrator, 0)

type ManageHandler struct {
	DbPool *pgxpool.Pool
}

func ErrorAsHtmlMessage(w http.ResponseWriter, r *http.Request, message string) {
	err := components.Notification(components.NotifyError, message).Render(r.Context(), w)
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
		ErrorAsHtmlMessage(w, r, "Database not working")
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	err = components.Dashboard(orchestrators).Render(r.Context(), w)

	if err != nil {
		log.Error("Could not render template: ", err)
		ErrorAsHtmlMessage(w, r, "Dashboard could not be rendered")
		return
	}

}

type NewOrchestratorData struct {
	name     string
	showLogs bool
}

func (h ManageHandler) NewOrchestrator(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	var data NewOrchestratorData
	err := json.NewDecoder(r.Body).Decode(&data)
	if err != nil {
		log.Error("Error decoding post: ", err)
		ErrorAsHtmlMessage(w, r, "Invlaid parameters")
		return
	}
	newOrchestrator, err := collection.CreateOrchestrator(collection.DefaultEnabledServices, nil)
	managementOrchestrator := components.ManagementOrchestrator{
		O:    newOrchestrator,
		Name: data.name,
	}

	if err != nil {
		log.Error("Error decoding post: ", err)
		ErrorAsHtmlMessage(w, r, "Invlaid parameters: %s")
		return
	}

	err = components.ManageOrchestratorList(managementOrchestrator).Render(r.Context(), w)

	if err != nil {
		log.Error("Could not render template: ", err)
		ErrorAsHtmlMessage(w, r, "Could not render orchestrator")
		return
	}
}
