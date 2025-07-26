package banner_test

import (
	"context"
	"log/slog"
	"os"
	"testing"

	"github.com/Pjt727/classy/collection/services/banner"
	"github.com/Pjt727/classy/collection/services/banner/testbanner"
	"github.com/Pjt727/classy/collection/services/testservice"
	"github.com/Pjt727/classy/data/logging-helpers"
	"github.com/Pjt727/classy/data/testdb"
)

func TestBannerData(t *testing.T) {
	err := testdb.SetupTestDb()
	if err != nil {
		t.Error(err)
		return
	}

	fileTestsBanner, err := testbanner.GetFileTestingService()
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

func TestBannerMockServer(t *testing.T) {
	err := testdb.SetupTestDb()
	if err != nil {
		t.Error(err)
		return
	}

	ctx := context.Background()
	frameHandler := logginghelpers.NewHandler(os.Stdout, &logginghelpers.Options{
		AddSource: true,
		Level:     slog.LevelInfo,
		NoColor:   false,
	})
	logger := slog.New(frameHandler)

	serverContext, cancel := context.WithCancel(ctx)
	defer cancel()

	testingMockService, err := testbanner.GetMockTestingService(*logger, serverContext)
	if err != nil {
		t.Error(err)
		return
	}
	err = testservice.RunServiceThroughTestOrchestrator(*logger, testingMockService, false)
	if err != nil {
		t.Error(err)
		return
	}
	serverContext.Done()
}

func TestBannerDryRun(t *testing.T) {
	err := testdb.SetupTestDb()
	if err != nil {
		t.Error(err)
		return
	}

	frameHandler := logginghelpers.NewHandler(os.Stdout, &logginghelpers.Options{
		AddSource: true,
		Level:     slog.LevelDebug,
		NoColor:   false,
	})
	logger := slog.New(frameHandler)
	service := banner.GetDefaultService()
	err = testservice.RunServiceThroughTestOrchestrator(*logger, service, true)

	if err != nil {
		t.Error(err)
		return
	}
}
