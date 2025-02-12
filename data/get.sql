-- name: GetTermCollection :one
SELECT sqlc.embed(term_collections)
FROM term_collections
WHERE school_id = @school_id
      AND id = @term_collection_id
LIMIT 1
;
-- name: GetSchool :one
SELECT sqlc.embed(schools)
FROM schools
WHERE id = @school_id
LIMIT 1
;
-- name: GetTermCollectionsForSchool :many
SELECT sqlc.embed(term_collections) 
FROM term_collections 
WHERE school_id = @school_id
      AND (year = @year OR @year IS NULL )
      AND (season = @season OR @season IS NULL)
;

-- name: GetSchoolsClassesForTermOrderedBySection :many
SELECT sqlc.embed(sections), sqlc.embed(courses), section_meetings.meeting_times
FROM section_meetings
JOIN sections ON sections."sequence" = section_meetings."sequence"
              AND sections.subject_code = section_meetings.subject_code
              AND sections.course_number = section_meetings.course_number
              AND sections.school_id = section_meetings.school_id
              AND sections.term_collection_id = section_meetings.term_collection_id
JOIN courses ON sections.subject_code = courses.subject_code
             AND sections.course_number = courses."number"
             AND sections.school_id = courses.school_id
WHERE sections.school_id = @school_id
      AND sections.term_collection_id = @term_collection_id
;
-- name: GetMostRecentTermCollection :many
SELECT sqlc.embed(term_collections) 
FROM term_collections t
JOIN previous_section_collections p 
                    ON t.school_id    = p.school_id
                    AND t.term_year   = p.term_year
                    AND t.term_season = p.term_season
                    AND t.season_kind = p.season_kind
ORDER BY p.time_collection DESC
LIMIT 1
;
