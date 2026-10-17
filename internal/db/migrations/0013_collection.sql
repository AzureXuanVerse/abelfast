-- the archives (CollectionProxy): every ship group ever owned, with the best
-- values reached
CREATE TABLE commander_ship_groups (
	commander_id INTEGER NOT NULL REFERENCES commanders (id),
	group_id     INTEGER NOT NULL, -- ship_data_template.group_type
	star         INTEGER NOT NULL,
	intimacy_max INTEGER NOT NULL,
	lv_max       INTEGER NOT NULL,
	marry_flag   INTEGER NOT NULL DEFAULT 0,
	heart_flag   INTEGER NOT NULL DEFAULT 0,
	PRIMARY KEY (commander_id, group_id)
);

-- medal_template progress that isn't derived from ships
CREATE TABLE commander_trophies (
	commander_id INTEGER NOT NULL REFERENCES commanders (id),
	trophy_id    INTEGER NOT NULL,
	progress     INTEGER NOT NULL,
	timestamp    INTEGER NOT NULL DEFAULT 0,
	PRIMARY KEY (commander_id, trophy_id)
);
