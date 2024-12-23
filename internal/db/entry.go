package entry

import (
	"database/sql"
)

type SeasonEnum string

type FacultyMember struct {
	ID           string         `db:"id"`
	Name         string         `db:"name"`
	EmailAddress sql.NullString `db:"email_address"`
	FirstName    sql.NullString `db:"first_name"`
	LastName     sql.NullString `db:"last_name"`
}

type Course struct {
	ID                 string         `db:"id"`
	SubjectCode        sql.NullString `db:"subject_code"`
	Number             sql.NullString `db:"number"`
	SubjectDescription sql.NullString `db:"subject_description"`
	Title              sql.NullString `db:"title"`
	Description        sql.NullString `db:"description"`
	CreditHours        int            `db:"credit_hours"`
}

type Section struct {
	ID                string         `db:"id"`
	CourseID          string         `db:"course_id"`
	MaxEnrollment     sql.NullInt64  `db:"max_enrollment"`
	InstructionMethod sql.NullString `db:"instruction_method"`
	Campus            sql.NullString `db:"campus"`
	Enrollment        sql.NullInt64  `db:"enrollment"`
	PrimaryFacultyID  sql.NullString `db:"primary_faculty_id"`
}

type MeetingTime struct {
	ID           int64          `db:"id"`
	SectionID    string         `db:"section_id"`
	CourseID     string         `db:"course_id"`
	StartDate    sql.NullTime   `db:"start_date"`
	EndDate      sql.NullTime   `db:"end_date"`
	MeetingType  sql.NullString `db:"meeting_type"`
	StartMinutes sql.NullInt64  `db:"start_minutes"`
	EndMinutes   sql.NullInt64  `db:"end_minutes"`
	IsMonday     bool           `db:"is_monday"`
	IsTuesday    bool           `db:"is_tuesday"`
	IsWednesday  bool           `db:"is_wednesday"`
	IsThursday   bool           `db:"is_thursday"`
	IsFriday     bool           `db:"is_friday"`
	IsSaturday   bool           `db:"is_saturday"`
	IsSunday     bool           `db:"is_sunday"`
}

func addClassesFlat() {
}
