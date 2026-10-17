-- name: CreateShip :one
INSERT INTO ships (commander_id, template_id, level, exp, energy, intimacy, max_level, is_locked, created_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
RETURNING *;

-- name: ListShipsByCommander :many
SELECT * FROM ships WHERE commander_id = ? ORDER BY id;
