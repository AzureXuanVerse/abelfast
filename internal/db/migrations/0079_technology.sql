-- Research Academy projects (TechnologyProxy, SC_63000)

-- the commander's research settings: refreshed_at when the projects were
-- last refreshed by hand (unix seconds, 0 never; refresh_flag is 1 on that
-- server day), focus the series pool 2 favours (its refresh_list target, 0
-- none), catchup_version / catchup_target the catch-up research
-- (technology_catchup_template id) and its ship group (0 none)
CREATE TABLE technology_states (
	commander_id    INTEGER NOT NULL PRIMARY KEY REFERENCES commanders (id),
	refreshed_at    BIGINT  NOT NULL DEFAULT 0,
	focus           INTEGER NOT NULL DEFAULT 0,
	catchup_version INTEGER NOT NULL DEFAULT 0,
	catchup_target  INTEGER NOT NULL DEFAULT 0
);

-- the projects offered (refresh_list), in their order: tech_id a
-- technology_data_template id of the pool pool_id, finish_at when it ends
-- once started (unix seconds, 0 not started)
CREATE TABLE technology_offers (
	commander_id INTEGER NOT NULL REFERENCES commanders (id),
	pos          INTEGER NOT NULL,
	pool_id      INTEGER NOT NULL,
	tech_id      INTEGER NOT NULL,
	finish_at    BIGINT  NOT NULL DEFAULT 0,
	PRIMARY KEY (commander_id, pos)
);

-- the research queue (queue): projects moved out of the offers while
-- running, claimed in finish_at order (unix seconds)
CREATE TABLE technology_queue (
	id           INTEGER NOT NULL PRIMARY KEY AUTOINCREMENT,
	commander_id INTEGER NOT NULL REFERENCES commanders (id),
	tech_id      INTEGER NOT NULL,
	finish_at    BIGINT  NOT NULL
);

CREATE INDEX technology_queue_commander ON technology_queue (commander_id);

-- catch-up blueprints obtained (catchup.pursuings): per catch-up version,
-- ship_group 0 counts the priority ones (shared by every priority ship of
-- the version), a decisive ship's group its own
CREATE TABLE technology_catchups (
	commander_id INTEGER NOT NULL REFERENCES commanders (id),
	version      INTEGER NOT NULL,
	ship_group   INTEGER NOT NULL,
	number       INTEGER NOT NULL,
	PRIMARY KEY (commander_id, version, ship_group)
);
