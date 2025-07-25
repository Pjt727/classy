package testbanner

import (
	"encoding/json"
	"log/slog"
	"path/filepath"

	"github.com/Pjt727/classy/collection/projectpath"
	"github.com/Pjt727/classy/collection/services/banner"
	"github.com/Pjt727/classy/collection/services/testservice"
	"github.com/Pjt727/classy/data/class-entry"
)

var TESTING_ASSETS_BASE_DIR = filepath.Join(
	projectpath.Root,
	"collection",
	"services",
	"banner",
	"test-assets",
)

func jsonToClassData(logger slog.Logger, data []byte) (classentry.ClassData, error) {
	var classData classentry.ClassData
	var sections banner.SectionSearch

	if err := json.Unmarshal(data, &sections); err != nil {
		logger.Error("Error decoding sections", "error", err)
		return classData, err
	}
	bannerInfo := banner.ProcessSectionSearch(sections)
	return bannerInfo.ToEntry(), nil
}

func GetFileTestingService() (*testservice.FileTestService, error) {
	schoolID := "marist"
	fileTestsBanner, err := testservice.NewService(
		[]testservice.TermDirectoryEntry{
			{
				SchoolID:       schoolID,
				TermCollection: testservice.NewTermCollection("202440", classentry.SeasonEnumFall, 2024),
				FilePath:       filepath.Join(TESTING_ASSETS_BASE_DIR, "marist", "fall-2024"),
			},
		},
		jsonToClassData,
	)

	if err != nil {
		return nil, err
	}

	return &fileTestsBanner, nil

}
