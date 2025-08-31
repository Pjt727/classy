package serversync

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"sync"

	"log/slog"

	"github.com/Pjt727/classy/data/db"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/sync/errgroup"
)

var DEFAULT_MAX_RECORDS uint32 = 500
var LIMIT_MAX_RECORDS uint32 = 10_000

type syncChange struct {
	Sequence       uint32         `json:"sequence"`
	TableName      string         `json:"table_name"`
	PkFields       map[string]any `json:"pk_fields"`
	SyncAction     string         `json:"sync_action"`
	RelevantFields map[string]any `json:"relevant_fields"`
}

type syncHandler struct {
	dbPool *pgxpool.Pool
	logger *slog.Logger
}

// all syncs
type syncResult struct {
	SyncData     []syncChange `json:"sync_data"`
	LastSequence uint32       `json:"new_latest_sync"`
	HasMore      bool         `json:"has_more"`
}

func (h *syncHandler) syncAll(w http.ResponseWriter, r *http.Request) {

	inputSequence := r.URL.Query().Get("lastSyncSequence")
	var sequence int
	if inputSequence == "" {
		// default time sequence which includes everything
		sequence = 0
	} else {
		var err error
		sequence, err = strconv.Atoi(inputSequence)
		if err != nil {
			http.Error(w, fmt.Sprintf("Could not parse sequence: %s", inputSequence), http.StatusBadRequest)
			return
		}
	}

	inputMaxRecords := r.URL.Query().Get("maxRecordsCount")
	var maxRecordsCount int
	if inputMaxRecords == "" {
		maxRecordsCount = int(DEFAULT_MAX_RECORDS)
	} else {
		var err error
		maxRecordsCount, err = strconv.Atoi(inputMaxRecords)
		if err != nil {
			http.Error(w, fmt.Sprintf("Could not parse records count: %s", inputMaxRecords), http.StatusBadRequest)
			return
		}
	}

	// default is true
	foldSyncData := r.URL.Query().Get("foldSyncData") != "false"
	ctx := r.Context()
	var err error
	var result syncResult
	if foldSyncData {
		result, err = h.getSyncAllFolded(ctx, sequence, maxRecordsCount)
	} else {
		result, err = h.getSyncAllUnFolded(ctx, sequence, maxRecordsCount)
	}

	if err != nil {
		h.logger.Error("Failed getting all sync row changes", "err", err)
		http.Error(w, http.StatusText(500), 500)
	}

	resultJson, err := json.Marshal(result)
	if err != nil {
		h.logger.Error("Could not marshal school rows", "err", err)
		http.Error(w, http.StatusText(500), 500)
		return
	}
	w.Header().Set("Content-Encoding", "gzip")
	w.Header().Set("Content-Type", "application/json")
	w.Write(resultJson)
}

func (h *syncHandler) getSyncAllFolded(ctx context.Context, lastSequence int, maxRecords int) (syncResult, error) {
	q := db.New(h.dbPool)
	syncChangeRows, err := q.SyncAll(ctx, db.SyncAllParams{
		LastSequence: int32(lastSequence),
		MaxRecords:   int32(min(maxRecords, int(LIMIT_MAX_RECORDS))),
	})
	if err != nil {
		return syncResult{}, err
	}
	syncChanges := make([]syncChange, len(syncChangeRows))
	for i, syncChangeRow := range syncChangeRows {
		// relevant fields can be nil in the case of a deletion
		syncChanges[i] = syncChange{
			Sequence:       uint32(syncChangeRow.Sequence),
			TableName:      syncChangeRow.TableName,
			PkFields:       syncChangeRow.PkFields.(map[string]any),
			SyncAction:     string(syncChangeRow.SyncAction),
			RelevantFields: syncChangeRow.RelevantFields.(map[string]any),
		}
	}

	newSyncSequence := uint32(lastSequence)
	hasMore := false

	if len(syncChangeRows) > 0 {
		newSyncSequence = uint32(syncChangeRows[len(syncChangeRows)-1].Sequence)
		hasMore = syncChangeRows[0].HasMore
	}

	return syncResult{
		SyncData:     syncChanges,
		LastSequence: newSyncSequence,
		HasMore:      hasMore,
	}, nil
}

func (h *syncHandler) getSyncAllUnFolded(ctx context.Context, lastSequence int, maxRecords int) (syncResult, error) {
	q := db.New(h.dbPool)
	limit := int32(min(maxRecords, int(LIMIT_MAX_RECORDS)))
	syncChangeRows, err := q.SyncAllUnfolded(ctx, db.SyncAllUnfoldedParams{
		LastSequence: int32(lastSequence),
		MaxRecords:   limit,
	})
	if err != nil {
		return syncResult{}, err
	}
	syncChangeRows, hasMore := normalizeLimits(syncChangeRows, limit)
	syncChanges := make([]syncChange, len(syncChangeRows))
	for i, syncChangeRow := range syncChangeRows {
		var pkFields map[string]any
		err := json.Unmarshal(syncChangeRow.PkFields, &pkFields)
		if err != nil {
			return syncResult{}, err
		}
		var relevantFields map[string]any
		err = json.Unmarshal(syncChangeRow.RelevantFields, &relevantFields)
		if err != nil {
			return syncResult{}, err
		}

		// relevant fields can be nil in the case of a deletion
		syncChanges[i] = syncChange{
			Sequence:       uint32(syncChangeRow.Sequence),
			TableName:      syncChangeRow.TableName,
			PkFields:       pkFields,
			SyncAction:     string(syncChangeRow.SyncAction),
			RelevantFields: relevantFields,
		}
	}

	newSyncSequence := uint32(lastSequence)
	if len(syncChangeRows) > 0 {
		newSyncSequence = uint32(syncChangeRows[len(syncChangeRows)-1].Sequence)
	}

	return syncResult{
		SyncData:     syncChanges,
		LastSequence: newSyncSequence,
		HasMore:      hasMore,
	}, nil
}

type selectSchoolEntry struct {
	/// either a or
	/// map[string]int            -> school to last sequence for or
	/// map[string]map[string]int -> school to terms to last sequence
	TermExclusions       map[string]map[string]uint32 `json:"exclude"`
	Schools              map[string]any               `json:"schools"`
	MaxRecordsPerRequest *uint32                      `json:"max_records_per_request"`
}

type syncTermsResult struct {
	NewSyncSequences map[string]any `json:"new_sync_term_sequences"`
	SyncData         []syncChange   `json:"sync_data"`
	AnyHasMore       bool           `json:"any_has_more"`
}

func (h *syncHandler) syncSchoolTerms(w http.ResponseWriter, r *http.Request) {

	// reuse this input at return updating values in return without a separate qwuery
	var syncData selectSchoolEntry
	err := json.NewDecoder(r.Body).Decode(&syncData)
	if err != nil {
		http.Error(w, "Request has incorrect shape: "+err.Error(), http.StatusBadRequest)
		return
	}
	ctx := r.Context()
	q := db.New(h.dbPool)

	// TODO: fix this pontential attack
	// it is a little odd that limiting has to be per school bc there is not a single sql query for everything
	// overloading this endpoint with difficult requests from bad actors is far too easy
	// perhaps implementing a max for schools * maxRequestsPerRequest would be better
	var maxRequestsPerRequest uint32
	if syncData.MaxRecordsPerRequest != nil {
		maxRequestsPerRequest = min(LIMIT_MAX_RECORDS, uint32(*syncData.MaxRecordsPerRequest))
	} else {
		maxRequestsPerRequest = DEFAULT_MAX_RECORDS
	}

	// process the full request to ensure its validity before making any calls to the db
	// this is import bc if rate limiting is done at a different layer based on bandwith
	// the db could get slammed with requests while still only sending back the error
	schoolToSequence := make(map[string]uint32)
	schoolToTermSequences := make(map[string]map[string]uint32)
	for schoolID, sequenceOrTermMap := range syncData.Schools {
		switch schoolChoice := sequenceOrTermMap.(type) {
		case float64: // this school last sequence
			if schoolChoice < 0 {
				http.Error(w, fmt.Sprintf("Invalid sequence number: %f", schoolChoice), http.StatusBadRequest)
				return
			}
			schoolToSequence[schoolID] = uint32(schoolChoice)
		case map[string]any: // this is a term mapping
			termMap := make(map[string]uint32)
			termsExclusions, checkTermExclusion := syncData.TermExclusions[schoolID]
			for term, seq := range schoolChoice {
				seqFloat, ok := seq.(float64)
				if !ok || seqFloat < 0 {
					http.Error(w, fmt.Sprintf("Invalid term sequence: %s", schoolChoice), http.StatusBadRequest)
					return
				}
				termMap[term] = uint32(seqFloat)

				// synced terms cannot be exlcuded
				if checkTermExclusion {
					_, ok = termsExclusions[term]
					if ok {
						http.Error(w, fmt.Sprintf("Term collect `%s` requested sync but it is also excluded", term), http.StatusBadRequest)
						return
					}
				}
			}

			schoolToTermSequences[schoolID] = termMap

		default: // invalid type
			http.Error(w, "Invalid body, schools must map to a sequence or term mapping", http.StatusBadRequest)
			return
		}
	}

	syncChanges := make([]syncChange, 0)
	var mu sync.Mutex
	syncGroup, syncCtx := errgroup.WithContext(ctx)

	anyHasMore := false

	// sync requests for all terms of a school
	for schoolID, sequence := range schoolToSequence {
		syncGroup.Go(func() error {
			newSyncChanges, hasMore, err := syncSchoolBasedOffSchool(q, syncCtx, schoolID, uint32(sequence), maxRequestsPerRequest)
			if err != nil {
				return fmt.Errorf("Could not get school=`%s` sequence=`%d` %w", schoolID, sequence, err)
			}
			mu.Lock()
			defer mu.Unlock()
			if len(newSyncChanges) > 0 {
				newLastestSync := newSyncChanges[len(newSyncChanges)-1].Sequence
				syncData.Schools[schoolID] = max(newLastestSync, uint32(sequence))
			}
			anyHasMore = anyHasMore || hasMore
			syncChanges = append(syncChanges, newSyncChanges...)
			return nil
		})
	}

	// sync requests for select terms of a school
	for schoolID, termSequences := range schoolToTermSequences {
		syncGroup.Go(func() error {
			termExclusions, ok := syncData.TermExclusions[schoolID]
			if !ok {
				termExclusions = make(map[string]uint32)
			}
			newSyncChanges, hasMore, err := getTerms(q, ctx, schoolID, termSequences, maxRequestsPerRequest, termExclusions)
			h.logger.Info("term results", "hasMore", hasMore)
			if err != nil {
				return err
			}
			mu.Lock()
			defer mu.Unlock()
			if len(newSyncChanges) > 0 {
				newLastestSync := newSyncChanges[len(newSyncChanges)-1].Sequence
				for term, seq := range termSequences {
					termSequences[term] = max(newLastestSync, seq)
				}
				syncData.Schools[schoolID] = termSequences
			}
			anyHasMore = anyHasMore || hasMore
			syncChanges = append(syncChanges, newSyncChanges...)
			return nil
		})
	}

	if err := syncGroup.Wait(); err != nil {
		h.logger.Error("Could not get term/school sync rows", "err", err)
		http.Error(w, "Problem getting the sync changes", http.StatusInternalServerError)
	}

	result := syncTermsResult{
		NewSyncSequences: syncData.Schools,
		SyncData:         syncChanges,
		AnyHasMore:       anyHasMore,
	}
	resultJson, err := json.Marshal(result)
	if err != nil {
		h.logger.Error("Could not marshal school rows", "err", err)
		http.Error(w, http.StatusText(500), 500)
		return
	}
	w.Header().Set("Content-Encoding", "gzip")
	w.Header().Set("Content-Type", "application/json")
	w.Write(resultJson)
}

type termCollectionSequencePair struct {
	Sequence uint32 `json:"sequence"`
	Id       string `json:"id"`
}

func getTerms(
	q *db.Queries,
	ctx context.Context,
	schoolID string,
	runningtermToLastSequence map[string]uint32,
	maxRecords uint32,
	termExclusions map[string]uint32,
) ([]syncChange, bool, error) {
	syncChanges := make([]syncChange, 0)
	hasMore := false

	commonSequencePairs := make([]termCollectionSequencePair, 0)
	termCollectionSequencePairs := make([]termCollectionSequencePair, 0)
	for collectionID, sequence := range runningtermToLastSequence {
		pair := termCollectionSequencePair{
			Sequence: sequence,
			Id:       collectionID,
		}
		commonSequencePairs = append(commonSequencePairs, pair)
		termCollectionSequencePairs = append(termCollectionSequencePairs, pair)
	}

	for collectionID, sequence := range termExclusions {
		pair := termCollectionSequencePair{
			Sequence: sequence,
			Id:       collectionID,
		}
		commonSequencePairs = append(commonSequencePairs, pair)
	}

	termBytes, err := json.Marshal(termCollectionSequencePairs)
	if err != nil {
		return syncChanges, hasMore, err
	}

	commonBytes, err := json.Marshal(commonSequencePairs)
	if err != nil {
		return syncChanges, hasMore, err
	}

	syncTermResultRows, err := q.SyncTerms(ctx, db.SyncTermsParams{
		SchoolID:                          schoolID,
		MaxRecords:                        int32(maxRecords),
		TermCollectionSequencePairs:       termBytes,
		CommonTermCollectionSequencePairs: commonBytes,
	})

	if err != nil {
		return syncChanges, hasMore, err
	}

	for _, r := range syncTermResultRows {
		syncChanges = append(syncChanges, syncChange{
			Sequence:       uint32(r.Sequence),
			TableName:      r.TableName,
			PkFields:       r.PkFields.(map[string]any),
			SyncAction:     string(r.SyncAction),
			RelevantFields: r.RelevantFields.(map[string]any),
		})
	}

	if len(syncTermResultRows) > 0 {
		hasMore = syncTermResultRows[0].HasMore
	}

	return syncChanges, hasMore, nil
}

// / does not update the last sequence
func syncSchoolBasedOffSchool(q *db.Queries, ctx context.Context, schoolID string, lastSequence uint32, maxRecords uint32) ([]syncChange, bool, error) {
	syncChanges := make([]syncChange, 0)

	syncSchoolResultRows, err := q.SyncSchool(ctx, db.SyncSchoolParams{
		LastSequence: int32(lastSequence),
		SchoolID:     schoolID,
		MaxRecords:   int32(maxRecords),
	})

	if err != nil {
		return syncChanges, false, err
	}

	for _, r := range syncSchoolResultRows {
		syncChanges = append(syncChanges, syncChange{
			Sequence:       uint32(r.Sequence),
			TableName:      r.TableName,
			PkFields:       r.PkFields.(map[string]any),
			SyncAction:     string(r.SyncAction),
			RelevantFields: r.RelevantFields.(map[string]any),
		})
	}

	hasMore := false
	if len(syncSchoolResultRows) > 0 {
		hasMore = syncSchoolResultRows[0].HasMore
	}

	return syncChanges, hasMore, nil
}

// TODO: maybe find a util place for this function or be fine with it in mulitple places
// used for results that use database quert with limit + 1
// determines if there were more results and changes the array if there were
func normalizeLimits[T any](s []T, limit int32) ([]T, bool) {
	if len(s) > int(limit) {
		return s[:limit], true
	}
	return s, false
}
