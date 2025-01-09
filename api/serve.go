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
	port := 3000
	log.Infof("Running server on :%d", port)
	http.ListenAndServe(fmt.Sprintf(":%d", port), r)
}
