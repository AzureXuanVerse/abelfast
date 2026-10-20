-- name: ListActivityKeyValues :many
SELECT * FROM commander_activity_key_values WHERE commander_id = ? ORDER BY activity_id, list_key, item_key;

-- name: AddActivityKeyValue :exec
INSERT INTO commander_activity_key_values (commander_id, activity_id, list_key, item_key, value) VALUES (?, ?, ?, ?, ?)
ON CONFLICT (commander_id, activity_id, list_key, item_key) DO UPDATE SET value = value + excluded.value;

-- name: SpendActivityKeyValue :execrows
UPDATE commander_activity_key_values SET value = value - sqlc.arg(amount)
WHERE commander_id = sqlc.arg(commander_id) AND activity_id = sqlc.arg(activity_id) AND list_key = sqlc.arg(list_key)
	AND item_key = sqlc.arg(item_key) AND value >= sqlc.arg(amount);

-- name: GetFeastRefreshTime :one
SELECT refresh_time FROM commander_feasts WHERE commander_id = ? AND activity_id = ?;

-- name: SetFeastRefreshTime :exec
INSERT INTO commander_feasts (commander_id, activity_id, refresh_time) VALUES (?, ?, ?)
ON CONFLICT (commander_id, activity_id) DO UPDATE SET refresh_time = excluded.refresh_time;

-- name: ListFeastShips :many
SELECT * FROM commander_feast_ships WHERE commander_id = ? AND activity_id = ? ORDER BY group_id;

-- name: DeleteFeastShips :exec
DELETE FROM commander_feast_ships WHERE commander_id = ? AND activity_id = ?;

-- name: SaveFeastShip :exec
INSERT INTO commander_feast_ships (commander_id, activity_id, group_id, bubble, speech_bubble) VALUES (?, ?, ?, ?, ?)
ON CONFLICT (commander_id, activity_id, group_id) DO UPDATE SET bubble = excluded.bubble, speech_bubble = excluded.speech_bubble;

-- name: ListFeastInvitations :many
SELECT * FROM commander_feast_invitations WHERE commander_id = ? AND activity_id = ? ORDER BY group_id;

-- name: SaveFeastInvitation :exec
INSERT INTO commander_feast_invitations (commander_id, activity_id, group_id, state, gift) VALUES (?, ?, ?, ?, ?)
ON CONFLICT (commander_id, activity_id, group_id) DO UPDATE SET state = excluded.state, gift = excluded.gift;
