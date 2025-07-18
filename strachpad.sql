SELECT * FROM terms;
SELECT id, description FROM courses WHERE school_id = 'temple';
SELECT * FROM schools;
SELECT * FROM term_collections
WHERE school_id = 'temple';

SELECT * FROM faculty_members;
SELECT * FROM sections;
SELECT course_id FROM staging_sections WHERE school_id = 'temple' GROUP BY course_id;
SELECT * FROM sections WHERE school_id = 'temple' AND subject_code = 'JPNS' AND course_number = '2111';
SELECT * FROM courses
WHERE school_id = 'temple' AND subject_code = 'JPNS' AND number = '2111';
SELECT * FROM sections WHERE school_id = 'marist';
SELECT * FROM staging_meeting_times;
SELECT * FROM meeting_times
WHERE start_minutes != NULL
;
SELECT MAX(counter) FROM
(
SELECT COUNT(*) counter FROM staging_meeting_times 
GROUP BY "sequence", section_id, term_season, course_id, school_id
ORDER BY counter
);
SELECT * FROM staging_meeting_times 
WHERE (section_id, term_season, term_year, course_id, school_id) IN (SELECT id, term_season, term_year, course_id, school_id FROM sections);

SELECT COUNT(*) FROM sections WHERE school_id = 'temple';

SELECT CASE 
        WHEN EXISTS (
            SELECT 1
            FROM schools
            WHERE id = 'marist'
        ) THEN true
    ELSE false
END;

SELECT * FROM section_meetings LIMIT 200;
SELECT * FROM meeting_times LIMIT 10;
SELECT * FROM courses;

SELECT count(*) FROM historic_class_information;

SELECT composite_hash, pk_fields, relevant_fields FROM historic_class_information
WHERE "table_name" = 'sections'
ORDER BY composite_hash, input_at;


SELECT composite_hash, count(*) AS total FROM historic_class_information
GROUP BY composite_hash ORDER BY total;

SELECT * FROM historic_class_information WHERE 
composite_hash = 'c8e548557957ae53832f91e657ebfaf1'
ORDER BY input_at;

SELECT * FROM meeting_times m WHERE m."sequence" = 0
AND m.subject_code = 'MUS'
AND m.course_number = '236N'
AND m.section_sequence = '111';

-- {"end_date": "2024-12-13T00:00:00", "is_friday": false, "is_monday": true, "is_sunday": false, "is_tuesday": false, "start_date": "2024-08-26T00:00:00", "end_minutes": "16:45:00", "is_saturday": false, "is_thursday": false, "is_wednesday": true, "meeting_type": "LEC", "start_minutes": "15:30:00"}
-- {"end_minutes": "18:15:00", "start_minutes": "17:00:00"}
-- {"end_minutes": "21:15:00", "is_thursday": true, "is_wednesday": false, "start_minutes": "20:00:00"}
--

SELECT composite_hash, combined_json((sync_action, relevant_fields)::sync_change ORDER BY input_at)
FROM historic_class_information 
GROUP BY composite_hash;


SELECT 
       jsonb_set(pk_fields, '{school_id}', school_id, true), 
       combined_json(
            (sync_action, relevant_fields)::sync_change
            ORDER BY input_at
        )
FROM historic_class_information 
GROUP BY table_name, composite_hash, pk_fields;

SELECT composite_hash
       pk_fields::jsonb ||, '{school_id}', school_id::text, true, 
       combined_json(
            (sync_action, relevant_fields)::sync_change
            ORDER BY input_at
        )
FROM historic_class_information 
GROUP BY table_name, composite_hash, pk_fields;

SELECT composite_hash, COUNT(*) AS foo FROM historic_class_information
WHERE sync_action = 'insert'
GROUP BY composite_hash
ORDER BY foo
;

SELECT * FROM historic_class_information
WHERE composite_hash = 'a8a3507fb542ee24f994bc5a0035719a';

SELECT * FROM historic_class_information
WHERE composite_hash = '09b6cdcafe1e6be24b6512f4fa62e782';

SELECT * FROM historic_class_information WHERE sync_action = 'delete';
-- WHERE composite_hash = 'a8a3507fb542ee24f994bc5a0035719a'
ORDER BY input_at;

SELECT * FROM sections WHERE course_number = '120L' AND subject_code = 'ENG' AND sequence = '105';


-- 6380
SELECT jsonb_set(pk_fields::jsonb, '{school_id}', to_jsonb(school_id), true) AS updated_pk_fields, 
    (combined_json(
            (sync_action, relevant_fields)::sync_change
            ORDER BY input_at
    ))
FROM 
    historic_class_information 
GROUP BY 
    composite_hash, 
    updated_pk_fields
;

SELECT * FROM historic_class_information;


SELECT table_name, sync_action, composite_hash, relevant_fields
FROM sync_diffs WHERE (school_id, table_name, composite_hash, updated_input_at) IN (
    SELECT s.school_id, s.table_name, s.composite_hash, MIN(s.updated_input_at)
    FROM sync_diffs s
    WHERE s.updated_input_at > '2020-03-19 22:29:05.546344+09'
    GROUP BY s.school_id, s.table_name, s.composite_hash
);

SELECT * FROM historic_class_information
WHERE composite_hash = '6827feb7f0b2c688fdd358d14e45de47';

SELECT tgname, tgrelid, tgisinternal, tgenabled
FROM pg_trigger
WHERE tgname LIKE '%_trigger';

SELECT composite_hash
FROM historic_class_information
WHERE sync_action = 'delete'
AND composite_hash NOT IN (
    SELECT composite_hash
    FROM historic_class_information
    WHERE sync_action IN ('insert', 'update')
);

select * from sync_diffs;


select * from sections where sections.primary_professor_id = 'Lauren.Yanks1@marist.edu';

select * from terms;


SELECT table_name, updated_pk_fields AS pk_fields, sync_action, relevant_fields
FROM sync_diffs WHERE (school_id, table_name, composite_hash, updated_input_at) IN (
    SELECT s.school_id, s.table_name, s.composite_hash, MIN(s.updated_input_at)
    FROM sync_diffs s
    WHERE s.sequence > 0
    GROUP BY s.school_id, s.table_name, s.composite_hash
);

-- cost=2422118.10..2422133.85
EXPLAIN (
SELECT h1.sequence,
       h1.table_name,
       h1.input_at AS updated_input_at,
       h1.composite_hash,
       h1.school_id,
       jsonb_set(h1.pk_fields::jsonb, '{school_id}', to_jsonb(h1.school_id), true) AS updated_pk_fields, 
       (SELECT combined_json(
                (h2.sync_action, h2.relevant_fields)::sync_change
                ORDER BY h2.sequence
       )
       FROM historic_class_information h2
       WHERE h2.school_id = h1.school_id
             AND h2.table_name = h1.table_name
             AND h2.composite_hash = h1.composite_hash
             AND h2.sequence >= h1.sequence
       ) AS sync_changes
FROM 
    historic_class_information h1
    ORDER BY h1.input_at
)
-- cost: 2422023.55..2422039.31
    EXPLAIN (
SELECT h1.sequence,
       h1.table_name,
       h1.input_at AS updated_input_at,
       h1.composite_hash,
       h1.school_id,
       jsonb_set(h1.pk_fields::jsonb, '{school_id}', to_jsonb(h1.school_id), true) AS updated_pk_fields, 
       (SELECT combined_json(
                (h2.sync_action, h2.relevant_fields)::sync_change
                ORDER BY h2.sequence
       )
       FROM historic_class_information h2
       WHERE h2.school_id = h1.school_id
             AND h2.table_name = h1.table_name
             AND h2.composite_hash = h1.composite_hash
             AND h2.sequence >= h1.sequence
       ).*
FROM 
    historic_class_information h1
ORDER BY h1.input_at);

SELECT table_name, updated_pk_fields AS pk_fields, (sync_changes).sync_action, (sync_changes).relevant_fields
FROM sync_diffs WHERE (school_id, table_name, composite_hash, updated_input_at) IN (
    SELECT s.school_id, s.table_name, s.composite_hash, MIN(s.updated_input_at)
    FROM sync_diffs s
    WHERE s.sequence > 0
    GROUP BY s.school_id, s.table_name, s.composite_hash
);

select * from term_collections;


SELECT
    (SELECT elem FROM UNNEST(ARRAY['a', 'b', 'c']) WITH ORDINALITY AS u(elem, idx) WHERE idx = i),
    (SELECT elem FROM UNNEST(ARRAY['x', 'y', 'z']) WITH ORDINALITY AS u(elem, idx) WHERE idx = i)
FROM generate_series(1, array_length(ARRAY['a', 'b', 'c'], 1)) AS i;

-- Sort  (cost=1951.61..1951.63 rows=10 width=126)
EXPLAIN (
SELECT sd.*
FROM sync_diffs sd
where sd."sequence" in (select h."sequence" from historic_class_information h where h."table_name" = 'professors')
);


 -- Hash Join  (cost=1212940.44..1212979.97 rows=10 width=126)
EXPLAIN (
SELECT sd.*
FROM sync_diffs sd
where sd."sequence" IN (SELECT generate_series(1, 10))
);

select * from sync_diffs;

EXPLAIN (
select (sync_diffs.sync_changes).sync_action, (sync_diffs.sync_changes).relevant_fields from sync_diffs where "table_name" = 'professors'
);

--                                                               QUERY PLAN                                                               
-- ---------------------------------------------------------------------------------------------------------------------------------------
--  Seq Scan on historic_class_information h1  (cost=0.00..82824.01 rows=429 width=32)
--    Filter: (table_name = 'professors'::text)
--    SubPlan 1
--      ->  Aggregate  (cost=192.12..192.13 rows=1 width=32)
--            ->  Index Scan using historic_class_information_pkey on historic_class_information h2  (cost=0.28..191.87 rows=1 width=208)
--                  Index Cond: (sequence >= h1.sequence)
--                  Filter: ((school_id = h1.school_id) AND (table_name = h1.table_name) AND (composite_hash = h1.composite_hash))
                                                               -- QUERY PLAN                                                                
-- -----------------------------------------------------------------------------------------------------------------------------------------
--  Seq Scan on historic_class_information h1  (cost=0.00..165246.17 rows=429 width=36)
--    Filter: (table_name = 'professors'::text)
--    SubPlan 1
--      ->  Aggregate  (cost=192.12..192.13 rows=1 width=32)
--            ->  Index Scan using historic_class_information_pkey on historic_class_information h2  (cost=0.28..191.87 rows=1 width=208)
--                  Index Cond: (sequence >= h1.sequence)
--                  Filter: ((school_id = h1.school_id) AND (table_name = h1.table_name) AND (composite_hash = h1.composite_hash))
--    SubPlan 2
--      ->  Aggregate  (cost=192.12..192.13 rows=1 width=32)
--            ->  Index Scan using historic_class_information_pkey on historic_class_information h2_1  (cost=0.28..191.87 rows=1 width=208)
--                  Index Cond: (sequence >= h1.sequence)
--                  Filter: ((school_id = h1.school_id) AND (table_name = h1.table_name) AND (composite_hash = h1.composite_hash))
EXPLAIN (
select 
        ...
       (SELECT combined_json(
                (h2.sync_action, h2.relevant_fields)::sync_change
                ORDER BY h2.sequence
       )
       FROM historic_class_information h2
       WHERE h2.school_id = h1.school_id
             AND h2.table_name = h1.table_name
             AND h2.composite_hash = h1.composite_hash
             AND h2.sequence >= h1.sequence
       )
       ...
;
);
EXPLAIN(
    select * from sync_diffs
);
EXPLAIN(
    select * from sync_diffs_nested
);

CREATE VIEW sync_diffs AS (
SELECT h1.sequence,
       h1.table_name,
       h1.input_at AS updated_input_at,
       h1.composite_hash,
       h1.school_id,
       jsonb_set(h1.pk_fields::jsonb, '{school_id}', to_jsonb(h1.school_id), true) AS updated_pk_fields, 
        -- unpacking this tuple or duplicating sync_changes seems to make the expensive json joins happen twice
        --    doubling the cost of the query
       (SELECT combined_json(
                (h2.sync_action, h2.relevant_fields)::sync_change
                ORDER BY h2.sequence
       )
       FROM historic_class_information h2
       WHERE h2.school_id = h1.school_id
             AND h2.table_name = h1.table_name
             AND h2.composite_hash = h1.composite_hash
             AND h2.sequence >= h1.sequence
       ) AS sync_changes
FROM 
    historic_class_information h1
ORDER BY h1.input_at
);

SELECT * FROM professors where id = 'Alan.Labouseur@marist.edu';

SELECT * FROM sections 
where primary_professor_id = 'Alan.Labouseur@marist.edu'; 

select * from courses where number = '424N';

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
    WHERE s.sequence > 3000
) as RankedData
WHERE
  rn = 1;


select * from historic_class_information where table_name = 'term_collections' ;
select * from historic_class_information where table_name = 'schools' ;
select * from historic_class_information where table_name = 'meeting_times' and sync_action = 'update' ;
select * from historic_class_information where table_name = 'professors' and sync_action = 'insert' ;

select * from sections;

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
    WHERE s.sequence > 0
) as RankedData
WHERE
  rn = 1
ORDER BY sequence
LIMIT 500
;

select * from term_collections;

select * from sections where term_collection_id = '202540';

select * from sections where primary_professor_id = 'Alan.Labouseur@marist.edu' and term_collection_id = '202540';

select * from sync_diffs where "sequence" > 9000;

select 
       MIN(sequence) as sequence, 
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
       ) as sync_changes
from historic_class_information
where sequence > 3000 
group by composite_hash, table_name, composite_hash, school_id, updated_pk_fields
HAVING combined_json(
                (sync_action, relevant_fields)::sync_change
                ORDER BY sequence
       ) IS NOT NULL



select * from historic_class_information where composite_hash = '0575f25903e00e9a3621ae0ba637a264';

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
    WHERE s.sequence > 0
) as RankedData
WHERE
  rn = 1
ORDER BY sequence;


select * from historic_class_information;

select * from professors p where p.id in (select primary_professor_id from sections where sections.term_collection_id = '202440')

EXPLAIN(
    SELECT DISTINCT s.school_id, s.term_collection_id
    FROM sections s WHERE s.primary_professor_id like '%A%';
);

EXPLAIN(
    select DISTINCT sections.term_collection_id from sections where sections.course_number = '100L'
);

SELECT array_agg(column_name::TEXT)
FROM information_schema.key_column_usage
WHERE table_name = 'professors'
  AND constraint_name = (
      SELECT constraint_name
      FROM information_schema.table_constraints
      WHERE table_name = 'professors'
        AND constraint_type = 'PRIMARY KEY'
  );


/*
1. professor inserted
2. section   inserted
3. section   triggers 

2. section   inserted

*/
WITH data AS (
    SELECT '{"field1": "value1", "field3": "value3", "field2": "value2"}'::jsonb AS _pk_fields
),
ordered_json AS (
    SELECT STRING_AGG(key || '%' || value, '%%' ORDER BY key) AS ordered_string
    FROM jsonb_each((SELECT _pk_fields FROM data))
)
SELECT md5(ordered_string)
FROM ordered_json;

WITH data AS (
  SELECT '{"school_id": "marist", "id": "Alan.Labouseur@marist.edu"}'::jsonb AS _pk_fields
)
SELECT
  md5(STRING_AGG(key || '%' || value, '%%' ORDER BY key))
FROM
  jsonb_each((SELECT _pk_fields FROM data));

select 
    md5('id' || '%"' || sections.primary_professor_id || '"')
from sections
where sections.primary_professor_id like '%Alan%';

select 
    md5('number' || '%"' || sections.course_number || '"%%'
    'subject_code' || '%"' || sections.subject_code || '"'), *
from sections
where sections.course_number = '103L' and 
sections.subject_code = 'CMPT'
LIMIT 1;

 -- id%Alan.Labouseur@marist.edu%%school_id%marist
 -- id%"Alan.Labouseur@marist.edu"%%school_id%"marist"
 -- id%"Alan.Labouseur@marist.edu"%%school_id%"marist"
SELECT *
FROM historic_class_information
WHERE CASE
    WHEN 
        historic_class_information.pk_fields ? 'number' THEN historic_class_information.pk_fields ->> 'number' = '103L'
    ELSE FALSE
END;
-- 71113f03f00ef2f5ee7fdccb21e81890

select * from professors where email_address like '%Alan%' ;


WITH data AS (
  SELECT '{"number": "103L", "subject_code": "CMPT"}'::jsonb AS _pk_fields
)
SELECT
  md5(STRING_AGG(key || '%' || value, '%%' ORDER BY key))
FROM
  jsonb_each((SELECT _pk_fields FROM data));

-- d540284f9ea6fe08737c67783bd1fc7a
-- d540284f9ea6fe08737c67783bd1fc7a

select * from historic_class_information where composite_hash = '71113f03f00ef2f5ee7fdccb21e81890';
select * from historic_class_information_term_dependencies;


SELECT
    s.sequence, s.table_name, s.composite_hash
    FROM
    sync_diffs s
    WHERE 
    s.sequence > 0
    AND (
    (s.composite_hash, s.table_name) IN (
        SELECT h.historic_composite_hash, h.table_name
        FROM historic_class_information_term_dependencies h
        WHERE h.term_collection_id IN ('202440')
    ) OR (s.pk_fields ? 'term_collection_id' AND s.pk_fields ->> 'term_collection_id' IN ('202440')));

select * from historic_class_information;

select * from term_collection_history;



SELECT
    t.id,
    SUM(CASE WHEN sync_action = 'insert' THEN 1 ELSE 0 END) AS insert_records,
    SUM(CASE WHEN sync_action = 'update' THEN 1 ELSE 0 END) AS updated_records,
    SUM(CASE WHEN sync_action = 'delete' THEN 1 ELSE 0 END) AS deleted_records,
    end_time - start_time AS elapsed_time
FROM term_collection_history t 
LEFT JOIN historic_class_information h ON t.id = h.term_collection_history_id
GROUP BY (t.id, t.end_time, t.start_time)
ORDER BY t.id
;

select * from professors where school_id = 'temple';
select COUNT(*) from professors where school_id = 'temple';

select *  from historic_class_information where school_id = 'temple' and table_name = 'professors';


select * from professors;

select * from courses;
select * from term_collections;

SELECT 
    t.*, 
    COUNT(select * from) as sections_count,
    COUNT(DISTINCT CASE WHEN h.table_name = 'courses' THEN h END) as course_depedents_count,
    COUNT(DISTINCT CASE WHEN h.table_name = 'professors' THEN h END) as professor_depedents_count
FROM term_collections t
INNER JOIN sections s ON s.school_id = t.school_id AND s.term_collection_id = t.id
INNER JOIN historic_class_information_term_dependencies h ON 
    h.school_id = t.school_id AND h.term_collection_id = t.id
GROUP BY t.id, t.school_id;
;


select description from courses where courses.description is not null and school_id = 'marist';

select * from sections where school_id = 'marist';
select * from courses where school_id = 'marist';


select COUNT(*) from staging_courses where description is not null and school_id = 'marist';

select * from term_collection_history;

select * from professors where school_id = 'marist';

select * from staging_courses where term_collection_history_id=7 and description is not null;

select * from staging_courses;


BEGIN;
INSERT INTO courses 
    (school_id, subject_code, number, subject_description, title,
        description, credit_hours, prerequisites, corequisites, other)
SELECT DISTINCT ON (school_id, subject_code, number) 
    school_id, subject_code, number, subject_description, title, 
    description, credit_hours, prerequisites, corequisites, other
FROM staging_courses WHERE term_collection_history_id = 7
ON CONFLICT (school_id, subject_code, number) DO UPDATE
SET subject_description = EXCLUDED.subject_description,
    title = EXCLUDED.title,
    description = EXCLUDED.description,
    credit_hours = EXCLUDED.credit_hours,
    prerequisites = EXCLUDED.prerequisites,
    corequisites = EXCLUDED.corequisites,
    other = EXCLUDED.other
WHERE courses.title != EXCLUDED.title
    OR courses.credit_hours != EXCLUDED.credit_hours
    -- these are considered "extra" fields that may no always be populated
    --     because they are difficult to get
    OR courses.description IS DISTINCT FROM EXCLUDED.description
    OR courses.subject_description != EXCLUDED.subject_description
    OR courses.other != EXCLUDED.other
    -- OR (EXCLUDED.description IS NOT NULL AND courses.description != EXCLUDED.description)
    -- OR (EXCLUDED.subject_description IS NOT NULL AND courses.subject_description != EXCLUDED.subject_description)
    -- OR (EXCLUDED.other IS NOT NULL AND courses.other != EXCLUDED.other)
;

select c.subject_description != s.subject_description,
    c.title != s.title,
    'foos' IS DISTINCT FROM 'foo',
    c.credit_hours != s.credit_hours,
    c.prerequisites != s.prerequisites,
    c.corequisites != s.corequisites,
    c.other != s.other
from courses c
inner join staging_courses s ON c.school_id = s.school_id AND c.subject_code = s.subject_code AND c.number = s.number
