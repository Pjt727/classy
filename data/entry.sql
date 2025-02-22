-- name: DeleteStagingSections :exec
DELETE FROM staging_sections
WHERE school_id = @school_id
    AND term_collection_id = @term_collection_id
;

-- name: DeleteStagingMeetingTimes :exec
DELETE FROM staging_meeting_times
WHERE school_id = @school_id
     AND term_collection_id = @term_collection_id
;

-- name: StageSections :copyfrom
INSERT INTO staging_sections 
    (sequence, campus, subject_code, course_number,
        school_id, term_collection_id,
        enrollment, max_enrollment, instruction_method,
        primary_professor_id, campus)
VALUES
    (@sequence, @campus, @subject_code, @course_number,
        @school_id, @term_collection_id,
        @enrollment, @max_enrollment, @instruction_method,
        @primary_professor_id, @campus);

-- name: StageMeetingTimes :copyfrom
INSERT INTO staging_meeting_times 
    (sequence, section_sequence, term_collection_id,
        subject_code, course_number, school_id, 
        start_date, end_date, meeting_type,
        start_minutes, end_minutes, is_monday,
        is_tuesday, is_wednesday, is_thursday,
        is_friday, is_saturday, is_sunday)
VALUES
    (@sequence, @section_sequence, @term_collection_id,
        @subject_code, @course_number, @school_id, 
        @start_date, @end_date, @meeting_type,
        @start_minutes, @end_minutes, @is_monday,
        @is_tuesday, @is_wednesday, @is_thursday,
        @is_friday, @is_saturday, @is_sunday);

-- name: UpsertProfessor :batchexec
INSERT INTO professors
    (id, school_id, name,
        email_address, first_name, last_name)
VALUES
    (@id, @school_id, @name,
        @email_address, @first_name, @last_name)
ON CONFLICT DO NOTHING;

-- name: UpsertCourses :batchexec
INSERT INTO courses
    (school_id, subject_code,
        number, subject_description, title,
        description, credit_hours)
VALUES 
    (@school_id, @subject_code,
        @number, @subject_description, @title,
        @description, @credit_hours)
ON CONFLICT DO NOTHING;

-- name: UpsertSchool :exec
INSERT INTO schools
    (id, name)
VALUES
    (@id, @name)
ON CONFLICT DO NOTHING;

-- name: UpsertTermCollection :batchexec
INSERT INTO term_collections
    (id, school_id, year, season, name, still_collecting)
VALUES
    (@id, @school_id, @year, @season, @name, @still_collecting)
ON CONFLICT (id, school_id) DO UPDATE
SET
    still_collecting = EXCLUDED.still_collecting,
    name = EXCLUDED.name;
;

-- name: UpsertTerm :batchexec
INSERT INTO terms
    (year, season)
VALUES
    (@year, @season)
ON CONFLICT DO NOTHING;

-- name: RemoveUnstagedSections :exec
DELETE FROM sections s
WHERE s.term_collection_id = @term_collection_id
  AND s.school_id = @school_id
  AND NOT EXISTS (
    SELECT 1 
    FROM staging_sections ss
    WHERE ss.sequence = s.sequence
      AND ss.term_collection_id = s.term_collection_id
      AND ss.subject_code = s.subject_code
      AND ss.course_number = s.course_number
      AND ss.school_id = s.school_id
  );

-- name: MoveStagedSections :exec
INSERT INTO sections 
    (sequence, term_collection_id, subject_code,
        course_number, school_id, max_enrollment, 
        instruction_method, campus, enrollment,
        primary_professor_id)
SELECT
    DISTINCT ON (sequence, term_collection_id, subject_code, course_number, school_id)
    sequence, term_collection_id, subject_code,
    course_number, school_id, max_enrollment, 
    instruction_method, campus, enrollment,
    primary_professor_id
FROM staging_sections
ON CONFLICT ("sequence", subject_code, course_number, school_id, term_collection_id) DO UPDATE
SET 
    campus = EXCLUDED.campus,
    enrollment = EXCLUDED.enrollment,
    max_enrollment = EXCLUDED.max_enrollment,
    instruction_method = EXCLUDED.instruction_method,
    primary_professor_id = EXCLUDED.primary_professor_id
WHERE sections.campus != EXCLUDED.campus
    OR sections.enrollment != EXCLUDED.enrollment
    OR sections.max_enrollment != EXCLUDED.max_enrollment
    OR sections.instruction_method != EXCLUDED.instruction_method
    OR sections.primary_professor_id != EXCLUDED.primary_professor_id
;

-- name: RemoveUnstagedMeetings :exec
DELETE FROM meeting_times mt
WHERE mt.term_collection_id = @term_collection_id
  AND mt.school_id = @school_id
  AND NOT EXISTS (
    SELECT 1 
    FROM staging_meeting_times smt
    WHERE smt."sequence" = mt."sequence"
      AND smt.term_collection_id = mt.term_collection_id
      AND smt.subject_code = mt.subject_code
      AND smt.course_number = mt.course_number
      AND smt.school_id = mt.school_id
      AND smt.section_sequence = mt.section_sequence
  )
;


-- name: MoveStagedMeetingTimes :exec
INSERT INTO meeting_times
    (sequence, section_sequence, subject_code,
        term_collection_id, course_number, school_id, 
        start_date, end_date, meeting_type,
        start_minutes, end_minutes, is_monday,
        is_tuesday, is_wednesday, is_thursday,
        is_friday, is_saturday, is_sunday)
SELECT 
    DISTINCT ON (sequence, section_sequence, term_collection_id, subject_code, course_number, school_id)
    sequence, section_sequence, subject_code, 
    term_collection_id, course_number, school_id, 
    start_date, end_date, meeting_type,
    start_minutes, end_minutes, is_monday,
    is_tuesday, is_wednesday, is_thursday,
    is_friday, is_saturday, is_sunday
FROM staging_meeting_times
ON CONFLICT ("sequence", section_sequence, subject_code, course_number, school_id, term_collection_id) DO UPDATE
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
