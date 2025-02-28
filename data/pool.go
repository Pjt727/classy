package data

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/Pjt727/classy/collection/projectpath"
	"github.com/joho/godotenv"
	log "github.com/sirupsen/logrus"

	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	dbPool *pgxpool.Pool
	pgOnce sync.Once
)

func init() {

	err := godotenv.Load(filepath.Join(projectpath.Root, ".env"))

	if err != nil {
		panic(fmt.Sprint("Error loading .env file: ", err))
	}
}

func NewPool(ctx context.Context) (*pgxpool.Pool, error) {

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
		return dbPool, poolErr
	}

	return dbPool, nil
}
