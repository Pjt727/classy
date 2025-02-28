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

func NewEntryQuery(database db.DBTX) *EntryQueries {
	return &EntryQueries{q: db.New(database)}
}

type EntryQueries struct {
	q *db.Queries
}

func (q *EntryQueries) WithTx(tx pgx.Tx) *EntryQueries {
	return &EntryQueries{
		q: q.q.WithTx(tx),
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
			SchoolID:         termCollection.SchoolID,
		}); err != nil {
			errCh <- err
		}
	}()
	go func() {
		defer wg.Done()
		if err := q.q.DeleteStagingSections(ctx, db.DeleteStagingSectionsParams{
			TermCollectionID: termCollection.ID,
			SchoolID:         termCollection.SchoolID,
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
		SchoolID:         termCollection.SchoolID,
	})
	if err != nil {
		return 0, fmt.Errorf("error unstaging sections %v", err)
	}
	err = q.q.RemoveUnstagedMeetings(ctx, db.RemoveUnstagedMeetingsParams{
		TermCollectionID: termCollection.ID,
		SchoolID:         termCollection.SchoolID,
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
	meetingTimes []StageMeetingTimesParams,
	dbSections []StageSectionsParams,
	professors []UpsertProfessorsParams,
	courses []UpsertCoursesParams,
) error {
	if len(meetingTimes) != 0 {
		_, err := q.q.StageMeetingTimes(ctx, meetingTimes)
		if err != nil {
			logger.Error("Staging meetings error ", err)
			return err
		}
	}

	if len(dbSections) != 0 {
		_, err := q.q.StageSections(ctx, dbSections)
		if err != nil {
			logger.Error("Staging sections error ", err)
			return err
		}
	}

	if len(professors) != 0 {
		buf := q.q.UpsertProfessors(ctx, []db.UpsertProfessorsParams(professors))

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
		bc := q.q.UpsertCourses(ctx, courses)
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
