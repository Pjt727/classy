package classentry

import (
	"context"
	"fmt"
	"sync"

	"github.com/jackc/pgx/v5"
	log "github.com/sirupsen/logrus"

	"github.com/Pjt727/classy/data/db"
)

// service collection is the main place that needs to be easy to verify
//   only certain db functions are used
// ideally other componements of the project might have their own interfaces
//   but it feels a bit pointless to wrapper functions everywhere

func NewEntryQuery(database db.DBTX, schoolID string, termCollectionID *string) *EntryQueries {
	return &EntryQueries{
		q:                db.New(database),
		schoolID:         schoolID,
		termCollectionID: termCollectionID,
	}
}

type EntryQueries struct {
	q                *db.Queries
	schoolID         string
	termCollectionID *string
}

func (q *EntryQueries) WithTx(tx pgx.Tx) *EntryQueries {
	return &EntryQueries{
		q:                q.q.WithTx(tx),
		schoolID:         q.schoolID,
		termCollectionID: q.termCollectionID,
	}
}

// the main purpose of this staging process in not effiency... It is to have the
//    correct postgress triggers on the sections/ meetings e.i.
//    insert, delete, and updates for records actaully means that said record
//    was inserted deleted or updated
// if those triggers did not matter then we would simply delete all respective meeting / section data
//    and use copy from or batch inserts directly on the table

func (q *EntryQueries) DeleteCoursesMeetingsStaging(ctx context.Context, termCollection TermCollection) error {
	errCh := make(chan error, 2)
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		if err := q.q.DeleteStagingMeetingTimes(ctx, db.DeleteStagingMeetingTimesParams{
			TermCollectionID: termCollection.ID,
			SchoolID:         q.schoolID,
		}); err != nil {
			errCh <- err
		}
	}()
	go func() {
		defer wg.Done()
		if err := q.q.DeleteStagingSections(ctx, db.DeleteStagingSectionsParams{
			TermCollectionID: termCollection.ID,
			SchoolID:         q.schoolID,
		}); err != nil {
			errCh <- err
		}
	}()
	go func() {
		wg.Wait()
		close(errCh)
	}()
	for err := range errCh {
		return err
	}
	return nil
}

// moves all staged
func (q *EntryQueries) MoveStagedCoursesAndMeetings(
	ctx context.Context,
	termCollection TermCollection,
) (int, error) {
	err := q.q.RemoveUnstagedSections(ctx, db.RemoveUnstagedSectionsParams{
		TermCollectionID: termCollection.ID,
		SchoolID:         q.schoolID,
	})
	if err != nil {
		return 0, fmt.Errorf("error unstaging sections %v", err)
	}
	err = q.q.RemoveUnstagedMeetings(ctx, db.RemoveUnstagedMeetingsParams{
		TermCollectionID: termCollection.ID,
		SchoolID:         q.schoolID,
	})
	if err != nil {
		return 0, fmt.Errorf("error unstaging meeting %v", err)
	}
	err = q.q.MoveStagedSections(ctx)
	if err != nil {
		return 0, fmt.Errorf("error staging sections %v", err)
	}
	err = q.q.MoveStagedMeetingTimes(ctx)
	if err != nil {
		return 0, fmt.Errorf("error staging meetings %v", err)
	}
	return 0, nil
}

// helper to add class information
func (q *EntryQueries) InsertClassData(
	logger *log.Entry,
	ctx context.Context,
	meetingTimes []MeetingTime,
	sections []Section,
	professors []Professor,
	courses []Course,
) error {
	if len(meetingTimes) != 0 {
		dbMeetingTimes := make([]db.StageMeetingTimesParams, len(meetingTimes))
		for i, mt := range meetingTimes {
			dbMeetingTimes[i] = db.StageMeetingTimesParams{
				SchoolID:         q.schoolID,
				TermCollectionID: *q.termCollectionID,
				Sequence:         mt.Sequence,
				SectionSequence:  mt.SectionSequence,
				SubjectCode:      mt.SubjectCode,
				CourseNumber:     mt.CourseNumber,
				StartDate:        mt.StartDate,
				EndDate:          mt.EndDate,
				MeetingType:      mt.MeetingType,
				StartMinutes:     mt.StartMinutes,
				EndMinutes:       mt.EndMinutes,
				IsMonday:         mt.IsMonday,
				IsTuesday:        mt.IsTuesday,
				IsWednesday:      mt.IsWednesday,
				IsThursday:       mt.IsThursday,
				IsFriday:         mt.IsFriday,
				IsSaturday:       mt.IsSaturday,
				IsSunday:         mt.IsSunday,
			}
		}
		_, err := q.q.StageMeetingTimes(ctx, dbMeetingTimes)
		if err != nil {
			logger.Error("Staging meetings error ", err)
			return err
		}
	}

	if len(sections) != 0 {
		dbSections := make([]db.StageSectionsParams, len(sections))
		for i, s := range sections {
			dbSections[i] = db.StageSectionsParams{
				Sequence:           s.Sequence,
				Campus:             s.Campus,
				SubjectCode:        s.SubjectCode,
				CourseNumber:       s.CourseNumber,
				SchoolID:           q.schoolID,
				TermCollectionID:   *q.termCollectionID,
				Enrollment:         s.Enrollment,
				MaxEnrollment:      s.MaxEnrollment,
				InstructionMethod:  s.InstructionMethod,
				PrimaryProfessorID: s.PrimaryProfessorID,
			}
		}
		_, err := q.q.StageSections(ctx, dbSections)
		if err != nil {
			logger.Error("Staging sections error ", err)
			return err
		}
	}

	if len(professors) != 0 {
		dbProfessors := make([]db.UpsertProfessorsParams, len(professors))
		for i, p := range professors {
			dbProfessors[i] = db.UpsertProfessorsParams{
				ID:           p.ID,
				SchoolID:     q.schoolID,
				Name:         p.Name,
				EmailAddress: p.EmailAddress,
				FirstName:    p.FirstName,
				LastName:     p.LastName,
			}
		}
		buf := q.q.UpsertProfessors(ctx, []db.UpsertProfessorsParams(dbProfessors))

		var outerErr error = nil
		buf.Exec(func(i int, err error) {
			if err != nil {
				outerErr = err
			}
		})
		if outerErr != nil {
			logger.Error("Error upserting fac ", outerErr)
			return outerErr
		}
	}

	if len(courses) != 0 {
		dbCourses := make([]db.UpsertCoursesParams, len(courses))
		for i, c := range courses {
			dbCourses[i] = db.UpsertCoursesParams{
				SchoolID:           q.schoolID,
				SubjectCode:        c.SubjectCode,
				Number:             c.Number,
				SubjectDescription: c.SubjectDescription,
				Title:              c.Title,
				Description:        c.Description,
				CreditHours:        c.CreditHours,
			}
		}
		bc := q.q.UpsertCourses(ctx, dbCourses)
		var outerErr error = nil
		bc.Exec(func(i int, err error) {
			if err != nil {
				outerErr = err
			}
		})
		if outerErr != nil {
			logger.Error("Error upserting course", outerErr)
			return outerErr
		}
	}

	return nil
}
