-- name: SyncAll :many
WITH sync_diffs AS (
    SELECT 
           MIN(sequence) AS sequence, 
           table_name,
           MIN(input_at) AS updated_input_at,
           composite_hash,
           school_id,
           CASE
               WHEN table_name = 'schools' THEN pk_fields::jsonb
               ELSE jsonb_set(pk_fields::jsonb, '{school_id}', to_jsonb(school_id), true)
           END AS updated_pk_fields,
           combined_json(
                    (sync_action, relevant_fields)::sync_change
                    ORDER BY sequence
           ) AS sync_changes
    FROM historic_class_information
    WHERE sequence > @last_sequence
    GROUP BY composite_hash, table_name, composite_hash, school_id, updated_pk_fields
    -- if combined_json is NULL it means that it was deleted
    HAVING combined_json(
                    (sync_action, relevant_fields)::sync_change
                    ORDER BY sequence
           ) IS NOT NULL
)
SELECT sequence, table_name, updated_input_at AS input_at, composite_hash, school_id, updated_pk_fields AS pk_fields,
    (sync_changes).sync_action::sync_kind AS sync_action,
    (sync_changes).relevant_fields AS relevant_fields,
    COUNT(*) OVER() AS total_rows
FROM sync_diffs
ORDER BY sequence
LIMIT @max_records::int
;

-- name: SyncSchool :many
WITH sync_diffs AS (
    SELECT 
           MIN(sequence) AS sequence, 
           table_name,
           MIN(input_at) AS updated_input_at,
           composite_hash,
           school_id,
           CASE
               WHEN table_name = 'schools' THEN pk_fields::jsonb
               ELSE jsonb_set(pk_fields::jsonb, '{school_id}', to_jsonb(school_id), true)
           END AS updated_pk_fields,
           combined_json(
                    (sync_action, relevant_fields)::sync_change
                    ORDER BY sequence
           ) AS sync_changes
    FROM historic_class_information
    WHERE sequence > @last_sequence
          AND school_id = @school_id
    GROUP BY composite_hash, table_name, composite_hash, school_id, updated_pk_fields
    -- if combined_json is NULL it means that it was deleted
    HAVING combined_json(
                    (sync_action, relevant_fields)::sync_change
                    ORDER BY sequence
           ) IS NOT NULL
)
SELECT sequence, table_name, updated_input_at AS input_at, composite_hash, school_id, updated_pk_fields AS pk_fields,
    (sync_changes).sync_action::sync_kind AS sync_action,
    (sync_changes).relevant_fields AS relevant_fields,
    COUNT(*) OVER() AS total_rows
FROM sync_diffs
ORDER BY sequence
LIMIT @max_records::int
;

-- name: SyncTerm :many
-- gives only the data that is directly related with that term at any point of time
WITH sync_diffs AS (
    SELECT 
           MIN(sequence) AS sequence, 
           table_name,
           MIN(input_at) AS updated_input_at,
           composite_hash,
           school_id,
           CASE
               WHEN table_name = 'schools' THEN pk_fields::jsonb
               ELSE jsonb_set(pk_fields::jsonb, '{school_id}', to_jsonb(school_id), true)
           END AS updated_pk_fields,
           combined_json(
                    (sync_action, relevant_fields)::sync_change
                    ORDER BY sequence
           ) AS sync_changes
    FROM historic_class_information hc
    WHERE sequence > @last_sequence
          AND hc.school_id = @school_id
          AND (
              -- records that are/ were invovled in the term e.i. professors teaching a section in that term
              (composite_hash, table_name) IN (
                      SELECT h.historic_composite_hash, h.table_name
                      FROM historic_class_information_term_dependencies h
                      WHERE h.term_collection_id = @term_collection_id and h.school_id = @school_id
                  )
              -- sections / meeting times that are directly in the term
              OR (pk_fields ? 'term_collection_id' AND pk_fields ->> 'term_collection_id' = @term_collection_id)
              -- possible updated data on the school 
              OR table_name = 'schools'
              -- possible updated data on the term collection
              OR (table_name = 'term_collections' AND pk_fields ->> 'id' = @term_collection_id)
              )
    GROUP BY composite_hash, table_name, composite_hash, school_id, updated_pk_fields
    -- if combined_json is NULL it means that it was deleted
    HAVING combined_json(
                    (sync_action, relevant_fields)::sync_change
                    ORDER BY sequence
           ) IS NOT NULL
)
SELECT sequence, table_name, updated_input_at AS input_at, composite_hash, school_id, updated_pk_fields AS pk_fields,
    (sync_changes).sync_action::sync_kind AS sync_action,
    (sync_changes).relevant_fields AS relevant_fields,
    COUNT(*) OVER() AS total_rows
FROM sync_diffs
ORDER BY sequence
LIMIT @max_records::int
;
