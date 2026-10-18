-- name: GetRandomFlagshipMode :one
SELECT mode FROM commander_random_flagship WHERE commander_id = ?;

-- name: SetRandomFlagshipMode :exec
INSERT INTO commander_random_flagship (commander_id, mode) VALUES (?, ?)
ON CONFLICT (commander_id) DO UPDATE SET mode = excluded.mode;

-- name: SetRandomFlagshipEnabled :exec
INSERT INTO commander_random_flagship (commander_id, enabled) VALUES (?, ?)
ON CONFLICT (commander_id) DO UPDATE SET enabled = excluded.enabled;

-- name: SetShipRandomFlag :exec
UPDATE ships SET random_flag = ? WHERE id = ? AND commander_id = ?;
