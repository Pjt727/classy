// Code generated by sqlc. DO NOT EDIT.
// versions:
//   sqlc v1.29.0

package db

import (
	"database/sql/driver"
	"fmt"

	"github.com/jackc/pgx/v5/pgtype"
)

type SeasonEnum string

const (
	SeasonEnumSpring SeasonEnum = "Spring"
	SeasonEnumFall   SeasonEnum = "Fall"
	SeasonEnumWinter SeasonEnum = "Winter"
	SeasonEnumSummer SeasonEnum = "Summer"
)

func (e *SeasonEnum) Scan(src interface{}) error {
	switch s := src.(type) {
	case []byte:
		*e = SeasonEnum(s)
	case string:
		*e = SeasonEnum(s)
	default:
		return fmt.Errorf("unsupported scan type for SeasonEnum: %T", src)
	}
	return nil
}

type NullSeasonEnum struct {
	SeasonEnum SeasonEnum `json:"season_enum"`
	Valid      bool       `json:"valid"` // Valid is true if SeasonEnum is not NULL
}

// Scan implements the Scanner interface.
func (ns *NullSeasonEnum) Scan(value interface{}) error {
	if value == nil {
		ns.SeasonEnum, ns.Valid = "", false
		return nil
	}
	ns.Valid = true
	return ns.SeasonEnum.Scan(value)
}

// Value implements the driver Valuer interface.
func (ns NullSeasonEnum) Value() (driver.Value, error) {
	if !ns.Valid {
		return nil, nil
	}
	return string(ns.SeasonEnum), nil
}

type SyncKind string

const (
	SyncKindUpdate SyncKind = "update"
	SyncKindDelete SyncKind = "delete"
	SyncKindInsert SyncKind = "insert"
)

func (e *SyncKind) Scan(src interface{}) error {
	switch s := src.(type) {
	case []byte:
		*e = SyncKind(s)
	case string:
		*e = SyncKind(s)
	default:
		return fmt.Errorf("unsupported scan type for SyncKind: %T", src)
	}
	return nil
}

type NullSyncKind struct {
	SyncKind SyncKind `json:"sync_kind"`
	Valid    bool     `json:"valid"` // Valid is true if SyncKind is not NULL
}

// Scan implements the Scanner interface.
func (ns *NullSyncKind) Scan(value interface{}) error {
	if value == nil {
		ns.SyncKind, ns.Valid = "", false
		return nil
	}
	ns.Valid = true
	return ns.SyncKind.Scan(value)
}

// Value implements the driver Valuer interface.
func (ns NullSyncKind) Value() (driver.Value, error) {
	if !ns.Valid {
		return nil, nil
	}
	return string(ns.SyncKind), nil
}

type TermCollectionStatusEnum string

const (
	TermCollectionStatusEnumActive  TermCollectionStatusEnum = "Active"
	TermCollectionStatusEnumSuccess TermCollectionStatusEnum = "Success"
	TermCollectionStatusEnumFailure TermCollectionStatusEnum = "Failure"
)

func (e *TermCollectionStatusEnum) Scan(src interface{}) error {
	switch s := src.(type) {
	case []byte:
		*e = TermCollectionStatusEnum(s)
	case string:
		*e = TermCollectionStatusEnum(s)
	default:
		return fmt.Errorf("unsupported scan type for TermCollectionStatusEnum: %T", src)
	}
	return nil
}

type NullTermCollectionStatusEnum struct {
	TermCollectionStatusEnum TermCollectionStatusEnum `json:"term_collection_status_enum"`
	Valid                    bool                     `json:"valid"` // Valid is true if TermCollectionStatusEnum is not NULL
}

// Scan implements the Scanner interface.
func (ns *NullTermCollectionStatusEnum) Scan(value interface{}) error {
	if value == nil {
		ns.TermCollectionStatusEnum, ns.Valid = "", false
		return nil
	}
	ns.Valid = true
	return ns.TermCollectionStatusEnum.Scan(value)
}

// Value implements the driver Valuer interface.
func (ns NullTermCollectionStatusEnum) Value() (driver.Value, error) {
	if !ns.Valid {
		return nil, nil
	}
	return string(ns.TermCollectionStatusEnum), nil
}

type Course struct {
	SchoolID           string      `json:"school_id"`
	SubjectCode        string      `json:"subject_code"`
	Number             string      `json:"number"`
	SubjectDescription pgtype.Text `json:"subject_description"`
	Title              pgtype.Text `json:"title"`
	Description        pgtype.Text `json:"description"`
	CreditHours        float32     `json:"credit_hours"`
	Prerequisites      pgtype.Text `json:"prerequisites"`
	Corequisites       pgtype.Text `json:"corequisites"`
	Other              []byte      `json:"other"`
}

type CourseHeuristic struct {
	SubjectCode        string             `json:"subject_code"`
	Number             string             `json:"number"`
	SchoolID           string             `json:"school_id"`
	PreviousProfessors []PartialProfessor `json:"previous_professors"`
	PreviousTerms      []PartialTerm      `json:"previous_terms"`
}

type HistoricClassInformation struct {
	Sequence                int32              `json:"sequence"`
	SchoolID                string             `json:"school_id"`
	TableName               string             `json:"table_name"`
	CompositeHash           string             `json:"composite_hash"`
	InputAt                 pgtype.Timestamptz `json:"input_at"`
	PkFields                []byte             `json:"pk_fields"`
	SyncAction              SyncKind           `json:"sync_action"`
	RelevantFields          []byte             `json:"relevant_fields"`
	TermCollectionHistoryID pgtype.Int4        `json:"term_collection_history_id"`
}

type HistoricClassInformationTermDependency struct {
	TableName             string `json:"table_name"`
	HistoricCompositeHash string `json:"historic_composite_hash"`
	TermCollectionID      string `json:"term_collection_id"`
	SchoolID              string `json:"school_id"`
}

type ManagementUser struct {
	Username          string `json:"username"`
	EncryptedPassword string `json:"encrypted_password"`
}

type MeetingTime struct {
	Sequence         int32            `json:"sequence"`
	SectionSequence  string           `json:"section_sequence"`
	TermCollectionID string           `json:"term_collection_id"`
	SubjectCode      string           `json:"subject_code"`
	CourseNumber     string           `json:"course_number"`
	SchoolID         string           `json:"school_id"`
	StartDate        pgtype.Timestamp `json:"start_date"`
	EndDate          pgtype.Timestamp `json:"end_date"`
	MeetingType      pgtype.Text      `json:"meeting_type"`
	StartMinutes     pgtype.Time      `json:"start_minutes"`
	EndMinutes       pgtype.Time      `json:"end_minutes"`
	IsMonday         bool             `json:"is_monday"`
	IsTuesday        bool             `json:"is_tuesday"`
	IsWednesday      bool             `json:"is_wednesday"`
	IsThursday       bool             `json:"is_thursday"`
	IsFriday         bool             `json:"is_friday"`
	IsSaturday       bool             `json:"is_saturday"`
	IsSunday         bool             `json:"is_sunday"`
	Other            []byte           `json:"other"`
}

type Professor struct {
	ID           string      `json:"id"`
	SchoolID     string      `json:"school_id"`
	Name         string      `json:"name"`
	EmailAddress pgtype.Text `json:"email_address"`
	FirstName    pgtype.Text `json:"first_name"`
	LastName     pgtype.Text `json:"last_name"`
	Other        []byte      `json:"other"`
}

type School struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type Section struct {
	Sequence           string      `json:"sequence"`
	TermCollectionID   string      `json:"term_collection_id"`
	SubjectCode        string      `json:"subject_code"`
	CourseNumber       string      `json:"course_number"`
	SchoolID           string      `json:"school_id"`
	MaxEnrollment      pgtype.Int4 `json:"max_enrollment"`
	InstructionMethod  pgtype.Text `json:"instruction_method"`
	Campus             pgtype.Text `json:"campus"`
	Enrollment         pgtype.Int4 `json:"enrollment"`
	PrimaryProfessorID pgtype.Text `json:"primary_professor_id"`
	Other              []byte      `json:"other"`
}

type SectionMeeting struct {
	Sequence         string               `json:"sequence"`
	TermCollectionID string               `json:"term_collection_id"`
	SubjectCode      string               `json:"subject_code"`
	CourseNumber     string               `json:"course_number"`
	SchoolID         string               `json:"school_id"`
	MeetingTimes     []PartialMeetingTime `json:"meeting_times"`
}

type StagingCourse struct {
	SchoolID                string      `json:"school_id"`
	SubjectCode             string      `json:"subject_code"`
	Number                  string      `json:"number"`
	SubjectDescription      pgtype.Text `json:"subject_description"`
	Title                   pgtype.Text `json:"title"`
	Description             pgtype.Text `json:"description"`
	CreditHours             float32     `json:"credit_hours"`
	Prerequisites           pgtype.Text `json:"prerequisites"`
	Corequisites            pgtype.Text `json:"corequisites"`
	Other                   []byte      `json:"other"`
	TermCollectionHistoryID int32       `json:"term_collection_history_id"`
}

type StagingMeetingTime struct {
	Sequence                int32            `json:"sequence"`
	SectionSequence         string           `json:"section_sequence"`
	TermCollectionID        string           `json:"term_collection_id"`
	SubjectCode             string           `json:"subject_code"`
	CourseNumber            string           `json:"course_number"`
	SchoolID                string           `json:"school_id"`
	StartDate               pgtype.Timestamp `json:"start_date"`
	EndDate                 pgtype.Timestamp `json:"end_date"`
	MeetingType             pgtype.Text      `json:"meeting_type"`
	StartMinutes            pgtype.Time      `json:"start_minutes"`
	EndMinutes              pgtype.Time      `json:"end_minutes"`
	IsMonday                bool             `json:"is_monday"`
	IsTuesday               bool             `json:"is_tuesday"`
	IsWednesday             bool             `json:"is_wednesday"`
	IsThursday              bool             `json:"is_thursday"`
	IsFriday                bool             `json:"is_friday"`
	IsSaturday              bool             `json:"is_saturday"`
	IsSunday                bool             `json:"is_sunday"`
	Other                   []byte           `json:"other"`
	TermCollectionHistoryID int32            `json:"term_collection_history_id"`
}

type StagingProfessor struct {
	ID                      string      `json:"id"`
	SchoolID                string      `json:"school_id"`
	Name                    string      `json:"name"`
	EmailAddress            pgtype.Text `json:"email_address"`
	FirstName               pgtype.Text `json:"first_name"`
	LastName                pgtype.Text `json:"last_name"`
	Other                   []byte      `json:"other"`
	TermCollectionHistoryID int32       `json:"term_collection_history_id"`
}

type StagingSection struct {
	Sequence                string      `json:"sequence"`
	TermCollectionID        string      `json:"term_collection_id"`
	SubjectCode             string      `json:"subject_code"`
	CourseNumber            string      `json:"course_number"`
	SchoolID                string      `json:"school_id"`
	MaxEnrollment           pgtype.Int4 `json:"max_enrollment"`
	InstructionMethod       pgtype.Text `json:"instruction_method"`
	Campus                  pgtype.Text `json:"campus"`
	Enrollment              pgtype.Int4 `json:"enrollment"`
	PrimaryProfessorID      pgtype.Text `json:"primary_professor_id"`
	Other                   []byte      `json:"other"`
	TermCollectionHistoryID int32       `json:"term_collection_history_id"`
}

type Term struct {
	Year   int32      `json:"year"`
	Season SeasonEnum `json:"season"`
}

type TermCollection struct {
	ID              string      `json:"id"`
	SchoolID        string      `json:"school_id"`
	Year            int32       `json:"year"`
	Season          SeasonEnum  `json:"season"`
	Name            pgtype.Text `json:"name"`
	StillCollecting bool        `json:"still_collecting"`
}

type TermCollectionHistory struct {
	ID               int32                    `json:"id"`
	Status           TermCollectionStatusEnum `json:"status"`
	TermCollectionID string                   `json:"term_collection_id"`
	SchoolID         string                   `json:"school_id"`
	StartTime        pgtype.Timestamptz       `json:"start_time"`
	EndTime          pgtype.Timestamptz       `json:"end_time"`
	IsFull           bool                     `json:"is_full"`
}
