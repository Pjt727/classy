-- "classes" is the term to reference everything associated
--   with any given section
CREATE VIEW section_meetings AS (
SELECT 
    s.sequence,
    s.term_collection_id,
    s.subject_code,
    s.course_number,
    s.school_id,
    JSON_AGG(
        JSON_BUILD_OBJECT(
            'start_date',    mt.start_date,
            'end_date',      mt.end_date,
            'meeting_type',  mt.meeting_type,
            'start_minutes', mt.start_minutes,
            'end_minutes',   mt.end_minutes,
            'is_monday',     mt.is_monday,
            'is_tuesday',    mt.is_tuesday,
            'is_wednesday',  mt.is_wednesday,
            'is_thursday',   mt.is_thursday,
            'is_friday',     mt.is_friday,
            'is_saturday',   mt.is_saturday,
            'is_sunday',     mt.is_sunday
        )
    ) AS meeting_times
FROM sections s
JOIN meeting_times mt ON s.sequence   = mt.section_sequence
             AND s.school_id          = mt.school_id
             AND s.term_collection_id = mt.term_collection_id
             AND s.subject_code       = mt.subject_code
             AND s.course_number      = mt.course_number
GROUP BY (
    s.sequence,
    s.term_collection_id,
    s.subject_code,
    s.course_number,
    s.school_id
)
);

CREATE VIEW course_heuristic AS (
SELECT
    c.subject_code,
    c.number,
    c.school_id,
    JSON_AGG(
        DISTINCT JSONB_BUILD_OBJECT(
            'id', p.id,
            'name', p.name,
            'email_address', p.email_address,
            'first_name', p.first_name,
            'last_name', p.last_name
        )
    ) AS previous_professors,
    JSON_AGG(
        DISTINCT JSONB_BUILD_OBJECT(
            'id', t.id,
            'year', t.year,
            'season', t.season,
            'name', t.name,
            'still_collecting', t.still_collecting
        )
    ) AS previous_terms
FROM courses c
LEFT JOIN sections s ON s.school_id = c.school_id
           AND s.course_number = c.number
           AND s.subject_code = c.subject_code
LEFT JOIN professors p ON p.school_id = s.school_id
           AND p.id = s.primary_professor_id
LEFT JOIN term_collections t ON t.school_id = s.school_id
           AND t.id = s.term_collection_id
GROUP BY (
    c.subject_code,
    c.number,
    c.school_id
)
);


CREATE VIEW sync_diffs AS (
SELECT h1.table_name,
       h1.input_at AS updated_input_at,
       h1.composite_hash,
       h1.school_id,
       jsonb_set(h1.pk_fields::jsonb, '{school_id}', to_jsonb(h1.school_id), true) AS updated_pk_fields, 
       (SELECT combined_json(
                (h2.sync_action, h2.relevant_fields)::sync_change
                ORDER BY h2.input_at
       )
       FROM historic_class_information h2
       WHERE h2.school_id = h1.school_id
             AND h2.table_name = h1.table_name
             AND h2.composite_hash = h1.composite_hash
             AND h2.input_at >= h1.input_at
       ).sync_action AS sync_action,
       (SELECT combined_json(
                (h2.sync_action, h2.relevant_fields)::sync_change
                ORDER BY h2.input_at
       )
       FROM historic_class_information h2
       WHERE h2.school_id = h1.school_id
             AND h2.table_name = h1.table_name
             AND h2.composite_hash = h1.composite_hash
             AND h2.input_at >= h1.input_at
       ).relevant_fields AS relevant_fields
FROM 
    historic_class_information h1
ORDER BY h1.input_at
);

