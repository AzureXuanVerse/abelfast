-- name: ListPlayRoomViews :many
SELECT * FROM commander_play_room_views WHERE commander_id = ? ORDER BY game_type;

-- name: SetPlayRoomView :exec
INSERT INTO commander_play_room_views (commander_id, game_type, ship_id, skin_id, color, dress_list) VALUES (?, ?, ?, ?, ?, ?)
ON CONFLICT (commander_id, game_type) DO UPDATE SET
	ship_id = excluded.ship_id, skin_id = excluded.skin_id, color = excluded.color, dress_list = excluded.dress_list;

-- name: GetPlayRoomScore :one
SELECT score, max_score FROM commander_play_room_scores
WHERE commander_id = ? AND game_type = ? AND season = ?;

-- name: SetPlayRoomScore :exec
INSERT INTO commander_play_room_scores (commander_id, game_type, season, score, max_score) VALUES (?, ?, ?, ?, ?)
ON CONFLICT (commander_id, game_type, season) DO UPDATE SET score = excluded.score, max_score = excluded.max_score;

-- name: ListPlayRoomRanking :many
SELECT commander_id, score FROM commander_play_room_scores
WHERE game_type = ? AND season = ?
ORDER BY score DESC, commander_id
LIMIT ?;
