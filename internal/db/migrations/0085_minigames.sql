-- mini-game hubs (mini_game_hub, MiniGameProxy, SC_26102): the stages the
-- commander completed (used_cnt), whether the final reward was claimed
-- (ultimate) and when the commander first saw the hub open (unix seconds,
-- the start of its daily tickets when its activity has no start time).
CREATE TABLE commander_minigame_hubs (
	commander_id INTEGER NOT NULL REFERENCES commanders (id),
	hub_id       INTEGER NOT NULL,
	used_cnt     INTEGER NOT NULL DEFAULT 0,
	ultimate     INTEGER NOT NULL DEFAULT 0,
	opened_at    BIGINT  NOT NULL,
	PRIMARY KEY (commander_id, hub_id)
);

-- best score of each mini-game (MiniGameOPCommand.CMD_HIGH_SCORE, the hubs'
-- maxscores, the friend rankings): score and its second value (time_data).
CREATE TABLE commander_minigame_scores (
	commander_id INTEGER NOT NULL REFERENCES commanders (id),
	game_id      INTEGER NOT NULL,
	score        INTEGER NOT NULL,
	extra        INTEGER NOT NULL,
	PRIMARY KEY (commander_id, game_id)
);
CREATE INDEX commander_minigame_scores_game ON commander_minigame_scores (game_id);

-- what a mini-game saves on the server (CMD_SPECIAL_GAME, MiniGameData's
-- runtime data): its elements (datas), and for the shrines the wishes left
-- (count) on the server day starting at count_day (unix seconds) and the
-- gold the commander offered (gold).
CREATE TABLE commander_minigame_data (
	commander_id INTEGER NOT NULL REFERENCES commanders (id),
	game_id      INTEGER NOT NULL,
	datas        TEXT    NOT NULL DEFAULT '[]',
	count        INTEGER NOT NULL DEFAULT 0,
	count_day    BIGINT  NOT NULL DEFAULT 0,
	gold         INTEGER NOT NULL DEFAULT 0,
	PRIMARY KEY (commander_id, game_id)
);

-- a mini-game's key-value lists (MINIGAMEDATA.date1_key_value_list, the
-- runtime kvpElements): entries of list list_key.
CREATE TABLE commander_minigame_values (
	commander_id INTEGER NOT NULL REFERENCES commanders (id),
	game_id      INTEGER NOT NULL,
	list_key     INTEGER NOT NULL,
	key          INTEGER NOT NULL,
	value        INTEGER NOT NULL,
	value2       INTEGER NOT NULL DEFAULT 0,
	PRIMARY KEY (commander_id, game_id, list_key, key)
);
