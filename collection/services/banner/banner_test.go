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
	"github.com/Pjt727/classy/collection/projectpath"
	"github.com/Pjt727/classy/collection/services/banner"
	"github.com/Pjt727/classy/data/db"
	datatest "github.com/Pjt727/classy/data/test"
	"github.com/jackc/pgx/v5/pgtype"
	log "github.com/sirupsen/logrus"
)

var TESTING_ASSETS_BASE_DIR = filepath.Join(projectpath.Root, "collection", "services", "banner", "test-assets")

type termDirectoryLocation struct {
	term          db.TermCollection
	directoryPath string
	jsonPaths     []string
	currentIndex  int
}

func newTermDirectoryLocation(term db.TermCollection, directoryPath string) (termDirectoryLocation, error) {
	termDirectory := termDirectoryLocation{
		term:          term,
		directoryPath: directoryPath,
		currentIndex:  0,
		jsonPaths:     []string{},
	}
	files, err := os.ReadDir(directoryPath)
	if err != nil {
		return termDirectory, err
	}
	for _, file := range files {
		if filepath.Ext(file.Name()) != ".json" {
			continue
		}
		termDirectory.jsonPaths = append(
			termDirectory.jsonPaths,
			filepath.Join(directoryPath, file.Name()),
		)
	}
	if len(termDirectory.jsonPaths) == 0 {
		return termDirectory, errors.New(fmt.Sprintf("Directory %s must at least one json file in them", directoryPath))
	}

	return termDirectory, nil
}

// cycles through json files in the path
func (t *termDirectoryLocation) nextJsonPath() string {
	nextPath := t.jsonPaths[t.currentIndex]
	t.currentIndex += 1
	if t.currentIndex >= len(t.jsonPaths) {
		t.currentIndex = 0
	}
	return nextPath
}

type fileTestsSchool struct {
	school                        db.School
	termsCollectionIDToTermForAdd map[string]*termDirectoryLocation
}

type fileTestsBanner struct {
	schoolIDToschooolForTest map[string]fileTestsSchool
}

func (t *fileTestsBanner) GetName() string {
	return "FileTestsForBanner"
}

func (t *fileTestsBanner) ListValidSchools(
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

func (t *fileTestsBanner) StageAllClasses(
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
	jsonPath := testTerm.nextJsonPath()
	// read all of the json files in the directory as section search json results
	data, err := os.ReadFile(jsonPath)
	if err != nil {
		return err
	}

	var sections banner.SectionSearch
	if err := json.Unmarshal(data, &sections); err != nil {
		logger.Error("Error decoding sections: ", err)
		return err
	}
	logger.Infof("Adding %d sections from %s", len(sections.Sections), jsonPath)
	classData := banner.ProcessSectionSearch(sections, term)
	err = db.InsertClassData(
		&logger,
		ctx,
		q,
		classData.MeetingTimes,
		classData.DbSections,
		classData.Professors,
		classData.Courses,
	)
	if err != nil {
		return err
	}

	return nil
}

func (t *fileTestsBanner) GetTermCollections(
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
	err := datatest.SetupDb()
	if err != nil {
		t.Error(err)
		return
	}
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

	directoryLocation, err := newTermDirectoryLocation(
		termCollection,
		filepath.Join(TESTING_ASSETS_BASE_DIR, "marist-fall-2024"),
	)
	if err != nil {
		t.Error(err)
		return
	}

	bannerTest := fileTestsSchool{
		school: maristSchool,
		termsCollectionIDToTermForAdd: map[string]*termDirectoryLocation{
			termCollection.ID: &directoryLocation,
		},
	}
	fileTestsBanner := fileTestsBanner{
		schoolIDToschooolForTest: map[string]fileTestsSchool{
			maristSchool.ID: bannerTest,
		},
	}
	orch, err := collection.CreateOrchestrator([]collection.Service{&fileTestsBanner})
	if err != nil {
		t.Error(err)
		return
	}
	err = orch.UpsertAllSchools(context.Background())
	if err != nil {
		t.Error(err)
		return
	}
	err = orch.UpsertAllTerms(context.Background())
	if err != nil {
		t.Error(err)
		return
	}

	// upload all 13 of the json for the particular school

	for i := 0; i < 13; i++ {
		err = orch.UpdateAllSectionsOfSchool(context.Background(), termCollection)
		if err != nil {
			t.Error(err)
			return
		}
	}
}
