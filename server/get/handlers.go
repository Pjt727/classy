package serverget

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"log/slog"

	"github.com/Pjt727/classy/data/db"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// TODO: possible edit data layer so that what these return is not a direct copy of the
// database objects or maybe this can be as intended
// maybe transition to versions of the api responses on first release

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
	limit := ctx.Value(LimitKey).(int32)
	offset := ctx.Value(OffsetKey).(int32)
	courseRows, err := q.GetCoursesForSchoolAndSubject(ctx, db.GetCoursesForSchoolAndSubjectParams{
		SchoolID:    chi.URLParam(r, "schoolID"),
		SubjectCode: chi.URLParam(r, "subjectCode"),
		Offsetvalue: offset,
		Limitvalue:  limit,
	})
	if err != nil {
		h.logger.Error("Could not get school rows", "err", err)
		http.Error(w, http.StatusText(500), 500)
		return
	}

	courseRows, didLimit := normalizeLimits(courseRows, limit)
	addPaginationLinks(&w, r, offset, limit, didLimit)

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
	limit := ctx.Value(LimitKey).(int32)
	offset := ctx.Value(OffsetKey).(int32)
	courseRows, err := q.GetCoursesForSchool(ctx, db.GetCoursesForSchoolParams{
		SchoolID:    chi.URLParam(r, "schoolID"),
		Offsetvalue: offset,
		Limitvalue:  limit,
	})
	if err != nil {
		h.logger.Error("Could not get school rows", "err", err)
		http.Error(w, http.StatusText(500), 500)
		return
	}
	courseRows, didLimit := normalizeLimits(courseRows, limit)
	addPaginationLinks(&w, r, offset, limit, didLimit)

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
	limit := ctx.Value(LimitKey).(int32)
	offset := ctx.Value(OffsetKey).(int32)
	termCollectionsRows, err := q.GetTermCollectionsForSchool(
		ctx,
		db.GetTermCollectionsForSchoolParams{
			SchoolID:    chi.URLParam(r, "schoolID"),
			Offsetvalue: offset,
			Limitvalue:  limit,
		},
	)
	if err != nil {
		h.logger.Error("Could not get school rows", "err", err)
		http.Error(w, http.StatusText(500), 500)
		return
	}

	termCollectionsRows, didLimit := normalizeLimits(termCollectionsRows, limit)
	fmt.Println(didLimit)
	addPaginationLinks(&w, r, offset, limit, didLimit)

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
	limit := ctx.Value(LimitKey).(int32)
	offset := ctx.Value(OffsetKey).(int32)
	schools, err := q.GetSchools(ctx, db.GetSchoolsParams{
		Offsetvalue: offset,
		Limitvalue:  limit,
	})
	if err != nil {
		h.logger.Error("Could not get school rows", "err", err)
		http.Error(w, http.StatusText(500), 500)
		return
	}

	schools, didLimit := normalizeLimits(schools, limit)
	addPaginationLinks(&w, r, offset, limit, didLimit)

	schoolsJSON, err := json.Marshal(schools)
	if err != nil {
		h.logger.Error("Could not marshal school rows", "err", err)
		http.Error(w, http.StatusText(500), 500)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(schoolsJSON)
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
	limit := ctx.Value(LimitKey).(int32)
	offset := ctx.Value(OffsetKey).(int32)
	classRows, err := q.GetSchoolsClassesForTermOrderedBySection(ctx,
		db.GetSchoolsClassesForTermOrderedBySectionParams{
			SchoolID:         chi.URLParam(r, "schoolID"),
			TermCollectionID: chi.URLParam(r, "termCollectionID"),
			Offsetvalue:      offset,
			Limitvalue:       limit,
		},
	)
	if err != nil {
		h.logger.Error("Could not get class rows", "err", err)
		http.Error(w, http.StatusText(500), 500)
		return
	}

	classRows, didLimit := normalizeLimits(classRows, limit)
	addPaginationLinks(&w, r, offset, limit, didLimit)

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

// used for results that use database quert with limit + 1
// determines if there were more results and changes the array if there were
func normalizeLimits[T any](s []T, limit int32) ([]T, bool) {
	if len(s) > int(limit) {
		return s[:limit], true
	}
	return s, false
}

func addPaginationLinks(w *http.ResponseWriter, req *http.Request, currentOffset int32, limit int32, didLimit bool) {
	linkHeader := ""
	if currentOffset > 0 {
		prevURL := req.URL
		q := prevURL.Query()
		q.Set("offset", strconv.Itoa(int(currentOffset-limit)))
		prevURL.RawQuery = q.Encode()
		linkHeader += fmt.Sprintf("<%s>; rel=\"prev\"", prevURL)
		if didLimit {
			// add separator between links
			linkHeader += ","
		}
	}

	if didLimit {
		nextURL := req.URL
		q := nextURL.Query()
		q.Set("offset", strconv.Itoa(int(currentOffset+limit)))
		nextURL.RawQuery = q.Encode()
		linkHeader += fmt.Sprintf("<%s>; rel=\"next\"", nextURL)
	}
	fmt.Println("Link: ", linkHeader)

	(*w).Header().Set("Link", linkHeader)
}
