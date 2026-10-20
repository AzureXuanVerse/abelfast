-- CS_10994 reports (CheaterMarkCommand): the client flags its own commander
-- when a battle fails its verification (900 + CC_TYPE_* in duels) or a
-- CN-only client-side switch is used (CC_TYPE_99 / 100), one row per report.
CREATE TABLE cheater_marks (
	id           INTEGER PRIMARY KEY AUTOINCREMENT,
	commander_id INTEGER NOT NULL,
	account_id   INTEGER NOT NULL,
	reason       INTEGER NOT NULL, -- CS_10994.type
	marked_at    BIGINT  NOT NULL  -- unix seconds
);
CREATE INDEX cheater_marks_commander ON cheater_marks (commander_id);
