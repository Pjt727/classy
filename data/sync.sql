-- name: SyncAll :many
SELECT
  sequence,
  table_name,
  pk_fields,
  sync_action,
  relevant_fields,
  COUNT(*) OVER() AS total_rows
FROM
(SELECT
    *,
    ROW_NUMBER() OVER (PARTITION BY school_id, table_name, composite_hash ORDER BY sequence ASC) AS rn
    FROM
    sync_diffs s
    WHERE s.sequence > @last_sequence
) as RankedData
WHERE
  rn = 1
ORDER BY sequence
LIMIT @max_records::int
;

-- name: SyncSchool :many
SELECT
  sequence,
  table_name,
  pk_fields,
  sync_action,
  relevant_fields,
  COUNT(*) OVER() AS total_rows
FROM
(SELECT
    *,
    ROW_NUMBER() OVER (PARTITION BY school_id, table_name, composite_hash ORDER BY sequence ASC) AS rn
    FROM
    sync_diffs s
    WHERE s.sequence > @last_sequence 
          AND s.school_id = @school_id
) as RankedData
WHERE
  rn = 1
ORDER BY sequence
LIMIT @max_records::int
;

-- name: SyncTerm :many
-- gives only the data that is directly related with that term at any point of time
SELECT
  sequence,
  table_name,
  pk_fields,
  sync_action,
  relevant_fields,
  COUNT(*) OVER() AS total_rows
FROM
(SELECT
    *,
    ROW_NUMBER() OVER (PARTITION BY school_id, table_name, composite_hash ORDER BY sequence ASC) AS rn
    FROM
    sync_diffs s
    WHERE 
    s.school_id = @school_id
    AND s.sequence > @last_term_sequence
    AND (
    -- records that are/ were invovled in the term e.i. professors teaching a section in that term
    (s.composite_hash, s.table_name) IN (
            SELECT h.historic_composite_hash, h.table_name
            FROM historic_class_information_term_dependencies h
            WHERE h.term_collection_id = @term_collection_id and h.school_id = @school_id
        )
    -- sections / meeting times that are directly in the term
    OR (s.pk_fields ? 'term_collection_id' AND s.pk_fields ->> 'term_collection_id' = @term_collection_id)
    -- possible updated data on the school 
    OR table_name = 'schools'
    -- possible updated data on the term collection
    OR (table_name = 'term_collections' AND s.pk_fields ->> 'id' = @term_collection_id)
    )
) as RankedData
WHERE
  rn = 1
ORDER BY sequence
LIMIT @max_records::int
;
