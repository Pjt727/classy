package api

import (
	"context"
	"fmt"
	"net/http"
	"strconv"

	"github.com/Pjt727/classy/api/handlers"
	"github.com/Pjt727/classy/data"
	"github.com/Pjt727/classy/data/db"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	log "github.com/sirupsen/logrus"
)

func Serve() {
	r := chi.NewRouter()
	ctx := context.Background()
	pool, err := data.NewPool(ctx)
	if err != nil {
		panic("Could not connect to database: " + err.Error())
	}
	getHandler := handlers.GetHandler{
		DbPool: pool,
	}
	r.Use(middleware.Logger)
	r.Use(populatePagnation)
	r.Get("/", getHandler.GetSchools)
	r.Route("/{schoolID}", func(r chi.Router) {
		r.Use(verifySchool)
		r.Get("/", getHandler.GetSchoolTerms)

		r.Route("/courses", func(r chi.Router) {
			r.Get("/", getHandler.GetCourses)
			r.Route("/{subjectCode}", func(r chi.Router) {
				r.Get("/", getHandler.GetCoursesForSubject)
				r.Get("/{courseNumber}", func(w http.ResponseWriter, r *http.Request) {

				})
			})
		})

		r.Route("/{termCollectionID}", func(r chi.Router) {
			r.Use(verifyTermCollection)
			r.Get("/classes", getHandler.GetClasses)
		})
	})
	port := 3000
	log.Infof("Running server on :%d", port)
	http.ListenAndServe(fmt.Sprintf(":%d", port), r)
}

func verifyCourse(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		dbPool, err := data.NewPool(ctx)
		if err != nil {
			http.Error(w, http.StatusText(500), 500)
			return
		}
		q := db.New(dbPool)
		courseExists, err := q.CourseExists(ctx,
			db.CourseExistsParams{
				SchoolID:     chi.URLParam(r, "schoolID"),
				SubjectCode:  chi.URLParam(r, "subjectCode"),
				CourseNumber: chi.URLParam(r, "courseNumber"),
			},
		)
		if err != nil {
			http.Error(w, http.StatusText(500), 500)
			return
		}
		if !courseExists {
			http.Error(w, http.StatusText(404), 404)
			return
		}
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func verifySchool(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		dbPool, err := data.NewPool(ctx)
		if err != nil {
			return
		}
		q := db.New(dbPool)
		schoolExists, err := q.SchoolExists(ctx, chi.URLParam(r, "schoolID"))
		if err != nil {
			http.Error(w, http.StatusText(500), 500)
			return
		}
		if !schoolExists {
			http.Error(w, http.StatusText(404), 404)
			return
		}
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func verifyTermCollection(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		dbPool, err := data.NewPool(ctx)
		if err != nil {
			return
		}
		q := db.New(dbPool)
		termCollectionExists, err := q.TermCollectionExists(ctx, db.TermCollectionExistsParams{
			SchoolID:         chi.URLParam(r, "schoolID"),
			TermCollectionID: chi.URLParam(r, "termCollectionID"),
		})
		if err != nil {
			http.Error(w, http.StatusText(500), 500)
			return
		}
		if !termCollectionExists {
			http.Error(w, http.StatusText(404), 404)
			return
		}
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func populatePagnation(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		offset := 0
		limit := 200
		queryOffset := r.URL.Query().Get("offset")
		if queryOffset != "" {
			newOffset, err := strconv.Atoi(queryOffset)
			if err != nil {
				http.Error(w, http.StatusText(400)+": Invalid query offset param", 400)
				return
			}
			offset = newOffset
		}
		queryLimit := r.URL.Query().Get("limit")
		if queryLimit != "" {
			setLimit, err := strconv.Atoi(queryLimit)
			if err != nil {
				http.Error(w, http.StatusText(400)+": Invalid query limit param", 400)
				return
			}
			limit = setLimit
		}
		ctx = context.WithValue(ctx, handlers.OffsetKey, int32(offset))
		ctx = context.WithValue(ctx, handlers.LimitKey, int32(limit))
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
