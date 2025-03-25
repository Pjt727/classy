CREATE TYPE sync_kind AS ENUM ('update', 'delete', 'insert');

CREATE TABLE historic_class_information (
    sequence SERIAL PRIMARY KEY,

    school_id TEXT,
    table_name TEXT,
    composite_hash TEXT,

    input_at TIMESTAMP WITH TIME ZONE,
    pk_fields jsonb,
    sync_action sync_kind,
    relevant_fields jsonb
);

CREATE TYPE sync_change AS (
    sync_action sync_kind,
    relevant_fields jsonb
);


CREATE OR REPLACE FUNCTION ordered_set_transition(state sync_change, next_element sync_change)
RETURNS sync_change
AS $$
DECLARE
    current_sync_kind sync_kind;
    next_sync_kind sync_kind;
    current_relevant_fields jsonb;
    next_relevant_fields jsonb;
BEGIN
    next_sync_kind := next_element.sync_action;
    next_relevant_fields := next_element.relevant_fields;

    -- Handle NULL state
    IF state IS NULL THEN
        RETURN next_element;
    END IF;

    current_sync_kind := (state).sync_action;
    current_relevant_fields := (state).relevant_fields;

    -- insert + insert = impossible
    -- insert + update = new insert with updated fields
    -- insert + delete = no operation (null)
    IF current_sync_kind = 'insert' THEN
        IF next_sync_kind = 'insert' THEN
            RAISE EXCEPTION 'Cannot have two inserts in a row';
        ELSIF next_sync_kind = 'update' THEN
            RETURN ROW('insert', current_relevant_fields || next_relevant_fields)::sync_change;
        ELSIF next_sync_kind = 'delete' THEN
            RETURN ROW('delete', jsonb_build_object())::sync_change;
        END IF;

    -- update + insert = impossible
    -- update + update = new update with updated fields
    -- update + delete = no operation (null)
    ELSIF current_sync_kind = 'update' THEN
        IF next_sync_kind = 'insert' THEN
            RAISE EXCEPTION 'Cannot have an insert after an update';
        ELSIF next_sync_kind = 'update' THEN
            RETURN ROW('update', current_relevant_fields || next_relevant_fields)::sync_change;
        ELSIF next_sync_kind = 'delete' THEN
            RETURN NULL;
        END IF;

    -- delete + insert = just do the insert
    -- delete + update = impossible
    -- delete + delete = impossible
    ELSIF current_sync_kind = 'delete' THEN
        IF next_sync_kind = 'insert' THEN
            RETURN next_element;
        ELSIF next_sync_kind = 'update' THEN
            RAISE EXCEPTION 'Cannot have an update after a delete';
        ELSIF next_sync_kind = 'delete' THEN
            RAISE EXCEPTION 'Cannot have two deletes in a row';
        END IF;
    END IF;
    RAISE EXCEPTION 'Unexpect enum';
END;
$$ LANGUAGE plpgsql;

-- Define the aggregate
CREATE AGGREGATE combined_json (sync_change)
(
    sfunc = ordered_set_transition,
    stype = sync_change
);
