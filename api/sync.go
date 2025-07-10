package api

import (
	"github.com/Pjt727/classy/api/handlers"
	"github.com/go-chi/chi/v5"

	"github.com/jackc/pgx/v5/pgxpool"
)

func populateSyncRoutes(r *chi.Router, pool *pgxpool.Pool) error {
	syncHandler := handlers.SyncHandler{
		DbPool: pool,
	}

	(*r).Get("/all", syncHandler.SyncAll)
	(*r).Post("/schools", syncHandler.SyncSchoolTerms)

	return nil
}
