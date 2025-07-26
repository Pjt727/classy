package serverget

import (
	"context"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/jackc/pgx/v5/pgxpool"
)

func PopulateGetRoutes(r *chi.Router, pool *pgxpool.Pool, logger slog.Logger) {
	getHandler := getHandler{
		dbPool: pool,
		logger: &logger,
	}

	(*r).Use(populatePagnation)
	(*r).Get("/", getHandler.getSchools)
	(*r).Route("/{schoolID}", func(r chi.Router) {
		r.Use(getHandler.verifySchool)
		r.Get("/", getHandler.getSchoolTerms)

		r.Route("/courses", func(r chi.Router) {
			r.Get("/", getHandler.getCourses)
			r.Route("/{subjectCode}", func(r chi.Router) {
				r.Get("/", getHandler.getCoursesForSubject)
				r.Route("/{courseNumber}", func(r chi.Router) {
					r.Use(getHandler.verifyCourse)
					r.Get("/", getHandler.getCourse)
				})
			})
		})

		r.Route("/{termCollectionID}", func(r chi.Router) {
			r.Use(getHandler.verifyTermCollection)
			r.Get("/", getHandler.getTermHueristics)
			r.Get("/classes", getHandler.getClasses)
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
		ctx = context.WithValue(ctx, OffsetKey, int32(offset))
		ctx = context.WithValue(ctx, LimitKey, int32(limit))
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
