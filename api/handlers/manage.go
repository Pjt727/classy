package handlers

import (
	"net/http"

	"github.com/Pjt727/classy/api/components"
	"github.com/Pjt727/classy/collection"
	"github.com/Pjt727/classy/data/db"

	// "github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	log "github.com/sirupsen/logrus"
)

type ManageHandler struct {
	DbPool       *pgxpool.Pool
	Orchestrator collection.Orchestrator
}

func (h ManageHandler) DashboardHome(w http.ResponseWriter, r *http.Request) {

	servicesForSchools := h.Orchestrator.GetSchoolsWithService()

	ctx := r.Context()
	q := db.New(h.DbPool)
	err := q.GetPreviousCollections(ctx)
	if err != nil {
		log.Error("Could not get school rows: ", err)
		http.Error(w, http.StatusText(500), 500)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	err = components.DashBoard().Render(r.Context(), w)

	if err != nil {
		log.Error("Could not render template: ", err)
		http.Error(w, http.StatusText(500), 500)
		return
	}

}
