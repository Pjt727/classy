package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/Pjt727/classy/data"
	"github.com/Pjt727/classy/data/db"
	log "github.com/sirupsen/logrus"
)

type ClassEntry struct {
	Section      db.Section       `json:"section"`
	Course       db.Course        `json:"course"`
	MeetingTimes []db.MeetingTime `json:"meeting_time"`
}

func GetClasses(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	termCollection, ok := ctx.Value("termCollection").(db.TermCollection)
	if !ok {
		http.Error(w, http.StatusText(404), 404)
		return
	}
	dbPool, err := data.NewPool(ctx)
	if err != nil {
		log.Trace(err)
		http.Error(w, http.StatusText(500), 500)
		return
	}
	q := db.New(dbPool)
	classRows, err := q.GetSchoolsClassesForTermOrderedBySection(ctx,
		db.GetSchoolsClassesForTermOrderedBySectionParams{
			SchoolID:         termCollection.SchoolID,
			TermCollectionID: termCollection.ID,
		},
	)
	if err != nil {
		log.Trace(err)
		http.Error(w, http.StatusText(500), 500)
		return
	}
	// group classes by their sections idk if there might be a way to do this
	//   sqlc I saw
	//   https://stackoverflow.com/questions/77964610/golang-sqlc-nested-data-as-array
	//   but that looks too complicated
	// var classEntries []ClassEntry
	// lastClassEntryIndex := -1
	// for _, classRow := range classRows {
	// 	if lastClassEntryIndex != -1 && classEntries[lastClassEntryIndex].Section.ID == classRow.Section.ID {
	// 		classEntry := &classEntries[lastClassEntryIndex]
	// 		classEntry.MeetingTimes = append(classEntry.MeetingTimes, classRow.MeetingTime)
	// 	} else {
	// 		classEntries = append(classEntries, ClassEntry{
	// 			Section:      classRow.Section,
	// 			Course:       classRow.Course,
	// 			MeetingTimes: []db.MeetingTime{classRow.MeetingTime},
	// 		})
	// 	}
	// }

	classRowsJSON, err := json.Marshal(classRows)
	if err != nil {
		log.Trace(err)
		http.Error(w, http.StatusText(500), 500)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write(classRowsJSON)
}
