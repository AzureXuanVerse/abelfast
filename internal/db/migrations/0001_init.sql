CREATE TABLE settings (
	key   TEXT PRIMARY KEY,
	value BLOB NOT NULL
);

CREATE TABLE commanders (
	id            INTEGER PRIMARY KEY AUTOINCREMENT,
	account_id    INTEGER NOT NULL UNIQUE,
	name          TEXT    NOT NULL,
	starter_ship  INTEGER NOT NULL,
	registered_at INTEGER NOT NULL -- unix seconds
);
