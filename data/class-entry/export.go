package classentry

import (
	"github.com/Pjt727/classy/data/db"
)

type SeasonEnum = db.SeasonEnum

var SeasonEnumSpring = db.SeasonEnumSpring
var SeasonEnumFall = db.SeasonEnumFall
var SeasonEnumWinter = db.SeasonEnumWinter
var SeasonEnumSummer = db.SeasonEnumSummer

type Term = db.Term
type School = db.School
type TermCollection = db.TermCollection
type Professor = db.Professor
type Course = db.Course
type Section = db.Section
type MeetingTime = db.MeetingTime
type UpsertTermCollectionParams = db.UpsertTermCollectionParams
type StageSectionsParams = db.StageSectionsParams
type StageMeetingTimesParams = db.StageMeetingTimesParams
type UpsertProfessorsParams = db.UpsertProfessorsParams
type UpsertCoursesParams = db.UpsertCoursesParams
