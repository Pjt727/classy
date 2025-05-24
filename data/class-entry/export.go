package classentry

import (
	"github.com/Pjt727/classy/data/db"
	"github.com/jackc/pgx/v5/pgtype"
)

type SeasonEnum = db.SeasonEnum

var SeasonEnumSpring = db.SeasonEnumSpring
var SeasonEnumFall = db.SeasonEnumFall
var SeasonEnumWinter = db.SeasonEnumWinter
var SeasonEnumSummer = db.SeasonEnumSummer

type Term = db.Term
type School = db.School

type TermCollection struct {
	ID              string
	Term            Term
	Name            pgtype.Text
	StillCollecting bool
}

type Professor struct {
	ID           string
	Name         string
	EmailAddress pgtype.Text
	FirstName    pgtype.Text
	LastName     pgtype.Text
}

type Course struct {
	Number             string
	SubjectCode        string
	SubjectDescription pgtype.Text
	Title              pgtype.Text
	Description        pgtype.Text
	CreditHours        float32
}

type Section struct {
	Sequence           string
	SubjectCode        string
	CourseNumber       string
	MaxEnrollment      pgtype.Int4
	InstructionMethod  pgtype.Text
	Campus             pgtype.Text
	Enrollment         pgtype.Int4
	PrimaryProfessorID pgtype.Text
}

type MeetingTime struct {
	Sequence        int32
	SectionSequence string
	SubjectCode     string
	CourseNumber    string
	StartDate       pgtype.Timestamp
	EndDate         pgtype.Timestamp
	MeetingType     pgtype.Text
	StartMinutes    pgtype.Time
	EndMinutes      pgtype.Time
	IsMonday        bool
	IsTuesday       bool
	IsWednesday     bool
	IsThursday      bool
	IsFriday        bool
	IsSaturday      bool
	IsSunday        bool
}
