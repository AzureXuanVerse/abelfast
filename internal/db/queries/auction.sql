-- name: GetAuction :one
SELECT * FROM commander_auction WHERE commander_id = ?;

-- name: SaveAuction :exec
INSERT INTO commander_auction (
	commander_id, activity_id, gold, game_num, buy_num, max_profit, acc_profit,
	acc_buy_price, acc_item_price, max_lot_value, item_list, pre_buy_state,
	pre_timestamp, pre_box, pre_line, match_time, is_forbidden, back_forbidden,
	inactive_num, inactive_state, get_relief_num, reset_day
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT (commander_id) DO UPDATE SET
	activity_id = excluded.activity_id,
	gold = excluded.gold,
	game_num = excluded.game_num,
	buy_num = excluded.buy_num,
	max_profit = excluded.max_profit,
	acc_profit = excluded.acc_profit,
	acc_buy_price = excluded.acc_buy_price,
	acc_item_price = excluded.acc_item_price,
	max_lot_value = excluded.max_lot_value,
	item_list = excluded.item_list,
	pre_buy_state = excluded.pre_buy_state,
	pre_timestamp = excluded.pre_timestamp,
	pre_box = excluded.pre_box,
	pre_line = excluded.pre_line,
	match_time = excluded.match_time,
	is_forbidden = excluded.is_forbidden,
	back_forbidden = excluded.back_forbidden,
	inactive_num = excluded.inactive_num,
	inactive_state = excluded.inactive_state,
	get_relief_num = excluded.get_relief_num,
	reset_day = excluded.reset_day;
