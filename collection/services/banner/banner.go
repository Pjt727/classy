package banner

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"golang.org/x/sync/errgroup"
	"golang.org/x/time/rate"

	"github.com/Pjt727/classy/collection/services"
	classentry "github.com/Pjt727/classy/data/class-entry"
	"github.com/PuerkitoBio/goquery"
	"github.com/jackc/pgx/v5/pgtype"
)

type bannerSchool struct {
	school   classentry.School
	hostname string

	// to limit the max amount of requests out that go without having an answer
	RegularCollectionSectionSemaphore int
	FullCollectionSectionSemaphore    int
	FullCollectionCourseSemaphore     int // keep in mind this is per section collection
	RequestRetryCount                 int
	MaxTermCount                      int
	MaxSectionPageCount               int
	rateLimiter                       services.RateLimiter
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
			school:                            marist,
			hostname:                          "ssb1-reg.banner.marist.edu",
			FullCollectionSectionSemaphore:    3,
			FullCollectionCourseSemaphore:     35,
			RegularCollectionSectionSemaphore: 5,
			MaxTermCount:                      100,
			MaxSectionPageCount:               200,
			RequestRetryCount:                 3,
			rateLimiter:                       services.NewAdaptiveRateLimiter(rate.Every(250*time.Millisecond), 5, rate.Every(500*time.Millisecond)),
		},
		temple.ID: {
			school:                            temple,
			hostname:                          "prd-xereg.temple.edu",
			FullCollectionSectionSemaphore:    2,
			FullCollectionCourseSemaphore:     20,
			RegularCollectionSectionSemaphore: 5,
			MaxTermCount:                      100,
			MaxSectionPageCount:               200,
			rateLimiter:                       services.NewAdaptiveRateLimiter(rate.Every(25*time.Millisecond), 5, rate.Every(50*time.Millisecond)),
		},
	}
	Service = &banner{schools: schools}
}

func (b *banner) ListValidSchools(
	logger slog.Logger,
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
	logger slog.Logger,
	ctx context.Context,
	school classentry.School,
) ([]classentry.TermCollection, error) {
	var termCollection []classentry.TermCollection

	bannerschool, err := b.getBannerSchool(school.ID)
	if err != nil {
		logger.Error("Error getting school entry", "error", err)
		return termCollection, err
	}
	termCollections, err := bannerschool.getTerms(ctx, logger)
	if err != nil {
		return termCollection, err
	}

	return termCollections, nil
}

func (b *banner) StageAllClasses(
	logger slog.Logger,
	ctx context.Context,
	q *classentry.EntryQueries,
	schoolID string,
	termCollection classentry.TermCollection,
	fullCollection bool,
) error {
	logger.Info("Starting full section")
	bannerSchool, err := b.getBannerSchool(schoolID)
	if err != nil {
		logger.Error("Error getting school entry", "error", err)
		return err
	}
	err = bannerSchool.stageAllClasses(
		logger,
		ctx,
		q,
		termCollection,
		fullCollection,
	)
	if err != nil {
		logger.Error("Error getting all school entry", "error", err)
		return err
	}
	return nil
}

func (b *banner) getBannerSchool(schoolID string) (*bannerSchool, error) {
	schoolEntry, ok := b.schools[schoolID]
	if !ok {
		err := fmt.Errorf(
			"%w school not known for this service: %s",
			services.ErrIncorrectAssumption,
			schoolID,
		)
		return nil, err
	}
	return &schoolEntry, nil
}

func termConversion(termEntry bannerTerm) (classentry.Term, error) {
	desc := strings.ToLower(termEntry.Description)
	var dbTerm classentry.Term
	if len(termEntry.Code) != 6 {
		err := fmt.Errorf(
			"%w term code `%s` incorrect length must be 6 characters long",
			services.ErrIncorrectAssumption,
			termEntry.Code,
		)
		return dbTerm, err
	}
	yearSection := termEntry.Code[:4]
	year, err := strconv.Atoi(yearSection)
	// year sanity check
	if err != nil || year > (time.Now().Year()+5) || year < 1850 {
		err := fmt.Errorf(
			"%w year section `%s` is a invalid/ infeasible year",
			services.ErrIncorrectAssumption,
			yearSection,
		)
		return dbTerm, err
	}
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
		err := fmt.Errorf(
			"%w season description `%s` has no season match in it",
			services.ErrIncorrectAssumption,
			desc,
		)
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
	ctx context.Context,
	logger slog.Logger,
) ([]classentry.TermCollection, error) {

	var termCollection []classentry.TermCollection
	hostname := b.hostname
	req, err := http.NewRequestWithContext(
		ctx,
		"GET",
		"https://"+hostname+"/StudentRegistrationSsb/ssb/classSearch/getTerms?searchTerm=&offset=1&max="+strconv.Itoa(b.MaxTermCount),
		nil,
	)
	if err != nil {
		logger.Error("Error creating term request", "error", err)
		return termCollection, errors.Join(services.ErrTemporaryNetworkFailure, err)
	}

	client := &http.Client{}
	services.AddRateLimiter(client, &b.rateLimiter)
	resp, err := client.Do(req)
	err = services.RespOrStatusErr(resp, err)
	if err != nil {
		logger.Error("Error getting term response", "error", err)
		return termCollection, err
	}
	defer resp.Body.Close()
	var terms []bannerTerm
	if err := json.NewDecoder(resp.Body).Decode(&terms); err != nil {
		logger.Error("Error decoding terms", "error", err)
		return termCollection, errors.Join(services.ErrIncorrectAssumption, err)
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
			logger.Error("Error decoding term code", "error", err)
			return termCollection, err
		}
		// the View Only appears on all marist terms which probably wont update
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

func (b *bannerSchool) refreshTermAssociatedCookies(
	logger slog.Logger,
	ctx context.Context,
	client *http.Client,
	bannerTerm string,
) error {
	// reset cookie jar
	newCookieJar, _ := cookiejar.New(nil)
	client.Jar = newCookieJar

	req, err := http.NewRequestWithContext(
		ctx,
		"GET",
		"https://"+b.hostname+"/StudentRegistrationSsb/ssb/term/termSelection?mode=search",
		nil,
	)
	if err != nil {
		logger.Error("Error creating cookie request", "error", err)
		return errors.Join(services.ErrIncorrectAssumption, err)
	}

	cookieResp, err := client.Do(req)
	err = services.RespOrStatusErr(cookieResp, err)
	if err != nil {
		logger.Error("Error getting cookie response", "error", err)
		return err
	}
	defer cookieResp.Body.Close()
	// Associate the cookie with a term
	formData := url.Values{
		"term": {bannerTerm},
	}
	req, err = http.NewRequestWithContext(
		ctx,
		"POST",
		"https://"+b.hostname+"/StudentRegistrationSsb/ssb/term/search?mode=search",
		bytes.NewBufferString(formData.Encode()),
	)
	if err != nil {
		logger.Error("Error creating request to set term", "error", err)
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	termAssociationResp, err := client.Do(req)
	err = services.RespOrStatusErr(termAssociationResp, err)
	if err != nil {
		logger.Error("Error setting term", "error", err)
		return err
	}
	defer termAssociationResp.Body.Close()

	return nil
}

func (b *bannerSchool) stageAllClasses(
	logger slog.Logger,
	ctx context.Context,
	q *classentry.EntryQueries,
	termCollection classentry.TermCollection,
	fullCollection bool,
) error {
	termStr := termCollection.ID
	// Get banner cookie(s)
	client := &http.Client{}
	err := b.refreshTermAssociatedCookies(logger, ctx, client, termStr)
	if err != nil {
		return fmt.Errorf("Error getting cookies %s", err)
	}

	// Make a request to get sections
	req, err := http.NewRequestWithContext(
		ctx,
		"GET",
		"https://"+b.hostname+"/StudentRegistrationSsb/ssb/searchResults/searchResults",
		nil,
	)
	if err != nil {
		logger.Error("Error creating request", "error", err)
		return err
	}
	queryParams := url.Values{
		"txt_term":    {termStr},
		"pageOffset":  {"0"},
		"pageMaxSize": {"1"},
	}
	req.URL.RawQuery = queryParams.Encode()
	resp, err := client.Do(req)
	err = services.RespOrStatusErr(resp, err)
	if err != nil {
		logger.Error("Error requesting first sections", "error", err)
		return err
	}
	defer resp.Body.Close()
	type Sectioncount struct {
		Count int32 `json:"totalCount"`
	}
	var sectionCount Sectioncount
	if err := json.NewDecoder(resp.Body).Decode(&sectionCount); err != nil {
		logger.Error("Error decoding first sections", "error", err)
		return err
	}
	count := sectionCount.Count
	logger.Info("starting collection", "sections", count)

	var actualTotalSectionCount int32
	var semaphore chan struct{}

	if fullCollection {
		semaphore = make(chan struct{}, b.FullCollectionSectionSemaphore)
	} else {
		semaphore = make(chan struct{}, b.RegularCollectionSectionSemaphore)
	}

	numberOfWorkers := math.Ceil(float64(count) / float64(b.MaxSectionPageCount))

	eg, ctx := errgroup.WithContext(ctx)
	eg.SetLimit(int(numberOfWorkers))

	// i is already scoped to each iteration for goroutines
	for i := range int(numberOfWorkers) {
		eg.Go(func() error {
			select {
			case semaphore <- struct{}{}: // Acquire semaphore
				defer func() { <-semaphore }() // Release semaphore
			case <-ctx.Done():
				return ctx.Err()
			}

			workersReq := req.Clone(req.Context())
			workerLog := logger.With(
				slog.String("pageOffSet", strconv.Itoa(i*b.MaxSectionPageCount)),
				slog.Int("pageMaxSize", b.MaxSectionPageCount),
			)
			queryParams := url.Values{
				"txt_term":    {termStr},
				"pageOffset":  {strconv.Itoa(i * b.MaxSectionPageCount)},
				"pageMaxSize": {strconv.Itoa(b.MaxSectionPageCount)},
			}
			workersReq.URL.RawQuery = queryParams.Encode()

			var sectionClient *http.Client
			if fullCollection {
				sectionClient = &http.Client{}
				services.AddRateLimiter(sectionClient, &b.rateLimiter)
				err := b.refreshTermAssociatedCookies(logger, ctx, sectionClient, termStr)
				if err != nil {
					return err
				}
			} else {
				sectionClient = client
			}

			var sectionCount int
			var err error
			for i := range b.RequestRetryCount {
				sectionCount, err = b.insertGroupOfSections(
					workerLog,
					workersReq,
					ctx,
					q,
					sectionClient,
					termCollection,
					fullCollection,
				)
				if err == nil {
					break
				}
				// regardless of full collection or now after an error should an http error
				//    should refresh client tokens
				// technically there could be other error in the code that aren't http related
				//    some better error types would be helpful
				logger.Info("Retrying section some failures", "failures", i+1)
				err = b.refreshTermAssociatedCookies(logger, ctx, sectionClient, termStr)
				if err != nil {
					return err
				}
			}
			if err != nil {
				return err
			}

			atomic.AddInt32(&actualTotalSectionCount, int32(sectionCount))
			return nil
		})
	}

	if err := eg.Wait(); err != nil {
		return fmt.Errorf("one or more errors occurred: %w", err)
	}

	if actualTotalSectionCount == count {
		logger.Info("sections finished getting", "sections", actualTotalSectionCount)
	} else {
		// A record could have been added / delete while the collection was happening
		// but if the difference is large there is likely something more sinister going on
		// this also should maybe be a full error to so that the move doesn't delete
		// sections that just weren't found
		logger.Warn("finished section collection but expected section count does not match actual count",
			"expected", count,
			"actual", actualTotalSectionCount,
			"difference", int(math.Abs(float64(count-actualTotalSectionCount))),
		)
	}

	return nil
}

func (b *bannerSchool) insertGroupOfSections(
	logger *slog.Logger,
	sectionReq *http.Request,
	ctx context.Context,
	q *classentry.EntryQueries,
	client *http.Client,
	termCollection classentry.TermCollection,
	fullCollection bool,
) (int, error) {
	resp, err := client.Do(sectionReq)
	err = services.RespOrStatusErr(resp, err)
	if err != nil {
		logger.Error("Error getting sections", "error", err)
		return 0, err
	}
	defer resp.Body.Close()

	var sections SectionSearch
	if err := json.NewDecoder(resp.Body).Decode(&sections); err != nil {
		logger.Error("Error decoding sections", "error", err)
		return 0, errors.Join(services.ErrIncorrectAssumption, err)
	}

	classData := ProcessSectionSearch(sections)

	// add all of the extra course data
	if fullCollection {
		for i := range b.RequestRetryCount {
			err := b.processFullCollection(
				ctx,
				logger,
				client,
				termCollection,
				&classData,
			)
			if err == nil {
				break
			}

			logger.Info("Retrying course get after failures", "failures", i+1)
			err = b.refreshTermAssociatedCookies(*logger, ctx, client, termCollection.ID)
			if err != nil {
				return 0, err
			}
		}
	}

	err = q.InsertClassData(
		logger,
		ctx,
		classData.ToEntry(),
	)

	if err != nil {
		logger.Error("Error inserting class data", "error", err)
		return 0, err
	}

	logger.Info(
		"Successfully added sections and their related information",
		"sections", len(classData.Sections),
	)

	return len(classData.Sections), nil
}

func (b *bannerSchool) processFullCollection(
	ctx context.Context,
	logger *slog.Logger,
	client *http.Client,
	termCollection classentry.TermCollection,
	classData *ClassData,
) error {
	var wg sync.WaitGroup
	var mu sync.Mutex
	semaphore := make(chan struct{}, b.FullCollectionCourseSemaphore)
	wg.Add(len(classData.CourseReferenceNumbers))
	for courseId, referenceNumber := range classData.CourseReferenceNumbers {
		semaphore <- struct{}{}                            // Acquire semaphore
		go func(courseId string, referenceNumber string) { // Pass loop variables to the goroutine
			defer func() {
				<-semaphore
				wg.Done()
			}()
			courseDesc, err := b.getCourseDetails(
				ctx,
				logger,
				client,
				termCollection,
				referenceNumber,
			)
			if err != nil || courseDesc == nil {
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

		}(courseId, referenceNumber) // Pass loop variables

	}

	wg.Wait()
	return nil
}

type ClassData struct {
	Sections               []classentry.Section
	MeetingTimes           []classentry.MeetingTime
	Professors             map[string]classentry.Professor
	Courses                map[string]classentry.Course
	CourseReferenceNumbers map[string]string
}

func (c *ClassData) ToEntry() classentry.ClassData {
	professors := make([]classentry.Professor, len(c.Professors))
	i := 0
	for _, professor := range c.Professors {
		professors[i] = professor
		i += 1
	}

	courses := make([]classentry.Course, len(c.Courses))
	i = 0
	for _, course := range c.Courses {
		courses[i] = course
		i += 1
	}
	return classentry.ClassData{
		MeetingTimes: c.MeetingTimes,
		Sections:     c.Sections,
		Professors:   professors,
		Courses:      courses,
	}
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
	ctx context.Context,
	logger *slog.Logger,
	client *http.Client,
	termCollection classentry.TermCollection,
	referenceNumber string,
) (*string, error) {
	formData := url.Values{
		"term":                  {termCollection.ID},
		"courseReferenceNumber": {referenceNumber},
	}
	courseDescReq, err := http.NewRequestWithContext(
		ctx,
		"POST",
		"https://"+b.hostname+"/StudentRegistrationSsb/ssb/searchResults/getCourseDescription",
		bytes.NewBufferString(formData.Encode()),
	)
	courseDescReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	if err != nil {
		logger.Debug("Error making course desc request", "error", err)
		return nil, errors.Join(services.ErrIncorrectAssumption, err)
	}

	resp, err := client.Do(courseDescReq)
	err = services.RespOrStatusErr(resp, err)
	if err != nil {
		logger.Debug("Error doing course desc request", "error", err)
		return nil, err
	}
	defer resp.Body.Close()

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		logger.Debug("Error parsing body", "error", err)
		return nil, errors.Join(services.ErrIncorrectAssumption, err)
	}

	courseDesc := doc.Find("section[aria-labelledby='courseDescription']").Text()

	if courseDesc == "" {
		return nil, nil
	}

	return &courseDesc, nil
}
