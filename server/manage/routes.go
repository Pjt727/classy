package servermanage

import (
	"log/slog"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/jackc/pgx/v5/pgxpool"
)

func PopulateManagementRoutes(r *chi.Router, pool *pgxpool.Pool, testPool *pgxpool.Pool, logger slog.Logger) error {
	h := getManageHandler(pool, testPool, &logger)
	(*r).Use(
		middleware.AllowContentType("application/x-www-form-urlencoded", "multipart/form-data"),
	)

	(*r).Use(ensureCookie)
	(*r).Get("/login", h.dashboardHome)
	(*r).Post("/login", h.dashboardHome)

	(*r).Get("/", h.dashboardHome)
	(*r).Post("/", h.newOrchestrator)
	(*r).Delete("/db", h.resetDatabase)

	(*r).Route("/{orchestratorLabel}", func(r chi.Router) {
		r.Use(h.validateOrchestrator)
		r.Get("/", h.orchestratorHome)
		r.Get("/watch-logs", h.loggingWebSocket)
		r.Post("/terms", h.orchestratorGetTerms)
		r.Patch("/terms", h.collectTerm)
	})

	return nil
}
