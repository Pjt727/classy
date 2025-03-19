package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/Pjt727/classy/data/db"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	log "github.com/sirupsen/logrus"
)

type SyncHandler struct {
	DbPool *pgxpool.Pool
}

func (h SyncHandler) SyncAllFromDate(w http.ResponseWriter, r *http.Request) {

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
