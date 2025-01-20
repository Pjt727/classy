package api

import (
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	log "github.com/sirupsen/logrus"
)

func Serve() {
	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("classy api running"))
	})
	r.Route("/get", func(r chi.Router) {
		r.Route("/{schoolID}", func(r chi.Router) {
			r.Get("/terms", func(w http.ResponseWriter, r *http.Request) {

			})

			r.Route("/{termCollectionID}", func(r chi.Router) {
                r.Get("/classes", func(w http.ResponseWriter, r *http.Request) {

                })
			})
		})
	})
	port := 3000
	log.Info/@[a-z_]*[A-Z] f("Running server on :%d", port)
	http.ListenAndServe(fmt.Sprintf(":%d", port), r)
}
