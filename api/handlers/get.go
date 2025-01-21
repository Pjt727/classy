package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/Pjt727/classy/data"
	"github.com/Pjt727/classy/data/db"
)

func GetClasses(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	termCollection, ok := ctx.Value("termCollection").(db.TermCollection)
	if !ok {
		http.Error(w, http.StatusText(404), 404)
		return
	}
	dbPool, err := data.NewPool(ctx)
	if err != nil {
		http.Error(w, http.StatusText(500), 500)
		return
	}
	q := db.New(dbPool)
	classRows, err := q.GetSchoolsClassesForTerm(ctx, db.GetSchoolsClassesForTermParams{
		SchoolID:         termCollection.SchoolID,
		TermCollectionID: termCollection.ID,
	})
	if err != nil {
		http.Error(w, http.StatusText(500), 500)
		return
	}
	classRowsJSON, err := json.Marshal(classRows)
	if err != nil {
		http.Error(w, http.StatusText(500), 500)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write(classRowsJSON)
}
