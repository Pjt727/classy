-- name: CourseExists :one
SELECT CASE 
        WHEN EXISTS (
            SELECT 1
            FROM courses
            WHERE school_id = @school_id
            AND subject_code = @subject_code
            AND number = @course_number
        ) THEN true
    ELSE false
END
;

-- name: TermCollectionExists :one
SELECT CASE 
        WHEN EXISTS (
            SELECT 1
            FROM term_collections
            WHERE school_id = @school_id
            AND id = @term_collection_id
        ) THEN true
    ELSE false
END
;

-- name: SchoolExists :one
SELECT CASE 
        WHEN EXISTS (
            SELECT 1
            FROM schools
            WHERE id = @school_id
        ) THEN true
    ELSE false
END
;

-- name: GetCourseWithHueristics :one
SELECT c.*, ch.previous_terms, ch.previous_professors
FROM courses c
INNER JOIN course_heuristic ch ON ch.number = c.number
            AND ch.subject_code = c.subject_code
            AND ch.school_id = c.school_id
WHERE c.school_id = @school_id
      AND c.subject_code = @subject_code
      AND c.number = @course_number
;

-- name: GetCoursesForSchoolAndSubject :many
SELECT courses.*
FROM courses
WHERE school_id = @school_id
      AND subject_code = @subject_code
LIMIT @limitValue
OFFSET @offsetValue
;

-- name: GetCoursesForSchool :many
SELECT courses.*
FROM courses
WHERE school_id = @school_id
LIMIT @limitValue
OFFSET @offsetValue
;

-- name: GetTermCollectionsForSchool :many
SELECT term_collections.*
FROM term_collections 
WHERE school_id = @school_id
LIMIT @limitValue
OFFSET @offsetValue
;

-- name: GetTermCollectionsForSchoolsSemester :many
SELECT sqlc.embed(term_collections) 
FROM term_collections 
WHERE school_id = @school_id
      AND (year = @year OR @year IS NULL )
      AND (season = @season OR @season IS NULL)
;

-- name: GetSchools :many
SELECT schools.*
FROM schools
LIMIT @limitValue
OFFSET @offsetValue
;

-- name: GetSchoolsClassesForTermOrderedBySection :many
SELECT sqlc.embed(sections), section_meetings.meeting_times
FROM section_meetings
JOIN sections ON sections."sequence"           = section_meetings."sequence"
              AND sections.term_collection_id  = section_meetings.term_collection_id
              AND sections.subject_code        = section_meetings.subject_code
              AND sections.course_number       = section_meetings.course_number
              AND sections.school_id           = section_meetings.school_id
WHERE sections.school_id = @school_id
      AND sections.term_collection_id = @term_collection_id
ORDER BY sections."sequence", sections.subject_code, sections.course_number, 
    sections.school_id, sections.term_collection_id
LIMIT @limitValue
OFFSET @offsetValue
;
