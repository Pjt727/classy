package testservice

import (
	"context"
	"errors"
	"fmt"
	"github.com/Pjt727/classy/data/class-entry"
	log "github.com/sirupsen/logrus"
	"os"
	"path/filepath"
)

type ClassData struct {
	Sections     []classentry.Section
	MeetingTimes []classentry.MeetingTime
	Professors   []classentry.Professor
	Courses      []classentry.Course
}

type termDirectory struct {
	term          classentry.TermCollection
	directoryPath string
	jsonPaths     []string
	currentIndex  int
}

func newTermDirectoryLocation(term classentry.TermCollection, directoryPath string) (termDirectory, error) {
	termDirectory := termDirectory{
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
		if file.IsDir() {
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
func (t *termDirectory) nextJsonPath() string {
	nextPath := t.jsonPaths[t.currentIndex]
	t.currentIndex += 1
	if t.currentIndex >= len(t.jsonPaths) {
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
	fileBytesToClassData     func(log.Entry, []byte) (ClassData, error)
}

type TermDirectoryEntry struct {
	SchoolID       string
	TermCollection classentry.TermCollection
	FilePath       string
}

// Entries must be unique on the termcollection for each school
// the fileMapper will be given every file in the given directory
// and must provide the respective class data
func NewService(
	entries []TermDirectoryEntry,
	fileMapper func(log.Entry, []byte) (ClassData, error),
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
	logger log.Entry,
	ctx context.Context,
) ([]classentry.School, error) {
	var schools []classentry.School
	for _, testSchool := range t.schoolIDToSchooolForTest {
		schools = append(schools, testSchool.school)
	}
	return schools, nil
}

func (t *FileTestService) StageAllClasses(
	logger log.Entry,
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
	logger.Infof("Adding %d sections from %s", len(classData.Sections), path)
	professors := make([]classentry.Professor, len(classData.Professors))
	i := 0
	for _, professor := range classData.Professors {
		professors[i] = professor
		i += 1
	}

	courses := make([]classentry.Course, len(classData.Courses))
	i = 0
	for _, course := range classData.Courses {
		courses[i] = course
		i += 1
	}

	err = q.InsertClassData(
		&logger,
		ctx,
		classData.MeetingTimes,
		classData.Sections,
		professors,
		courses,
	)
	if err != nil {
		return err
	}

	return nil
}

func (t *FileTestService) GetTermCollections(
	logger log.Entry,
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
