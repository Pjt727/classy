-- "classes" is the term to reference everything associated
--   with any given section
CREATE VIEW classes AS (
SELECT 
    JSON_AGG(sections.*) AS section,
    JSON_AGG(courses.*) AS course,
    JSON_AGG(meeting_times.*) AS meeting_times
FROM sections
JOIN courses ON sections.course_id             = courses.id
             AND sections.school_id            = courses.school_id
JOIN meeting_times ON sections.id              = meeting_times.section_id
             AND sections.school_id            = meeting_times.school_id
             AND sections.term_collection_id   = meeting_times.term_collection_id
GROUP BY (sections.id, sections.school_id, sections.term_collection_id)
);
