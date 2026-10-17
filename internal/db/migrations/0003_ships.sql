-- the commander's ships (SHIPINFO)
CREATE TABLE ships (
	id                    INTEGER PRIMARY KEY AUTOINCREMENT,
	commander_id          INTEGER NOT NULL REFERENCES commanders (id),
	template_id           INTEGER NOT NULL,
	level                 INTEGER NOT NULL,
	exp                   INTEGER NOT NULL,
	energy                INTEGER NOT NULL,
	intimacy              INTEGER NOT NULL,
	max_level             INTEGER NOT NULL,
	is_locked             BOOLEAN NOT NULL,
	created_at            BIGINT  NOT NULL, -- unix seconds
	state                 INTEGER NOT NULL DEFAULT 0,
	proficiency           INTEGER NOT NULL DEFAULT 0,
	skin_id               INTEGER NOT NULL DEFAULT 0,
	propose               INTEGER NOT NULL DEFAULT 0,
	name                  TEXT    NOT NULL DEFAULT '',
	change_name_timestamp INTEGER NOT NULL DEFAULT 0,
	common_flag           INTEGER NOT NULL DEFAULT 0
);

CREATE INDEX ships_commander_id ON ships (commander_id);

-- what a ship has equipped, one row per slot (1-5), equip_id 0 when empty;
-- equip_id is an equip_data_template id
CREATE TABLE ship_equipments (
	ship_id  INTEGER NOT NULL REFERENCES ships (id) ON DELETE CASCADE,
	pos      INTEGER NOT NULL,
	equip_id INTEGER NOT NULL,
	skin_id  INTEGER NOT NULL DEFAULT 0,
	PRIMARY KEY (ship_id, pos)
);

-- learnt skills (skill ids from ship_data_template.buff_list)
CREATE TABLE ship_skills (
	ship_id  INTEGER NOT NULL REFERENCES ships (id) ON DELETE CASCADE,
	skill_id INTEGER NOT NULL,
	level    INTEGER NOT NULL,
	exp      INTEGER NOT NULL DEFAULT 0,
	PRIMARY KEY (ship_id, skill_id)
);

-- the archives (CollectionProxy): every ship group ever owned, with the best
-- values reached
CREATE TABLE commander_ship_groups (
	commander_id INTEGER NOT NULL REFERENCES commanders (id),
	group_id     INTEGER NOT NULL, -- ship_data_template.group_type
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
