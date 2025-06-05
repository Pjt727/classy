package handlers

import (
	"encoding/json"
	"net/http"
	"slices"
	"strconv"
	"sync"

	"github.com/Pjt727/classy/data/db"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	log "github.com/sirupsen/logrus"
)

type SyncHandler struct {
	DbPool *pgxpool.Pool
}

type syncResult struct {
	SyncData     []db.GetLastestSyncChangesRow `json:"sync_data"`
	LastSequence int                           `json:"last_update"`
}

func (h SyncHandler) SyncAll(w http.ResponseWriter, r *http.Request) {

	inputSequence := chi.URLParam(r, "lastSyncSequence")
	var sequence int
	if inputSequence == "" {
		// default time sequence which includes everything
		sequence = 0
	} else {
		var err error
		sequence, err = strconv.Atoi(inputSequence)
		if err != nil {
			log.Error("Could not parse sequence", err)
			http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
			return
		}
	}

	ctx := r.Context()
	q := db.New(h.DbPool)
	var wg sync.WaitGroup
	wg.Add(2)
	errCh := make(chan error, 2)
	var syncChangeRows []db.GetLastestSyncChangesRow
	go func() {
		defer wg.Done()
		var err error
		syncChangeRows, err = q.GetLastestSyncChanges(ctx, int32(sequence))
		if err != nil {
			errCh <- err
			log.Error("Could not get lastest sync rows: ", err)
			return
		}

	}()
	var lastCommonUpdate int32
	go func() {
		defer wg.Done()
		var err error
		lastCommonUpdate, err = q.GetLastSequence(ctx)
		if err != nil {
			errCh <- err
			log.Error("Could not get lastest sync common rows: ", err)
			return
		}

	}()

	wg.Wait()
	if len(errCh) > 0 {
		http.Error(w, http.StatusText(500), 500)
	}

	result := syncResult{
		SyncData:     syncChangeRows,
		LastSequence: int(lastCommonUpdate),
	}
	resultJson, err := json.Marshal(result)
	if err != nil {
		log.Error("Could not marshal school rows", err)
		http.Error(w, http.StatusText(500), 500)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write(resultJson)
}

type perTerm struct {
	SchoolID         string `json:"school_id"`
	TermCollectionID string `json:"term_collection_id"`
	LastSequence     int32  `json:"last_sequence"`
}

type perSchool struct {
	SchoolID     string `json:"school_id"`
	LastSequence int32  `json:"last_sequence"`
}

type syncDataPerTermParams struct {
	TermSequences   []perTerm   `json:"term_sequences"`
	SchoolSequences []perSchool `json:"school_sequences"`
}

type syncTermsResults struct {
	SyncData           []db.GetLastestSyncChangesForTermsRow `json:"sync_data"`
	PerTermSequences   []perTerm                             `json:"last_term_sequence"`
	PerSchoolSequences []perSchool                           `json:"last_common_sequence"`
}

type tableSyncEntry struct {
	TableName string `json:"table_name"`
	LastSync  *int   `json:"last_sync"`
}

type SelectSchoolEntry struct {
	CommonTables []tableSyncEntry `json:"common_tables"`
	SelectTerms  []string         `json:"select_terms"`
}

type syncTermsResult struct {
	AllSchools         []tableSyncEntry          `json:"all_schools"`
	SelectSchools      map[string]tableSyncEntry `json:"select_schools"`
	PerSchoolSequences []perSchool               `json:"last_common_sequence"`
}

// Tables will propagate sync values to their depenencies e.i. if they are not given or greater than
//
//	a term table entry with a sync number of 100 necessitates a sync of at least 100 from the school table
func (h SyncHandler) SyncTerms(w http.ResponseWriter, r *http.Request) {

	var syncData syncDataPerTermParams
	err := json.NewDecoder(r.Body).Decode(&syncData)
	if err != nil {
		log.Error("Could not parse sequence", err)
		http.Error(w, "Could not parse body: "+err.Error(), http.StatusBadRequest)
		return
	}

	tSchoolIDs := make([]string, len(syncData.TermSequences))
	tTermCollectionIDs := make([]string, len(syncData.TermSequences))
	tLastSequences := make([]int32, len(syncData.TermSequences))

	for i, term := range syncData.TermSequences {
		tSchoolIDs[i] = term.SchoolID
		tTermCollectionIDs[i] = term.TermCollectionID
		tLastSequences[i] = int32(term.LastSequence)
	}

	sSchoolIDs := make([]string, len(syncData.SchoolSequences))
	sLastSequences := make([]int32, len(syncData.SchoolSequences))
	for i, term := range syncData.SchoolSequences {
		sSchoolIDs[i] = term.SchoolID
		sLastSequences[i] = int32(term.LastSequence)
	}

	for _, schoolID := range tSchoolIDs {
		if !slices.Contains(sSchoolIDs, schoolID) {
			log.Error("Term school id not in schools", err)
			http.Error(w, "Term school id not in schools ", http.StatusBadRequest)
			return
		}
	}

	ctx := r.Context()
	q := db.New(h.DbPool)
	var wg sync.WaitGroup
	wg.Add(3)
	errCh := make(chan error, 3)
	var syncChangeRows []db.GetLastestSyncChangesForTermsRow
	go func() {
		defer wg.Done()
		var err error
		syncChangeRows, err = q.GetLastestSyncChangesForTerms(
			ctx,
			db.GetLastestSyncChangesForTermsParams{
				TSchoolIds:         tSchoolIDs,
				TTermCollectionIds: tTermCollectionIDs,
				TLastSequences:     tLastSequences,
				SSchoolIds:         sSchoolIDs,
				SLastSequences:     sLastSequences,
			},
		)
		if err != nil {
			log.Error("Could not get lastest sync rows: ", err)
			return
		}
	}()

	var lastCommonSequences []perSchool
	go func() {
		defer wg.Done()
		lastCommonSequenceRows, err := q.GetLastCommonSequences(ctx, sSchoolIDs)
		for i, row := range lastCommonSequenceRows {
			lastCommonSequences[i] = perSchool(row)
		}
		if err != nil {
			log.Error("Could not get lastest sync rows: ", err)
			return
		}

	}()

	var lastTermUpdates []perTerm
	go func() {
		defer wg.Done()
		termUpdateRows, err := q.GetLastSyncTimesForTerms(ctx, db.GetLastSyncTimesForTermsParams{
			SchoolIds:         tSchoolIDs,
			TermCollectionIds: tTermCollectionIDs,
		})
		lastTermUpdates := make([]perTerm, len(termUpdateRows))
		for i, row := range termUpdateRows {
			lastTermUpdates[i] = perTerm(row)
		}

		if err != nil {
			errCh <- err
			log.Error("Could not get lastest sync term rows: ", err)
			return
		}
	}()

	wg.Wait()
	if len(errCh) > 0 {
		http.Error(w, http.StatusText(500), 500)
	}

	result := syncTermsResult{
		SyncData:           syncChangeRows,
		PerTermSequences:   lastTermUpdates,
		PerSchoolSequences: lastCommonSequences,
	}
	resultJson, err := json.Marshal(result)
	if err != nil {
		log.Error("Could not marshal school rows", err)
		http.Error(w, http.StatusText(500), 500)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write(resultJson)
}
