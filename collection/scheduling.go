package collection

import (
	"context"
	"encoding/json"
	"sync/atomic"
	"time"

	"github.com/Pjt727/classy/data/db"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/sync/errgroup"
)

const COLLECTION_TIMEOUT = 10 * time.Minute
const COLLECITON_BATCH_SIZE = 10
const POLLING_INTERVAL = 200 * time.Millisecond
const POLLING_TIME = -5 * time.Second

type Scheduler struct {
	orch Orchestrator
	pool *pgxpool.Pool
}

func NewScheduler(pool *pgxpool.Pool) Scheduler {
	return Scheduler{
		orch: GetDefaultOrchestrator(pool),
		pool: pool,
	}
}

type CollectionMessage struct {
	TermCollectionID string      `json:"term_collection_id"`
	ServiceName      pgtype.Text `json:"service"`
	IsFullCollection pgtype.Text `json:"is_full_collection"`
}

// this function will block for the duration
func (s *Scheduler) pollForCollections(ctx context.Context) (uint32, error) {
	// TODO: cancel context
	q := db.New(s.pool)
	readPollingRows, err := q.ConsumeScheduledCollections(ctx, db.ReadPollingParams{
		QueueName:                   "collection_jobs",
		SecondsUntilRescheduled:     int32(COLLECTION_TIMEOUT.Seconds()),
		JobCount:                    COLLECITON_BATCH_SIZE,
		SecondsPollingTime:          int32(POLLING_TIME.Seconds()),
		MillisecondsPollingInterval: int32(POLLING_INTERVAL.Seconds()),
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
			err = s.executeCollection(row.MessageID, message)
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

func (s *Scheduler) executeCollection(collectionJobId int32, collectionMessage CollectionMessage) error {

	// once we know this completes successfully then we delete queue
	return nil
}
