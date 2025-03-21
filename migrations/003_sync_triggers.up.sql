CREATE OR REPLACE FUNCTION log_historic_class_information()
RETURNS TRIGGER AS $$
DECLARE
    _relevant_fields JSONB;
    _pk_fields JSONB;
    _hash_text TEXT;
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

    -- Determine the operation type
    IF TG_OP = 'INSERT' THEN
        _sync_action := 'insert';
        _relevant_fields := jsonb_object_agg(key, value)
                    FROM jsonb_each(to_jsonb(NEW))
                    WHERE NOT key = ANY(_pk_columns);
        _pk_fields := jsonb_object_agg(key, value) -- Extract only PK fields
                     FROM jsonb_each(to_jsonb(NEW))
                     WHERE key = ANY(_pk_columns);
        _hash_text := STRING_AGG(key || '%' || value, '%%' ORDER BY key)
                FROM jsonb_each(to_jsonb(NEW))
                WHERE key = ANY(_pk_columns)
                AND key <> 'school_id';
    ELSIF TG_OP = 'UPDATE' THEN
        _sync_action := 'update';
        _relevant_fields := jsonb_object_agg(new_data.key, new_data.value) FROM (
            SELECT key, value
            FROM jsonb_each(to_jsonb(NEW))
            WHERE NOT key = ANY(_pk_columns)
        ) AS new_data
        JOIN LATERAL jsonb_each(to_jsonb(OLD)) AS old_data(key, value) ON new_data.key = old_data.key
        WHERE new_data.value IS DISTINCT FROM old_data.value;
        _pk_fields := jsonb_object_agg(key, value) -- Extract only PK fields
                     FROM jsonb_each(to_jsonb(NEW))
                     WHERE key = ANY(_pk_columns);
        _hash_text := STRING_AGG(key || '%' || value, '%%' ORDER BY key)
                FROM jsonb_each(to_jsonb(NEW))
                WHERE key = ANY(_pk_columns)
                AND key <> 'school_id';
    ELSIF TG_OP = 'DELETE' THEN
        _sync_action := 'delete';
        _relevant_fields := NULL;
        _pk_fields := jsonb_object_agg(key, value) -- Extract only PK fields
                     FROM jsonb_each(to_jsonb(OLD))
                     WHERE key = ANY(_pk_columns);
        _hash_text := STRING_AGG(key || '%%' || value, '%%' ORDER BY key)
                FROM jsonb_each(to_jsonb(OLD))
                WHERE key = ANY(_pk_columns);
    END IF;

    INSERT INTO historic_class_information (
        school_id,
        table_name,
        composite_hash,
        input_at,
        pk_fields,
        sync_action,
        relevant_fields
    ) VALUES (
        COALESCE(OLD.school_id, NEW.school_id),
        TG_TABLE_NAME,
        md5(_hash_text::text),
        NOW(), -- Current timestamp
        _pk_fields,
        _sync_action,
        _relevant_fields
    );

    -- Return appropriate value based on operation
    IF TG_OP = 'DELETE' THEN
        RETURN OLD;
    ELSE
        RETURN NEW;
    END IF;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER term_collections_trigger
AFTER INSERT OR UPDATE OR DELETE ON term_collections
FOR EACH ROW
EXECUTE FUNCTION log_historic_class_information();

CREATE TRIGGER professors_trigger
AFTER INSERT OR UPDATE OR DELETE ON professors
FOR EACH ROW
EXECUTE FUNCTION log_historic_class_information();

CREATE TRIGGER courses_trigger
AFTER INSERT OR UPDATE OR DELETE ON courses
FOR EACH ROW
EXECUTE FUNCTION log_historic_class_information();

CREATE TRIGGER sections_trigger
AFTER INSERT OR UPDATE OR DELETE ON sections
FOR EACH ROW
EXECUTE FUNCTION log_historic_class_information();

CREATE TRIGGER meeting_times_trigger
AFTER INSERT OR UPDATE OR DELETE ON meeting_times
FOR EACH ROW
EXECUTE FUNCTION log_historic_class_information();
