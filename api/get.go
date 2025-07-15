package api

import (
	"context"
	"net/http"
	"strconv"

	"github.com/Pjt727/classy/api/handlers"
	"github.com/go-chi/chi/v5"

	"github.com/jackc/pgx/v5/pgxpool"
)

func populateGetRoutes(r *chi.Router, pool *pgxpool.Pool) {
	getHandler := handlers.GetHandler{
		DbPool: pool,
	}

	(*r).Use(populatePagnation)
	(*r).Get("/", getHandler.GetSchools)
	(*r).Route("/{schoolID}", func(r chi.Router) {
		r.Use(getHandler.VerifySchool)
		r.Get("/", getHandler.GetSchoolTerms)

		r.Route("/courses", func(r chi.Router) {
			r.Get("/", getHandler.GetCourses)
			r.Route("/{subjectCode}", func(r chi.Router) {
				r.Get("/", getHandler.GetCoursesForSubject)
				r.Route("/{courseNumber}", func(r chi.Router) {
					r.Use(getHandler.VerifyCourse)
					r.Get("/", getHandler.GetCourse)
				})
			})
		})

		r.Route("/{termCollectionID}", func(r chi.Router) {
			r.Use(getHandler.VerifyTermCollection)
			r.Get("/", getHandler.GetTermHueristics)
			r.Get("/classes", getHandler.GetClasses)
		})
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
				http.Error(w, "Invalid query offset param", http.StatusBadRequest)
				return
			}
			offset = newOffset
		}
		queryLimit := r.URL.Query().Get("limit")
		if queryLimit != "" {
			setLimit, err := strconv.Atoi(queryLimit)
			if err != nil {
				http.Error(w, "Invalid query limit param", http.StatusBadRequest)
				return
			}
			limit = setLimit
		}
		ctx = context.WithValue(ctx, handlers.OffsetKey, int32(offset))
		ctx = context.WithValue(ctx, handlers.LimitKey, int32(limit))
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
