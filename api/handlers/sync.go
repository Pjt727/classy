package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/Pjt727/classy/data/db"
	"github.com/jackc/pgx/v5/pgxpool"
	log "github.com/sirupsen/logrus"
)

type SyncChange struct {
	Sequence       int32                  `json:"sequence"`
	TableName      string                 `json:"table_name"`
	PkFields       map[string]interface{} `json:"pk_fields"`
	SyncAction     string                 `json:"sync_action"`
	RelevantFields map[string]interface{} `json:"relevant_fields"`
}

type SyncHandler struct {
	DbPool *pgxpool.Pool
}

// all syncs
type syncResult struct {
	SyncData     []db.GetLastestSyncChangesRow `json:"data"`
	LastSequence int                           `json:"new_latest_sync"`
}

func (h *SyncHandler) SyncAll(w http.ResponseWriter, r *http.Request) {

	inputSequence := r.URL.Query().Get("lastSyncSequence")
	log.Info(inputSequence)
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
	errCh := make(chan error, 2)
	syncChangeRows, err := q.GetLastestSyncChanges(ctx, db.GetLastestSyncChangesParams{
		LastSequence: int32(sequence),
		MaxRecords:   500, // TODO: Change to what is in query params
	})
	if err != nil {
		errCh <- err
		log.Error("Could not get lastest sync rows: ", err)
		return
	}
	syncChanges := make([]SyncChange, len(syncChangeRows))
	for i, syncChangeRow := range syncChangeRows {
		syncChanges[i] = SyncChange{
			Sequence:       syncChangeRow.Sequence,
			TableName:      syncChangeRow.TableName,
			PkFields:       syncChangeRow.PkFields,
			SyncAction:     syncChangeRow.SyncAction,
			RelevantFields: syncChangeRow.RelevantFields,
		}
	}

	if len(errCh) > 0 {
		http.Error(w, http.StatusText(500), 500)
	}

	result := syncResult{
		SyncData:     syncChangeRows,
		LastSequence: int(syncChangeRows[len(syncChangeRows)-1].Sequence),
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

// select syncs

type CommonTable string

const (
	School         CommonTable = "School"
	TermCollection CommonTable = "TermCollection"
	Professor      CommonTable = "Professor"
	Course         CommonTable = "Course"
)

func (e CommonTable) validate() error {
	switch e {
	case School, TermCollection, Professor, Course:
		return nil
	default:
		return fmt.Errorf("invalid CommonTable value: %s", e)
	}
}

func (e *CommonTable) Scan(src interface{}) error {
	var source string
	switch s := src.(type) {
	case []byte:
		source = string(s)
	case string:
		source = s
	default:
		return fmt.Errorf("unsupported scan type for CommonTable: %T", src)
	}

	ct := CommonTable(source)
	if err := ct.validate(); err != nil {
		return err
	}

	*e = ct
	return nil
}

type commonTableSyncEntry struct {
	TableName string `json:"table_name"`
	LastSync  int    `json:"last_sync"`
}

type selectTermEntry struct {
	TermCollectionID string `json:"term_collection_id"`
	LastSync         int    `json:"last_sync"`
}

type selectSchoolEntry struct {
	CommonTables []commonTableSyncEntry `json:"common_tables"`
	SelectTerms  []selectTermEntry      `json:"select_terms"`
}

type syncTerms struct {
	SelectSchools map[string]selectSchoolEntry `json:"select_schools"`
}

type syncTermsResult struct {
	NewSyncTermSequences map[string]selectSchoolEntry          `json:"new_sync_term_sequences"`
	SyncData             []db.GetLastestSyncChangesForTermsRow `json:"sync_data"`
}

func (h *SyncHandler) SyncTerms(w http.ResponseWriter, r *http.Request) {

	// reuse this input at return updating values in return without a separate qwuery
	var syncData syncTerms
	err := json.NewDecoder(r.Body).Decode(&syncData)
	if err != nil {
		log.Error("Could not parse sequence", err)
		http.Error(w, "Could not parse body: "+err.Error(), http.StatusBadRequest)
		return
	}
	// flatten the data structures
	commonTables := make([]string, 0)
	commonTableSequences := make([]int32, 0)
	termCollectionIDs := make([]string, 0)
	termCollectionSequences := make([]int32, 0)
	schoolIDs := make([]string, 0)

	ctx := r.Context()
	q := db.New(h.DbPool)
	for schoolID, schoolEntry := range syncData.SelectSchools {
		for _, commonTableEntry := range schoolEntry.CommonTables {
			commonTables = append(commonTables, commonTableEntry.TableName)
			commonTableSequences = append(commonTableSequences, int32(commonTableEntry.LastSync))
			schoolIDs = append(schoolIDs, schoolID)
		}
		for _, selectTermEntry := range schoolEntry.SelectTerms {
			commonTables = append(commonTables, selectTermEntry.TermCollectionID)
			commonTableSequences = append(commonTableSequences, int32(selectTermEntry.LastSync))
		}

	}

	syncRows, err := q.GetLastestSyncChangesForTerms(
		ctx,
		db.GetLastestSyncChangesForTermsParams{
			CommonTables:            commonTables,
			SchoolID:                schoolIDs,
			CommonSequences:         commonTableSequences,
			TermCollectionID:        termCollectionIDs,
			TermCollectionSequences: termCollectionSequences,
			MaxRecords:              500, // TODO: Change to what is in body
		},
	)

	syncChanges := make([]SyncChange, len(syncRows))
	for i, syncChangeRow := range syncRows {
		syncChanges[i] = SyncChange{
			Sequence:       syncChangeRow.Sequence,
			TableName:      syncChangeRow.TableName,
			PkFields:       syncChangeRow.PkFields,
			SyncAction:     syncChangeRow.SyncAction,
			RelevantFields: syncChangeRow.RelevantFields,
		}
	}

	result := syncTermsResult{
		NewSyncTermSequences: map[string]selectSchoolEntry{},
		SyncData:             syncRows,
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
