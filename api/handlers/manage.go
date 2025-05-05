package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/Pjt727/classy/api/components"
	"github.com/Pjt727/classy/collection"
	"github.com/Pjt727/classy/data/db"

	// "github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	log "github.com/sirupsen/logrus"
)

type managementOrchestrator struct {
	o    collection.Orchestrator
	name string
}

var orchestrators []managementOrchestrator = make([]managementOrchestrator, 0)

type ManageHandler struct {
	DbPool *pgxpool.Pool
}

func (h ManageHandler) DashboardHome(w http.ResponseWriter, r *http.Request) {

	// servicesForSchools := h.Orchestrator.GetSchoolsWithService()

	ctx := r.Context()
	q := db.New(h.DbPool)
	err := q.GetPreviousCollections(ctx)
	if err != nil {
		log.Error("Could not get school rows: ", err)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	err = components.Dashboard().Render(r.Context(), w)

	if err != nil {
		log.Error("Could not render template: ", err)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

}

type NewOrchestratorData struct {
	name     string
	showLogs bool
}

func (h ManageHandler) NewOrchestrator(w http.ResponseWriter, r *http.Request) {
	var data NewOrchestratorData
	err := json.NewDecoder(r.Body).Decode(&data)
	if err != nil {
		log.Error("Error decoding post: ", err)
		http.Error(w, "Invlaid parameters", http.StatusBadRequest)
		return
	}
	newOrchestrator, err := collection.CreateOrchestrator(collection.DefaultEnabledServices, nil)
	newOrchestrator.GetSchoolById("marist")

	if err != nil {
		http.Error(w, "Error creating orchestrator", http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	err = components.Dashboard().Render(r.Context(), w)

	if err != nil {
		log.Error("Could not render template: ", err)
		http.Error(w, http.StatusText(500), 500)
		return
	}
}
