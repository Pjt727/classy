package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/Pjt727/classy/data/db"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	log "github.com/sirupsen/logrus"
)

type GetHandler struct {
	DbPool *pgxpool.Pool
}

type GetQueriesParam int

const (
	OffsetKey GetQueriesParam = iota
	LimitKey
)

func (h GetHandler) GetCourse(w http.ResponseWriter, r *http.Request) {

	ctx := r.Context()
	q := db.New(h.DbPool)
	courseRows, err := q.GetCourseWithHueristics(ctx, db.GetCourseWithHueristicsParams{
		SchoolID:     chi.URLParam(r, "schoolID"),
		SubjectCode:  chi.URLParam(r, "subjectCode"),
		CourseNumber: chi.URLParam(r, "courseNumber"),
	})
	if err != nil {
		log.Trace("Could not get school rows: ", err)
		http.Error(w, http.StatusText(500), 500)
		return
	}

	courses, err := json.Marshal(courseRows)
	if err != nil {
		log.Trace("Could not marshal school rows", err)
		http.Error(w, http.StatusText(500), 500)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write(courses)
}

func (h GetHandler) GetCoursesForSubject(w http.ResponseWriter, r *http.Request) {

	ctx := r.Context()
	q := db.New(h.DbPool)
	courseRows, err := q.GetCoursesForSchoolAndSubject(ctx, db.GetCoursesForSchoolAndSubjectParams{
		SchoolID:    chi.URLParam(r, "schoolID"),
		SubjectCode: chi.URLParam(r, "subjectCode"),
		Offsetvalue: ctx.Value(OffsetKey).(int32),
		Limitvalue:  ctx.Value(LimitKey).(int32),
	})
	if err != nil {
		log.Trace("Could not get school rows: ", err)
		http.Error(w, http.StatusText(500), 500)
		return
	}

	courses, err := json.Marshal(courseRows)
	if err != nil {
		log.Trace("Could not marshal school rows", err)
		http.Error(w, http.StatusText(500), 500)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write(courses)
}

func (h GetHandler) GetCourses(w http.ResponseWriter, r *http.Request) {

	ctx := r.Context()
	q := db.New(h.DbPool)
	courseRows, err := q.GetCoursesForSchool(ctx, db.GetCoursesForSchoolParams{
		SchoolID:    chi.URLParam(r, "schoolID"),
		Offsetvalue: ctx.Value(OffsetKey).(int32),
		Limitvalue:  ctx.Value(LimitKey).(int32),
	})
	if err != nil {
		log.Trace("Could not get school rows: ", err)
		http.Error(w, http.StatusText(500), 500)
		return
	}

	courses, err := json.Marshal(courseRows)
	if err != nil {
		log.Trace("Could not marshal school rows", err)
		http.Error(w, http.StatusText(500), 500)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write(courses)
}
func (h GetHandler) GetSchoolTerms(w http.ResponseWriter, r *http.Request) {

	ctx := r.Context()
	q := db.New(h.DbPool)
	termCollectionsRows, err := q.GetTermCollectionsForSchool(ctx, db.GetTermCollectionsForSchoolParams{
		SchoolID:    chi.URLParam(r, "schoolID"),
		Offsetvalue: ctx.Value(OffsetKey).(int32),
		Limitvalue:  ctx.Value(LimitKey).(int32),
	})
	if err != nil {
		log.Trace("Could not get school rows: ", err)
		http.Error(w, http.StatusText(500), 500)
		return
	}

	termCollections, err := json.Marshal(termCollectionsRows)
	if err != nil {
		log.Trace("Could not marshal school rows", err)
		http.Error(w, http.StatusText(500), 500)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write(termCollections)
}

func (h GetHandler) GetSchools(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	q := db.New(h.DbPool)
	schools, err := q.GetSchools(ctx, db.GetSchoolsParams{
		Offsetvalue: ctx.Value(OffsetKey).(int32),
		Limitvalue:  ctx.Value(LimitKey).(int32),
	})
	if err != nil {
		log.Trace("Could not get school rows: ", err)
		http.Error(w, http.StatusText(500), 500)
		return
	}

	classRowsJSON, err := json.Marshal(schools)
	if err != nil {
		log.Trace("Could not marshal school rows", err)
		http.Error(w, http.StatusText(500), 500)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write(classRowsJSON)
}

func (h GetHandler) GetClasses(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	q := db.New(h.DbPool)
	classRows, err := q.GetSchoolsClassesForTermOrderedBySection(ctx,
		db.GetSchoolsClassesForTermOrderedBySectionParams{
			SchoolID:         chi.URLParam(r, "schoolID"),
			TermCollectionID: chi.URLParam(r, "termCollectionID"),
			Offsetvalue:      ctx.Value(OffsetKey).(int32),
			Limitvalue:       ctx.Value(LimitKey).(int32),
		},
	)
	if err != nil {
		log.Trace("Could not get class rows: ", err)
		http.Error(w, http.StatusText(500), 500)
		return
	}

	classRowsJSON, err := json.Marshal(classRows)
	if err != nil {
		log.Trace("Could not marshal class rows", err)
		http.Error(w, http.StatusText(500), 500)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write(classRowsJSON)
}
