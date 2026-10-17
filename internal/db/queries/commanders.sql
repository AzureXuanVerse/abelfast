-- name: GetCommanderByAccount :one
SELECT * FROM commanders WHERE account_id = ?;

-- name: CreateCommander :one
INSERT INTO commanders (account_id, name, starter_ship, registered_at)
VALUES (?, ?, ?, ?)
RETURNING *;
