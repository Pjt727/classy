package banner_test

import (
	"context"
	"log/slog"
	"os"
	"testing"

	"github.com/Pjt727/classy/collection"
	test_banner "github.com/Pjt727/classy/collection/services/banner/testbanner"
	"github.com/Pjt727/classy/data"
	"github.com/Pjt727/classy/data/db"
	logginghelpers "github.com/Pjt727/classy/data/logging-helpers"
	dbhelpers "github.com/Pjt727/classy/data/testdb"
	"github.com/jackc/pgx/v5/pgtype"
)

func TestBannerFileInput(t *testing.T) {
	err := dbhelpers.SetupTestDb()
	if err != nil {
		t.Error(err)
		return
	}

	fileTestsBanner, err := test_banner.GetFileTestingService()
	if err != nil {
		t.Error(err)
		return
	}

	err = fileTestsBanner.RunThroughOrchestrator()
	if err != nil {
		t.Error(err)
		return
	}
}

// TODO: make a RunThroughOrchestrator on the helper function that runs
//
//	the given service through an orchestrator using the first school or all schools with the first term collection
//	to reduce this boilerplate
func TestBannerMockServer(t *testing.T) {
	err := dbhelpers.SetupTestDb()
	if err != nil {
		t.Error(err)
		return
	}

	ctx := context.Background()

	testDb, err := data.NewPool(ctx, true)
	if err != nil {
		t.Error(err)
		return
	}

	testingMockService, err := test_banner.GetMockTestingService(*slog.Default(), ctx)
	if err != nil {
		t.Error(err)
		return
	}

	frameLogger := logginghelpers.NewHandler(os.Stdout, &logginghelpers.Options{
		AddSource: true,
		Level:     slog.LevelInfo,
		NoColor:   false,
	})
	logger := slog.New(frameLogger)

	testOrchestrator, err := collection.CreateOrchestrator(
		[]collection.Service{testingMockService},
		logger,
		testDb,
	)
	if err != nil {
		t.Error(err)
		return
	}

	err = testOrchestrator.UpsertSchoolTermsWithService(
		ctx,
		*logger,
		db.School{ID: "marist", Name: "Marist University"},
		testingMockService.GetName(),
	)
	if err != nil {
		t.Error(err)
		return
	}

	err = testOrchestrator.UpdateAllSectionsOfSchoolWithService(
		ctx,
		db.TermCollection{
			ID: "202440",
			Name: pgtype.Text{
				String: "blah blah",
				Valid:  true,
			},
			SchoolID:        "marist",
			Year:            2025,
			Season:          "Fall",
			StillCollecting: false,
		},
		*logger,
		testingMockService.GetName(),
		true,
	)
}
