package collection

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"sync/atomic"
	"time"

	"github.com/Pjt727/classy/collection/services"
	"github.com/Pjt727/classy/data/db"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/sync/errgroup"
)

// queues
const SECTIONS_OF_TERM_COLLECTIONS = "collection_jobs"

const COLLECTION_TIMEOUT = 10 * time.Minute
const COLLECITON_BATCH_SIZE = 10
const POLLING_INTERVAL = 200 * time.Millisecond
const POLLING_TIME = 5 * time.Second

// intervals that dictate when to reschedule this collection
// there will be a more dynamic algorithm used later
const RESCHEDULE_UNCHANGED = 24 * time.Hour
const RESCHEDULE_CHANGE = 5 * time.Minute
const RESCHEDULE_ERROR = 10 * time.Minute

type Scheduler struct {
	orch   Orchestrator
	dbPool *pgxpool.Pool
	logger *slog.Logger
}

func NewScheduler(pool *pgxpool.Pool) Scheduler {
	return Scheduler{
		orch:   GetDefaultOrchestrator(pool),
		dbPool: pool,
		logger: slog.Default(),
	}
}

type CollectionMessage struct {
	TermCollectionID string      `json:"term_collection_id"`
	SchoolID         string      `json:"school_id"`
	Debug            bool        `json:"debug"`
	ServiceName      pgtype.Text `json:"service"`
	IsFullCollection pgtype.Bool `json:"is_full_collection"`
}

// this calls uses internal postgres polling to return practically instantly if there are messages
func (s *Scheduler) PollForCollections(ctx context.Context) (uint32, error) {
	q := db.New(s.dbPool)
	readPollingRows, err := q.ReadPollingQueue(ctx, db.ReadPollingParams{
		QueueName:                   SECTIONS_OF_TERM_COLLECTIONS,
		SecondsUntilRescheduled:     int32(COLLECTION_TIMEOUT.Seconds()),
		JobCount:                    COLLECITON_BATCH_SIZE,
		SecondsPollingTime:          int32(POLLING_TIME.Seconds()),
		MillisecondsPollingInterval: int32(POLLING_INTERVAL.Milliseconds()),
	})
	if err != nil {
		return 0, err
	}

	var successfulTasks atomic.Uint32
	var eg errgroup.Group
	for _, row := range readPollingRows {
		eg.Go(func() error {
			var message CollectionMessage
			err := json.Unmarshal(row.Message, &message)
			if err != nil {
				return err
			}
			err = s.executeCollection(ctx, row.MessageID, message)
			if err != nil {
				return err
			}
			successfulTasks.Add(1)
			return nil
		})
	}

	if err = eg.Wait(); err != nil {
		return successfulTasks.Load(), nil
	}

	return successfulTasks.Load(), nil
}

func (s *Scheduler) executeCollection(ctx context.Context, collectionJobId int32, collectionMessage CollectionMessage) error {
	logger := s.logger.With(
		"school",
		collectionMessage.SchoolID,
		"term collection",
		collectionMessage.TermCollectionID,
	)
	q := db.New(s.dbPool)
	termCollection, err := q.GetTermCollection(ctx, db.GetTermCollectionParams{
		ID:       collectionMessage.TermCollectionID,
		SchoolID: collectionMessage.SchoolID,
	})
	if err != nil {
		return err
	}

	config := DefualtUpdateSectionsConfig()
	if collectionMessage.ServiceName.Valid {
		config.SetServiceName(collectionMessage.ServiceName.String)
	}
	if collectionMessage.IsFullCollection.Valid {
		config.SetFullCollection(collectionMessage.IsFullCollection.Bool)
	}

	results, collectionError := s.orch.UpdateAllSectionsOfSchool(ctx, termCollection, config)
	if collectionError != nil {
		logger.Error("Failed collection", "error", err)
		didReschedule, err := s.rescheduleTermCollectionJob(ctx, collectionJobId, collectionMessage, CollectionResult{}, collectionError)
		if err != nil {
			return fmt.Errorf("Failed to reschedule collection %w", err)
		}
		logger.Info("Managed collection's scheduling", "didReschedule", didReschedule)
		return nil
	}

	logger.Info(
		"Successfully Completed Scheduled collection",
		"inserted",
		results.Inserted,
		"updated",
		results.Updated,
		"deleted",
		results.Deleted,
		"duration",
		results.Duration,
	)
	didReschedule, err := s.rescheduleTermCollectionJob(ctx, collectionJobId, collectionMessage, results, nil)
	if err != nil {
		return fmt.Errorf("Failed to reschedule collection %w", err)
	}
	logger.Info("Managed collection's scheduling", "didReschedule", didReschedule)

	return nil
}

// deletes the job from the queue and reschedules a new one only if needed
// return whether the job was rescheduled
// TODO: ensure there are not active term collections requests
func (s *Scheduler) rescheduleTermCollectionJob(
	ctx context.Context,
	collectionJobId int32,
	oldCollectionMessage CollectionMessage,
	collectionResult CollectionResult,
	collectionError error,
) (bool, error) {
	// might use the collection result later
	_ = collectionResult

	deleteParams := db.DeleteFromQueueParams{
		QueueName: SECTIONS_OF_TERM_COLLECTIONS,
		MessageID: collectionJobId,
	}

	// there might be some manual changes that need to be done before so do not reschedule
	if errors.Is(collectionError, services.ErrIncorrectAssumption) {
		q := db.New(s.dbPool)
		err := q.DeleteFromQueue(ctx, deleteParams)
		if err != nil {
			return false, err
		}
		return false, nil
	}
	// currently any other error we will resechedule with the error timeout
	var secondsToSchedule int32
	var doDebug bool
	if collectionError == nil {
		if collectionResult.AreChanges() {
			secondsToSchedule = int32(RESCHEDULE_CHANGE.Seconds())
		} else {
			secondsToSchedule = int32(RESCHEDULE_UNCHANGED.Seconds())
		}
		doDebug = false
	} else {
		secondsToSchedule = int32(RESCHEDULE_ERROR.Seconds())
		doDebug = true
	}

	tx, err := s.dbPool.Begin(ctx)
	if err != nil {
		return false, err
	}
	defer tx.Rollback(ctx)
	q := db.New(s.dbPool).WithTx(tx)
	err = q.DeleteFromQueue(ctx, deleteParams)
	if err != nil {
		return false, err
	}
	nextCollectionMessage := CollectionMessage{
		TermCollectionID: oldCollectionMessage.TermCollectionID,
		SchoolID:         oldCollectionMessage.SchoolID,
		Debug:            doDebug,
		ServiceName:      oldCollectionMessage.ServiceName,
		IsFullCollection: pgtype.Bool{
			Bool:  false,
			Valid: true,
		},
	}
	messageBytes, err := json.Marshal(nextCollectionMessage)
	if err != nil {
		return false, err
	}
	err = q.AddToQueue(ctx, db.AddToQueueParams{
		QueueName:             SECTIONS_OF_TERM_COLLECTIONS,
		Message:               messageBytes,
		SecondsUntilAvailable: int(secondsToSchedule),
	})
	if err != nil {
		return false, err
	}

	err = tx.Commit(ctx)
	if err != nil {
		return false, err
	}

	return true, nil
}
