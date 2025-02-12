-- "classes" is the term to reference everything associated
--   with any given section
-- CREATE VIEW classes AS (
-- SELECT 
--     ROW_TO_JSON(sections.*) AS section,
--     ROW_TO_JSON(courses.*) AS course,
--     JSON_AGG(meeting_times.*) AS meeting_times
-- FROM sections
-- JOIN courses ON sections.course_id             = courses.id
--              AND sections.school_id            = courses.school_id
-- JOIN meeting_times ON sections.id              = meeting_times.section_id
--              AND sections.school_id            = meeting_times.school_id
--              AND sections.term_collection_id   = meeting_times.term_collection_id
-- GROUP BY (sections.*, courses.*)
-- );
-- todo reformat AS a small JOIN AND the other queries will USE this WITH INNER joins
--      TO GET the correct types
CREATE VIEW section_meetings AS (
SELECT 
    s.sequence,
    s.school_id,
    s.term_collection_id,
    s.subject_code,
    s.course_number,
    JSON_AGG(meeting_times.*) AS meeting_times
FROM sections s
JOIN meeting_times ON s.sequence              = meeting_times.section_sequence
             AND s.school_id            = meeting_times.school_id
             AND s.term_collection_id   = meeting_times.term_collection_id
GROUP BY (s.sequence, s.school_id, s.term_collection_id, s.subject_code, s.course_number)
);
