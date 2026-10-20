-- the auction minigame (ActivityConst.ACTIVITY_TYPE_AUCTION_GAME,
-- AuctionGameBaseProxy, SC_23431): the commander's Bid Coins (gold, the
-- virtual_type 31 item, capped at gameset auction_currency_ceiling) and
-- statistics, kept for the activity it was played in (activity_id: another
-- auction activity starts from nothing). No row until the commander first
-- gets coins or plays.
--   - game_num, buy_num: auctions attended, auctions won; max_profit,
--     acc_profit: the best and the total earnings (a win's lot value minus
--     the winning bid, or a dividend), acc_buy_price / acc_item_price the
--     winning bids and the values won in total, max_lot_value the most
--     valuable lot won (the "in a single auction" tasks)
--   - item_list: the auction_collection ids ever won (the item logbook)
--   - pre_buy_state 1 while an ordered box (pre_box, the PTGC_BID_KING_ITEM
--     list as JSON, pre_line its grid lines) waits to be opened, from
--     pre_timestamp
--   - match_time: no matchmaking until then; is_forbidden / back_forbidden
--     forbid it for good (never set by belfast)
--   - inactive_num: the matches the commander was removed from for
--     inactivity today, inactive_state 1 once warned about it;
--     get_relief_num the relief payments taken today, all three reset
--     when reset_day (the midnight, server time, they were counted from)
--     passes
CREATE TABLE commander_auction (
	commander_id   INTEGER NOT NULL PRIMARY KEY REFERENCES commanders (id),
	activity_id    INTEGER NOT NULL,
	gold           INTEGER NOT NULL DEFAULT 0,
	game_num       INTEGER NOT NULL DEFAULT 0,
	buy_num        INTEGER NOT NULL DEFAULT 0,
	max_profit     INTEGER NOT NULL DEFAULT 0,
	acc_profit     BIGINT  NOT NULL DEFAULT 0,
	acc_buy_price  INTEGER NOT NULL DEFAULT 0,
	acc_item_price INTEGER NOT NULL DEFAULT 0,
	max_lot_value  INTEGER NOT NULL DEFAULT 0,
	item_list      TEXT    NOT NULL DEFAULT '[]',
	pre_buy_state  INTEGER NOT NULL DEFAULT 0,
	pre_timestamp  BIGINT  NOT NULL DEFAULT 0,
	pre_box        TEXT    NOT NULL DEFAULT '',
	pre_line       INTEGER NOT NULL DEFAULT 0,
	match_time     BIGINT  NOT NULL DEFAULT 0,
	is_forbidden   INTEGER NOT NULL DEFAULT 0,
	back_forbidden INTEGER NOT NULL DEFAULT 0,
	inactive_num   INTEGER NOT NULL DEFAULT 0,
	inactive_state INTEGER NOT NULL DEFAULT 0,
	get_relief_num INTEGER NOT NULL DEFAULT 0,
	reset_day      BIGINT  NOT NULL DEFAULT 0
);
