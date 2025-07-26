package serverget

import (
	"encoding/json"
	"net/http"

	"github.com/Pjt727/classy/data/db"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"log/slog"
)

type getHandler struct {
	dbPool *pgxpool.Pool
	logger *slog.Logger
}

type GetQueriesParam int

const (
	OffsetKey GetQueriesParam = iota
	LimitKey
)

func (h *getHandler) getCourse(w http.ResponseWriter, r *http.Request) {

	ctx := r.Context()
	q := db.New(h.dbPool)
	courseRows, err := q.GetCourseWithHueristics(ctx, db.GetCourseWithHueristicsParams{
		SchoolID:     chi.URLParam(r, "schoolID"),
		SubjectCode:  chi.URLParam(r, "subjectCode"),
		CourseNumber: chi.URLParam(r, "courseNumber"),
	})
	if err != nil {
		h.logger.Error("Could not get school rows", "err", err)
		http.Error(w, http.StatusText(500), 500)
		return
	}

	courses, err := json.Marshal(courseRows)
	if err != nil {
		h.logger.Error("Could not marshal school rows", "err", err)
		http.Error(w, http.StatusText(500), 500)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write(courses)
}

func (h *getHandler) getCoursesForSubject(w http.ResponseWriter, r *http.Request) {

	ctx := r.Context()
	q := db.New(h.dbPool)
	courseRows, err := q.GetCoursesForSchoolAndSubject(ctx, db.GetCoursesForSchoolAndSubjectParams{
		SchoolID:    chi.URLParam(r, "schoolID"),
		SubjectCode: chi.URLParam(r, "subjectCode"),
		Offsetvalue: ctx.Value(OffsetKey).(int32),
		Limitvalue:  ctx.Value(LimitKey).(int32),
	})
	if err != nil {
		h.logger.Error("Could not get school rows", "err", err)
		http.Error(w, http.StatusText(500), 500)
		return
	}

	courses, err := json.Marshal(courseRows)
	if err != nil {
		h.logger.Error("Could not marshal school rows", "err", err)
		http.Error(w, http.StatusText(500), 500)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write(courses)
}

func (h *getHandler) getCourses(w http.ResponseWriter, r *http.Request) {

	ctx := r.Context()
	q := db.New(h.dbPool)
	courseRows, err := q.GetCoursesForSchool(ctx, db.GetCoursesForSchoolParams{
		SchoolID:    chi.URLParam(r, "schoolID"),
		Offsetvalue: ctx.Value(OffsetKey).(int32),
		Limitvalue:  ctx.Value(LimitKey).(int32),
	})
	if err != nil {
		h.logger.Error("Could not get school rows", "err", err)
		http.Error(w, http.StatusText(500), 500)
		return
	}

	courses, err := json.Marshal(courseRows)
	if err != nil {
		h.logger.Error("Could not marshal school rows", "err", err)
		http.Error(w, http.StatusText(500), 500)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write(courses)
}

func (h *getHandler) getSchoolTerms(w http.ResponseWriter, r *http.Request) {

	ctx := r.Context()
	q := db.New(h.dbPool)
	termCollectionsRows, err := q.GetTermCollectionsForSchool(
		ctx,
		db.GetTermCollectionsForSchoolParams{
			SchoolID:    chi.URLParam(r, "schoolID"),
			Offsetvalue: ctx.Value(OffsetKey).(int32),
			Limitvalue:  ctx.Value(LimitKey).(int32),
		},
	)
	if err != nil {
		h.logger.Error("Could not get school rows", "err", err)
		http.Error(w, http.StatusText(500), 500)
		return
	}

	termCollections, err := json.Marshal(termCollectionsRows)
	if err != nil {
		h.logger.Error("Could not marshal school rows", "err", err)
		http.Error(w, http.StatusText(500), 500)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write(termCollections)
}

func (h *getHandler) getSchools(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	q := db.New(h.dbPool)
	schools, err := q.GetSchools(ctx, db.GetSchoolsParams{
		Offsetvalue: ctx.Value(OffsetKey).(int32),
		Limitvalue:  ctx.Value(LimitKey).(int32),
	})
	if err != nil {
		h.logger.Error("Could not get school rows", "err", err)
		http.Error(w, http.StatusText(500), 500)
		return
	}

	classRowsJSON, err := json.Marshal(schools)
	if err != nil {
		h.logger.Error("Could not marshal school rows", "err", err)
		http.Error(w, http.StatusText(500), 500)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write(classRowsJSON)
}

func (h *getHandler) getTermHueristics(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	q := db.New(h.dbPool)
	termHueristics, err := q.GetTermHueristics(ctx,
		db.GetTermHueristicsParams{
			SchoolID:         chi.URLParam(r, "schoolID"),
			TermCollectionID: chi.URLParam(r, "termCollectionID"),
		},
	)
	if err != nil {
		h.logger.Error("Could not get term hueristics rows", "err", err)
		http.Error(w, http.StatusText(500), 500)
		return
	}

	classRowsJSON, err := json.Marshal(termHueristics)
	if err != nil {
		h.logger.Error("Could not marshal term hueristics rows", "err", err)
		http.Error(w, http.StatusText(500), 500)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write(classRowsJSON)
}
func (h *getHandler) getClasses(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	q := db.New(h.dbPool)
	classRows, err := q.GetSchoolsClassesForTermOrderedBySection(ctx,
		db.GetSchoolsClassesForTermOrderedBySectionParams{
			SchoolID:         chi.URLParam(r, "schoolID"),
			TermCollectionID: chi.URLParam(r, "termCollectionID"),
			Offsetvalue:      ctx.Value(OffsetKey).(int32),
			Limitvalue:       ctx.Value(LimitKey).(int32),
		},
	)
	if err != nil {
		h.logger.Error("Could not get class rows", "err", err)
		http.Error(w, http.StatusText(500), 500)
		return
	}

	classRowsJSON, err := json.Marshal(classRows)
	if err != nil {
		h.logger.Error("Could not marshal class rows", "err", err)
		http.Error(w, http.StatusText(500), 500)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write(classRowsJSON)
}

func (h *getHandler) verifyCourse(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		q := db.New(h.dbPool)
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

func (h *getHandler) verifySchool(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		q := db.New(h.dbPool)
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

func (h *getHandler) verifyTermCollection(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		q := db.New(h.dbPool)
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
