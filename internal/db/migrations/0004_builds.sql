-- ships under construction; their position in the docks is their rank by id
CREATE TABLE builds (
	id           INTEGER PRIMARY KEY AUTOINCREMENT,
	commander_id INTEGER NOT NULL REFERENCES commanders (id),
	pool_id      INTEGER NOT NULL, -- ship_data_create_material id
	template_id  INTEGER NOT NULL, -- the ship it yields, decided at build time
	duration     INTEGER NOT NULL, -- seconds
	-- unix seconds; finish_time - duration is when it started, later than
	-- now while queued
	finish_time  BIGINT  NOT NULL
);

CREATE INDEX builds_commander_id ON builds (commander_id);

-- build points (ship_data_create_material.exchange_count); the builds ever
-- started are the TaskSubBuild task counter
CREATE TABLE commander_build_states (
	commander_id   INTEGER PRIMARY KEY REFERENCES commanders (id),
	exchange_count INTEGER NOT NULL DEFAULT 0
);
