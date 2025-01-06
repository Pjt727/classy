-- name: GetSchoolsClassesForTerm :many
SELECT sqlc.embed(sections), sqlc.embed(courses), sqlc.embed(meeting_times)
FROM sections
JOIN courses ON sections.course_id = courses.id
             AND sections.school_id = courses.school_id
             AND sections.term_year = courses.term_year
             AND sections.term_season = courses.term_season
JOIN meeting_times ON sections.id = meeting_times.section_id
             AND sections.school_id = meeting_times.school_id
             AND sections.term_year = meeting_times.term_year
             AND sections.term_season = meeting_times.term_season
WHERE sections.school_id = @schoolId
      AND sections.term_year = @termYear
      AND sections.term_season = @termSeason;
