package handlers

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/Pjt727/classy/data/db"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	log "github.com/sirupsen/logrus"
)

type SyncHandler struct {
	DbPool *pgxpool.Pool
}

func (h SyncHandler) SyncAllFromDate(w http.ResponseWriter, r *http.Request) {

	timeLayout := "2025-03-19 22:29:05.546344+09"
	inputTime := chi.URLParam(r, "lastSyncTimeStamp")
	var t time.Time
	if inputTime == "" {
		// default time which
		t = time.Date(2000, time.January, 1, 0, 0, 0, 0, time.FixedZone("UTC+0", 0))
	} else {
		var err error
		t, err = time.Parse(timeLayout, inputTime)
		if err != nil {
			log.Error("Could not parse time", err)
			http.Error(w, http.StatusText(400), 400)
			return
		}

	}

	ctx := r.Context()
	q := db.New(h.DbPool)
	syncChangeRows, err := q.GetLastestSyncChanges(ctx, pgtype.Timestamptz{
		Time:  t,
		Valid: true,
	})
	if err != nil {
		log.Error("Could not get lastest sync rows: ", err)
		http.Error(w, http.StatusText(500), 500)
		return
	}

	courses, err := json.Marshal(syncChangeRows)
	if err != nil {
		log.Error("Could not marshal school rows", err)
		http.Error(w, http.StatusText(500), 500)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write(courses)
}
