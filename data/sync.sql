-- name: SyncAll :many
WITH sync_diffs AS (
    SELECT 
           MAX(sequence) AS sequence, 
           table_name,
           MAX(input_at) AS updated_input_at,
           composite_hash,
           school_id,
           ANY_VALUE(CASE
                    WHEN table_name = 'schools' THEN pk_fields::jsonb
                    ELSE jsonb_set(pk_fields::jsonb, '{school_id}', to_jsonb(school_id), true)
           END) AS updated_pk_fields,
           combined_json(
                    (sync_action, relevant_fields)::sync_change
                    ORDER BY sequence
           ) AS sync_changes
    FROM historic_class_information
    WHERE sequence > @last_sequence
    GROUP BY composite_hash, table_name, school_id
)
SELECT sequence::int, table_name, updated_input_at AS input_at, composite_hash, school_id, updated_pk_fields AS pk_fields,
    (sync_changes).sync_action::sync_kind AS sync_action,
    (sync_changes).relevant_fields AS relevant_fields,
    COUNT(*) OVER() AS total_rows
FROM sync_diffs
WHERE (sync_changes).sync_action::sync_kind IS NOT NULL
ORDER BY sequence
LIMIT @max_records::int
;

-- name: SyncSchool :many
WITH sync_diffs AS (
    SELECT 
           MAX(sequence) AS sequence, 
           table_name,
           MAX(input_at) AS updated_input_at,
           composite_hash,
           school_id,
           ANY_VALUE(CASE
               WHEN table_name = 'schools' THEN pk_fields::jsonb
               ELSE jsonb_set(pk_fields::jsonb, '{school_id}', to_jsonb(school_id), true)
           END) AS updated_pk_fields,
           combined_json(
                    (sync_action, relevant_fields)::sync_change
                    ORDER BY sequence
           ) AS sync_changes
    FROM historic_class_information
    WHERE sequence > @last_sequence
          AND school_id = @school_id
    GROUP BY composite_hash, table_name, school_id
)
SELECT sequence::int, table_name, updated_input_at AS input_at, composite_hash, school_id, updated_pk_fields AS pk_fields,
    (sync_changes).sync_action::sync_kind AS sync_action,
    (sync_changes).relevant_fields AS relevant_fields,
    COUNT(*) OVER() AS total_rows
FROM sync_diffs
WHERE (sync_changes).sync_action::sync_kind IS NOT NULL
ORDER BY sequence
LIMIT @max_records::int
;

-- name: SyncTerms :many
-- 1. Get all the sequences that are already covered
-- 2. Get all sequences
WITH 
    included_commons AS (
          SELECT h.historic_composite_hash, h.table_name
          FROM historic_class_information_term_dependencies h
          WHERE h.term_collection_id = ANY(@term_exclusion_collection_ids::TEXT[])
                AND h.school_id = @school_id
    ),
    sync_diffs AS (
    SELECT 
           MAX(sequence) AS sequence, 
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
                    ORDER BY sequence
           ) AS sync_changes
    FROM historic_class_information hc
    -- UNNESTS are not supported in sqlc so using json work around
    -- https://github.com/sqlc-dev/sqlc/issues/958 :(
    -- JOIN UNNEST(@term_collection_ids::TEXT[], @term_sequences::INTEGER[]) AS checks(term_collection_id, term_sequence)
    JOIN (
        SELECT
            value ->> 'id' AS term_collection_id,
            (value ->> 'sequence')::INTEGER AS term_sequence
        FROM jsonb_array_elements(@term_collection_sequence_pairs::jsonb)
    ) AS checks
        ON hc.sequence > checks.term_sequence AND (
        -- sections / meeting times that are directly in the term
        (pk_fields ? 'term_collection_id' AND pk_fields ->> 'term_collection_id' = checks.term_collection_id)
        -- possible updated data on the school
        OR table_name = 'schools'
        -- possible updated data on the term collection
        OR (table_name = 'term_collections' AND pk_fields ->> 'id' = checks.term_collection_id)
        -- related  
        OR (hc.composite_hash, hc.table_name) IN (SELECT * FROM included_commons)
    )
    WHERE school_id = @school_id
    GROUP BY hc.composite_hash, hc.table_name, hc.school_id
    )
SELECT sequence::int, table_name, updated_input_at AS input_at, composite_hash, school_id, updated_pk_fields AS pk_fields,
    (sync_changes).sync_action::sync_kind AS sync_action,
    (sync_changes).relevant_fields AS relevant_fields,
    COUNT(*) OVER() AS total_rows
FROM sync_diffs
WHERE (sync_changes).sync_action::sync_kind IS NOT NULL
ORDER BY sequence
LIMIT @max_records::int
;
