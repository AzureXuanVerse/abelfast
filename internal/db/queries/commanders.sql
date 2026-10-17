-- name: GetCommanderByAccount :one
SELECT * FROM commanders WHERE account_id = ?;

-- name: CreateCommander :one
INSERT INTO commanders (account_id, name, starter_ship, registered_at)
VALUES (?, ?, ?, ?)
RETURNING *;

-- name: GetCommander :one
SELECT * FROM commanders WHERE id = ?;

-- name: SetGuideIndex :exec
UPDATE commanders SET guide_index = ? WHERE id = ?;

-- name: SetNewGuideIndex :exec
UPDATE commanders SET new_guide_index = ? WHERE id = ?;

-- name: AddStory :exec
INSERT INTO commander_stories (commander_id, story_id) VALUES (?, ?) ON CONFLICT DO NOTHING;

-- name: ListStories :many
SELECT story_id FROM commander_stories WHERE commander_id = ? ORDER BY story_id;
