-- server-wide key/value settings (the ticket key...)
CREATE TABLE settings (
	key   TEXT PRIMARY KEY,
	value BLOB NOT NULL
);

-- SC_11003 commander state. The defaults are what the official server gives
-- a new commander: the capacities are gameset shipyard_capacity,
-- equip_capacity and commander_capacity, the rest is captured.
CREATE TABLE commanders (
	id                INTEGER PRIMARY KEY AUTOINCREMENT,
	account_id        INTEGER NOT NULL UNIQUE,
	name              TEXT    NOT NULL,
	starter_ship      INTEGER NOT NULL, -- ship_data_template id picked at creation
	registered_at     BIGINT  NOT NULL, -- unix seconds
	-- tutorial progress, the old (SG001) and new (SG002) guides count separately
	guide_index       INTEGER NOT NULL DEFAULT 1,
	new_guide_index   INTEGER NOT NULL DEFAULT 1,
	level             INTEGER NOT NULL DEFAULT 1,
	exp               INTEGER NOT NULL DEFAULT 0,
	ship_bag_max      INTEGER NOT NULL DEFAULT 150,
	equip_bag_max     INTEGER NOT NULL DEFAULT 300,
	commander_bag_max INTEGER NOT NULL DEFAULT 40,
	rank              INTEGER NOT NULL DEFAULT 1,
	chat_room_id      INTEGER NOT NULL DEFAULT 2,
	mail_storeroom_lv INTEGER NOT NULL DEFAULT 1,
	manifesto         TEXT    NOT NULL DEFAULT ''
);

-- stories the commander has seen, so they don't replay
CREATE TABLE commander_stories (
	commander_id INTEGER NOT NULL REFERENCES commanders (id),
	story_id     INTEGER NOT NULL,
	PRIMARY KEY (commander_id, story_id)
);

-- secretaries (SC_11003.character), position 1 is the main one
CREATE TABLE commander_secretaries (
	commander_id INTEGER NOT NULL REFERENCES commanders (id),
	position     INTEGER NOT NULL,
	ship_id      INTEGER NOT NULL,
	phantom_id   INTEGER NOT NULL,
	PRIMARY KEY (commander_id, position)
);

-- owned backyard furniture (furniture_data_template ids)
CREATE TABLE commander_furniture (
	commander_id INTEGER NOT NULL REFERENCES commanders (id),
	furniture_id INTEGER NOT NULL,
	count        INTEGER NOT NULL,
	PRIMARY KEY (commander_id, furniture_id)
);
