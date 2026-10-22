-- name: ListCommanders :many
SELECT id, account_id, name, level, registered_at, last_online FROM commanders ORDER BY id;
