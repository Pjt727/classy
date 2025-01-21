SELECT * FROM terms;
SELECT * FROM schools;
SELECT * FROM classes;
SELECT * FROM term_collections
WHERE school_id = 'temple';
SELECT * FROM faculty_members;
SELECT * FROM staging_sections;
SELECT * FROM sections;
SELECT * FROM staging_meeting_times;
SELECT * FROM meeting_times
WHERE start_minutes != NULL
;
SELECT MAX(counter) FROM
(
SELECT COUNT(*) counter FROM staging_meeting_times 
GROUP BY "sequence", section_id, term_season, course_id, school_id
ORDER BY counter
);
SELECT * FROM staging_meeting_times 
WHERE (section_id, term_season, term_year, course_id, school_id) IN (SELECT id, term_season, term_year, course_id, school_id FROM sections);

SELECT * FROM sections

