-- Meowfficers, the client's "commanders" (CommanderProxy, protocol p25),
-- sent as COMMANDERINFO
CREATE TABLE meowfficers (
	id              INTEGER PRIMARY KEY AUTOINCREMENT,
	commander_id    INTEGER NOT NULL REFERENCES commanders (id),
	template_id     INTEGER NOT NULL, -- commander_data_template
	level           INTEGER NOT NULL DEFAULT 1,
	exp             INTEGER NOT NULL DEFAULT 0, -- toward the next level
	locked          BOOLEAN NOT NULL DEFAULT 0,
	name            TEXT    NOT NULL DEFAULT '', -- '' is the template's
	rename_time     BIGINT  NOT NULL DEFAULT 0, -- unix seconds of the last rename
	-- talents (commander_ability_template ids): learned (ability), the
	-- ones it came with (ability_origin, what a reset goes back to), the
	-- ones offered to learn (CS_25010) and when the offer was drawn
	talents         TEXT    NOT NULL DEFAULT '[]',
	talent_origins  TEXT    NOT NULL DEFAULT '[]',
	talent_choices  TEXT    NOT NULL DEFAULT '[]',
	talent_time     BIGINT  NOT NULL DEFAULT 0, -- ability_time
	used_pt         INTEGER NOT NULL DEFAULT 0, -- talent points spent
	-- its skill (commander_skill_template, the template's skill_id at first)
	skill_id        INTEGER NOT NULL,
	skill_exp       INTEGER NOT NULL DEFAULT 0,
	-- unix seconds of the last cattery clean / play / feed it took part in
	home_clean_time BIGINT  NOT NULL DEFAULT 0,
	home_play_time  BIGINT  NOT NULL DEFAULT 0,
	home_feed_time  BIGINT  NOT NULL DEFAULT 0
);

CREATE INDEX meowfficers_commander_id ON meowfficers (commander_id);

-- cat boxes in training, 1..gameset commander_box_count; no row is an
-- empty box. What it holds is drawn when training starts (each meowfficer
-- trains in its own time); a box queued behind the ones training starts at
-- begin_time (later than now), it is done from finish_time.
CREATE TABLE meowfficer_boxes (
	commander_id INTEGER NOT NULL REFERENCES commanders (id),
	box_id       INTEGER NOT NULL,
	pool_id      INTEGER NOT NULL, -- commander_data_create_material
	template_id  INTEGER NOT NULL, -- the meowfficer it yields
	begin_time   BIGINT  NOT NULL,
	finish_time  BIGINT  NOT NULL,
	PRIMARY KEY (commander_id, box_id)
);

-- the per-commander state: Akashi's cat boxes bought on reserve_day's day
-- (SC_25001 usage_count) and ever, and the cattery (SC_25027: level, exp
-- toward the next, cleanliness)
CREATE TABLE meowfficer_states (
	commander_id   INTEGER NOT NULL PRIMARY KEY REFERENCES commanders (id),
	reserve_count  INTEGER NOT NULL DEFAULT 0,
	reserve_day    BIGINT  NOT NULL DEFAULT 0, -- unix seconds of the last purchase
	reserve_total  INTEGER NOT NULL DEFAULT 0,
	home_level     INTEGER NOT NULL DEFAULT 1, -- commander_home
	home_exp       INTEGER NOT NULL DEFAULT 0,
	home_clean     INTEGER NOT NULL DEFAULT 0
);

-- the cattery's slots (Comf-Forts) that ever changed; the others are empty
-- with every daily op available. exp_time is when the meowfficer's EXP was
-- last settled, *_time when the slot's daily ops were last done.
CREATE TABLE meowfficer_catteries (
	commander_id  INTEGER NOT NULL REFERENCES commanders (id),
	slot_id       INTEGER NOT NULL,
	meowfficer_id INTEGER NOT NULL DEFAULT 0, -- 0 empty
	style         INTEGER NOT NULL DEFAULT 1, -- commander_home_style
	exp_time      BIGINT  NOT NULL DEFAULT 0,
	clean_time    BIGINT  NOT NULL DEFAULT 0,
	feed_time     BIGINT  NOT NULL DEFAULT 0,
	play_time     BIGINT  NOT NULL DEFAULT 0,
	PRIMARY KEY (commander_id, slot_id)
);

-- meowfficers assigned to a formation fleet (GROUPINFO_P12.commanders):
-- pos 1 command, 2 support
CREATE TABLE fleet_meowfficers (
	commander_id  INTEGER NOT NULL REFERENCES commanders (id),
	fleet_id      INTEGER NOT NULL,
	pos           INTEGER NOT NULL,
	meowfficer_id INTEGER NOT NULL REFERENCES meowfficers (id),
	PRIMARY KEY (commander_id, fleet_id, pos),
	UNIQUE (meowfficer_id)
);

-- the saved meowfficer pairs (SC_25001 presets, CommanderProxy prefabFleet,
-- 1..MAX_PREFAB_FLEET)
CREATE TABLE meowfficer_presets (
	commander_id INTEGER NOT NULL REFERENCES commanders (id),
	preset_id    INTEGER NOT NULL,
	name         TEXT    NOT NULL DEFAULT '',
	PRIMARY KEY (commander_id, preset_id)
);

CREATE TABLE meowfficer_preset_slots (
	commander_id  INTEGER NOT NULL,
	preset_id     INTEGER NOT NULL,
	pos           INTEGER NOT NULL,
	meowfficer_id INTEGER NOT NULL REFERENCES meowfficers (id),
	PRIMARY KEY (commander_id, preset_id, pos),
	FOREIGN KEY (commander_id, preset_id) REFERENCES meowfficer_presets (commander_id, preset_id)
);
