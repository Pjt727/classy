package db

import (
	"context"

	"github.com/jackc/pgx/v5/pgtype"
)

// sqlc does not have support for entensions so these queries have to be written manually
// pgmq docs:
// https://github.com/pgmq/pgmq/blob/main/docs/api/sql/functions.md

const readPollingMessages = `
SELECT FROM pgmq.read_with_poll(
			queue_name       => $1::string,
			vt               => $2::int,
			qty              => $3::int,
			max_poll_seconds => $4::int,
			poll_interval_ms => $5::int
)
`

type ReadPollingParams struct {
	QueueName                   string `json:"queue_name"`
	SecondsUntilRescheduled     int32  `json:"seconds_until_rescheduled"`
	JobCount                    int32  `json:"job_count"`
	SecondsPollingTime          int32  `json:"seconds_polling_time"`
	MillisecondsPollingInterval int32  `json:"milliseconds_polling_interval"`
}

// https://github.com/pgmq/pgmq/blob/main/docs/api/sql/types.md
type ReadPollingRow struct {
	MessageID  int32            `json:"msg_id"`
	ReadAmount string           `json:"read_ct"`
	EnquededAt pgtype.Timestamp `json:"enqueued_at"`
	VisibleAt  pgtype.Timestamp `json:"vt"`
	Message    []byte           `json:"message"`
}

// https://github.com/pgmq/pgmq/blob/main/docs/api/sql/functions.md#read_with_poll
func (q *Queries) ReadPollingQueue(ctx context.Context, arg ReadPollingParams) ([]ReadPollingRow, error) {
	rows, err := q.db.Query(ctx, readPollingMessages,
		arg.QueueName,
		arg.SecondsUntilRescheduled,
		arg.JobCount,
		arg.SecondsPollingTime,
		arg.MillisecondsPollingInterval,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []ReadPollingRow
	for rows.Next() {
		var i ReadPollingRow
		if err := rows.Scan(); err != nil {
			return nil, err
		}
		items = append(items, i)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

type DeleteFromQueueParams struct {
	QueueName string `json:"queue_name"`
	MessageID int32  `json:"msg_id"`
}

const deleteFromQueue = `
SELECT * FROM pgmq.delete(
			queue_name       => $1::string,
			msg_id           => $2::int
)
`

func (q *Queries) DeleteFromQueue(ctx context.Context, arg DeleteFromQueueParams) error {
	_, err := q.db.Exec(ctx, deleteFromQueue,
		arg.QueueName,
		arg.MessageID,
	)
	return err
}

type AddToQueueParams struct {
	QueueName             string `json:"queue_name"`
	Message               []byte `json:"msg"`
	SecondsUntilAvailable int    `json:"delay"`
}

const addToQueue = `
SELECT * FROM pgmq.send(
			queue_name       => $1::string,
			msg              => $2::jsonb,
			delay            => $2::int
)
`

func (q *Queries) AddToQueue(ctx context.Context, arg AddToQueueParams) error {
	_, err := q.db.Exec(ctx, addToQueue,
		arg.QueueName,
		arg.Message,
		arg.SecondsUntilAvailable,
	)
	return err
}
