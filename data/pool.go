package data

import (
	"context"
	"fmt"
	"os"
	"sync"

	"github.com/Pjt727/classy/data/db"
	"github.com/joho/godotenv"

	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	pgQuery *db.Queries
	pgOnce  sync.Once
)

func NewQueries(ctx context.Context) (*db.Queries, error) {

	err := godotenv.Load()

	if err != nil {
		panic("Error loading .env file")
	}
	connString := os.Getenv("DB_CONN")

	pgOnce.Do(func() {

		dbPool, err := pgxpool.New(ctx, connString)
		if err != nil {
			panic(fmt.Errorf("unable to create connection pool: %w", err))
		}

		pgQuery = db.New(dbPool)
	})

	return pgQuery, nil
}
