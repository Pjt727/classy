DROP AGGREGATE IF EXISTS combined_json(sync_change);
DROP FUNCTION IF EXISTS ordered_set_transition(state sync_change, next_element sync_change);
DROP TABLE IF EXISTS historic_class_information;
DROP TYPE IF EXISTS sync_change;
DROP TYPE IF EXISTS sync_kind;

