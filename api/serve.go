package api

import (
	"fmt"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	log "github.com/sirupsen/logrus"
	"net/http"
)

func Serve() {
	r := chi.NewRouter()
	cors := cors.New(cors.Options{
		AllowedOrigins:   []string{"*"}, // Allow all origins
		AllowedMethods:   []string{"GET"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token"},
		ExposedHeaders:   []string{"Link"},
		AllowCredentials: true,
		MaxAge:           300, // Maximum age for preflight requests
	})
	r.Use(cors.Handler)
	r.Use(middleware.Logger)

	r.Route("/get", func(r chi.Router) {
		populateGetRoutes(&r)
	})
	r.Route("/sync", func(r chi.Router) {
		populateSyncRoutes(&r)
	})
	port := 3000
	log.Infof("Running server on :%d", port)
	http.ListenAndServe(fmt.Sprintf(":%d", port), r)
}
