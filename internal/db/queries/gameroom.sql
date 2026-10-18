-- name: GetGameRoomFirstEnter :one
SELECT first_enter FROM commander_game_room WHERE commander_id = ?;

-- name: SetGameRoomFirstEnter :execrows
INSERT INTO commander_game_room (commander_id, first_enter) VALUES (?, 1)
ON CONFLICT (commander_id) DO UPDATE SET first_enter = 1 WHERE first_enter = 0;
