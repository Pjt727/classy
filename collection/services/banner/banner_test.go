package banner_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/Pjt727/classy/collection"
	"github.com/Pjt727/classy/collection/services/banner"
	"github.com/Pjt727/classy/data/db"
	"github.com/jackc/pgx/v5/pgtype"
	log "github.com/sirupsen/logrus"
)

var TESTING_ASSETS_BASE_DIR = filepath.Join("collection", "services", "test-assets")

type termDirectoryLocation struct {
	term          db.TermCollection
	directoryPath string
}

type fileTestsSchool struct {
	school                        db.School
	termsCollectionIDToTermForAdd map[string]termDirectoryLocation
}

type fileTestsBanner struct {
	schoolIDToschooolForTest map[string]fileTestsSchool
}

func (t fileTestsBanner) GetName() string {
	return "FileTestsForBanner"
}

func (t fileTestsBanner) ListValidSchools(
	logger log.Entry,
	ctx context.Context,
	q *db.Queries,
) ([]db.School, error) {
	var schools []db.School
	for _, testSchool := range t.schoolIDToschooolForTest {
		schools = append(schools, testSchool.school)
	}
	return schools, nil
}

func (t fileTestsBanner) StageAllClasses(
	logger log.Entry,
	ctx context.Context,
	q *db.Queries,
	term db.TermCollection,
	fullCollection bool,
) error {
	testSchool, ok := t.schoolIDToschooolForTest[term.SchoolID]
	if !ok {
		return errors.New(fmt.Sprint("Could not find school ", testSchool.school.ID))
	}
	testTerm, ok := testSchool.termsCollectionIDToTermForAdd[term.ID]
	if !ok {
		return errors.New(fmt.Sprintf("Could not find term test term for term collection id %s with school id %s", term.ID, term.SchoolID))
	}

	// read all of the json files in the directory as section search json results
	files, err := os.ReadDir(testTerm.directoryPath)
	if err != nil {
		return err
	}
	for _, file := range files {
		if filepath.Ext(file.Name()) != "json" {
			continue
		}
		data, err := os.ReadFile(file.Name())
		if err != nil {
			return err
		}

		var sections banner.SectionSearch
		if err := json.Unmarshal(data, &sections); err != nil {
			logger.Error("Error decoding sections: ", err)
			return err
		}
	}

	return nil
}

func (t fileTestsBanner) GetTermCollections(
	logger log.Entry,
	ctx context.Context,
	school db.School,
) ([]db.UpsertTermCollectionParams, error) {
	var termCollections []db.UpsertTermCollectionParams
	for _, testSchool := range t.schoolIDToschooolForTest {
		for _, termToAdd := range testSchool.termsCollectionIDToTermForAdd {
			termCollections = append(termCollections, db.UpsertTermCollectionParams{
				ID:              termToAdd.term.ID,
				SchoolID:        termToAdd.term.SchoolID,
				Year:            termToAdd.term.Year,
				Season:          termToAdd.term.Season,
				Name:            termToAdd.term.Name,
				StillCollecting: termToAdd.term.StillCollecting,
			})
		}
	}

	return termCollections, nil
}

func TestBannerFileInput(t *testing.T) {
	maristSchool := db.School{
		ID:   "marist",
		Name: "Marist University",
	}
	termCollection := db.TermCollection{
		ID:       "202440",
		SchoolID: maristSchool.ID,
		Year:     2024,
		Season:   "Fall",
		Name: pgtype.Text{
			String: "Fall 2024",
			Valid:  true,
		},
		StillCollecting: false,
	}
	bannerTest := fileTestsSchool{
		school: maristSchool,
		termsCollectionIDToTermForAdd: map[string]termDirectoryLocation{
			termCollection.ID: {
				term:          termCollection,
				directoryPath: filepath.Join(""),
			},
		},
	}
	fileTestsBanner := fileTestsBanner{
		schoolIDToschooolForTest: map[string]fileTestsSchool{
			maristSchool.ID: bannerTest,
		},
	}
	orch, _ := collection.CreateOrchestrator([]collection.Service{fileTestsBanner})
	orch.UpsertAllSchools(context.Background())
	orch.UpsertAllTerms(context.Background())
	orch.UpdateAllSectionsOfSchool(context.Background(), termCollection)

}
