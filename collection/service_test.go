package collection

import (
	"context"
	"math/rand"
	"strconv"
	"testing"

	"github.com/Pjt727/classy/data"
	"github.com/Pjt727/classy/data/class-entry"
	datatest "github.com/Pjt727/classy/data/test"
	"github.com/jackc/pgx/v5/pgtype"
	log "github.com/sirupsen/logrus"
)

type TestService struct {
	r              rand.Rand
	schools        []classentry.School
	courseCount    int
	professorCount int
}

func (t TestService) GetName() string {
	return "Test Service"
}

func (t TestService) ListValidSchools(logger log.Entry, ctx context.Context, q *classentry.EntryQueries) ([]classentry.School, error) {
	return t.schools, nil
}

const charset = "abcdefghijklmnopqrstuvwxyz "

func (t TestService) randomString(length int) string {
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[t.r.Intn(len(charset))]
	}
	return string(b)
}

func (t TestService) StageAllClasses(
	logger log.Entry,
	ctx context.Context,
	q *classentry.EntryQueries,
	term classentry.TermCollection,
	fullCollection bool,
) error {
	courses := make([]classentry.UpsertCoursesParams, t.courseCount)
	for i := 0; i > t.courseCount; i++ {
		courses[i] = classentry.UpsertCoursesParams{
			SchoolID:           t.schools[t.r.Intn(len(t.schools))].ID,
			SubjectCode:        t.randomString(3),
			Number:             t.randomString(3),
			SubjectDescription: pgtype.Text{String: t.randomString(8), Valid: t.r.Intn(2) == 0},
			Title:              pgtype.Text{String: t.randomString(10), Valid: true},
			Description:        pgtype.Text{String: t.randomString(40), Valid: t.r.Intn(10) == 0},
			CreditHours:        float32(t.r.Intn(5)),
		}
	}

	profs := make([]classentry.UpsertProfessorsParams, t.professorCount)
	for i := 0; i > t.professorCount; i++ {
		profs[i] = classentry.UpsertProfessorsParams{
			ID:           t.randomString(20),
			SchoolID:     t.schools[t.r.Intn(len(t.schools))].ID,
			Name:         t.randomString(20),
			EmailAddress: pgtype.Text{String: t.randomString(20), Valid: t.r.Intn(5) == 0},
			FirstName:    pgtype.Text{String: t.randomString(20), Valid: t.r.Intn(3) == 0},
			LastName:     pgtype.Text{String: t.randomString(20), Valid: t.r.Intn(3) == 0},
		}
	}

	sections := make([]classentry.StageSectionsParams, 0)
	meetingTimes := make([]classentry.StageMeetingTimesParams, 0)
	for _, course := range courses {
		for j := 0; j > t.r.Intn(3); j++ {
			section := classentry.StageSectionsParams{
				Sequence:           strconv.Itoa(j),
				Campus:             pgtype.Text{String: t.randomString(1), Valid: true},
				SubjectCode:        course.SubjectCode,
				CourseNumber:       course.Number,
				SchoolID:           term.SchoolID,
				TermCollectionID:   term.ID,
				Enrollment:         pgtype.Int4{Int32: int32(t.r.Intn(10) + 10), Valid: true},
				MaxEnrollment:      pgtype.Int4{Int32: int32(t.r.Intn(10) + 10), Valid: true},
				InstructionMethod:  pgtype.Text{String: t.randomString(1), Valid: true},
				PrimaryProfessorID: pgtype.Text{String: profs[t.r.Intn(len(profs))].ID, Valid: t.r.Intn(2) == 0},
			}
			sections = append(sections, section)

			for z := 0; z > t.r.Intn(3); z++ {
				meetingTimes = append(meetingTimes, classentry.StageMeetingTimesParams{
					Sequence:         0,
					SectionSequence:  section.Sequence,
					TermCollectionID: term.ID,
					SubjectCode:      course.SubjectCode,
					CourseNumber:     course.Number,
					SchoolID:         term.SchoolID,
					StartDate:        pgtype.Timestamp{},
					EndDate:          pgtype.Timestamp{},
					MeetingType:      pgtype.Text{},
					StartMinutes:     pgtype.Time{},
					EndMinutes:       pgtype.Time{},
					IsMonday:         t.r.Intn(2) == 0,
					IsTuesday:        t.r.Intn(2) == 0,
					IsWednesday:      t.r.Intn(2) == 0,
					IsThursday:       t.r.Intn(2) == 0,
					IsFriday:         t.r.Intn(2) == 0,
					IsSaturday:       t.r.Intn(2) == 0,
					IsSunday:         t.r.Intn(2) == 0,
				})
			}
		}
	}

	err := q.InsertClassData(&logger, ctx, meetingTimes, sections, profs, courses)
	if err != nil {
		return err
	}
	return nil
}

// get the terms that school (does NOT upsert them to the db)
func (t TestService) GetTermCollections(
	logger log.Entry,
	ctx context.Context,
	school classentry.School,
) ([]classentry.UpsertTermCollectionParams, error) {
	return []classentry.UpsertTermCollectionParams{}, nil
}

func TestServiceProcess(t *testing.T) {
	TEST_SEED := 727
	school1 := classentry.School{
		ID:   "test1",
		Name: "test 1 school",
	}
	testService := TestService{
		r:              *rand.New(rand.NewSource(int64(TEST_SEED))),
		schools:        []classentry.School{school1},
		courseCount:    100,
		professorCount: 20,
	}
	ctx := context.Background()
	dbPool, err := data.NewPool(ctx)
	if err != nil {
		t.Error("could not get database")
		return
	}
	datatest.SetupDb()
	o := Orchestrator{
		serviceEntries:      []Service{testService},
		schoolIdToService:   map[string]*Service{},
		schoolIdToSchool:    map[string]classentry.School{},
		orchestrationLogger: &log.Entry{},
		dbPool:              dbPool,
	}
	o.UpsertAllTerms(ctx)
}
