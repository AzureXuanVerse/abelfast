-- research ships (Dev. Dock, TechnologyProxy's blueprints, SC_63100)

-- a ship_data_blueprint the commander touched: start_time when its research
-- (re)started (unix seconds, 0 when not running), duration how long it ran
-- before it was stopped (BLUPRINTINFO.start_duration, 0 while running), and
-- ship_id once the ship is built (0 before). Its development level and exp
-- are the ship's own (ship_strengths, the one SHIPINFO.strength_list entry
-- of a research ship: attr_id the level, fate simulation levels included).
CREATE TABLE commander_blueprints (
	commander_id INTEGER NOT NULL REFERENCES commanders (id),
	blueprint_id INTEGER NOT NULL,
	ship_id      INTEGER NOT NULL DEFAULT 0,
	start_time   BIGINT  NOT NULL DEFAULT 0,
	duration     BIGINT  NOT NULL DEFAULT 0,
	PRIMARY KEY (commander_id, blueprint_id)
);

-- cold_time: when another research may start (unix seconds); pursued /
-- pursued_ur: the catch-up enhancements bought with coins on pursue_day
-- (the unix start of that day, 04:00 server time), Priority and Decisive
CREATE TABLE commander_blueprint_state (
	commander_id INTEGER PRIMARY KEY REFERENCES commanders (id),
	cold_time    BIGINT  NOT NULL DEFAULT 0,
	pursue_day   BIGINT  NOT NULL DEFAULT 0,
	pursued      INTEGER NOT NULL DEFAULT 0,
	pursued_ur   INTEGER NOT NULL DEFAULT 0
);
