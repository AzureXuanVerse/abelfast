-- the Arcade's periodic state (GameRoomProxy, SC_26120): weekly_free_week is
-- the week (its Monday 00:00 server time, unix) whose free coins were taken
-- (GameRoomWeekCoinCommand, CS_26122); pay_coin_count the coins bought with
-- gold in the week starting at pay_week (GameRoomExchangeCoinCommand,
-- CS_26124, their price rises with it); monthly_ticket the tickets earned in
-- the month starting at ticket_month (GameRoomSuccessCommand, CS_26126)
ALTER TABLE commander_game_room ADD COLUMN weekly_free_week BIGINT NOT NULL DEFAULT 0;
ALTER TABLE commander_game_room ADD COLUMN pay_week BIGINT NOT NULL DEFAULT 0;
ALTER TABLE commander_game_room ADD COLUMN pay_coin_count INTEGER NOT NULL DEFAULT 0;
ALTER TABLE commander_game_room ADD COLUMN ticket_month BIGINT NOT NULL DEFAULT 0;
ALTER TABLE commander_game_room ADD COLUMN monthly_ticket INTEGER NOT NULL DEFAULT 0;

-- the best score of each Arcade game (game_room_template id), SC_26120 rooms
CREATE TABLE commander_game_room_scores (
	commander_id INTEGER NOT NULL REFERENCES commanders (id),
	room_id      INTEGER NOT NULL,
	max_score    INTEGER NOT NULL,
	PRIMARY KEY (commander_id, room_id)
);

-- gameroom_shop_template goods bought in the Prize Shop (MiniGameShopBuyCommand,
-- CS_26152), by the month (its first day 00:00 server time, unix) they were
-- bought in: counted against goods_purchase_limit for good, or only within
-- the month for the goods with month_re set
CREATE TABLE commander_mini_game_shop_purchases (
	commander_id INTEGER NOT NULL REFERENCES commanders (id),
	goods_id     INTEGER NOT NULL,
	month_start  BIGINT  NOT NULL,
	count        INTEGER NOT NULL,
	PRIMARY KEY (commander_id, goods_id, month_start)
);
