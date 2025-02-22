SELECT * FROM terms;
SELECT id, description FROM courses WHERE school_id = 'temple';
SELECT * FROM schools;
SELECT * FROM term_collections
WHERE school_id = 'temple';
SELECT * FROM faculty_members;
SELECT * FROM sections;
SELECT course_id FROM staging_sections WHERE school_id = 'temple' GROUP BY course_id;
SELECT * FROM sections WHERE school_id = 'temple' AND subject_code = 'JPNS' AND course_number = '2111';
SELECT * FROM courses
WHERE school_id = 'temple' AND subject_code = 'JPNS' AND number = '2111';
SELECT * FROM sections WHERE school_id = 'marist';
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

SELECT COUNT(*) FROM sections WHERE school_id = 'temple';

SELECT CASE 
        WHEN EXISTS (
            SELECT 1
            FROM schools
            WHERE id = 'marist'
        ) THEN true
    ELSE false
END;

SELECT * FROM section_meetings LIMIT 200;
SELECT * FROM meeting_times LIMIT 10;
SELECT * FROM courses;
