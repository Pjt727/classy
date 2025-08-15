-- name: SyncAll :many
WITH historic_subset AS (
    SELECT * FROM historic_class_information
    WHERE sequence > @last_sequence
    ORDER BY sequence
    LIMIT @max_records::int
    ),
    sync_diffs AS (
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
    FROM historic_subset
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
;

-- name: SyncSchool :many
WITH historic_subset AS (
    SELECT * FROM historic_class_information
    WHERE sequence > @last_sequence
          AND school_id = @school_id
    ORDER BY sequence
    LIMIT @max_records::int
    ),
    sync_diffs AS (
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
    FROM historic_subset
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
-- excludes the data that's already synced among given terms
-- including common data such as professor's that have already been synced form a different term
WITH 
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
    school_historic_class_information AS (
    SELECT * FROM historic_class_information hc
    WHERE hc.school_id = @school_id
    ),
    included_commons AS (
    SELECT hc.sequence, hc.table_name, hc.input_at, 
           hc.composite_hash, hc.school_id, hc.pk_fields, 
           hc.sync_action, hc.relevant_fields
    FROM historic_class_information_term_dependencies h
    JOIN school_historic_class_information hc ON hc.table_name = h.table_name
                                       AND hc.composite_hash = h.historic_composite_hash
    JOIN (
        SELECT
            value ->> 'id' AS term_collection_id,
            (value ->> 'sequence')::INTEGER AS term_sequence
        FROM jsonb_array_elements(@common_term_collection_sequence_pairs::jsonb)
    ) AS checks ON h.term_collection_id = checks.term_collection_id
    GROUP BY hc.sequence, hc.table_name, hc.input_at, 
           hc.composite_hash, hc.school_id, hc.pk_fields, 
           hc.sync_action, hc.relevant_fields
    HAVING BOOL_AND((GREATEST(h.first_sequence, hc.sequence) > checks.term_sequence)::bool)
    ),
    included_term_data AS (
    SELECT hc1.sequence, hc1.table_name, hc1.input_at, 
           hc1.composite_hash, hc1.school_id, hc1.pk_fields, 
           hc1.sync_action, hc1.relevant_fields
    FROM school_historic_class_information hc1
    -- UNNESTS are not supported in sqlc so using json work around
    -- https://github.com/sqlc-dev/sqlc/issues/958 :(
    -- JOIN UNNEST(@term_collection_ids::TEXT[], @term_sequences::INTEGER[]) AS checks(term_collection_id, term_sequence)
    JOIN (
        SELECT
            value ->> 'id' AS term_collection_id,
            (value ->> 'sequence')::INTEGER AS term_sequence
        FROM jsonb_array_elements(@term_collection_sequence_pairs::jsonb)
    ) AS checks ON (
        -- sections / meeting times that are directly in the term
        (pk_fields ? 'term_collection_id' AND pk_fields ->> 'term_collection_id' = checks.term_collection_id)
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
    FROM (SELECT * FROM historic_data_to_sync hc ORDER BY sequence LIMIT @max_records) AS hc
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
