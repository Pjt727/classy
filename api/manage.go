package api

import (
	"context"

	"github.com/Pjt727/classy/api/handlers"
	"github.com/Pjt727/classy/data"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

func populateManagementRoutes(r *chi.Router) error {

	ctx := context.Background()
	pool, err := data.NewPool(ctx)
	if err != nil {
		return err
	}
	h := handlers.ManageHandler{
		DbPool: pool,
	}
	(*r).Use(
		middleware.AllowContentType("application/x-www-form-urlencoded", "multipart/form-data"),
	)
	(*r).Use(handlers.EnsureCookie)

	(*r).Get("/", h.DashboardHome)
	(*r).Post("/", h.NewOrchestrator)

	(*r).Route("/{orchestratorLabel}", func(r chi.Router) {
		r.Use(handlers.ValidateOrchestrator)
		r.Get("/", h.OrchestratorHome)
		r.Get("/watch-logs", h.LoggingWebSocket)
		r.Post("/terms", h.OrchestratorGetTerms)
		r.Patch("/terms", h.CollectTerm)
	})

	return nil
}
