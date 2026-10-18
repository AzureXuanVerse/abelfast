-- name: ListCooldowns :many
SELECT key, until FROM commander_cooldowns WHERE commander_id = ? ORDER BY key;

-- name: GetCooldown :one
SELECT until FROM commander_cooldowns WHERE commander_id = ? AND key = ?;

-- name: SetCooldown :exec
INSERT INTO commander_cooldowns (commander_id, key, until) VALUES (?, ?, ?)
ON CONFLICT (commander_id, key) DO UPDATE SET until = excluded.until;

-- name: CommanderNameTaken :one
SELECT EXISTS (SELECT 1 FROM commanders WHERE name = ? AND id != ?);

-- name: SetCommanderName :exec
UPDATE commanders SET name = ? WHERE id = ?;

-- name: SetManifesto :exec
UPDATE commanders SET manifesto = ? WHERE id = ?;

-- name: DeleteCommonFlag :exec
DELETE FROM commander_common_flags WHERE commander_id = ? AND flag_id = ?;
