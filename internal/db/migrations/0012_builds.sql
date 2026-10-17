-- ships under construction; their position in the docks is their rank by id
CREATE TABLE builds (
	id           INTEGER PRIMARY KEY AUTOINCREMENT,
	commander_id INTEGER NOT NULL REFERENCES commanders (id),
	pool_id      INTEGER NOT NULL, -- ship_data_create_material id
	template_id  INTEGER NOT NULL, -- the ship it yields, decided at build time
	duration     INTEGER NOT NULL, -- seconds
	start_time   INTEGER NOT NULL, -- unix seconds, later than now while queued
	finish_time  INTEGER NOT NULL  -- unix seconds
);

CREATE INDEX builds_commander_id ON builds (commander_id);

-- construction counters
CREATE TABLE commander_build_states (
	commander_id   INTEGER PRIMARY KEY REFERENCES commanders (id),
	build_count    INTEGER NOT NULL DEFAULT 0, -- builds ever started
	exchange_count INTEGER NOT NULL DEFAULT 0  -- build points (ship_data_create_material.exchange_count)
);
