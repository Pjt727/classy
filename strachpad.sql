WITH included_commons AS (
    SELECT h.historic_composite_hash, h.table_name, h.term_collection_id
    FROM historic_class_information_term_dependencies h
    ),
    annotated_historic_info AS (
    SELECT hc1.*, (hc1.sequence > checks.term_sequence) AS not_synced FROM historic_class_information hc1
    -- UNNESTS are not supported in sqlc so using json work around
    -- https://github.com/sqlc-dev/sqlc/issues/958 :(
    -- JOIN UNNEST(@term_collection_ids::TEXT[], @term_sequences::INTEGER[]) AS checks(term_collection_id, term_sequence)
    JOIN (
        SELECT
            value ->> 'id' AS term_collection_id,
            (value ->> 'sequence')::INTEGER AS term_sequence
        FROM jsonb_array_elements('[{"id": "202440", "sequence": 0},{"id": "202540", "sequence": 12406}]'::jsonb)
    ) AS checks ON (
        -- sections / meeting times that are directly in the term
        (pk_fields ? 'term_collection_id' AND pk_fields ->> 'term_collection_id' = checks.term_collection_id)
        -- possible updated data on the school
        OR table_name = 'schools'
        -- possible updated data on the term collection
        OR (table_name = 'term_collections' AND pk_fields ->> 'id' = checks.term_collection_id)
        -- related  
        OR (hc1.composite_hash, hc1.table_name, checks.term_collection_id) IN (SELECT * FROM included_commons)
    )
)select sequence, not_synced from annotated_historic_info order by sequence
    ),
    historic_subset AS (
    -- SELECT DISTINCT a1.sequence, a1.table_name, a1.input_at, 
    --        a1.composite_hash, a1.school_id, a1.pk_fields, 
    --        a1.sync_action, a1.relevant_fields
    SELECT a1.sequence, a1.table_name, a1.input_at, 
           a1.composite_hash, a1.school_id, a1.pk_fields, 
           a1.sync_action, a1.relevant_fields, BOOL_AND(a1.not_synced)
    FROM annotated_historic_info a1
    WHERE (a1.composite_hash, a1.table_name) IN (
        SELECT a2.composite_hash, a2.table_name 
        FROM annotated_historic_info a2
        GROUP BY a2.composite_hash, a2.table_name
        -- HAVING BOOL_AND(a2.not_synced)
    )
    GROUP BY a1.sequence, a1.table_name, a1.input_at, 
            a1.composite_hash, a1.school_id, a1.pk_fields, 
            a1.sync_action, a1.relevant_fields
    ORDER BY a1.sequence
    )
-- select * from historic_subset order by sequence
    sync_diffs AS (
    SELECT 
           MAX(hc.sequence) AS sequence, 
           hc.table_name,
           MAX(input_at) AS updated_input_at,
           hc.composite_hash,
           hc.school_id,
           ANY_VALUE(CASE
               WHEN table_name = 'schools' THEN pk_fields::jsonb
               ELSE jsonb_set(pk_fields::jsonb, '{school_id}', to_jsonb(school_id), true)
           END) AS updated_pk_fields,
           combined_json(
                    (sync_action, relevant_fields)::sync_change
                    ORDER BY hc.sequence
           ) AS sync_changes
    FROM historic_subset hc
    GROUP BY hc.composite_hash, hc.table_name, hc.school_id
    )
SELECT sequence::int, table_name, updated_input_at AS input_at, composite_hash, school_id, updated_pk_fields AS pk_fields,
    (sync_changes).sync_action::sync_kind AS sync_action,
    (sync_changes).relevant_fields AS relevant_fields,
    COUNT(*) OVER() AS total_rows
FROM sync_diffs
WHERE (sync_changes).sync_action::sync_kind IS NOT NULL
ORDER BY sequence
;



-- 41 | courses          | 2025-08-01 15:50:05.517334+00 | 32a65cc5116aff1ecad78f4fac234874 | marist    | {"number": "194N", "school_id": "marist", "subject_code": "ART"}                                                                                   | insert      | {"other": null, "title": "ST:FOUNDATIONS OF PHOTOGRAPHY", "description": null, "corequisites": null, "credit_hours": 3, "prerequisites": null, "subject_description": "Art"}                                                                                                                                              |       4883
select * from historic_class_information_term_dependencies where historic_composite_hash = '32a65cc5116aff1ecad78f4fac234874'
SELECT * FROM historic_class_information;
     -- 8156 | marist    | sections         | 90f5996126542fdcf0cf981fd948b60b | 2025-08-01 20:36:59.212891+00 | {"sequence": "103", "subject_code": "HONR", "course_number": "401L", "term_collection_id": "202540"}                        | insert      | {"other": null, "campus": "Marist University Campus", "enrollment": 3, "max_enrollment": 0, "instruction_method": "", "primary_professor_id": "Joanne.Gavin@marist.edu"}                                                                                                                                                  |                          4
     -- 7597 | courses          | 2025-08-01 20:36:59.212891+00 | 286b3c01ed719b0abd75855c826db495 | marist    | {"number": "477N", "subject_code": "ACCT"}                                                                                  | insert      | {"other": null, "title": "CUR ISSUES ACCT", "description": null, "corequisites": null, "credit_hours": 3, "prerequisites": null, "subject_description": "Accounting"}                                                                                                                                                     | f

WITH included_commons AS (
    SELECT h.historic_composite_hash, h.table_name, h.term_collection_id
    FROM historic_class_information_term_dependencies h
    ),
    annotated_historic_info AS (
    SELECT hc1.*, (hc1.sequence > checks.term_sequence) AS not_synced FROM historic_class_information hc1
    -- UNNESTS are not supported in sqlc so using json work around
    -- https://github.com/sqlc-dev/sqlc/issues/958 :(
    -- JOIN UNNEST(@term_collection_ids::TEXT[], @term_sequences::INTEGER[]) AS checks(term_collection_id, term_sequence)
    JOIN (
        SELECT
            value ->> 'id' AS term_collection_id,
            (value ->> 'sequence')::INTEGER AS term_sequence
        FROM jsonb_array_elements('[{"id": "202440", "sequence": 0},{"id": "202540", "sequence": 12406}]'::jsonb)
    ) AS checks ON (
        -- sections / meeting times that are directly in the term
        (pk_fields ? 'term_collection_id' AND pk_fields ->> 'term_collection_id' = checks.term_collection_id)
        -- possible updated data on the school
        OR table_name = 'schools'
        -- possible updated data on the term collection
        OR (table_name = 'term_collections' AND pk_fields ->> 'id' = checks.term_collection_id)
        -- related  
        OR (hc1.composite_hash, hc1.table_name, checks.term_collection_id) IN (SELECT * FROM included_commons)
    )
-- )select sequence, not_synced from annotated_historic_info order by sequence
    ),
    historic_subset AS (
    SELECT DISTINCT a1.sequence, a1.table_name, a1.input_at, 
           a1.composite_hash, a1.school_id, a1.pk_fields, 
           a1.sync_action, a1.relevant_fields
    FROM annotated_historic_info a1
    WHERE (a1.composite_hash, a1.table_name) IN (
        SELECT a2.composite_hash, a2.table_name 
        FROM annotated_historic_info a2
        GROUP BY a2.composite_hash, a2.table_name
        HAVING BOOL_AND(a2.not_synced)
    )
    ORDER BY a1.sequence
    ),
-- select * from historic_subset order by sequence
    sync_diffs AS (
    SELECT 
           MAX(hc.sequence) AS sequence, 
           hc.table_name,
           MAX(input_at) AS updated_input_at,
           hc.composite_hash,
           hc.school_id,
           ANY_VALUE(CASE
               WHEN table_name = 'schools' THEN pk_fields::jsonb
               ELSE jsonb_set(pk_fields::jsonb, '{school_id}', to_jsonb(school_id), true)
           END) AS updated_pk_fields,
           combined_json(
                    (sync_action, relevant_fields)::sync_change
                    ORDER BY hc.sequence
           ) AS sync_changes
    FROM historic_subset hc
    GROUP BY hc.composite_hash, hc.table_name, hc.school_id
    )
SELECT sequence::int, table_name, updated_input_at AS input_at, composite_hash, school_id, updated_pk_fields AS pk_fields,
    (sync_changes).sync_action::sync_kind AS sync_action,
    (sync_changes).relevant_fields AS relevant_fields,
    COUNT(*) OVER() AS total_rows
FROM sync_diffs
WHERE (sync_changes).sync_action::sync_kind IS NOT NULL
ORDER BY sequence
;

ALTER DATABASE classy REFRESH COLLATION VERSION;

select * from schools;
INSERT INTO schools (id, name) VALUES ('foo', 'bar')

select * from historic_class_information_term_dependencies













EXPLAIN ANALYZE WITH 
    -- getting the sequences from common tables which are not synced by any of the terms
    -- common tables are tables which are shared among different terms (currently just professors and courses)
    -- import things to consider:
    -- * a dependent record will always have its first sequence less than the sequence the dependent was added
    --       * this is because the dependency is created for a section which relies on the previosly create dependent record
    -- * still a dependent record may have a sequence greater than the sequence of the section which created the dependency
    --       * this happens if the dependent record gets updated after it was added a dependent
    -- this cte works by matching all the depedent records with the section sequence their dependency was introducted with
    -- the last sequence synced must would have included the class information record if the last sequence is >= to the sequence the
    --    depedent was introduced in (h.first_sequence) AND the possible update(s) for the depedent record thus the MAX
    -- the aggregate is it ensure that this has not been synced by any of the term sequences
    included_commons AS (
    SELECT hc.sequence, hc.table_name, hc.input_at, 
           hc.composite_hash, hc.school_id, hc.pk_fields, 
           hc.sync_action, hc.relevant_fields
    FROM historic_class_information_term_dependencies h
    JOIN historic_class_information hc ON hc.table_name = h.table_name
                                       AND hc.composite_hash = h.historic_composite_hash
    JOIN (
        SELECT
            value ->> 'id' AS term_collection_id,
            (value ->> 'sequence')::INTEGER AS term_sequence
        FROM jsonb_array_elements('[{"id": "202440", "sequence": 0},{"id": "202540", "sequence": 12406}]'::jsonb)
    ) AS checks ON h.term_collection_id = checks.term_collection_id
    GROUP BY hc.sequence, hc.table_name, hc.input_at, 
           hc.composite_hash, hc.school_id, hc.pk_fields, 
           hc.sync_action, hc.relevant_fields
    HAVING BOOL_AND(GREATEST(h.first_sequence, hc.sequence) > checks.term_sequence)
    ),
    included_term_data AS (
    SELECT hc1.sequence, hc1.table_name, hc1.input_at, 
           hc1.composite_hash, hc1.school_id, hc1.pk_fields, 
           hc1.sync_action, hc1.relevant_fields
    FROM historic_class_information hc1
    -- UNNESTS are not supported in sqlc so using json work around
    -- https://github.com/sqlc-dev/sqlc/issues/958 :(
    -- JOIN UNNEST(@term_collection_ids::TEXT[], @term_sequences::INTEGER[]) AS checks(term_collection_id, term_sequence)
    JOIN (
        SELECT
            value ->> 'id' AS term_collection_id,
            (value ->> 'sequence')::INTEGER AS term_sequence
        FROM jsonb_array_elements('[{"id": "202440", "sequence": 0},{"id": "202540", "sequence": 12406}]'::jsonb)
    ) AS checks ON (
        -- sections / meeting times that are directly in the term
        (pk_fields ? 'term_collection_id' AND pk_fields ->> 'term_collection_id' = checks.term_collection_id)
        -- possible updated data on the school
        OR table_name = 'schools'
        -- possible updated data on the term collection
        OR (table_name = 'term_collections' AND pk_fields ->> 'id' = checks.term_collection_id)
    )
    GROUP BY hc1.sequence, hc1.table_name, hc1.input_at, 
           hc1.composite_hash, hc1.school_id, hc1.pk_fields, 
           hc1.sync_action, hc1.relevant_fields
    HAVING BOOL_AND(hc1.sequence > checks.term_sequence)
    ),
    historic_data_to_sync AS (
        SELECT DISTINCT ic.sequence, ic.table_name, ic.input_at, 
               ic.composite_hash, ic.school_id, ic.pk_fields, 
               ic.sync_action, ic.relevant_fields
        FROM included_commons ic
        UNION ALL
        SELECT DISTINCT it.sequence, it.table_name, it.input_at, 
               it.composite_hash, it.school_id, it.pk_fields, 
               it.sync_action, it.relevant_fields
        FROM included_term_data it
    ),
    sync_diffs AS (
    SELECT 
           MAX(hc.sequence) AS sequence, 
           hc.table_name,
           MAX(input_at) AS updated_input_at,
           hc.composite_hash,
           hc.school_id,
           ANY_VALUE(CASE
               WHEN table_name = 'schools' THEN pk_fields::jsonb
               ELSE jsonb_set(pk_fields::jsonb, '{school_id}', to_jsonb(school_id), true)
           END) AS updated_pk_fields,
           combined_json(
                    (sync_action, relevant_fields)::sync_change
                    ORDER BY hc.sequence
           ) AS sync_changes
    FROM (SELECT * FROM historic_data_to_sync hc ORDER BY sequence) as hc
    GROUP BY hc.composite_hash, hc.table_name, hc.school_id
    )
SELECT sequence::int, table_name, updated_input_at AS input_at, composite_hash, school_id, updated_pk_fields AS pk_fields,
    (sync_changes).sync_action::sync_kind AS sync_action,
    (sync_changes).relevant_fields AS relevant_fields,
    COUNT(*) OVER() AS total_rows
FROM sync_diffs
WHERE (sync_changes).sync_action::sync_kind IS NOT NULL
ORDER BY sequence
;


select * from historic_class_information
