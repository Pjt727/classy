SELECT * FROM schools;
SELECT * FROM term_collections;
SELECT * FROM faculty_members;
SELECT * FROM staging_sections;
SELECT * FROM meeting_times;
SELECT MAX(counter) FROM
(
SELECT COUNT(*) counter FROM staging_meeting_times 
GROUP BY "sequence", section_id, term_season, course_id, school_id
ORDER BY counter
);
SELECT * FROM staging_meeting_times;
