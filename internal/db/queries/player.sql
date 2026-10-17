-- name: SetChatRoom :exec
UPDATE commanders SET chat_room_id = ? WHERE id = ?;

-- name: CountCommanders :one
SELECT COUNT(*) FROM commanders;

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

-- name: ListActivities :many
SELECT * FROM commander_activities WHERE commander_id = ?;

-- name: GetActivity :one
SELECT * FROM commander_activities WHERE commander_id = ? AND activity_id = ?;

-- name: SaveActivity :exec
INSERT INTO commander_activities (commander_id, activity_id, data1, data2, data3, data4, data1_list) VALUES (?, ?, ?, ?, ?, ?, ?)
ON CONFLICT (commander_id, activity_id) DO UPDATE SET
	data1 = excluded.data1, data2 = excluded.data2, data3 = excluded.data3, data4 = excluded.data4, data1_list = excluded.data1_list;
