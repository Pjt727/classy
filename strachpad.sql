
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
