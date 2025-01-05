package services

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
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

type bannerSchool struct {
	school   db.School
	hostname string
}

type banner struct {
	schools map[string]bannerSchool
}

var Banner *banner
var once sync.Once

func init() {
	marist := db.School{ID: "marist", Name: "Marist University"}
	schools := map[string]bannerSchool{
		marist.ID: {school: marist, hostname: "ssb1-reg.banner.marist.edu"},
	}
	Banner = &banner{schools: schools}
}

func (b *banner) ListValidSchools(
	logger log.Entry,
	ctx context.Context,
	q *db.Queries,
) ([]db.School, error) {
	schools := make([]db.School, len(b.schools))
	i := 0
	for _, schoolEntry := range b.schools {
		schools[i] = schoolEntry.school
		i++
	}
	return schools, nil
}

type bannerTerm struct {
	Code        string `json:"code"`
	Description string `json:"description"`
}

func (b *banner) GetName() string { return "Banner" }

func (b *banner) GetTermCollections(
	logger log.Entry,
	ctx context.Context,
	school db.School,
) ([]db.UpsertTermCollectionParams, error) {
	const MAX_TERMS_COUNT = "100"
	var termCollection []db.UpsertTermCollectionParams

	hostname, err := b.getHostname(school)
	if err != nil {
		logger.Error("Error getting school entry: ", err)
		return termCollection, err
	}
	req, err := http.NewRequest(
		"GET",
		"https://"+hostname+"/StudentRegistrationSsb/ssb/classSearch/getTerms?searchTerm=&offset=1&max="+MAX_TERMS_COUNT,
		nil,
	)
	if err != nil {
		logger.Error("Error creating term request: ", err)
		return termCollection, err
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		logger.Error("Error getting term response: ", err)
		return termCollection, err
	}
	defer resp.Body.Close()
	var terms []bannerTerm
	if err := json.NewDecoder(resp.Body).Decode(&terms); err != nil {
		logger.Error("Error decoding terms: ", err)
		return termCollection, err
	}

	for _, term := range terms {
		// example terms:
		// {
		//   "code": "202520",
		//   "description": "Spring 2025"
		// }
		// {
		//   "code": "202510",
		//   "description": "Winter 2025 (View Only)"
		// },
		t, err := inverseTermConversion(term.Code)
		if err != nil {
			logger.Error("Error decoding term code: ", err)
			return termCollection, err
		}
		// the View Only appears on all terms which probably wont update
		stillCollecting := !strings.HasSuffix(term.Description, "(View Only)")

		termCollection = append(termCollection, db.UpsertTermCollectionParams{
			Schoolid:        school.ID,
			Year:            t.Year,
			Season:          t.Season,
			Stillcollecting: stillCollecting,
		})
	}

	return termCollection, nil
}

func (b *banner) StageAllClasses(
	logger log.Entry,
	ctx context.Context,
	q *db.Queries,
	school db.School,
	term db.Term,
) error {
	const MAX_PAGE_SIZE = 200 // the max this value can do is 500 more
	logger.Info("Starting full section")
	termStr := termConversion(term)
	hostname, err := b.getHostname(school)
	if err != nil {
		logger.Error("Error getting school entry: ", err)
		return err
	}

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
		logger.Error("Error getting cookie response: ", err)
		return err
	}
	resp.Body.Close()
	// Extract the JSESSIONID cookie
	// JSESSIONID
	cookie := resp.Cookies()[0]
	// Associate the cookie with a term
	formData := url.Values{
		"term": {termStr},
	}
	req, err = http.NewRequest(
		"POST",
		"https://"+hostname+"/StudentRegistrationSsb/ssb/term/search?mode=search",
		bytes.NewBufferString(formData.Encode()),
	)
	if err != nil {
		logger.Error("Error creating request to set term: ", err)
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
	queryParams := url.Values{
		"txt_term":    {termStr},
		"pageOffset":  {"0"},
		"pageMaxSize": {"1"},
	}
	req.URL.RawQuery = queryParams.Encode()
	// dump, _ = httputil.DumpRequestOut(req, true)
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
	logger.Infof("starting collection on %d sections", count)

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
		queryParams := url.Values{
			"txt_term":    {termStr},
			"pageOffset":  {strconv.Itoa(i * MAX_PAGE_SIZE)},
			"pageMaxSize": {strconv.Itoa(MAX_PAGE_SIZE)},
		}
		workersReq.URL.RawQuery = queryParams.Encode()
		go func(req http.Request) {
			defer wg.Done()
			err := insertGroupOfSections(workerLog, workersReq, ctx, q, school.ID, term)
			if err != nil {
				errCh <- err
			}
		}(*workersReq)
	}

	go func() {
		wg.Wait()
		close(errCh)
	}()

	for err := range errCh {
		logger.Error("There was an error searching sections: ", err)
	}
	if len(errCh) > 0 {
		return errors.New("error searching sections")
	}

	logger.Infof("sections finished getting %d sections", count)
	// TODO change this is something more accurate possibly
	return nil
}

func (b *banner) getHostname(school db.School) (string, error) {
	schoolEntry, ok := b.schools[school.ID]
	if !ok {
		err := errors.New(fmt.Sprint("school not there: ", school))
		return "", err
	}
	hostname := schoolEntry.hostname
	return hostname, nil
}

func termConversion(term db.Term) string {
	var seasonString string
	if term.Season == db.SeasonEnumWinter {
		seasonString = "10"
	} else if term.Season == db.SeasonEnumSpring {
		seasonString = "20"
	} else if term.Season == db.SeasonEnumSummer {
		seasonString = "30"
	} else if term.Season == db.SeasonEnumFall {
		seasonString = "40"
	} else {
		panic("Invalid year / season")
	}
	return strconv.Itoa(int(term.Year)) + seasonString
}

func inverseTermConversion(term string) (db.Term, error) {
	var dbTerm db.Term
	if len(term) != 6 {
		err := errors.New(fmt.Sprint("term not right size must be 6: ", term))
		return dbTerm, err
	}
	year, _ := strconv.Atoi(term[:4])
	strSeason := term[4:]
	var season db.SeasonEnum
	if strSeason == "10" {
		season = db.SeasonEnumWinter
	} else if strSeason == "20" {
		season = db.SeasonEnumSpring
	} else if strSeason == "30" {
		season = db.SeasonEnumSummer
	} else if strSeason == "40" {
		season = db.SeasonEnumFall
	} else {
		err := errors.New(fmt.Sprint("invalid season section of term: ", term))
		return dbTerm, err
	}
	dbTerm = db.Term{Year: int32(year), Season: season}
	return dbTerm, nil
}

// json types for the insertions

type meetingTime struct {
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

type meetingsFaculty struct {
	MeetingTime meetingTime `json:"meetingTime"`
}

type faculty struct {
	DisplayName      string `json:"displayName"`
	EmailAddress     string `json:"emailAddress"`
	PrimaryIndicator bool   `json:"primaryIndicator"`
}

type section struct {
	ID                  int               `json:"id"`
	Term                string            `json:"term"`
	CourseNumber        string            `json:"courseNumber"`
	Subject             string            `json:"subject"`
	SequenceNumber      string            `json:"sequenceNumber"`
	CourseTitle         string            `json:"courseTitle"`
	SeatsAvailable      int32             `json:"seatsAvailable"`
	MaximumEnrollment   int32             `json:"maximumEnrollment"`
	InstructionalMethod string            `json:"instructionalMethod"`
	OpenSection         bool              `json:"openSection"`
	MeetingFaculty      []meetingsFaculty `json:"meetingsFaculty"`
	Faculty             []faculty         `json:"faculty"`
	Credits             uint8             `json:"creditHourLow"`
	SubjectCourse       string            `json:"subjectCourse"`
	SubjectDescription  string            `json:"subjectDescription"`
	CampusDescription   string            `json:"campusDescription"`
}

type sectionSearch struct {
	Sections []section `json:"data"`
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
	ctx context.Context,
	q *db.Queries,
	schoolId string,
	term db.Term,
) error {
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		logger.Error("Error getting sections")
		return err
	}

	var sections sectionSearch
	if err := json.NewDecoder(resp.Body).Decode(&sections); err != nil {
		logger.Error("Error decoding sections: ", err)
		return err
	}

	var dbSections []db.StageSectionsParams
	var meetingTimes []db.StageMeetingTimesParams
	facultyMembers := make(map[string]db.UpsertFacultyParams)
	courses := make(map[string]db.UpsertCoursesParams)
	for _, s := range sections.Sections {
		primaryFac := pgtype.Text{String: "", Valid: false}
		// add all fac regardless of whether they are the main teacher
		for _, fac := range s.Faculty {
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

			facultyMember := db.UpsertFacultyParams{
				ID:           facID,
				Schoolid:     schoolId,
				Name:         fac.DisplayName,
				Emailaddress: pgtype.Text{String: fac.EmailAddress, Valid: true},
				Firstname:    firstName,
				Lastname:     lastName,
			}
			facultyMembers[facID] = facultyMember
		}
		courseId := s.Subject + "," + s.CourseNumber
		sectionId := courseId + "," + s.SequenceNumber
		for i, meeting := range s.MeetingFaculty {
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
			dbMeetingTime := db.StageMeetingTimesParams{
				Sequence:     int32(i),
				Sectionid:    sectionId,
				Termseason:   term.Season,
				Termyear:     term.Year,
				Courseid:     courseId,
				Schoolid:     schoolId,
				Startdate:    startDate,
				Enddate:      endDate,
				Meetingtype:  pgtype.Text{String: meetingTime.MeetingType, Valid: false},
				Startminutes: toBannerTime(meetingTime.StartTime),
				Endminutes:   toBannerTime(meetingTime.EndTime),
				Ismonday:     meetingTime.Monday,
				Istuesday:    meetingTime.Tuesday,
				Iswednesday:  meetingTime.Wednesday,
				Isthursday:   meetingTime.Thursday,
				Isfriday:     meetingTime.Friday,
				Issaturday:   meetingTime.Saturday,
				Issunday:     meetingTime.Sunday,
			}
			meetingTimes = append(meetingTimes, dbMeetingTime)

		}
		dbSection := db.StageSectionsParams{
			ID:                sectionId,
			Termseason:        term.Season,
			Termyear:          term.Year,
			CourseID:          courseId,
			Schoolid:          schoolId,
			Maxenrollment:     pgtype.Int4{Int32: s.MaximumEnrollment, Valid: true},
			Instructionmethod: pgtype.Text{String: s.InstructionalMethod, Valid: true},
			Campus:            pgtype.Text{String: s.CampusDescription, Valid: true},
			Enrollment:        pgtype.Int4{Int32: s.MaximumEnrollment, Valid: true},
			Primaryfacultyid:  primaryFac,
		}
		course := db.UpsertCoursesParams{
			ID:                 courseId,
			Schoolid:           schoolId,
			Subjectcode:        pgtype.Text{String: s.Subject, Valid: true},
			Number:             pgtype.Text{String: s.CourseNumber, Valid: true},
			Subjectdescription: pgtype.Text{String: s.SubjectDescription, Valid: true},
			Title:              pgtype.Text{String: s.CourseTitle, Valid: true},
			// cannot get the description from here 😥
			Description: pgtype.Text{String: "", Valid: false},
			Credithours: 0,
		}
		courses[courseId] = course
		dbSections = append(dbSections, dbSection)
	}

	// insert all of the meetings

	_, err = q.StageMeetingTimes(ctx, meetingTimes)
	if err != nil {
		logger.Error("Staging meetings error ", err)
		return err
	}

	_, err = q.StageSections(ctx, dbSections)
	if err != nil {
		logger.Error("Staging sections error ", err)
		return err
	}

	batchFacultyMembers := make([]db.UpsertFacultyParams, len(facultyMembers))
	i := 0
	for _, facMem := range facultyMembers {
		batchFacultyMembers[i] = facMem
		i += 1
	}
	buf := q.UpsertFaculty(ctx, batchFacultyMembers)

	var outerErr error = nil
	buf.Exec(func(i int, err error) {
		if err != nil {
			outerErr = err
		}
	})

	if outerErr != nil {
		logger.Error("Error upserting fac ", outerErr)
		return err
	}

	batchCourses := make([]db.UpsertCoursesParams, len(courses))
	i = 0
	for _, course := range courses {
		batchCourses[i] = course
		i += 1
	}

	bc := q.UpsertCourses(ctx, batchCourses)
	bc.Exec(func(i int, err error) {
		if err != nil {
			outerErr = err
		}
	})

	if outerErr != nil {
		logger.Error("Error upserting course", outerErr)
		return outerErr
	}

	return nil
}
