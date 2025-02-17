-- "classes" is the term to reference everything associated
--   with any given section
CREATE VIEW section_meetings AS (
SELECT 
    s.sequence,
    s.school_id,
    s.term_collection_id,
    s.subject_code,
    s.course_number,
    JSON_AGG(
        JSON_BUILD_OBJECT(
            'start_date', mt.start_date,
            'end_date', mt.end_date,
            'meeting_type', mt.meeting_type,
            'start_minutes', mt.start_minutes,
            'end_minutes', mt.end_minutes,
            'is_monday', mt.is_monday,
            'is_tuesday', mt.is_tuesday,
            'is_wednesday', mt.is_wednesday,
            'is_thursday', mt.is_thursday,
            'is_friday', mt.is_friday,
            'is_saturday', mt.is_saturday,
            'is_sunday', mt.is_sunday
        )
    ) AS meeting_times
FROM sections s
JOIN meeting_times mt ON s.sequence   = mt.section_sequence
             AND s.school_id          = mt.school_id
             AND s.term_collection_id = mt.term_collection_id
             AND s.subject_code       = mt.subject_code
             AND s.course_number      = mt.course_number
GROUP BY (s.sequence, s.school_id, s.term_collection_id, s.subject_code, s.course_number)
);
