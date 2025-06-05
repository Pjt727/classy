package testdb

import (
	"errors"
	"os"

	"github.com/Pjt727/classy/collection/projectpath"
	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
)

func SetupTestDb() error {
	testDb := os.Getenv("TEST_DB_CONN")

	m, err := migrate.New("file://"+projectpath.Root+"/migrations", testDb)
	if err != nil {
		return err
	}
	m.Force(4)
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

func ReloadDb() error {
	// this is really scary so only reset the actual database if this env variable is set
	//    the real database should be reset manually if ever needed
	isLocal := os.Getenv("LOCAL") == "true"
	if !isLocal {
		return errors.New("Reset database manually or set the LOCAL=\"true\" env variable")
	}

	db := os.Getenv("DB_CONN")
	m, err := migrate.New("file://"+projectpath.Root+"/migrations", db)
	if err != nil {
		return err
	}
	m.Force(4)
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
