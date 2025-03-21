-- name: GetLastestSyncChanges :many
SELECT table_name, updated_pk_fields AS pk_fields, sync_action, relevant_fields
FROM sync_diffs WHERE (school_id, table_name, composite_hash, updated_input_at) IN (
    SELECT s.school_id, s.table_name, s.composite_hash, MIN(s.updated_input_at)
    FROM sync_diffs s
    WHERE s.updated_input_at > @last_sync_time
    GROUP BY s.school_id, s.table_name, s.composite_hash
)
;

-- name: GetLastSyncTime :one
SELECT MAX(updated_input_at)::timestamptz FROM sync_diffs;
