package data

import (
	"os"
)

// overwrites the db conn
func SetupDb() {
	testDb := os.Getenv("TEST_DB_CONN")
	os.Setenv("DB_CONN", testDb)
}
