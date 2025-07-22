package data

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/Pjt727/classy/collection/projectpath"
	"github.com/joho/godotenv"
	"log/slog"

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

var (
	dbPools  = make(map[string]*pgxpool.Pool)
	poolOnce = make(map[string]*sync.Once)
	mu       sync.Mutex
)

func NewPool(ctx context.Context, isTestDb bool) (*pgxpool.Pool, error) {
	var connString string
	var poolKey string

	if isTestDb {
		connString = os.Getenv("TEST_DB_CONN")
		poolKey = "test"

	} else {
		connString = os.Getenv("DB_CONN")
		poolKey = "regular"
	}

	mu.Lock()
	if _, ok := poolOnce[poolKey]; !ok {
		poolOnce[poolKey] = &sync.Once{}
	}
	mu.Unlock()

	var poolErr error
	poolOnce[poolKey].Do(func() {
		config, err := pgxpool.ParseConfig(connString)
		if err != nil {
			poolErr = fmt.Errorf("failed to parse connection string: %s", err)
			return
		}

		pgPool, err := pgxpool.NewWithConfig(ctx, config)

		if err != nil {
			poolErr = fmt.Errorf("unable to create connection pool: %s", err)
			slog.Error("unable to create pool", "err", poolErr)
			return
		}

		mu.Lock()
		dbPools[poolKey] = pgPool
		mu.Unlock()
	})

	if poolErr != nil {
		return nil, poolErr
	}

	mu.Lock()
	pool := dbPools[poolKey]
	mu.Unlock()

	return pool, nil
}
