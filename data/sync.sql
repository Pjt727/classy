-- name: GetLastestSyncChanges :many
SELECT * FROM sync_diffs WHERE input_at = @last_sync_time;
