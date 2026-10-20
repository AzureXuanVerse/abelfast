-- name: GetNewEducate :one
SELECT state, permanent FROM commander_new_educate
WHERE commander_id = ? AND character_id = ?;

-- name: SaveNewEducate :exec
INSERT INTO commander_new_educate (commander_id, character_id, state, permanent)
VALUES (?, ?, ?, ?)
ON CONFLICT (commander_id, character_id) DO UPDATE
SET state = excluded.state, permanent = excluded.permanent;
