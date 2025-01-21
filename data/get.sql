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

-- name: GetSchoolsClassesForTerm :many
SELECT sqlc.embed(sections), sqlc.embed(courses), sqlc.embed(meeting_times)
FROM sections
JOIN courses ON sections.course_id             = courses.id
             AND sections.school_id            = courses.school_id
JOIN meeting_times ON sections.id              = meeting_times.section_id
             AND sections.school_id            = meeting_times.school_id
             AND sections.term_collection_id   = meeting_times.term_collection_id
WHERE sections.school_id = @school_id
      AND sections.term_collection_id = @term_collection_id
;
-- name: GetMostRecentTermCollection :many
SELECT sqlc.embed(term_collections) 
FROM term_collections t
JOIN previous_full_section_collections p 
                    ON t.school_id    = p.school_id
                    AND t.term_year   = p.term_year
                    AND t.term_season = p.term_season
                    AND t.season_kind = p.season_kind
ORDER BY p.time_collection DESC
LIMIT 1
;
