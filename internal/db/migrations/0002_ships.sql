CREATE TABLE ships (
	id           INTEGER PRIMARY KEY AUTOINCREMENT,
	commander_id INTEGER NOT NULL REFERENCES commanders (id),
	template_id  INTEGER NOT NULL,
	level        INTEGER NOT NULL,
	exp          INTEGER NOT NULL,
	energy       INTEGER NOT NULL,
	intimacy     INTEGER NOT NULL,
	max_level    INTEGER NOT NULL,
	is_locked    INTEGER NOT NULL,
	created_at   INTEGER NOT NULL -- unix seconds
);

CREATE INDEX ships_commander_id ON ships (commander_id);
