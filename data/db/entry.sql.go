// Code generated by sqlc. DO NOT EDIT.
// versions:
//   sqlc v1.27.0
// source: entry.sql

package db

import (
	"context"

	"github.com/jackc/pgx/v5/pgtype"
)

const listCourses = `-- name: ListCourses :many
SELECT id, school_id, subject_code, number, subject_description, title, description, credit_hours FROM courses
`

func (q *Queries) ListCourses(ctx context.Context) ([]Course, error) {
	rows, err := q.db.Query(ctx, listCourses)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []Course
	for rows.Next() {
		var i Course
		if err := rows.Scan(
			&i.ID,
			&i.SchoolID,
			&i.SubjectCode,
			&i.Number,
			&i.SubjectDescription,
			&i.Title,
			&i.Description,
			&i.CreditHours,
		); err != nil {
			return nil, err
		}
		items = append(items, i)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

type UpsertSectionsParams struct {
	ID                string
	Campus            pgtype.Text
	CourseID          string
	SchoolID          string
	TermYear          int32
	TermSeason        SeasonEnum
	Enrollment        pgtype.Int4
	MaxEnrollment     pgtype.Int4
	InstructionMethod pgtype.Text
	PrimaryFacultyID  pgtype.Text
}