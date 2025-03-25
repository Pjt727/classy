-- name: GetLastestSyncChanges :many
SELECT table_name, updated_pk_fields AS pk_fields, sync_action, relevant_fields
FROM sync_diffs WHERE (school_id, table_name, composite_hash, updated_input_at) IN (
    SELECT s.school_id, s.table_name, s.composite_hash, MIN(s.updated_input_at)
    FROM sync_diffs s
    WHERE s.sequence > @last_sequence
    GROUP BY s.school_id, s.table_name, s.composite_hash
)
;


-- name: GetLastSyncTime :one
SELECT MAX(updated_input_at)::timestamptz FROM historic_class_information;

-- name: GetLastestSyncChangesForTerm :many
-- The strategy of this query is get the next sync diff which is after
--    the last sync
SELECT table_name, updated_pk_fields AS pk_fields, sync_action, relevant_fields
FROM sync_diffs
WHERE sequence IN (
    SELECT MIN(h.sequence)
    FROM historic_class_information h
    WHERE EXISTS (
        SELECT 1
        FROM 
            UNNEST(@school_ids::string[]) WITH ORDINALITY AS sch_id,
            UNNEST(@term_collection_ids::string[]) WITH ORDINALITY AS term_collection_id,
            UNNEST(@last_term_sequences::int[]) WITH ORDINALITY AS seq
        WHERE h.school_id = sch_id
              AND h.term_collection_id = term_collection_id 
              AND h.sequence > seq
    )
    GROUP BY h.school_id, h.table_name, h.composite_hash
) OR sequence IN (
    SELECT MIN(h.sequence)
    FROM historic_class_information h
    WHERE h.term_collection_id IS NULL AND h.sequence > @last_common_sequence
    GROUP BY h.school_id, h.table_name, h.composite_hash
);

-- name: GetLastSyncTimesForTerm :one
SELECT table_name, MAX(updated_input_at)::timestamptz 
FROM historic_class_information
WHERE school_id = @school_id
      AND (relevant_fields->'term_collection_id' IS NULL 
            OR relevant_fields->'term_collection_id' = @term_collection_id)
GROUP BY table_name;
