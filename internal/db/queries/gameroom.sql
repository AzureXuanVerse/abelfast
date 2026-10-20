-- name: GetGameRoomFirstEnter :one
SELECT first_enter FROM commander_game_room WHERE commander_id = ?;

-- name: SetGameRoomFirstEnter :execrows
INSERT INTO commander_game_room (commander_id, first_enter) VALUES (?, 1)
ON CONFLICT (commander_id) DO UPDATE SET first_enter = 1 WHERE first_enter = 0;

-- name: GetGameRoom :one
SELECT first_enter, weekly_free_week, pay_week, pay_coin_count, ticket_month, monthly_ticket
FROM commander_game_room WHERE commander_id = ?;

-- name: SetGameRoomWeeklyFree :execrows
INSERT INTO commander_game_room (commander_id, weekly_free_week) VALUES (?, ?)
ON CONFLICT (commander_id) DO UPDATE SET weekly_free_week = excluded.weekly_free_week
WHERE weekly_free_week < excluded.weekly_free_week;

-- name: AddGameRoomPayCoins :exec
INSERT INTO commander_game_room (commander_id, pay_week, pay_coin_count) VALUES (?, ?, ?)
ON CONFLICT (commander_id) DO UPDATE SET
	pay_coin_count = CASE WHEN pay_week = excluded.pay_week THEN pay_coin_count + excluded.pay_coin_count ELSE excluded.pay_coin_count END,
	pay_week = excluded.pay_week;

-- name: AddGameRoomMonthlyTickets :exec
INSERT INTO commander_game_room (commander_id, ticket_month, monthly_ticket) VALUES (?, ?, ?)
ON CONFLICT (commander_id) DO UPDATE SET
	monthly_ticket = CASE WHEN ticket_month = excluded.ticket_month THEN monthly_ticket + excluded.monthly_ticket ELSE excluded.monthly_ticket END,
	ticket_month = excluded.ticket_month;

-- name: ListGameRoomScores :many
SELECT room_id, max_score FROM commander_game_room_scores
WHERE commander_id = ? ORDER BY room_id;

-- name: SetGameRoomScore :exec
INSERT INTO commander_game_room_scores (commander_id, room_id, max_score) VALUES (?, ?, ?)
ON CONFLICT (commander_id, room_id) DO UPDATE SET max_score = excluded.max_score
WHERE max_score < excluded.max_score;

-- name: ListMiniGameShopPurchases :many
SELECT goods_id, month_start, count FROM commander_mini_game_shop_purchases
WHERE commander_id = ? ORDER BY goods_id, month_start;

-- name: AddMiniGameShopPurchase :exec
INSERT INTO commander_mini_game_shop_purchases (commander_id, goods_id, month_start, count) VALUES (?, ?, ?, ?)
ON CONFLICT (commander_id, goods_id, month_start) DO UPDATE SET count = count + excluded.count;
