package data

import (
	"context"
	"fmt"
	"os"
	"sync"

	"github.com/joho/godotenv"
	log "github.com/sirupsen/logrus"

	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	dbPool *pgxpool.Pool
	pgOnce sync.Once
)

func NewQueries(ctx context.Context) (*pgxpool.Pool, error) {

	err := godotenv.Load()

	if err != nil {
		panic("Error loading .env file")
	}
	connString := os.Getenv("DB_CONN")

	var poolErr error = nil
	pgOnce.Do(func() {

		pgPool, err := pgxpool.New(ctx, connString)
		if err != nil {
			log.Error(fmt.Errorf("Unable to create connection pool: %w", err))
			poolErr = err
		}
		dbPool = pgPool
	})
	if poolErr != nil {
		return dbPool, err
	}

	return dbPool, nil
}
