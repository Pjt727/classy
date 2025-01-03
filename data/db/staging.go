package db

import (
	"context"
)

func (q *Queries) ReadyCoursesMeetingsStaging(ctx context.Context) error {
	err := q.TruncateStagingMeetingTimes(ctx)
	if err != nil {
		return err
	}
	err = q.TruncateStagingSections(ctx)
	if err != nil {
		return err
	}
	return nil
}

func (q *Queries) CleanupCoursesMeetingsStaging(ctx context.Context) error {
	err := q.TruncateStagingMeetingTimes(ctx)
	if err != nil {
		return err
	}
	err = q.TruncateStagingSections(ctx)
	if err != nil {
		return err
	}
	return nil
}

func (q *Queries) MoveStagedCoursesAndMeetings(ctx context.Context) (int, error) {
	return 0, nil
}
