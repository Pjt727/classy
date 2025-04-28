package api

import (
	"context"

	"github.com/Pjt727/classy/api/handlers"
	"github.com/Pjt727/classy/data"
	"github.com/go-chi/chi/v5"
)

func populateManagementRoutes(r *chi.Router) error {
	ctx := context.Background()
	pool, err := data.NewPool(ctx)
	if err != nil {
		return err
	}
	syncHandler := handlers.SyncHandler{
		DbPool: pool,
	}

	(*r).Get("/all", syncHandler.SyncAll)
	(*r).Get("/schools/terms", syncHandler.SyncTerms)

	return nil
}
