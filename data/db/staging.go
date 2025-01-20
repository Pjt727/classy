package db

import (
	"context"
	"fmt"
	"sync"
)

// the main purpose of this staging process in not effiency... It is to have the
//    correct postgress triggers on the sections/ meetings e.i.
//    insert, delete, and updates for records actaully means that said record
//    was inserted deleted or updated
// if those triggers did not matter then we would simply delete all respective meeting / section data
//    and use copy from or batch inserts directly on the table

func (q *Queries) DeleteCoursesMeetingsStaging(ctx context.Context, termCollection TermCollection) error {
	errCh := make(chan error, 2)
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		if err := q.DeleteStagingMeetingTimes(ctx, DeleteStagingMeetingTimesParams{
			TermCollectionID: termCollection.ID,
			SchoolID:         termCollection.SchoolID,
		}); err != nil {
			errCh <- err
		}
	}()
	go func() {
		defer wg.Done()
		if err := q.DeleteStagingSections(ctx, DeleteStagingSectionsParams{
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
func (q *Queries) MoveStagedCoursesAndMeetings(
	ctx context.Context,
	termCollection TermCollection,
) (int, error) {
	err := q.RemoveUnstagedSections(ctx, RemoveUnstagedSectionsParams{
		TermCollectionID: termCollection.ID,
		SchoolID:         termCollection.SchoolID,
	})
	if err != nil {
		return 0, fmt.Errorf("error unstaging sections %v", err)
	}
	err = q.RemoveUnstagedMeetings(ctx, RemoveUnstagedMeetingsParams{
		TermCollectionID: termCollection.ID,
		SchoolID:         termCollection.SchoolID,
	})
	if err != nil {
		return 0, fmt.Errorf("error unstaging meeting %v", err)
	}
	err = q.MoveStagedSections(ctx)
	if err != nil {
		return 0, fmt.Errorf("error staging sections %v", err)
	}
	err = q.MoveStagedMeetingTimes(ctx)
	if err != nil {
		return 0, fmt.Errorf("error staging meetings %v", err)
	}
	return 0, nil
}
