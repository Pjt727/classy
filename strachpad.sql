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

SELECT count(*) FROM historic_class_information;

SELECT composite_hash, pk_fields, relevant_fields FROM historic_class_information
WHERE "table_name" = 'sections'
ORDER BY composite_hash, input_at;


SELECT composite_hash, count(*) AS total FROM historic_class_information
GROUP BY composite_hash ORDER BY total;

SELECT * FROM historic_class_information WHERE 
composite_hash = 'c8e548557957ae53832f91e657ebfaf1'
ORDER BY input_at;

SELECT * FROM meeting_times m WHERE m."sequence" = 0
AND m.subject_code = 'MUS'
AND m.course_number = '236N'
AND m.section_sequence = '111';

-- {"end_date": "2024-12-13T00:00:00", "is_friday": false, "is_monday": true, "is_sunday": false, "is_tuesday": false, "start_date": "2024-08-26T00:00:00", "end_minutes": "16:45:00", "is_saturday": false, "is_thursday": false, "is_wednesday": true, "meeting_type": "LEC", "start_minutes": "15:30:00"}
-- {"end_minutes": "18:15:00", "start_minutes": "17:00:00"}
-- {"end_minutes": "21:15:00", "is_thursday": true, "is_wednesday": false, "start_minutes": "20:00:00"}
--

SELECT composite_hash, combined_json((sync_action, relevant_fields)::sync_change ORDER BY input_at)
FROM historic_class_information 
GROUP BY composite_hash;
