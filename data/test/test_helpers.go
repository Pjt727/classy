package datatest

import (
	"os"

	"github.com/Pjt727/classy/collection/projectpath"
	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
)

// overwrites the db conn and resets the database apply down and up migrations
func SetupDb() error {
	testDb := os.Getenv("TEST_DB_CONN")
	os.Setenv("DB_CONN", testDb)

	m, err := migrate.New("file://"+projectpath.Root+"/migrations", testDb)
	if err != nil {
		return err
	}
	err = m.Down()
	if err != nil {
		return err
	}
	err = m.Up()
	if err != nil {
		return err
	}
	return nil
}
