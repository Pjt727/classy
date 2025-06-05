package api

import (
	"github.com/Pjt727/classy/api/handlers"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/jackc/pgx/v5/pgxpool"
)

func populateManagementRoutes(r *chi.Router, pool *pgxpool.Pool, testPool *pgxpool.Pool) error {
	h := handlers.GetManageHandler(pool, testPool)
	(*r).Use(
		middleware.AllowContentType("application/x-www-form-urlencoded", "multipart/form-data"),
	)
	(*r).Use(handlers.EnsureCookie)

	(*r).Get("/", h.DashboardHome)
	(*r).Post("/", h.NewOrchestrator)
	(*r).Delete("/db", h.ResetDatabase)

	(*r).Route("/{orchestratorLabel}", func(r chi.Router) {
		r.Use(h.ValidateOrchestrator)
		r.Get("/", h.OrchestratorHome)
		r.Get("/watch-logs", h.LoggingWebSocket)
		r.Post("/terms", h.OrchestratorGetTerms)
		r.Patch("/terms", h.CollectTerm)
	})

	return nil
}
