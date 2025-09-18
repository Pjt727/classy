package servermanage

import (
	"log/slog"
	"os"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/jackc/pgx/v5/pgxpool"
)

func PopulateManagementRoutes(r *chi.Router, pool *pgxpool.Pool, testPool *pgxpool.Pool, logger slog.Logger) error {
	h := getManageHandler(pool, testPool, &logger)
	(*r).Use(
		middleware.AllowContentType("application/x-www-form-urlencoded", "multipart/form-data"),
	)
	isLocal := os.Getenv("LOCAL") == "true"

	// disable authentication if running locally
	if !isLocal {
		(*r).Get("/login", h.loginView)
		(*r).Post("/login", h.login)
	}

	(*r).Group(func(r chi.Router) {
		if !isLocal {
			r.Use(ensureLoggedIn)
		}

		r.Get("/", h.dashboardHome)
		r.Delete("/db", h.resetDatabase)

		r.Route("/schedule", func(r chi.Router) {
			r.Get("/", h.scheduleCollectionForm)
		})

		r.Route("/{orchestratorLabel}", func(r chi.Router) {
			r.Use(h.validateOrchestrator)
			r.Get("/", h.orchestratorHome)
			r.Get("/watch-logs", h.loggingWebSocket)
			r.Post("/terms", h.orchestratorGetTerms)
			r.Patch("/terms", h.collectTerm)
		})
	})

	return nil
}
