package banner_test

import (
	"testing"

	test_banner "github.com/Pjt727/classy/collection/services/banner/test"
	dbhelpers "github.com/Pjt727/classy/data/testdb"
)

func TestBannerFileInput(t *testing.T) {
	err := dbhelpers.SetupTestDb()
	if err != nil {
		t.Error(err)
		return
	}

	fileTestsBanner, err := test_banner.GetTestingService()

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
