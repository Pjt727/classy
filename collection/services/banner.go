package services

import (
	"bytes"
	"encoding/json"
	"errors"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Pjt727/classy/data/db"
	"github.com/jackc/pgx/v5/pgtype"
	log "github.com/sirupsen/logrus"
)

func termConversion(year int, season db.SeasonEnum) string {
	var seasonString string
	if season == db.SeasonEnumWinter {
		seasonString = "10"
	} else if season == db.SeasonEnumSpring {
		seasonString = "20"
	} else if season == db.SeasonEnumSummer {
		seasonString = "30"
	} else if season == db.SeasonEnumFall {
		seasonString = "40"
	} else {
		panic("Invalid year / season")
	}
	return strconv.Itoa(year) + seasonString
}

func getSections(
	logger *log.Entry,
	schoolId string,
	hostname string,
	year int,
	season db.SeasonEnum,
) error {
	const MAX_PAGE_SIZE = 200 // the max this value can do is 500 more
	logger.Info("Starting full section")
	term := termConversion(year, season)
	// Get a banner cookie
	req, err := http.NewRequest(
		"GET",
		"https://"+hostname+"/StudentRegistrationSsb/ssb/term/termSelection?mode=search",
		nil,
	)
	if err != nil {
		logger.Error("Error creating cookie request: ", err)
		return err
	}
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		logger.Error("Error getting cookie request: ", err)
		return err
	}
	resp.Body.Close()
	// Extract the JSESSIONID cookie
	cookie := resp.Cookies()[0]
	// Associate the cookie with a term
	formData := url.Values{
		"term":            {term},
		"studyPath":       {""},
		"studyPathText":   {""},
		"startDatepicker": {""},
		"endDatepicker":   {""},
	}
	req, err = http.NewRequest(
		"POST",
		"https://"+hostname+"/StudentRegistrationSsb/ssb/term/search?mode=search",
		bytes.NewBufferString(formData.Encode()),
	)
	if err != nil {
		logger.Error("Error making request to set term: ", err)
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	req.AddCookie(cookie)
	resp, err = client.Do(req)
	if err != nil {
		logger.Error("Error setting term: ", err)
		return err
	}
	resp.Body.Close()
	// Make a request to get sections
	req, err = http.NewRequest(
		"GET",
		"https://"+hostname+"/StudentRegistrationSsb/ssb/searchResults/searchResults",
		nil,
	)
	if err != nil {
		log.Error("Error creating request:", err)
		return err
	}
	req.AddCookie(cookie)
	req.URL.Query().Set("txt_term", term)
	req.URL.Query().Set("pageOffSet", "0")
	// get amount of sections for the term
	req.URL.Query().Set("pageMaxSize", "1")
	resp, err = client.Do(req)
	if err != nil {
		log.Println("Error requesting first sections: ", err)
		return err
	}
	defer resp.Body.Close()
	type Sectioncount struct {
		Count int `json:"totalCount"`
	}
	var sectionCount Sectioncount
	if err := json.NewDecoder(resp.Body).Decode(&sectionCount); err != nil {
		logger.Error("Error decoding first sections: ", err)
		return err
	}
	count := sectionCount.Count
	logger.Info("starting collection on", count, "sections")

	var wg sync.WaitGroup
	errCh := make(chan error)
	numberOfWorkers := math.Ceil(float64(count) / MAX_PAGE_SIZE)
	wg.Add(int(numberOfWorkers))
	for i := 0; i < int(numberOfWorkers); i++ {
		workersReq := req.Clone(req.Context())
		workerLog := logger.WithFields(log.Fields{
			"pageOffSet":  strconv.Itoa(i * MAX_PAGE_SIZE),
			"pageMaxSize": strconv.Itoa(MAX_PAGE_SIZE),
		})
		workersReq.URL.Query().Set("pageOffSet", strconv.Itoa(i*MAX_PAGE_SIZE))
		workersReq.URL.Query().Set("pageMaxSize", strconv.Itoa(MAX_PAGE_SIZE))
		go func(req http.Request) {
			defer wg.Done()
			err := insertGroupOfSections(workerLog, workersReq, schoolId, season, year)
			if err != nil {
				errCh <- err
			}
		}(*workersReq)
	}
	wg.Wait()

	for err := range errCh {
		logger.Error("There was an error searching sections: ", err)
	}
	if len(errCh) > 0 {
		return errors.New("error searching sections")
	}

	logger.Info(count, "sections finished")
	return nil
}

// json types for the insertions

type MeetingTime struct {
	Monday      bool        `json:"monday"`
	Tuesday     bool        `json:"tuesday"`
	Wednesday   bool        `json:"wednesday"`
	Thursday    bool        `json:"thursday"`
	Friday      bool        `json:"friday"`
	Saturday    bool        `json:"saturday"`
	Sunday      bool        `json:"sunday"`
	Campus      pgtype.Text `json:"campus"`
	EndTime     pgtype.Text `json:"end_time"`
	StartTime   pgtype.Text `json:"start_time"`
	MeetingType string      `json:"meeting_type"`
	StartDate   string      `json:"startDate"`
	EndDate     string      `json:"endDate"`
}

type MeetingsFaculty struct {
	MeetingTime MeetingTime `json:"meetingTime"`
}

type Faculty struct {
	DisplayName      string `json:"displayName"`
	EmailAddress     string `json:"emailAddress"`
	PrimaryIndicator bool   `json:"primaryIndicator"`
}

type Section struct {
	ID                  string            `json:"id"`
	Term                string            `json:"term"`
	CourseNumber        string            `json:"courseNumber"`
	Subject             string            `json:"subject"`
	SequenceNumber      string            `json:"sequenceNumber"`
	CourseTitle         string            `json:"courseTitle"`
	SeatsAvailable      int32             `json:"seatsAvailable"`
	MaximumEnrollment   int32             `json:"maximumEnrollment"`
	InstructionalMethod string            `json:"instructionalMethod"`
	OpenSection         bool              `json:"openSection"`
	MeetingFaculty      []MeetingsFaculty `json:"meetingsFaculty"`
	Faculty             []Faculty         `json:"faculty"`
	Credits             uint8             `json:"creditHourLow"`
	SubjectCourse       string            `json:"subjectCourse"`
	SubjectDescription  string            `json:"subjectDescription"`
	CampusDescription   string            `json:"campusDescription"`
}

type SectionSearch struct {
	Sections []Section `json:"data"`
}

func toBannerTime(dbTime pgtype.Text) pgtype.Time {
	pgTime := pgtype.Time{}
	if !dbTime.Valid {
		return pgTime
	}
	time := dbTime.String
	if len(time) != 4 {
		return pgTime
	}
	hoursStr, minutesStr := time[:2], time[2:]
	hours, err := strconv.Atoi(hoursStr)
	if err != nil {
		return pgTime
	}
	minutes, err := strconv.Atoi(minutesStr)
	if err != nil {
		return pgTime
	}
	const hourToMicro int64 = 3_600_000_000
	const minuteToMicro int64 = 60_000_000
	pgTime.Microseconds = int64(hours)*hourToMicro + int64(minutes)*minuteToMicro
	pgTime.Valid = true

	return pgTime
}

func insertGroupOfSections(
	logger *log.Entry,
	req *http.Request,
	schoolId string,
	season db.SeasonEnum,
	year int,
) error {
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		logger.Error("Error getting sections")
		return err
	}

	var sections SectionSearch
	if err := json.NewDecoder(resp.Body).Decode(&sections); err != nil {
		logger.Error("Error decoding sections: ", err)
		return err
	}

	var dbSections []db.Section
	var meetingTimes []db.MeetingTime
	facultyMembers := make(map[string]db.FacultyMember)
	courses := make(map[string]db.Course)
	for _, section := range sections.Sections {
		primaryFac := pgtype.Text{String: "", Valid: false}
		// add all fac regardless of whether they are the main teacher
		for _, fac := range section.Faculty {
			if fac.EmailAddress == "" {
				// professors without emails do not deserve to be put in the database
				continue
			}
			// using emails as PK's is not ideal 😥
			facID := fac.EmailAddress
			if fac.PrimaryIndicator {
				primaryFac.String = facID
				primaryFac.Valid = true
			}
			// ex data
			// "displayName": "Friedman, Carol",
			// "emailAddress": "Carol.Friedman@marist.edu",
			splitName := strings.Split(fac.DisplayName, ", ")
			firstName := pgtype.Text{String: "", Valid: false}
			lastName := pgtype.Text{String: "", Valid: false}
			if len(splitName) == 2 {
				firstName.String = splitName[0]
				firstName.Valid = true
				lastName.String = splitName[1]
				lastName.Valid = true
			}

			facultyMember := db.FacultyMember{
				ID:           facID,
				SchoolID:     schoolId,
				Name:         fac.DisplayName,
				EmailAddress: pgtype.Text{String: fac.EmailAddress, Valid: true},
				FirstName:    firstName,
				LastName:     lastName,
			}
			facultyMembers[facID] = facultyMember
		}
		courseId := section.Subject + "," + section.CourseNumber
		sectionId := courseId + "," + section.SequenceNumber
		for _, meeting := range section.MeetingFaculty {
			meetingTime := meeting.MeetingTime
			// ex format:
			// "meetingTime": {
			//   ...
			//   "beginTime": "0930",
			//   "endTime": "1045",
			//   "startDate": "01/22/2025",
			//   "endDate": "05/16/2025",
			//   ...
			// },
			const dateLayout = "01/02/2006"
			startDate := pgtype.Timestamp{}
			endDate := pgtype.Timestamp{}
			startDateTime, err1 := time.Parse(dateLayout, meetingTime.StartDate)
			endDateTime, err2 := time.Parse(dateLayout, meetingTime.EndDate)
			if err1 == nil && err2 == nil {
				startDate.Time = startDateTime
				startDate.Valid = true
				endDate.Time = endDateTime
				endDate.Valid = true
			}
			dbMeetingTime := db.MeetingTime{
				SectionID:    sectionId,
				TermSeason:   season,
				TermYear:     int32(year),
				CourseID:     courseId,
				SchoolID:     schoolId,
				StartDate:    startDate,
				EndDate:      endDate,
				MeetingType:  pgtype.Text{String: meetingTime.MeetingType, Valid: false},
				StartMinutes: toBannerTime(meetingTime.StartTime),
				EndMinutes:   toBannerTime(meetingTime.EndTime),
				IsMonday:     meetingTime.Monday,
				IsTuesday:    meetingTime.Tuesday,
				IsWednesday:  meetingTime.Wednesday,
				IsThursday:   meetingTime.Thursday,
				IsFriday:     meetingTime.Friday,
				IsSaturday:   meetingTime.Saturday,
				IsSunday:     meetingTime.Sunday,
			}
			meetingTimes = append(meetingTimes, dbMeetingTime)

		}
		dbSection := db.Section{
			ID:                sectionId,
			TermSeason:        season,
			TermYear:          int32(year),
			CourseID:          courseId,
			SchoolID:          schoolId,
			MaxEnrollment:     pgtype.Int4{Int32: section.MaximumEnrollment, Valid: true},
			InstructionMethod: pgtype.Text{String: section.InstructionalMethod, Valid: true},
			Campus:            pgtype.Text{String: section.CampusDescription, Valid: true},
			Enrollment:        pgtype.Int4{Int32: section.MaximumEnrollment, Valid: true},
			PrimaryFacultyID:  primaryFac,
		}
		course := db.Course{
			ID:                 courseId,
			SchoolID:           schoolId,
			SubjectCode:        pgtype.Text{String: section.Subject, Valid: true},
			Number:             pgtype.Text{String: section.CourseNumber, Valid: true},
			SubjectDescription: pgtype.Text{String: section.SubjectDescription, Valid: true},
			Title:              pgtype.Text{String: section.CourseTitle, Valid: true},
			// cannot get the description from here 😥
			Description: pgtype.Text{String: "", Valid: false},
			CreditHours: 0,
		}
		courses[courseId] = course
		dbSections = append(dbSections, dbSection)
	}

	return nil
}
