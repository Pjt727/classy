package serversync

import (
	"github.com/go-chi/chi/v5"

	"github.com/jackc/pgx/v5/pgxpool"
)

func PopulateSyncRoutes(r *chi.Router, pool *pgxpool.Pool) error {
	syncHandler := syncHandler{
		DbPool: pool,
	}

	(*r).Get("/all", syncHandler.syncAll)
	(*r).Post("/schools", syncHandler.syncSchoolTerms)

	return nil
}
