package api

import (
	"context"
	"fmt"
	"net/http"

	"github.com/Pjt727/classy/api/handlers"
	"github.com/Pjt727/classy/data"
	"github.com/Pjt727/classy/data/db"
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
			r.Use(SchoolCtx)
			r.Get("/terms", func(w http.ResponseWriter, r *http.Request) {

			})

			r.Route("/{termCollectionID}", func(r chi.Router) {
				r.Use(TermCollectionCtx)
				r.Get("/classes", handlers.GetClasses)
			})
		})
	})
	port := 3000
	log.Infof("Running server on :%d", port)
	http.ListenAndServe(fmt.Sprintf(":%d", port), r)
}

func SchoolCtx(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		dbPool, err := data.NewPool(ctx)
		if err != nil {
			return
		}
		q := db.New(dbPool)
		schoolRow, err := q.GetSchool(ctx, chi.URLParam(r, "schoolID"))
		if err != nil {
			http.Error(w, http.StatusText(404), 404)
			return
		}
		ctx = context.WithValue(ctx, "school", schoolRow.School)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func TermCollectionCtx(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		dbPool, err := data.NewPool(ctx)
		if err != nil {
			return
		}
		q := db.New(dbPool)
		termCollectionRow, err := q.GetTermCollection(ctx, db.GetTermCollectionParams{
			SchoolID:         chi.URLParam(r, "schoolID"),
			TermCollectionID: chi.URLParam(r, "termCollectionID"),
		})
		if err != nil {
			http.Error(w, http.StatusText(404), 404)
			return
		}
		ctx = context.WithValue(ctx, "termCollection", termCollectionRow.TermCollection)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
