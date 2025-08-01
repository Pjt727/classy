CREATE TYPE sync_kind AS ENUM ('update', 'delete', 'insert');

CREATE TABLE historic_class_information (
    sequence SERIAL PRIMARY KEY,

    -- these three fields keep track of a given record
    -- e.i. a single section that is updated deleted etc will have
    -- the same three of these fields
    school_id TEXT NOT NULL,
    table_name TEXT NOT NULL,
    composite_hash TEXT NOT NULL,

    input_at TIMESTAMP WITH TIME ZONE,
    pk_fields jsonb NOT NULL,
    sync_action sync_kind NOT NULL,
    relevant_fields jsonb NOT NULL,
    term_collection_history_id INTEGER,
    FOREIGN KEY (term_collection_history_id) REFERENCES term_collection_history(id)
);

-- stores the dependent courses/ professors by composite_hash
--    note that hash collisions 
CREATE TABLE historic_class_information_term_dependencies (
    table_name TEXT,
    historic_composite_hash TEXT,
    term_collection_id TEXT,
    school_id TEXT,

    PRIMARY KEY (table_name, historic_composite_hash, term_collection_id, school_id)
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
            RETURN NULL;
        END IF;

    -- update + insert = impossible
    -- update + update = new update with updated fields
    -- update + delete = delete (it is NOT a null op ex: 
    --                           create -> update -> delete = create -> delete = no op
    --                           update -> delete           = delete)
    ELSIF current_sync_kind = 'update' THEN
        IF next_sync_kind = 'insert' THEN
            RAISE EXCEPTION 'Cannot have an insert after an update';
        ELSIF next_sync_kind = 'update' THEN
            RETURN ROW('update', current_relevant_fields || next_relevant_fields)::sync_change;
        ELSIF next_sync_kind = 'delete' THEN
            RETURN ROW('delete', '{}'::jsonb)::sync_change;
        END IF;

    -- delete + insert = do an update of all rows because we don't have the original row
    --                   the only way a delete could happen is if the row currently exists
    --                   which is why this is not an insert
    -- delete + update = impossible
    -- delete + delete = impossible
    ELSIF current_sync_kind = 'delete' THEN
        IF next_sync_kind = 'insert' THEN
            RETURN ROW('update', next_relevant_fields)::sync_change;
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
