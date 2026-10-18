-- SC_11003.cd_list: when the commander may next do something rate limited,
-- by Player cdList key (model/vo/player.lua, 1 = CHANGE_NAME_KEY)
CREATE TABLE commander_cooldowns (
	commander_id INTEGER NOT NULL REFERENCES commanders (id),
	key          INTEGER NOT NULL,
	until        BIGINT  NOT NULL, -- unix seconds
	PRIMARY KEY (commander_id, key)
);
