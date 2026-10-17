-- campaign progress per chapter (SC_13001 CHAPTERINFO)
CREATE TABLE chapter_progress (
	commander_id       INTEGER NOT NULL REFERENCES commanders (id),
	chapter_id         INTEGER NOT NULL,
	progress           INTEGER NOT NULL DEFAULT 0, -- 100 once cleared
	kill_boss_count    INTEGER NOT NULL DEFAULT 0,
	kill_enemy_count   INTEGER NOT NULL DEFAULT 0,
	take_box_count     INTEGER NOT NULL DEFAULT 0,
	defeat_count       INTEGER NOT NULL DEFAULT 0,
	today_defeat_count INTEGER NOT NULL DEFAULT 0,
	pass_count         INTEGER NOT NULL DEFAULT 0,
	PRIMARY KEY (commander_id, chapter_id)
);

-- the chapter being played, at most one per commander
CREATE TABLE chapter_states (
	commander_id INTEGER PRIMARY KEY REFERENCES commanders (id),
	state        BLOB    NOT NULL, -- CURRENTCHAPTERINFO (SC_13102 / SC_13000), its id is the chapter's
	ambushed     TEXT    NOT NULL DEFAULT '' -- "row,column;..." scripted ambush cells already sprung
);

-- commissions on offer or running (EventProxy, SC_13002 COLLECTIONINFO)
CREATE TABLE commander_commissions (
	commander_id  INTEGER NOT NULL REFERENCES commanders (id),
	collection_id INTEGER NOT NULL, -- collection_template id
	finish_time   INTEGER NOT NULL DEFAULT 0,
	over_time     INTEGER NOT NULL DEFAULT 0,
	PRIMARY KEY (commander_id, collection_id)
);
