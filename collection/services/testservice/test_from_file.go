package testservice

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/Pjt727/classy/collection"
	"github.com/Pjt727/classy/data"
	"github.com/Pjt727/classy/data/class-entry"
	"github.com/Pjt727/classy/data/db"
	"github.com/jackc/pgx/v5/pgtype"
)

type termDirectory struct {
	term          classentry.TermCollection
	directoryPath string
	filesPaths    []string
	currentIndex  int
}

func newTermDirectoryLocation(
	term classentry.TermCollection,
	directoryPath string,
) (termDirectory, error) {
	termDirectory := termDirectory{
		term:          term,
		directoryPath: directoryPath,
		currentIndex:  0,
		filesPaths:    []string{},
	}
	files, err := os.ReadDir(directoryPath)
	if err != nil {
		return termDirectory, err
	}
	for _, file := range files {
		if file.IsDir() {
			continue
		}
		termDirectory.filesPaths = append(
			termDirectory.filesPaths,
			filepath.Join(directoryPath, file.Name()),
		)
	}
	if len(termDirectory.filesPaths) == 0 {
		return termDirectory, errors.New(
			fmt.Sprintf("Directory %s must at least one json file in them", directoryPath),
		)
	}

	return termDirectory, nil
}

// cycles through json files in the path
func (t *termDirectory) nextJsonPath() string {
	nextPath := t.filesPaths[t.currentIndex]
	t.currentIndex += 1
	if t.currentIndex >= len(t.filesPaths) {
		t.currentIndex = 0
	}
	return nextPath
}

type fileTestsSchool struct {
	school                        classentry.School
	termsCollectionIDToTermForAdd map[string]*termDirectory
}

type FileTestService struct {
	schoolIDToSchooolForTest map[string]fileTestsSchool
	fileBytesToClassData     func(logger slog.Logger, data []byte) (classentry.ClassData, error)
}

type TermDirectoryEntry struct {
	SchoolID       string
	TermCollection classentry.TermCollection
	FilePath       string
}

// helper function to quickly define term collections
func NewTermCollection(
	id string,
	season classentry.SeasonEnum,
	year int32,
) classentry.TermCollection {
	termName := fmt.Sprintf(
		"%s %d",
		season,
		year,
	)

	return classentry.TermCollection{
		ID: id,
		Term: classentry.Term{
			Year:   year,
			Season: season,
		},
		Name: pgtype.Text{
			String: termName,
			Valid:  true,
		},
		StillCollecting: true,
	}

}

// Entries must be unique on the termcollection for each school
// the fileMapper will be given every file in the given directory
// and must provide the respective class data
func NewService(
	entries []TermDirectoryEntry,
	fileMapper func(logger slog.Logger, data []byte) (classentry.ClassData, error),
) (FileTestService, error) {
	var service FileTestService
	schoolIDToSchooolForTest := make(map[string]fileTestsSchool)

	for _, e := range entries {
		school, exists := schoolIDToSchooolForTest[e.SchoolID]
		if exists {
			_, termExists := school.termsCollectionIDToTermForAdd[e.TermCollection.ID]
			if termExists {
				return service, errors.New(fmt.Sprintf("Ducplate term id: %s", e.TermCollection.ID))
			}
		} else {
			school = fileTestsSchool{
				school: classentry.School{
					ID:   e.SchoolID,
					Name: fmt.Sprint("Test: ", e.SchoolID),
				},
				termsCollectionIDToTermForAdd: map[string]*termDirectory{},
			}
		}
		termDir, err := newTermDirectoryLocation(
			e.TermCollection, e.FilePath,
		)
		if err != nil {
			return service, err
		}
		school.termsCollectionIDToTermForAdd[e.TermCollection.ID] = &termDir
		schoolIDToSchooolForTest[e.SchoolID] = school
	}
	service.schoolIDToSchooolForTest = schoolIDToSchooolForTest
	service.fileBytesToClassData = fileMapper
	return service, nil
}

func (t *FileTestService) GetName() string {
	return "FileTestsForBanner"
}

func (t *FileTestService) ListValidSchools(
	logger slog.Logger,
	ctx context.Context,
) ([]classentry.School, error) {
	var schools []classentry.School
	for _, testSchool := range t.schoolIDToSchooolForTest {
		schools = append(schools, testSchool.school)
	}
	return schools, nil
}

func (t *FileTestService) StageAllClasses(
	logger slog.Logger,
	ctx context.Context,
	q *classentry.EntryQueries,
	schoolID string,
	termCollection classentry.TermCollection,
	fullCollection bool,
) error {
	testSchool, ok := t.schoolIDToSchooolForTest[schoolID]
	if !ok {
		return errors.New(fmt.Sprint("Could not find school ", testSchool.school.ID))
	}
	termDir, ok := testSchool.termsCollectionIDToTermForAdd[termCollection.ID]
	if !ok {
		return errors.New(fmt.Sprintf(
			"Could not find term test term for term collection id %s with school id %s",
			termCollection.ID,
			schoolID,
		))
	}
	path := termDir.nextJsonPath()
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	classData, err := t.fileBytesToClassData(logger, data)
	if err != nil {
		return err
	}
	logger.Info("Adding sections", slog.Int("sections", len(classData.Sections)), slog.String("path", path))

	err = q.InsertClassData(
		&logger,
		ctx,
		classData,
	)
	if err != nil {
		return err
	}

	return nil
}

func (t *FileTestService) GetTermCollections(
	logger slog.Logger,
	ctx context.Context,
	school classentry.School,
) ([]classentry.TermCollection, error) {
	var termCollections []classentry.TermCollection
	for _, testSchool := range t.schoolIDToSchooolForTest {
		for _, termToAdd := range testSchool.termsCollectionIDToTermForAdd {
			termCollections = append(termCollections, classentry.TermCollection{
				ID: termToAdd.term.ID,
				Term: classentry.Term{
					Year:   termToAdd.term.Term.Year,
					Season: termToAdd.term.Term.Season,
				},
				Name:            termToAdd.term.Name,
				StillCollecting: termToAdd.term.StillCollecting,
			})
		}
	}

	return termCollections, nil
}

// adds class data from every files as well as the needs schools
//
//	and terms for the classdata
func (t *FileTestService) RunThroughOrchestrator() error {
	ctx := context.Background()

	testDb, err := data.NewPool(ctx, true)
	if err != nil {
		return err
	}

	orch, err := collection.CreateOrchestrator([]collection.Service{t}, nil, testDb)
	if err != nil {
		return err
	}
	err = orch.UpsertAllSchools(context.Background())
	if err != nil {
		return err
	}
	err = orch.UpsertAllTerms(context.Background())
	if err != nil {
		return err
	}

	for schoolID, fileTest := range t.schoolIDToSchooolForTest {
		for collectionID, termDirectory := range fileTest.termsCollectionIDToTermForAdd {
			dbTermCollection := db.TermCollection{
				ID:              collectionID,
				SchoolID:        schoolID,
				Year:            termDirectory.term.Term.Year,
				Season:          termDirectory.term.Term.Season,
				Name:            termDirectory.term.Name,
				StillCollecting: true,
			}
			for range termDirectory.filesPaths {
				err = orch.UpdateAllSectionsOfSchool(context.Background(), dbTermCollection)
				if err != nil {
					return err
				}
			}
		}
	}

	return nil
}
