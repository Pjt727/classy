package db

import "github.com/jackc/pgx/v5/pgtype"

type PartialMeetingTime struct {
	StartDate    pgtype.Timestamp `json:"start_date"`
	EndDate      pgtype.Timestamp `json:"end_date"`
	MeetingType  pgtype.Text      `json:"meeting_type"`
	StartMinutes pgtype.Text      `json:"start_minutes"`
	EndMinutes   pgtype.Text      `json:"end_minutes"`
	IsMonday     bool             `json:"is_monday"`
	IsTuesday    bool             `json:"is_tuesday"`
	IsWednesday  bool             `json:"is_wednesday"`
	IsThursday   bool             `json:"is_thursday"`
	IsFriday     bool             `json:"is_friday"`
	IsSaturday   bool             `json:"is_saturday"`
	IsSunday     bool             `json:"is_sunday"`
}
