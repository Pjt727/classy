package classentry

import (
	"context"

	"github.com/jackc/pgx/v5"
	log "github.com/sirupsen/logrus"
	"golang.org/x/sync/errgroup"

	"github.com/Pjt727/classy/data/db"
)

// service collection is the main place that needs to be easy to verify
//   only certain db functions are used
// ideally other componements of the project might have their own interfaces
//   but it feels a bit pointless to wrapper functions everywhere

func NewEntryQuery(
	database db.DBTX,
	schoolID string,
	termCollectionID *string,
	termCollectionHistoryID *int32,
) *EntryQueries {
	return &EntryQueries{
		q:                       db.New(database),
		schoolID:                schoolID,
		termCollectionID:        termCollectionID,
		termCollectionHistoryID: termCollectionHistoryID,
	}
}

type EntryQueries struct {
	q                       *db.Queries
	schoolID                string
	termCollectionID        *string
	termCollectionHistoryID *int32
}

func (q *EntryQueries) WithTx(tx pgx.Tx) *EntryQueries {
	return &EntryQueries{
		q:                q.q.WithTx(tx),
		schoolID:         q.schoolID,
		termCollectionID: q.termCollectionID,
	}
}

// Staging is needed for multiple reasons:
//   1. Accurate triggers for changing records
//    For instance, to know if a section is deleted you must collect all sections and see that the
//       section is not in that collection
//   2. Concurrent database writes
//     You can not use a single transacation with concurrent writes, so to ensure db integrity
// Both of these could be done by storing all term data in memory then sending it all at once to the
//    database, but this is not scalable

// / helper to add class information concurrently
func (q *EntryQueries) InsertClassData(
	logger *log.Entry,
	ctx context.Context,
	meetingTimes []MeetingTime,
	sections []Section,
	professors []Professor,
	courses []Course,
) error {
	var eg errgroup.Group

	eg.Go(func() error { return q.StageMeetingTimes(ctx, meetingTimes, logger) })
	eg.Go(func() error { return q.StageSections(ctx, sections, logger) })
	eg.Go(func() error { return q.StageProfessors(ctx, professors, logger) })
	eg.Go(func() error { return q.StageCourses(ctx, courses, logger) })

	if err := eg.Wait(); err != nil {
		return err
	}

	return nil
}

func (q *EntryQueries) StageMeetingTimes(ctx context.Context, meetingTimes []MeetingTime, logger *log.Entry) error {
	if len(meetingTimes) == 0 {
		return nil
	}

	dbMeetingTimes := make([]db.StageMeetingTimesParams, len(meetingTimes))
	for i, mt := range meetingTimes {
		dbMeetingTimes[i] = db.StageMeetingTimesParams{
			TermCollectionHistoryID: *q.termCollectionHistoryID,
			SchoolID:                q.schoolID,
			TermCollectionID:        *q.termCollectionID,
			Sequence:                mt.Sequence,
			SectionSequence:         mt.SectionSequence,
			SubjectCode:             mt.SubjectCode,
			CourseNumber:            mt.CourseNumber,
			StartDate:               mt.StartDate,
			EndDate:                 mt.EndDate,
			MeetingType:             mt.MeetingType,
			StartMinutes:            mt.StartMinutes,
			EndMinutes:              mt.EndMinutes,
			IsMonday:                mt.IsMonday,
			IsTuesday:               mt.IsTuesday,
			IsWednesday:             mt.IsWednesday,
			IsThursday:              mt.IsThursday,
			IsFriday:                mt.IsFriday,
			IsSaturday:              mt.IsSaturday,
			IsSunday:                mt.IsSunday,
			Other:                   mt.Other,
		}
	}
	_, err := q.q.StageMeetingTimes(ctx, dbMeetingTimes)
	if err != nil {
		logger.Error("Staging meetings error ", err)
		return err
	}
	return nil
}

func (q *EntryQueries) StageSections(ctx context.Context, sections []Section, logger *log.Entry) error {
	if len(sections) == 0 {
		return nil
	}

	dbSections := make([]db.StageSectionsParams, len(sections))
	for i, s := range sections {
		dbSections[i] = db.StageSectionsParams{
			TermCollectionHistoryID: *q.termCollectionHistoryID,
			Sequence:                s.Sequence,
			Campus:                  s.Campus,
			SubjectCode:             s.SubjectCode,
			CourseNumber:            s.CourseNumber,
			SchoolID:                q.schoolID,
			TermCollectionID:        *q.termCollectionID,
			Enrollment:              s.Enrollment,
			MaxEnrollment:           s.MaxEnrollment,
			InstructionMethod:       s.InstructionMethod,
			PrimaryProfessorID:      s.PrimaryProfessorID,
			Other:                   s.Other,
		}
	}
	_, err := q.q.StageSections(ctx, dbSections)
	if err != nil {
		logger.Error("Staging sections error ", err)
		return err
	}
	return nil
}

func (q *EntryQueries) StageProfessors(ctx context.Context, professors []Professor, logger *log.Entry) error {
	if len(professors) == 0 {
		return nil
	}

	dbProfessors := make([]db.StageProfessorsParams, len(professors))
	for i, p := range professors {
		dbProfessors[i] = db.StageProfessorsParams{
			TermCollectionHistoryID: *q.termCollectionHistoryID,
			ID:                      p.ID,
			SchoolID:                q.schoolID,
			Name:                    p.Name,
			EmailAddress:            p.EmailAddress,
			FirstName:               p.FirstName,
			LastName:                p.LastName,
			Other:                   p.Other,
		}
	}
	_, err := q.q.StageProfessors(ctx, dbProfessors)
	if err != nil {
		logger.Error("Error upserting fac ", err)
		return err
	}
	return nil
}

func (q *EntryQueries) StageCourses(ctx context.Context, courses []Course, logger *log.Entry) error {
	if len(courses) == 0 {
		return nil
	}

	dbCourses := make([]db.StageCoursesParams, len(courses))
	for i, c := range courses {
		dbCourses[i] = db.StageCoursesParams{
			TermCollectionHistoryID: *q.termCollectionHistoryID,
			SchoolID:                q.schoolID,
			SubjectCode:             c.SubjectCode,
			Number:                  c.Number,
			SubjectDescription:      c.SubjectDescription,
			Title:                   c.Title,
			Description:             c.Description,
			CreditHours:             c.CreditHours,
			Other:                   c.Other,
		}
	}
	_, err := q.q.StageCourses(ctx, dbCourses)
	if err != nil {
		logger.Error("Error upserting course", err)
		return err
	}
	return nil
}
