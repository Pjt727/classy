DROP TYPE IF EXISTS sync_kind;
CREATE TYPE sync_kind AS ENUM ('update', 'delete', 'insert');

CREATE TABLE historic_entries (
    input_at TIMESTAMP WITH TIME ZONE,
    composite_hash TEXT,
    table_name TEXT,

    pk_fields jsonb,
    sync_action sync_kind,
    relevant_fields jsonb,
    PRIMARY KEY (input_at, sync_action, composite_hash)
);

