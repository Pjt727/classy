package serversync

import (
	"log/slog"

	"github.com/go-chi/chi/v5"

	"github.com/jackc/pgx/v5/pgxpool"
)

func PopulateSyncRoutes(r *chi.Router, pool *pgxpool.Pool, logger slog.Logger) error {
	syncHandler := syncHandler{
		dbPool: pool,
		logger: &logger,
	}

	(*r).Get("/all", syncHandler.syncAll)
	(*r).Post("/schools", syncHandler.syncSchoolTerms)

	return nil
}
