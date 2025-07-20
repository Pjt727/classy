-- name: StageSections :copyfrom
INSERT INTO staging_sections 
    (term_collection_history_id, sequence, campus, subject_code, course_number,
        school_id, term_collection_id,
        enrollment, max_enrollment, instruction_method,
        primary_professor_id, campus, other)
VALUES
    (@term_collection_history_id, @sequence, @campus, @subject_code, @course_number,
        @school_id, @term_collection_id,
        @enrollment, @max_enrollment, @instruction_method,
        @primary_professor_id, @campus, @other);

-- name: StageMeetingTimes :copyfrom
INSERT INTO staging_meeting_times 
    (term_collection_history_id, sequence, section_sequence, term_collection_id,
        subject_code, course_number, school_id, 
        start_date, end_date, meeting_type,
        start_minutes, end_minutes, is_monday,
        is_tuesday, is_wednesday, is_thursday,
        is_friday, is_saturday, is_sunday, other)
VALUES
    (@term_collection_history_id, @sequence, @section_sequence, @term_collection_id,
        @subject_code, @course_number, @school_id, 
        @start_date, @end_date, @meeting_type,
        @start_minutes, @end_minutes, @is_monday,
        @is_tuesday, @is_wednesday, @is_thursday,
        @is_friday, @is_saturday, @is_sunday, @other);

-- name: StageProfessors :copyfrom
INSERT INTO staging_professors
    (term_collection_history_id, id, school_id, name,
        email_address, first_name, last_name, other)
VALUES
    (@term_collection_history_id, @id, @school_id, @name,
        @email_address, @first_name, @last_name, @other);

-- name: StageCourses :copyfrom
INSERT INTO staging_courses
    (term_collection_history_id, school_id, subject_code,
        number, subject_description, title,
        description, credit_hours, other)
VALUES 
    (@term_collection_history_id, @school_id, @subject_code,
        @number, @subject_description, @title,
        @description, @credit_hours, @other);

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
    name = EXCLUDED.name
WHERE term_collections.still_collecting != EXCLUDED.still_collecting
      OR term_collections.name != EXCLUDED.name
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
        primary_professor_id, other)
SELECT
    DISTINCT ON (sequence, term_collection_id, subject_code, course_number, school_id)
    sequence, term_collection_id, subject_code,
    course_number, school_id, max_enrollment, 
    instruction_method, campus, enrollment,
    primary_professor_id, other
FROM staging_sections WHERE term_collection_history_id = @term_collection_history_id
ON CONFLICT ("sequence", subject_code, course_number, school_id, term_collection_id) DO UPDATE
SET 
    campus = COALESCE(EXCLUDED.campus, sections.campus),
    enrollment = COALESCE(EXCLUDED.enrollment, sections.enrollment),
    max_enrollment = COALESCE(EXCLUDED.max_enrollment, sections.max_enrollment),
    instruction_method = COALESCE(EXCLUDED.instruction_method, sections.instruction_method),
    primary_professor_id = COALESCE(EXCLUDED.primary_professor_id, sections.primary_professor_id),
    other = COALESCE(EXCLUDED.other, sections.other)
  -- reducing write locks makes this way faster ALSO simplfies trigger logic
  WHERE (sections.campus IS DISTINCT FROM EXCLUDED.campus AND EXCLUDED.campus IS NOT NULL)
      OR (sections.enrollment IS DISTINCT FROM EXCLUDED.enrollment AND EXCLUDED.enrollment IS NOT NULL)
      OR (sections.max_enrollment IS DISTINCT FROM EXCLUDED.max_enrollment AND EXCLUDED.max_enrollment IS NOT NULL)
      OR (sections.instruction_method IS DISTINCT FROM EXCLUDED.instruction_method AND EXCLUDED.instruction_method IS NOT NULL)
      OR (sections.primary_professor_id IS DISTINCT FROM EXCLUDED.primary_professor_id AND EXCLUDED.primary_professor_id IS NOT NULL)
      OR (sections.other IS DISTINCT FROM EXCLUDED.other AND EXCLUDED.other IS NOT NULL)
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
        is_friday, is_saturday, is_sunday, other)
SELECT 
    DISTINCT ON (sequence, section_sequence, term_collection_id, subject_code, course_number, school_id)
    sequence, section_sequence, subject_code, 
    term_collection_id, course_number, school_id, 
    start_date, end_date, meeting_type,
    start_minutes, end_minutes, is_monday,
    is_tuesday, is_wednesday, is_thursday,
    is_friday, is_saturday, is_sunday, other
FROM staging_meeting_times WHERE term_collection_history_id = @term_collection_history_id
ON CONFLICT ("sequence", section_sequence, subject_code, course_number, school_id, term_collection_id) DO UPDATE
SET 
    start_date = COALESCE(EXCLUDED.start_date, meeting_times.start_date),
    end_date = COALESCE(EXCLUDED.end_date, meeting_times.end_date),
    meeting_type = COALESCE(EXCLUDED.meeting_type, meeting_times.meeting_type),
    start_minutes = COALESCE(EXCLUDED.start_minutes, meeting_times.start_minutes),
    end_minutes = COALESCE(EXCLUDED.end_minutes, meeting_times.end_minutes),
    is_monday = EXCLUDED.is_monday,
    is_tuesday = EXCLUDED.is_tuesday,
    is_wednesday = EXCLUDED.is_wednesday,
    is_thursday = EXCLUDED.is_thursday,
    is_friday = EXCLUDED.is_friday,
    is_saturday = EXCLUDED.is_saturday,
    is_sunday = EXCLUDED.is_sunday,
    other = COALESCE(EXCLUDED.other, meeting_times.other)
  -- reducing write locks makes this way faster AND for triggers
  WHERE (meeting_times.start_date IS DISTINCT FROM EXCLUDED.start_date AND EXCLUDED.start_date IS NOT NULL)
      OR (meeting_times.end_date IS DISTINCT FROM EXCLUDED.end_date AND EXCLUDED.end_date IS NOT NULL)
      OR (meeting_times.meeting_type IS DISTINCT FROM EXCLUDED.meeting_type AND EXCLUDED.meeting_type IS NOT NULL)
      OR (meeting_times.start_minutes IS DISTINCT FROM EXCLUDED.start_minutes AND EXCLUDED.start_minutes IS NOT NULL)
      OR (meeting_times.end_minutes IS DISTINCT FROM EXCLUDED.end_minutes AND EXCLUDED.end_minutes IS NOT NULL)
      OR meeting_times.is_monday IS DISTINCT FROM EXCLUDED.is_monday
      OR meeting_times.is_tuesday IS DISTINCT FROM EXCLUDED.is_tuesday
      OR meeting_times.is_wednesday IS DISTINCT FROM EXCLUDED.is_wednesday
      OR meeting_times.is_thursday IS DISTINCT FROM EXCLUDED.is_thursday
      OR meeting_times.is_friday IS DISTINCT FROM EXCLUDED.is_friday
      OR meeting_times.is_saturday IS DISTINCT FROM EXCLUDED.is_saturday
      OR meeting_times.is_sunday IS DISTINCT FROM EXCLUDED.is_sunday
      OR (meeting_times.other IS DISTINCT FROM EXCLUDED.other AND EXCLUDED.other IS NOT NULL)
;

-- name: MoveProfessors :exec
INSERT INTO professors (id, school_id, name, email_address, first_name, last_name, other)
SELECT DISTINCT ON (id, school_id) id, school_id, name, email_address, first_name, last_name, other
FROM staging_professors WHERE term_collection_history_id = @term_collection_history_id
ON CONFLICT (id, school_id) DO UPDATE
SET name = COALESCE(EXCLUDED.name, professors.name),
    email_address = COALESCE(EXCLUDED.email_address, professors.email_address),
    first_name = COALESCE(EXCLUDED.first_name, professors.first_name),
    last_name = COALESCE(EXCLUDED.last_name, professors.last_name),
    other = COALESCE(EXCLUDED.other, professors.other)
  WHERE (professors.name IS DISTINCT FROM EXCLUDED.name AND EXCLUDED.name IS NOT NULL)
      OR (professors.email_address IS DISTINCT FROM EXCLUDED.email_address AND EXCLUDED.email_address IS NOT NULL)
      OR (professors.first_name IS DISTINCT FROM EXCLUDED.first_name AND EXCLUDED.first_name IS NOT NULL)
      OR (professors.last_name IS DISTINCT FROM EXCLUDED.last_name AND EXCLUDED.last_name IS NOT NULL)
      OR (professors.other IS DISTINCT FROM EXCLUDED.other AND EXCLUDED.other IS NOT NULL)
;

-- name: MoveCourses :exec
INSERT INTO courses 
    (school_id, subject_code, number, subject_description, title,
        description, credit_hours, prerequisites, corequisites, other)
SELECT DISTINCT ON (school_id, subject_code, number) 
    school_id, subject_code, number, subject_description, title, 
    description, credit_hours, prerequisites, corequisites, other
FROM staging_courses WHERE term_collection_history_id = @term_collection_history_id
ON CONFLICT (school_id, subject_code, number) DO UPDATE
SET subject_description = COALESCE(EXCLUDED.subject_description, courses.subject_description),
    title = COALESCE(EXCLUDED.title, courses.title),
    credit_hours = COALESCE(EXCLUDED.credit_hours, courses.credit_hours),
    description = COALESCE(EXCLUDED.description, courses.description),
    prerequisites = COALESCE(EXCLUDED.prerequisites, courses.prerequisites),
    corequisites = COALESCE(EXCLUDED.corequisites, courses.corequisites),
    other = COALESCE(EXCLUDED.other, courses.other)
  WHERE (courses.title IS DISTINCT FROM EXCLUDED.title AND EXCLUDED.title IS NOT NULL)
      OR (courses.credit_hours IS DISTINCT FROM EXCLUDED.credit_hours AND EXCLUDED.credit_hours IS NOT NULL)
      OR (courses.subject_description IS DISTINCT FROM EXCLUDED.subject_description AND EXCLUDED.subject_description IS NOT NULL)
      -- these are considered "extra" fields that may no always be populated
      --     because they are difficult to get
      OR (courses.prerequisites IS DISTINCT FROM EXCLUDED.prerequisites AND EXCLUDED.prerequisites IS NOT NULL)
      OR (courses.corequisites IS DISTINCT FROM EXCLUDED.corequisites AND EXCLUDED.corequisites IS NOT NULL)
      OR (courses.description IS DISTINCT FROM EXCLUDED.description AND EXCLUDED.description IS NOT NULL)
      OR (courses.other IS DISTINCT FROM EXCLUDED.other AND EXCLUDED.other IS NOT NULL)
;

-- name: InsertTermCollectionHistory :one
INSERT INTO term_collection_history
    (term_collection_id, school_id, is_full)
VALUES (@term_collection_id, @school_id, @is_full)
RETURNING id;

-- name: FinishTermCollectionHistory :exec
UPDATE term_collection_history SET
    status = @new_finished_status,
    end_time = now()
WHERE id = @term_collection_history_id
;

-- name: GetChangesFromMoveTermCollection :one
SELECT
    t.id,
    SUM(CASE WHEN sync_action = 'insert' THEN 1 ELSE 0 END) AS insert_records,
    SUM(CASE WHEN sync_action = 'update' THEN 1 ELSE 0 END) AS updated_records,
    SUM(CASE WHEN sync_action = 'delete' THEN 1 ELSE 0 END) AS deleted_records,
    (end_time - start_time)::INTERVAL AS elapsed_time
FROM term_collection_history t 
LEFT JOIN historic_class_information h ON t.id = h.term_collection_history_id
WHERE t.id = @term_collection_history_id::INTEGER
GROUP BY (t.id, t.end_time, t.start_time)
;

-- name: DeleteStagingCourses :exec
DELETE FROM staging_courses
WHERE term_collection_history_id = @term_collection_history_id
;

-- name: DeleteStagingProfessors :exec
DELETE FROM staging_professors
WHERE term_collection_history_id = @term_collection_history_id
;

-- name: DeleteStagingSections :exec
DELETE FROM staging_sections
WHERE term_collection_history_id = @term_collection_history_id
;

-- name: DeleteStagingMeetingTimes :exec
DELETE FROM staging_meeting_times
WHERE term_collection_history_id = @term_collection_history_id
;
