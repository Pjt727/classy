package test_banner

import (
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/Pjt727/classy/collection/projectpath"
	"github.com/Pjt727/classy/collection/services/banner"
	"github.com/Pjt727/classy/collection/services/testservice"
	"github.com/Pjt727/classy/data/class-entry"
	dbhelpers "github.com/Pjt727/classy/data/testdb"
	log "github.com/sirupsen/logrus"
)

var TESTING_ASSETS_BASE_DIR = filepath.Join(
	projectpath.Root,
	"collection",
	"services",
	"banner",
	"test-assets",
)

func jsonToClassData(logger log.Entry, data []byte) (testservice.ClassData, error) {
	var classData testservice.ClassData
	var sections banner.SectionSearch

	if err := json.Unmarshal(data, &sections); err != nil {
		logger.Error("Error decoding sections: ", err)
		return classData, err
	}
	bannerInfo := banner.ProcessSectionSearch(sections)
	professors := make([]classentry.Professor, len(bannerInfo.Professors))
	i := 0
	for _, professor := range bannerInfo.Professors {
		professors[i] = professor
		i += 1
	}

	courses := make([]classentry.Course, len(bannerInfo.Courses))
	i = 0
	for _, course := range bannerInfo.Courses {
		courses[i] = course
		i += 1
	}
	classData.Courses = courses
	classData.Professors = professors
	classData.MeetingTimes = bannerInfo.MeetingTimes
	classData.Sections = bannerInfo.Sections
	return classData, nil
}

func GetTestingService() (*testservice.FileTestService, error) {
	schoolID := "marist"
	fileTestsBanner, err := testservice.NewService(
		[]testservice.TermDirectoryEntry{
			{
				SchoolID:       schoolID,
				TermCollection: testservice.NewTermCollection("202440", "Fall", 2024),
				FilePath:       filepath.Join(TESTING_ASSETS_BASE_DIR, "marist-fall-2024"),
			},
		},
		jsonToClassData,
	)

	if err != nil {
		return nil, err
	}

	return &fileTestsBanner, nil

}

func TestBannerFileInput(t *testing.T) {
	err := dbhelpers.SetupTestDb()
	if err != nil {
		t.Error(err)
		return
	}

	fileTestsBanner, err := GetTestingService()

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
