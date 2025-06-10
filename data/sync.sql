-- name: GetLastestSyncChanges :many
SELECT
  sequence,
  table_name,
  pk_fields,
  sync_action,
  relevant_fields
FROM
(SELECT
    *,
    ROW_NUMBER() OVER (PARTITION BY school_id, table_name, composite_hash ORDER BY sequence ASC) AS rn
    FROM
    sync_diffs s
    WHERE sync_diffs.sequence > @last_sequence
) as RankedData
WHERE
  rn = 1;

-- name: GetLastSequence :one
SELECT MAX(sequence)::int FROM historic_class_information;

-- name: GetLastestSyncChangesForTerms :many
-- all array inputs must be flattened to be the same length
SELECT
  sequence,
  table_name,
  pk_fields,
  sync_action,
  relevant_fields
FROM
(SELECT
    *,
    ROW_NUMBER() OVER (PARTITION BY school_id, table_name, composite_hash ORDER BY sequence ASC) AS rn
    FROM
    sync_diffs
    WHERE (updated_pk_fields->'term_collection_id' IS NULL AND EXISTS ( -- see if this improves performance
            SELECT 1
            FROM generate_series(1, array_length(@common_tables::string[], 1)) AS i
            WHERE school_id = (@school_id::string[])[i]
            AND table_name = (@common_tables::string[])[i]
            AND sequence > (@common_sequences::int[])[i]
            ))
            OR (
                SELECT 1
                FROM generate_series(1, array_length(@term_collection_id::string[], 1)) AS i
                WHERE school_id = (@school_id::string[])[i]
                AND term_collection_id = (@term_collection_id::string[])[i]
                AND sequence > (@term_collection_sequences::int[])[i]
            )) as RankedData
WHERE
  rn = 1;
