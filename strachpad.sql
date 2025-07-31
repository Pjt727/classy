
BEGIN;
DELETE FROM sections s
WHERE s.term_collection_id = '202520'
  AND s.school_id = 'marist'
  AND (s.sequence, s.term_collection_id, s.subject_code, s.course_number, s.school_id )NOT IN (
    SELECT ss.sequence, ss.term_collection_id, ss.subject_code, ss.course_number, ss.school_id  
    FROM staging_sections ss
    WHERE ss.term_collection_id = '202520'
        AND ss.school_id = 'marist'
  );
ROLLBACK;

BEGIN;
EXPLAIN ANALYZE DELETE FROM sections s
LEFT JOIN staging_sections ss USING (term_collection_id, subject_code, course_number, school_id)
WHERE s.term_collection_id = '202520'
  AND s.school_id = 'marist'
  AND ss.sequence IS NULL; -- This condition ensures it's an anti-join
ROLLBACK;

DELETE FROM staging_sections
WHERE course_number = '102L';



select * from historic_class_information where  sync_action = 'delete';

SELECT * FROM historic_class_information;

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
    WHERE s.sequence > 6303
) as RankedData
WHERE
  rn = 1
ORDER BY sequence
;

select * from sync_diffs where sequence > 6303 and sync_action != 'insert';

-- Subquery Scan on sync_diffs  (cost=220.29..569.59 rows=1201 width=130) (actual time=3.291..5.476 rows=882 loops=1)
EXPLAIN ANALYZE WITH sync_diffs AS (
    -- GroupAggregate  (cost=220.29..569.59 rows=1201 width=126) (actual time=3.570..5.693 rows=882 loops=1)
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
    WHERE sequence > 6303
    GROUP BY composite_hash, table_name, composite_hash, school_id, updated_pk_fields
    -- if combined_json is NULL it means that it was deleted
    HAVING combined_json(
                    (sync_action, relevant_fields)::sync_change
                    ORDER BY sequence
           ) IS NOT NULL
)
select sequence, table_name, updated_input_at, composite_hash, school_id, updated_pk_fields,
    (sync_changes).sync_action::sync_kind AS sync_action,
    (sync_changes).relevant_fields AS relevant_fields
FROM sync_diffs
