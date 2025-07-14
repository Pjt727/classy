package banner

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/net/html"
	"golang.org/x/time/rate"

	classentry "github.com/Pjt727/classy/data/class-entry"
	"github.com/jackc/pgx/v5/pgtype"
	log "github.com/sirupsen/logrus"
)

type bannerSchool struct {
	school              classentry.School
	hostname            string
	MaxTermCount        int
	MaxSectionPageCount int
	RequestLimiter      *rate.Limiter
}

type banner struct {
	schools map[string]bannerSchool
}

var Service *banner
var once sync.Once

func init() {
	marist := classentry.School{ID: "marist", Name: "Marist University"}
	temple := classentry.School{ID: "temple", Name: "Temple University"}
	schools := map[string]bannerSchool{
		marist.ID: {
			school:              marist,
			hostname:            "ssb1-reg.banner.marist.edu",
			MaxTermCount:        100,
			MaxSectionPageCount: 200,
			RequestLimiter:      rate.NewLimiter(rate.Limit(100), 25),
		},
		temple.ID: {
			school:              temple,
			hostname:            "prd-xereg.temple.edu",
			MaxTermCount:        100,
			MaxSectionPageCount: 200,
			RequestLimiter:      rate.NewLimiter(rate.Limit(100), 10),
		},
	}
	Service = &banner{schools: schools}
}

func (b *banner) ListValidSchools(
	logger log.Entry,
	ctx context.Context,
) ([]classentry.School, error) {
	schools := make([]classentry.School, len(b.schools))
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
	school classentry.School,
) ([]classentry.TermCollection, error) {
	var termCollection []classentry.TermCollection

	bannerschool, err := b.getBannerSchool(school.ID)
	if err != nil {
		logger.Error("Error getting school entry: ", err)
		return termCollection, err
	}
	termCollections, err := bannerschool.getTerms(logger)
	if err != nil {
		return termCollection, err
	}

	return termCollections, nil
}

func (b *banner) StageAllClasses(
	logger log.Entry,
	ctx context.Context,
	q *classentry.EntryQueries,
	schoolID string,
	termCollection classentry.TermCollection,
	fullCollection bool,
) error {
	logger.Info("Starting full section")
	bannerSchool, err := b.getBannerSchool(schoolID)
	if err != nil {
		logger.Error("Error getting school entry: ", err)
		return err
	}
	bannerSchool.stageAllClasses(
		logger,
		ctx,
		q,
		termCollection,
		fullCollection,
	)
	return nil
}

func (b *banner) getBannerSchool(schoolID string) (*bannerSchool, error) {
	schoolEntry, ok := b.schools[schoolID]
	if !ok {
		err := errors.New(fmt.Sprint("school not there: ", schoolID))
		return nil, err
	}
	return &schoolEntry, nil
}

func termConversion(termEntry bannerTerm) (classentry.Term, error) {
	desc := strings.ToLower(termEntry.Description)
	var dbTerm classentry.Term
	if len(termEntry.Code) != 6 {
		err := errors.New(fmt.Sprint("term not right size must be 6: ", termEntry.Code))
		return dbTerm, err
	}
	year, _ := strconv.Atoi(termEntry.Code[:4])
	var season classentry.SeasonEnum
	if strings.Contains(desc, "winter") {
		season = classentry.SeasonEnumWinter
	} else if strings.Contains(desc, "spring") {
		season = classentry.SeasonEnumSpring
	} else if strings.Contains(desc, "summer") {
		season = classentry.SeasonEnumSummer
	} else if strings.Contains(desc, "fall") {
		season = classentry.SeasonEnumFall
	} else {
		err := errors.New(fmt.Sprint("invalid season section of term: ", termEntry.Code))
		return dbTerm, err
	}
	dbTerm = classentry.Term{Year: int32(year), Season: season}
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
	EndTime     pgtype.Text `json:"endTime"`
	StartTime   pgtype.Text `json:"beginTime"`
	MeetingType string      `json:"meetingType"`
	StartDate   string      `json:"startDate"`
	EndDate     string      `json:"endDate"`
}

type meetingsFaculty struct {
	MeetingTime meetingTime `json:"meetingTime"`
}

type professor struct {
	DisplayName      string `json:"displayName"`
	EmailAddress     string `json:"emailAddress"`
	PrimaryIndicator bool   `json:"primaryIndicator"`
}

type section struct {
	ID                    int               `json:"id"`
	Term                  string            `json:"term"`
	CourseReferenceNumber string            `json:"courseReferenceNumber"`
	CourseNumber          string            `json:"courseNumber"`
	Subject               string            `json:"subject"`
	SequenceNumber        string            `json:"sequenceNumber"`
	CourseTitle           string            `json:"courseTitle"`
	SeatsAvailable        int32             `json:"seatsAvailable"`
	MaximumEnrollment     int32             `json:"maximumEnrollment"`
	Enrollment            int32             `json:"enrollment"`
	InstructionalMethod   string            `json:"instructionalMethod"`
	OpenSection           bool              `json:"openSection"`
	MeetingFaculty        []meetingsFaculty `json:"meetingsFaculty"`
	Faculty               []professor       `json:"faculty"`
	Credits               float32           `json:"creditHourLow"`
	SubjectCourse         string            `json:"subjectCourse"`
	SubjectDescription    string            `json:"subjectDescription"`
	CampusDescription     string            `json:"campusDescription"`
}

type SectionSearch struct {
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

func (b *bannerSchool) getTerms(
	logger log.Entry,
) ([]classentry.TermCollection, error) {
	const MAX_TERMS_COUNT = "100"
	var termCollection []classentry.TermCollection
	hostname := b.hostname
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
		logger.Trace(resp.Body)
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
		t, err := termConversion(term)
		if err != nil {
			logger.Error("Error decoding term code: ", err)
			return termCollection, err
		}
		// the View Only appears on all terms which probably wont update
		stillCollecting := !strings.HasSuffix(term.Description, "(View Only)")

		termCollection = append(termCollection, classentry.TermCollection{
			ID: term.Code,
			Term: classentry.Term{
				Year:   t.Year,
				Season: t.Season,
			},
			Name: pgtype.Text{
				String: term.Description,
				Valid:  true,
			},
			StillCollecting: stillCollecting,
		})
	}

	return termCollection, nil

}
func (b *bannerSchool) stageAllClasses(
	logger log.Entry,
	ctx context.Context,
	q *classentry.EntryQueries,
	termCollection classentry.TermCollection,
	fullCollection bool,
) error {
	termStr := termCollection.ID
	// Get banner cookie(s)
	req, err := http.NewRequest(
		"GET",
		"https://"+b.hostname+"/StudentRegistrationSsb/ssb/term/termSelection?mode=search",
		nil,
	)
	if err != nil {
		logger.Error("Error creating cookie request: ", err)
		return err
	}
	jar, _ := cookiejar.New(nil)
	client := http.Client{Jar: jar}
	resp, err := client.Do(req)
	if err != nil {
		logger.Error("Error getting cookie response: ", err)
		return err
	}
	resp.Body.Close()
	// Associate the cookie with a term
	formData := url.Values{
		"term": {termStr},
	}
	req, err = http.NewRequest(
		"POST",
		"https://"+b.hostname+"/StudentRegistrationSsb/ssb/term/search?mode=search",
		bytes.NewBufferString(formData.Encode()),
	)
	if err != nil {
		logger.Error("Error creating request to set term: ", err)
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	resp, err = client.Do(req)
	if err != nil {
		logger.Error("Error setting term: ", err)
		return err
	}
	resp.Body.Close()

	// Make a request to get sections
	req, err = http.NewRequest(
		"GET",
		"https://"+b.hostname+"/StudentRegistrationSsb/ssb/searchResults/searchResults",
		nil,
	)
	if err != nil {
		log.Error("Error creating request:", err)
		return err
	}
	queryParams := url.Values{
		"txt_term":    {termStr},
		"pageOffset":  {"0"},
		"pageMaxSize": {"1"},
	}
	req.URL.RawQuery = queryParams.Encode()
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
	numberOfWorkers := math.Ceil(float64(count) / float64(b.MaxSectionPageCount))
	wg.Add(int(numberOfWorkers))
	for i := 0; i < int(numberOfWorkers); i++ {
		workersReq := req.Clone(req.Context())
		workerLog := logger.WithFields(log.Fields{
			"pageOffSet":  strconv.Itoa(i * b.MaxSectionPageCount),
			"pageMaxSize": strconv.Itoa(b.MaxSectionPageCount),
		})
		queryParams := url.Values{
			"txt_term":    {termStr},
			"pageOffset":  {strconv.Itoa(i * b.MaxSectionPageCount)},
			"pageMaxSize": {strconv.Itoa(b.MaxSectionPageCount)},
		}
		workersReq.URL.RawQuery = queryParams.Encode()
		go func(req http.Request) {
			defer wg.Done()
			err := b.insertGroupOfSections(
				workerLog,
				workersReq,
				ctx,
				q,
				&client,
				termCollection,
				fullCollection,
			)
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

func dumpRequestInfo(req *http.Request) {
	// Dump request method, URL, and headers
	fmt.Printf("Method: %s\n", req.Method)
	fmt.Printf("URL: %s\n", req.URL.String())
	fmt.Println("Headers:")
	for key, values := range req.Header {
		for _, value := range values {
			fmt.Printf("%s: %s\n", key, value)
		}
	}
}

func (b *bannerSchool) insertGroupOfSections(
	logger *log.Entry,
	sectionReq *http.Request,
	ctx context.Context,
	q *classentry.EntryQueries,
	client *http.Client,
	termCollection classentry.TermCollection,
	fullCollection bool,
) error {
	if err := b.RequestLimiter.Wait(context.Background()); err != nil {
		logger.Error("Limiter error:", err)
		return err
	}
	resp, err := client.Do(sectionReq)
	if err != nil {
		logger.Error("Error getting sections")
		return err
	}
	defer resp.Body.Close()

	var sections SectionSearch
	if err := json.NewDecoder(resp.Body).Decode(&sections); err != nil {
		logger.Error("Error decoding sections: ", err)
		return err
	}
	classData := ProcessSectionSearch(sections)

	// add all of the extra course data
	if fullCollection {
		var wg sync.WaitGroup
		var mu sync.Mutex
		wg.Add(len(classData.CourseReferenceNumbers))
		for courseId, referenceNumber := range classData.CourseReferenceNumbers {
			go func() {
				defer wg.Done()
				courseDesc, err := b.getCourseDetails(
					logger,
					client,
					termCollection,
					referenceNumber,
				)
				if err != nil {
					return
				}
				if courseDesc == nil {
					return
				}
				mu.Lock()
				defer mu.Unlock()
				course := classData.Courses[courseId]
				course.Description = pgtype.Text{
					String: strings.TrimSpace(strings.TrimSpace(*courseDesc)),
					Valid:  true,
				}
				classData.Courses[courseId] = course

			}()
		}

		wg.Wait()
	}

	professors := make([]classentry.Professor, len(classData.Professors))
	i := 0
	for _, professor := range classData.Professors {
		professors[i] = professor
		i += 1
	}

	courses := make([]classentry.Course, len(classData.Courses))
	i = 0
	for _, course := range classData.Courses {
		courses[i] = course
		i += 1
	}

	err = q.InsertClassData(
		logger,
		ctx,
		classData.MeetingTimes,
		classData.Sections,
		professors,
		courses,
	)

	if err != nil {
		logger.Error("Error inserting class data", err)
		return err
	}

	logger.Infof(
		"Successfully added %d sections and their related information",
		len(classData.Sections),
	)

	return nil
}

type ClassData struct {
	Sections               []classentry.Section
	MeetingTimes           []classentry.MeetingTime
	Professors             map[string]classentry.Professor
	Courses                map[string]classentry.Course
	CourseReferenceNumbers map[string]string
}

func ProcessSectionSearch(sectionData SectionSearch) ClassData {
	var sections []classentry.Section
	var meetingTimes []classentry.MeetingTime
	professors := make(map[string]classentry.Professor)
	courses := make(map[string]classentry.Course)
	courseReferenceNumbers := make(map[string]string)
	for _, s := range sectionData.Sections {
		primaryProf := pgtype.Text{String: "", Valid: false}
		// add all fac regardless of whether they are the main teacher
		for _, prof := range s.Faculty {
			if prof.EmailAddress == "" {
				// professors without emails do not deserve to be put in the database
				continue
			}
			// using emails as PK's is not ideal 😥
			professorID := prof.EmailAddress
			if prof.PrimaryIndicator {
				primaryProf.String = professorID
				primaryProf.Valid = true
			}
			// ex data
			// "displayName": "Friedman, Carol",
			// "emailAddress": "Carol.Friedman@marist.edu",
			splitName := strings.Split(prof.DisplayName, ", ")
			firstName := pgtype.Text{String: "", Valid: false}
			lastName := pgtype.Text{String: "", Valid: false}
			if len(splitName) == 2 {
				firstName.String = splitName[0]
				firstName.Valid = true
				lastName.String = splitName[1]
				lastName.Valid = true
			}

			facultyMember := classentry.Professor{
				ID:           professorID,
				Name:         prof.DisplayName,
				EmailAddress: pgtype.Text{String: prof.EmailAddress, Valid: true},
				FirstName:    firstName,
				LastName:     lastName,
			}
			professors[professorID] = facultyMember
		}
		courseId := s.Subject + "," + s.CourseNumber
		sectionSequence := s.SequenceNumber
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
			dbMeetingTime := classentry.MeetingTime{
				Sequence:        int32(i),
				SectionSequence: sectionSequence,
				SubjectCode:     s.Subject,
				CourseNumber:    s.CourseNumber,
				StartDate:       startDate,
				EndDate:         endDate,
				MeetingType:     pgtype.Text{String: meetingTime.MeetingType, Valid: true},
				StartMinutes:    toBannerTime(meetingTime.StartTime),
				EndMinutes:      toBannerTime(meetingTime.EndTime),
				IsMonday:        meetingTime.Monday,
				IsTuesday:       meetingTime.Tuesday,
				IsWednesday:     meetingTime.Wednesday,
				IsThursday:      meetingTime.Thursday,
				IsFriday:        meetingTime.Friday,
				IsSaturday:      meetingTime.Saturday,
				IsSunday:        meetingTime.Sunday,
			}
			meetingTimes = append(meetingTimes, dbMeetingTime)

		}
		dbSection := classentry.Section{
			Sequence:           sectionSequence,
			Campus:             pgtype.Text{String: s.CampusDescription, Valid: true},
			SubjectCode:        s.Subject,
			CourseNumber:       s.CourseNumber,
			Enrollment:         pgtype.Int4{Int32: s.Enrollment, Valid: true},
			MaxEnrollment:      pgtype.Int4{Int32: s.MaximumEnrollment, Valid: true},
			InstructionMethod:  pgtype.Text{String: s.InstructionalMethod, Valid: true},
			PrimaryProfessorID: primaryProf,
		}
		course := classentry.Course{
			SubjectCode:        s.Subject,
			Number:             s.CourseNumber,
			SubjectDescription: pgtype.Text{String: s.SubjectDescription, Valid: true},
			Title:              pgtype.Text{String: s.CourseTitle, Valid: true},
			// cannot get the description from here 😥
			Description: pgtype.Text{String: "", Valid: false},
			CreditHours: s.Credits,
		}
		courses[courseId] = course
		courseReferenceNumbers[courseId] = s.CourseReferenceNumber
		sections = append(sections, dbSection)
	}
	return ClassData{
		Sections:               sections,
		MeetingTimes:           meetingTimes,
		Professors:             professors,
		Courses:                courses,
		CourseReferenceNumbers: courseReferenceNumbers,
	}
}

func (b *bannerSchool) getCourseDetails(
	logger *log.Entry,
	client *http.Client,
	termCollection classentry.TermCollection,
	referenceNumber string,
) (*string, error) {
	if err := b.RequestLimiter.Wait(context.Background()); err != nil {
		logger.Error("Limiter error:", err)
		return nil, err
	}
	formData := url.Values{
		"term":                  {termCollection.ID},
		"courseReferenceNumber": {referenceNumber},
	}
	courseDescReq, err := http.NewRequest(
		"POST",
		"https://"+b.hostname+"/StudentRegistrationSsb/ssb/searchResults/getCourseDescription",
		bytes.NewBufferString(formData.Encode()),
	)
	courseDescReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	if err != nil {
		logger.Trace("Error making course desc request: ", err)
		return nil, err
	}

	resp, err := client.Do(courseDescReq)
	if err != nil {
		logger.Trace("Error doing course desc request: ", err)
		return nil, err
	}
	defer resp.Body.Close()
	doc, err := html.Parse(resp.Body)
	if err != nil {
		logger.Trace("Error parsing body: ", err)
		return nil, err
	}
	// example target element
	// <section aria-labelledby="courseDescription">
	// ...
	// </section>
	// should we try to sanitize ouput e.i. marist (maybe other schools) do the following
	// Text for no desc: "No course description is available."
	// Prefix for desc (in different tag): "No course description is available."
	var courseNode *html.Node
	courseDesc := ""

	var f func(*html.Node)
	f = func(n *html.Node) {

		for _, attr := range n.Attr {
			if attr.Key == "aria-labelledby" && attr.Val == "courseDescription" {
				courseNode = n
			}
		}

		if courseNode != nil {
			if n.Type == html.TextNode {
				courseDesc += n.Data
			}
		}

		for c := n.FirstChild; c != nil; c = c.NextSibling {
			f(c)
		}

		if n == courseNode {
			return
		}
	}
	f(doc)
	if courseDesc == "" {
		return nil, nil
	}
	return &courseDesc, nil
}
