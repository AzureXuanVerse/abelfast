-- name: ListAttireFrames :many
SELECT * FROM attire_frames WHERE commander_id = ? ORDER BY drop_type, frame_id;

-- name: AddAttireFrame :exec
INSERT INTO attire_frames (commander_id, drop_type, frame_id) VALUES (?, ?, ?) ON CONFLICT DO NOTHING;

-- name: HasAttireFrame :one
SELECT EXISTS (SELECT 1 FROM attire_frames WHERE commander_id = ? AND drop_type = ? AND frame_id = ?);
