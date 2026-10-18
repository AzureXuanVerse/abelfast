-- the backyard (DormProxy, SC_19001) besides its food (commanders.dorm_food)
-- and furniture (commander_furniture); no row is the new commander's: level
-- 1, one floor, two training slots, unnamed, nothing settled yet
CREATE TABLE commander_dorms (
	commander_id            INTEGER NOT NULL PRIMARY KEY REFERENCES commanders (id),
	level                   INTEGER NOT NULL DEFAULT 1, -- dorm_data_template id
	floor_num               INTEGER NOT NULL DEFAULT 1,
	exp_pos                 INTEGER NOT NULL DEFAULT 2, -- training (and resting) slots
	food_max_increase       INTEGER NOT NULL DEFAULT 0,
	food_max_increase_count INTEGER NOT NULL DEFAULT 0,
	name                    TEXT    NOT NULL DEFAULT '',
	-- the training exp is settled up to load_time (unix seconds), next
	-- settlement at next_timestamp (SC_19027)
	load_time               BIGINT  NOT NULL DEFAULT 0,
	next_timestamp          BIGINT  NOT NULL DEFAULT 0
);

-- the ships in the backyard (SHIPINFO_IN_DORM): floor 1 trains, floor 2
-- rests (DormShip.FLOOR_1 / FLOOR_2); pop_icon / pop_intimacy are the
-- coins and affection waiting above it, pop_at when it next may pop some
CREATE TABLE dorm_ships (
	ship_id      INTEGER NOT NULL PRIMARY KEY REFERENCES ships (id) ON DELETE CASCADE,
	commander_id INTEGER NOT NULL REFERENCES commanders (id),
	floor        INTEGER NOT NULL,
	pop_icon     INTEGER NOT NULL DEFAULT 0,
	pop_intimacy INTEGER NOT NULL DEFAULT 0,
	pop_at       BIGINT  NOT NULL
);

CREATE INDEX dorm_ships_commander_id ON dorm_ships (commander_id);

-- the furniture placed on each floor (PutFurnitureCommand, CS_19008): a
-- FURFLOORPUTINFO as the client sent it
CREATE TABLE dorm_floors (
	commander_id INTEGER NOT NULL REFERENCES commanders (id),
	floor        INTEGER NOT NULL,
	layout       BLOB    NOT NULL,
	PRIMARY KEY (commander_id, floor)
);

-- the commander's own theme templates (AddBackYardThemeTemplateCommand,
-- CS_19109), by slot: a DORMTHEME
CREATE TABLE dorm_themes (
	commander_id INTEGER NOT NULL REFERENCES commanders (id),
	pos          INTEGER NOT NULL,
	theme        BLOB    NOT NULL,
	PRIMARY KEY (commander_id, pos)
);
