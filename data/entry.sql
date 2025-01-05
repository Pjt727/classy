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
    (sequence, section_id, term_season, 
        term_year, course_id, school_id, 
        start_date, end_date, meeting_type,
        start_minutes, end_minutes, is_monday,
        is_tuesday, is_wednesday, is_thursday,
        is_friday, is_saturday, is_sunday)
VALUES
    (@sequence, @sectionId, @termSeason, 
        @termYear, @courseId, @schoolId, 
        @startDate, @endDate, @meetingType,
        @startMinutes, @endMinutes, @isMonday,
        @isTuesday, @isWednesday, @isThursday,
        @isFriday, @isSaturday, @isSunday);

-- name: UpsertFaculty :batchexec
INSERT INTO faculty_members
    (id, school_id, name,
        email_address, first_name, last_name)
VALUES
    (@id, @schoolId, @name,
        @emailAddress, @firstName, @lastName)
ON CONFLICT DO NOTHING;

-- name: UpsertCourses :batchexec
INSERT INTO courses
    (id, school_id, subject_code,
        number, subject_description, title,
        description, credit_hours)
VALUES 
    (@id, @schoolId, @subjectCode,
        @number, @subjectDescription, @title,
        @description, @creditHours)
ON CONFLICT DO NOTHING;

-- name: UpsertSchools :exec
INSERT INTO schools
    (id, name)
VALUES
    (@id, @name)
ON CONFLICT DO NOTHING;

-- name: UpsertTermCollection :batchexec
INSERT INTO term_collections
    (school_id, year, season, still_collecting)
VALUES
    (@schoolId, @year, @season, @stillCollecting)
ON CONFLICT (school_id, year, season) DO UPDATE
SET
    still_collecting = EXCLUDED.still_collecting;
;

-- name: UpsertTerm :batchexec
INSERT INTO terms
    (year, season)
VALUES
    (@year, @season)
ON CONFLICT DO NOTHING;

-- name: RemoveUnstagedSections :exec
DELETE FROM sections s
WHERE s.term_season = @termSeason 
  AND s.term_year = @termYear 
  AND s.school_id = @school_id
  AND NOT EXISTS (
    SELECT 1 
    FROM staging_sections ss
    WHERE ss.id = s.id
      AND ss.term_season = s.term_season
      AND ss.term_year = s.term_year
      AND ss.course_id = s.course_id
      AND ss.school_id = s.school_id
  );

-- name: MoveStagedSections :exec
INSERT INTO sections 
    (id, term_season, term_year, 
        course_id, school_id, max_enrollment, 
        instruction_method, campus, enrollment,
        primary_faculty_id)
SELECT
    id, term_season, term_year, 
    course_id, school_id, max_enrollment, 
    instruction_method, campus, enrollment,
    primary_faculty_id
FROM staging_sections
ON CONFLICT (id, course_id, school_id, term_year, term_season) DO UPDATE
SET 
    campus = EXCLUDED.campus,
    enrollment = EXCLUDED.enrollment,
    max_enrollment = EXCLUDED.max_enrollment,
    instruction_method = EXCLUDED.instruction_method,
    primary_faculty_id = EXCLUDED.primary_faculty_id
WHERE sections.campus != EXCLUDED.campus
    OR sections.enrollment != EXCLUDED.enrollment
    OR sections.max_enrollment != EXCLUDED.max_enrollment
    OR sections.instruction_method != EXCLUDED.instruction_method
    OR sections.primary_faculty_id != EXCLUDED.primary_faculty_id
;

-- name: RemoveUnstagedMeetings :exec
DELETE FROM meeting_times mt
WHERE mt.term_season = @termSeason 
  AND mt.term_year = @termYear 
  AND mt.school_id = @school_id
  AND NOT EXISTS (
    SELECT 1 
    FROM staging_meeting_times smt
    WHERE smt."sequence" = mt."sequence"
      AND smt.term_season = mt.term_season
      AND smt.term_year = mt.term_year
      AND smt.course_id = mt.course_id
      AND smt.school_id = mt.school_id
      AND smt.section_id = mt.section_id
  )
;


-- name: MoveStagedMeetingTimes :exec
INSERT INTO meeting_times
    (sequence, section_id, term_season, 
        term_year, course_id, school_id, 
        start_date, end_date, meeting_type,
        start_minutes, end_minutes, is_monday,
        is_tuesday, is_wednesday, is_thursday,
        is_friday, is_saturday, is_sunday)
SELECT 
    sequence, section_id, term_season, 
    term_year, course_id, school_id, 
    start_date, end_date, meeting_type,
    start_minutes, end_minutes, is_monday,
    is_tuesday, is_wednesday, is_thursday,
    is_friday, is_saturday, is_sunday
FROM staging_meeting_times
ON CONFLICT ("sequence", section_id, course_id, school_id, term_year, term_season) DO UPDATE
SET 
    start_date = EXCLUDED.start_date,
    end_date = EXCLUDED.end_date,
    meeting_type = EXCLUDED.meeting_type,
    start_minutes = EXCLUDED.start_minutes,
    end_minutes = EXCLUDED.end_minutes,
    is_monday = EXCLUDED.is_monday,
    is_tuesday = EXCLUDED.is_tuesday,
    is_wednesday = EXCLUDED.is_wednesday,
    is_thursday = EXCLUDED.is_thursday,
    is_friday = EXCLUDED.is_friday,
    is_saturday = EXCLUDED.is_saturday,
    is_sunday = EXCLUDED.is_sunday
WHERE meeting_times.start_date != EXCLUDED.start_date
    OR meeting_times.end_date != EXCLUDED.end_date
    OR meeting_times.meeting_type != EXCLUDED.meeting_type
    OR meeting_times.start_minutes != EXCLUDED.start_minutes
    OR meeting_times.end_minutes != EXCLUDED.end_minutes
    OR meeting_times.is_monday != EXCLUDED.is_monday
    OR meeting_times.is_tuesday != EXCLUDED.is_tuesday
    OR meeting_times.is_wednesday != EXCLUDED.is_wednesday
    OR meeting_times.is_thursday != EXCLUDED.is_thursday
    OR meeting_times.is_friday != EXCLUDED.is_friday
    OR meeting_times.is_saturday != EXCLUDED.is_saturday
    OR meeting_times.is_sunday != EXCLUDED.is_sunday
;
