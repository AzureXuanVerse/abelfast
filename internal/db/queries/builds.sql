-- name: CreateBuild :one
INSERT INTO builds (commander_id, pool_id, template_id, duration, finish_time)
VALUES (?, ?, ?, ?, ?)
RETURNING *;

-- name: ListBuilds :many
SELECT * FROM builds WHERE commander_id = ? ORDER BY id;

-- name: SetBuildFinishTime :exec
UPDATE builds SET finish_time = ? WHERE id = ?;

-- name: DeleteBuild :exec
DELETE FROM builds WHERE id = ?;

-- name: GetBuildPoints :one
SELECT exchange_count FROM commander_build_states WHERE commander_id = ?;

-- name: SetBuildPoints :exec
INSERT INTO commander_build_states (commander_id, exchange_count) VALUES (?, ?)
ON CONFLICT (commander_id) DO UPDATE SET exchange_count = excluded.exchange_count;
