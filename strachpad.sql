
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
WITH sync_diffs AS (
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
    WHERE sequence > 8221
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
FROM sync_diffs;


SELECT * FROM historic_class_information where 
            pk_fields ->> 'course_number' = '120L' 
            and pk_fields ->> 'sequence' = '105'
            and pk_fields ->> 'subject_code' = 'ENG'
;


WITH sync_diffs AS (
    SELECT 
           MAX(sequence) AS sequence, 
           table_name,
           MAX(input_at) AS updated_input_at,
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
-- 1593 
-- 7536 
-- 8226 
    WHERE sequence > 0 
            -- and pk_fields ->> 'course_number' = '120L' 
            -- and pk_fields ->> 'sequence' = '105'
            -- and pk_fields ->> 'subject_code' = 'ENG'
    GROUP BY composite_hash, table_name, composite_hash, school_id, updated_pk_fields
    -- if combined_json is NULL it means that it was created and then deleted
)
SELECT sequence::int, table_name, updated_input_at AS input_at, composite_hash, school_id, updated_pk_fields AS pk_fields,
    (sync_changes).sync_action::sync_kind AS sync_action,
    (sync_changes).relevant_fields AS relevant_fields,
    COUNT(*) OVER() AS total_rows
FROM sync_diffs
WHERE (sync_changes).sync_action::sync_kind IS NOT NULL
ORDER BY sequence;

select * from historic_class_information where sequence > 7530
            and pk_fields ->> 'course_number' = '120L' 
            and pk_fields ->> 'sequence' = '105'
            and pk_fields ->> 'subject_code' = 'ENG';

SELECT COUNT(DISTINCT composite_hash)
FROM historic_class_information
where sequence in (select max(sequence) from historic_class_information group by composite_hash)
    AND sync_action = 'delete'
;


SELECT * FROM historic_class_information h1
JOIN historic_class_information h2 ON h1.composite_hash = h2.composite_hash
where h1.pk_fields != h1.pk_fields;

SELECT DISTINCT sequence FROM historic_class_information h
JOIN UNNEST(['202440']::TEXT[], ['4']::INTEGER[]) AS checks(term_sequence, term_collection_id)
    ON h.sequence <= checks.term_sequence AND (
        -- sections / meeting times that are directly in the term
        (pk_fields ? 'term_collection_id' AND pk_fields ->> 'term_collection_id' = @term_collection_id)
        -- possible updated data on the school 
        OR table_name = 'schools'
        -- possible updated data on the term collection
        OR (table_name = 'term_collections' AND pk_fields ->> 'id' = @term_collection_id)
    )
)



WITH included_commons AS (
      SELECT h.historic_composite_hash, h.table_name
      FROM historic_class_information_term_dependencies h
      WHERE h.term_collection_id = ANY(ARRAY['202440']::TEXT[])
)
SELECT sequence FROM historic_class_information h
JOIN UNNEST(ARRAY['202440']::TEXT[], ARRAY[4]::INTEGER[]) AS checks(term_collection_id, term_sequence)
    ON h.sequence > checks.term_sequence AND (
        -- sections / meeting times that are directly in the term
        (pk_fields ? 'term_collection_id' AND pk_fields ->> 'term_collection_id' = checks.term_collection_id)
        -- possible updated data on the school
        OR table_name = 'schools'
        -- possible updated data on the term collection
        OR (table_name = 'term_collections' AND pk_fields ->> 'id' = checks.term_collection_id)
        OR (h.composite_hash, h.table_name) IN (SELECT * FROM included_commons)
    )
;

WITH included_commons AS (
      SELECT h.historic_composite_hash, h.table_name
      FROM historic_class_information_term_dependencies h
      WHERE h.term_collection_id = ANY(ARRAY['202440']::TEXT[])
)
SELECT sequence FROM historic_class_information h
JOIN (
    SELECT
        value ->> 'term_collection_id' AS term_collection_id,
        (value ->> 'term_sequence')::INTEGER AS term_sequence
    FROM jsonb_array_elements('[{"term_collection_id": "202440", "term_sequence": 4}]'::jsonb)
) AS checks
    ON h.sequence > checks.term_sequence AND (
        -- sections / meeting times that are directly in the term
        (pk_fields ? 'term_collection_id' AND pk_fields ->> 'term_collection_id' = checks.term_collection_id)
        -- possible updated data on the school
        OR table_name = 'schools'
        -- possible updated data on the term collection
        OR (table_name = 'term_collections' AND pk_fields ->> 'id' = checks.term_collection_id)
        OR (h.composite_hash, h.table_name) IN (SELECT * FROM included_commons)
    )
;

SELECT * FROM term_collection_history;

SELECT * FROM historic_class_information WHERE term_collection_history_id = 7 and sync_action = 'update';

SELECT * FROM historic_class_information WHERE composite_hash = '57dbc37e5660777db0ce8e32205c4a35';

WITH 
    included_commons AS (
          SELECT h.historic_composite_hash, h.table_name
          FROM historic_class_information_term_dependencies h
          WHERE h.term_collection_id = ANY(ARRAY['202440']::TEXT[])
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
        FROM jsonb_array_elements('[{"id": "202440", "sequence": 4}]'::jsonb)
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
