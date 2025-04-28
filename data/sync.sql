-- name: GetLastestSyncChanges :many
SELECT table_name, updated_pk_fields AS pk_fields, sync_action, relevant_fields
FROM sync_diffs WHERE (school_id, table_name, composite_hash, updated_input_at) IN (
    SELECT s.school_id, s.table_name, s.composite_hash, MIN(s.updated_input_at)
    FROM sync_diffs s
    WHERE s.sequence > @last_sequence
    GROUP BY s.school_id, s.table_name, s.composite_hash
);


-- name: GetLastSequence :one
SELECT MAX(sequence)::int FROM historic_class_information;

-- name: GetLastestSyncChangesForTerms :many
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
            UNNEST(@t_school_ids::string[]) WITH ORDINALITY AS sch_id,
            UNNEST(@t_term_collection_ids::string[]) WITH ORDINALITY AS term_collection_id,
            UNNEST(@t_last_sequences::int[]) WITH ORDINALITY AS seq
        WHERE h.school_id = sch_id
              AND h.pk_fields->'term_collection_id' = term_collection_id 
              AND h.sequence > seq
    )
    GROUP BY h.school_id, h.table_name, h.composite_hash
) OR sequence IN (
    SELECT MIN(h.sequence)
    FROM historic_class_information h
    WHERE h.pk_fields->'term_collection_id' IS NULL AND EXISTS (
        SELECT 1
        FROM
            UNNEST(@s_school_ids::string[]) WITH ORDINALITY AS sch_id,
            UNNEST(@s_last_sequences::int[]) WITH ORDINALITY AS seq
        WHERE h.school_id = sch_id 
            AND h.sequence > seq
    )
    GROUP BY h.school_id, h.table_name, h.composite_hash
);

-- name: GetLastSyncTimesForTerms :many
SELECT school_id, (pk_fields->'term_collection_id')::text as term_collection_id, sequence as last_sequence
FROM historic_class_information
WHERE sequence IN (
    SELECT MAX(h.sequence)
    FROM historic_class_information h
    WHERE EXISTS (
        SELECT 1
        FROM 
            UNNEST(@school_ids::string[]) WITH ORDINALITY AS sch_id,
            UNNEST(@term_collection_ids::string[]) WITH ORDINALITY AS term_collection_id
        WHERE h.school_id = sch_id
              AND h.pk_fields->'term_collection_id' = term_collection_id 
    )
    GROUP BY h.school_id, h.table_name, h.composite_hash
);

-- name: GetLastCommonSequences :many
SELECT school_id, MAX(sequence)::int as last_sequence
FROM historic_class_information
WHERE pk_fields->'term_collection_id' IS NULL
      AND school_id IN (@school_ids::string[])
GROUP BY school_id;

