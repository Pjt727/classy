-- name: GetLastestSyncChanges :many
-- min is used incase for some reason the json fields arrange themselves differently
SELECT table_name, composite_hash, MIN(pk_fields), 
       combined_json(
            (sync_action, relevant_fields)::sync_change
            ORDER BY input_at
        )
FROM historic_class_information 
WHERE input_at > @last_sync_time
GROUP BY table_name, composite_hash, pk_fields;
