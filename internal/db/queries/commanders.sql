-- name: GetCommanderByAccount :one
SELECT * FROM commanders WHERE account_id = ?;

-- name: CreateCommander :one
INSERT INTO commanders (account_id, name, starter_ship, registered_at)
VALUES (?, ?, ?, ?)
RETURNING *;

-- name: GetCommander :one
SELECT * FROM commanders WHERE id = ?;

-- name: CountCommanders :one
SELECT COUNT(*) FROM commanders;

-- name: SetGuideIndex :exec
UPDATE commanders SET guide_index = ? WHERE id = ?;

-- name: SetNewGuideIndex :exec
UPDATE commanders SET new_guide_index = ? WHERE id = ?;

-- name: SetChatRoom :exec
UPDATE commanders SET chat_room_id = ? WHERE id = ?;

-- name: AddStory :exec
INSERT INTO commander_stories (commander_id, story_id) VALUES (?, ?) ON CONFLICT DO NOTHING;

-- name: ListStories :many
SELECT story_id FROM commander_stories WHERE commander_id = ? ORDER BY story_id;

-- name: AddCommonFlag :exec
INSERT INTO commander_common_flags (commander_id, flag_id) VALUES (?, ?) ON CONFLICT DO NOTHING;

-- name: ListCommonFlags :many
SELECT flag_id FROM commander_common_flags WHERE commander_id = ? ORDER BY flag_id;

-- name: ListSecretaries :many
SELECT * FROM commander_secretaries WHERE commander_id = ? ORDER BY position;

-- name: SetSecretary :exec
INSERT INTO commander_secretaries (commander_id, position, ship_id, phantom_id) VALUES (?, ?, ?, ?)
ON CONFLICT (commander_id, position) DO UPDATE SET ship_id = excluded.ship_id, phantom_id = excluded.phantom_id;

-- name: ListFurniture :many
SELECT furniture_id, count FROM commander_furniture WHERE commander_id = ? ORDER BY furniture_id;

-- name: AddFurniture :exec
INSERT INTO commander_furniture (commander_id, furniture_id, count) VALUES (?, ?, ?)
ON CONFLICT (commander_id, furniture_id) DO UPDATE SET count = count + excluded.count;
