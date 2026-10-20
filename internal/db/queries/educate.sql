-- name: GetEducateState :one
SELECT state FROM commander_educate WHERE commander_id = ?;

-- name: SaveEducateState :exec
INSERT INTO commander_educate (commander_id, state) VALUES (?, ?)
ON CONFLICT (commander_id) DO UPDATE SET state = excluded.state;

-- name: SetChildDisplay :exec
UPDATE commanders SET child_display = ? WHERE id = ?;
