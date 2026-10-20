-- War Archives (ChapterProxy remaster fields): the archive tickets
-- (ITEM_ID_REACT_CHAPTER_TICKET, SC_13001 react_chapter.count) and the server
-- day the daily ones were last claimed (CS_13503, react_chapter.daily_count).
CREATE TABLE remaster_states (
	commander_id INTEGER PRIMARY KEY REFERENCES commanders (id),
	tickets      INTEGER NOT NULL DEFAULT 0,
	claimed_day  BIGINT  NOT NULL DEFAULT 0 -- unix time of the claim day's midnight
);

-- The archive clear awards (re_map_template.drop_gain, CS_13505 / CS_13507):
-- clears counted towards each award and whether it was taken.
CREATE TABLE remaster_awards (
	commander_id INTEGER NOT NULL REFERENCES commanders (id),
	act_id       INTEGER NOT NULL, -- re_map_template.activity_id, 0 for the archives
	chapter_id   INTEGER NOT NULL, -- chapter_template id
	pos          INTEGER NOT NULL, -- 1-based index in drop_gain
	count        INTEGER NOT NULL DEFAULT 0,
	received     BOOLEAN NOT NULL DEFAULT FALSE,
	PRIMARY KEY (commander_id, act_id, chapter_id, pos)
);
