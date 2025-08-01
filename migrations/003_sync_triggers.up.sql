CREATE OR REPLACE FUNCTION log_historic_class_information()
RETURNS TRIGGER AS $$
DECLARE
    _relevant_fields JSONB;
    _pk_fields JSONB;
    _hash_text TEXT;
    _school_id TEXT;
    _sync_action sync_kind;
    _pk_columns TEXT[];
BEGIN

    -- hash collisions would be isolated on table and school and
    --  would be excedingly rare it is likely no feasible combination
    --  of key values would ever produce a collision

    -- Determine the primary key columns for the table
    SELECT array_agg(column_name::TEXT)
    INTO _pk_columns
    FROM information_schema.key_column_usage
    WHERE table_name = TG_TABLE_NAME
      AND table_schema = TG_TABLE_SCHEMA
      AND constraint_name = (
          SELECT constraint_name
          FROM information_schema.table_constraints
          WHERE table_name = TG_TABLE_NAME
            AND table_schema = TG_TABLE_SCHEMA
            AND constraint_type = 'PRIMARY KEY'
      );
    -- turn the pk_fields into a json to be stored
    _pk_fields := jsonb_object_agg(key, value)
                 FROM jsonb_each(to_jsonb(COALESCE(NEW, OLD)))
                 WHERE key = ANY(_pk_columns)
                     AND key != 'school_id'
                 ;
    _hash_text := STRING_AGG(key || '%' || value::TEXT, '%%' ORDER BY key)
            FROM jsonb_each(_pk_fields);
    IF TG_OP = 'INSERT' THEN
        _sync_action := 'insert';
        _relevant_fields := jsonb_object_agg(key, value)
                    FROM jsonb_each(to_jsonb(NEW))
                    WHERE NOT key = ANY(_pk_columns);
    ELSIF TG_OP = 'UPDATE' THEN
        _sync_action := 'update';
        _relevant_fields := jsonb_object_agg(new_data.key, new_data.value) FROM (
            SELECT key, value
            FROM jsonb_each(to_jsonb(NEW))
            WHERE NOT key = ANY(_pk_columns)
        ) AS new_data
        JOIN LATERAL jsonb_each(to_jsonb(OLD)) AS old_data(key, value) ON new_data.key = old_data.key
        WHERE new_data.value IS DISTINCT FROM old_data.value;
    ELSIF TG_OP = 'DELETE' THEN
        _sync_action := 'delete';
        _relevant_fields := '{}'::jsonb;
    END IF;

    -- school's id is just "id"
    IF TG_TABLE_NAME = 'schools' THEN
        _school_id = COALESCE(NEW.id, OLD.id);
    ELSE
        _school_id = COALESCE(OLD.school_id, NEW.school_id);
    END IF;

    INSERT INTO historic_class_information (
        school_id,
        table_name,
        composite_hash,
        input_at,
        pk_fields,
        sync_action,
        relevant_fields,
        term_collection_history_id
    ) VALUES (
        _school_id,
        TG_TABLE_NAME,
        md5(_hash_text::text),
        NOW(),
        _pk_fields,
        _sync_action,
        _relevant_fields,
        current_setting('app.term_collection_history_id', TRUE)::INTEGER
    );

    RETURN COALESCE(NEW, OLD);

END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER historic_log
AFTER INSERT OR UPDATE OR DELETE ON schools
FOR EACH ROW
EXECUTE FUNCTION log_historic_class_information();

CREATE TRIGGER historic_log
AFTER INSERT OR UPDATE OR DELETE ON term_collections
FOR EACH ROW
EXECUTE FUNCTION log_historic_class_information();

CREATE TRIGGER historic_log
AFTER INSERT OR UPDATE OR DELETE ON professors
FOR EACH ROW
EXECUTE FUNCTION log_historic_class_information();

CREATE TRIGGER historic_log
AFTER INSERT OR UPDATE OR DELETE ON courses
FOR EACH ROW
EXECUTE FUNCTION log_historic_class_information();

CREATE TRIGGER historic_log
AFTER INSERT OR UPDATE OR DELETE ON sections
FOR EACH ROW
EXECUTE FUNCTION log_historic_class_information();

CREATE TRIGGER historic_log
AFTER INSERT OR UPDATE OR DELETE ON meeting_times
FOR EACH ROW
EXECUTE FUNCTION log_historic_class_information();


-- these are triggers to populate the dependencies for a particular term
--    they help to answer the question of what associated records do I need from
--    the historic data for this particular term
-- for instance a section being added which has a particular course and professor
--    knowing which data is need for each term in  h
-- it is OK for a term to have a dependency even if that dependency is not used in all
--     snapshots e.i. a course was dropped and no longer is taught during a semester
-- the main reason why this is worth doing is that schools which have many terms will accrue 
--     a ton of courses + professors which are not needed when looking at a specific term
CREATE OR REPLACE FUNCTION section_term_depedents_trigger()
RETURNS TRIGGER AS $$
DECLARE
    _professor_hash_text TEXT;
    _course_hash_text TEXT;
BEGIN
    -- insert professor
    IF NEW.primary_professor_id IS NOT NULL THEN
        _professor_hash_text := md5('id' || '%"' || NEW.primary_professor_id || '"');
        INSERT INTO historic_class_information_term_dependencies
        (table_name, historic_composite_hash, term_collection_id, school_id)
        VALUES ('professors', _professor_hash_text, NEW.term_collection_id, NEW.school_id)
        ON CONFLICT DO NOTHING;
    END IF;

    -- insert courses
    _course_hash_text := md5('number' || '%"' || NEW.course_number || '"%%'
                             'subject_code' || '%"' || NEW.subject_code || '"');
    INSERT INTO historic_class_information_term_dependencies
    (table_name, historic_composite_hash, term_collection_id, school_id)
    VALUES ('courses', _course_hash_text, NEW.term_collection_id, NEW.school_id)
    ON CONFLICT DO NOTHING;


    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER insert_term_dependencies
AFTER INSERT OR UPDATE ON sections
FOR EACH ROW
EXECUTE FUNCTION section_term_depedents_trigger();

