package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/Pjt727/classy/data"
	"github.com/Pjt727/classy/data/db"
	"github.com/go-chi/chi/v5"
	log "github.com/sirupsen/logrus"
)

type GetQueriesParam int

const (
	OffsetKey GetQueriesParam = iota
	LimitKey
)

func GetClasses(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	dbPool, err := data.NewPool(ctx)
	if err != nil {
		log.Trace(err)
		http.Error(w, http.StatusText(500), 500)
		return
	}
	q := db.New(dbPool)
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
