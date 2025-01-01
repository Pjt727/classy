-- name: ListCourses :many
SELECT * FROM courses;

-- name: TruncateStagingSections :exec
TRUNCATE TABLE staging_sections;

-- name: TruncateStagingMeetingTimes :exec
TRUNCATE TABLE staging_meeting_times;

-- name: StageSections :copyfrom
INSERT INTO staging_sections 
    (id, campus, course_id, 
        school_id, term_year, term_season, 
        enrollment, max_enrollment, instruction_method,
        primary_faculty_id, campus)
VALUES
    (@id, @campus, @course_id,
        @schoolId, @termYear, @termSeason,
        @enrollment, @maxEnrollment, @instructionMethod,
        @primaryFacultyId, @campus);

-- name: StageMeetingTimes :copyfrom
INSERT INTO staging_meeting_times 
    (section_id, term_season, 
        term_year, course_id, school_id, 
        start_date, end_date, meeting_type,
        start_minutes, end_minutes, is_monday,
        is_tuesday, is_wednesday, is_thursday,
        is_friday, is_saturday, is_sunday)
VALUES
    (@sectionId, @termSeason, 
        @termYear, @courseId, @schoolId, 
        @startDate, @endDate, @meetingType,
        @startMinutes, @endMinutes, @isMonday,
        @isTuesday, @isWednesday, @isThursday,
        @isFriday, @isSaturday, @isSunday);

-- name: UpsertFaculty :exec
INSERT INTO faculty_members
    (id, school_id, name,
        email_address, first_name, last_name)
VALUES
    (@id, @schoolId, @name,
        @emailAddress, @firstName, @lastName)
ON CONFLICT DO NOTHING;

-- name: UpsertCourses :exec
INSERT INTO courses
    (id, school_id, subject_code,
        number, subject_description, title,
        description, credit_hours)
VALUES 
    (@id, @schoolId, @subjectCode,
        @number, @subjectDescription, @title,
        @description, @creditHours)
ON CONFLICT DO NOTHING;

-- name: UpdateSchools :exec
INSERT INTO schools
    ()
;
