package db

import (
	"context"
	"sync"

	log "github.com/sirupsen/logrus"
)

// the main purpose of this staging process in not effiency... It is to have the
//    correct postgress triggers on the sections/ meetings e.i.
//    insert, delete, and updates for records actaully means that said record
//    was inserted deleted or updated
// if those triggers did not matter then we would simply delete all respective meeting / section data
//    and use copy from or batch inserts directly on the table

// the table should already be empty this is a double check
func (q *Queries) ReadyCoursesMeetingsStaging(ctx context.Context) error {
	errCh := make(chan error, 2)
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		if err := q.TruncateStagingMeetingTimes(ctx); err != nil {
			errCh <- err
		}
	}()
	go func() {
		defer wg.Done()
		if err := q.TruncateStagingSections(ctx); err != nil {
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

func (q *Queries) CleanupCoursesMeetingsStaging(ctx context.Context) error {
	// for now they are the same thing
	return q.ReadyCoursesMeetingsStaging(ctx)
}

// moves all staged
func (q *Queries) MoveStagedCoursesAndMeetings(
	ctx context.Context,
	schoolId string,
	term Term,
) (int, error) {
	err := q.RemoveUnstagedSections(ctx, RemoveUnstagedSectionsParams{
		Termseason: term.Season,
		Termyear:   term.Year,
		SchoolID:   schoolId,
	})
	if err != nil {
		log.Trace("remove sections error propagating: ", err)
		return 0, err
	}
	err = q.RemoveUnstagedMeetings(ctx, RemoveUnstagedMeetingsParams{
		Termseason: term.Season,
		Termyear:   term.Year,
		SchoolID:   schoolId,
	})
	if err != nil {
		log.Trace("remove meeting error propagating: ", err)
		return 0, err
	}
	err = q.MoveStagedSections(ctx)
	if err != nil {
		log.Trace("move sections error propagating: ", err)
		return 0, err
	}
	err = q.MoveStagedMeetingTimes(ctx)
	if err != nil {
		log.Trace("move meeting times error propagating: ", err)
		return 0, err
	}
	return 0, nil
}
