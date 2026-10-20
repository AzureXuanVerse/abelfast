-- The Archives' appreciation (AppreciateProxy, SC_11003.appreciation and
-- cartoon_*_mark): the gallery pictures and songs unlocked (CS_17501,
-- CS_17503) and liked (CS_17505, CS_17507), the manga chapters read and
-- liked (CS_17509, CS_17511), in the order they were added.
CREATE TABLE commander_appreciation (
	commander_id INTEGER NOT NULL REFERENCES commanders (id),
	kind         INTEGER NOT NULL, -- appreciation* in internal/game/appreciation.go
	item_id      INTEGER NOT NULL, -- gallery_config, music_collect_config or cartoon id
	PRIMARY KEY (commander_id, kind, item_id)
);

-- the main scene's music player (CS_17513): music_no is 0 (none), 999 (the
-- liked songs) or the first song of an album, music_mode the loop type
-- (0 list, 1 random, 2 one); no row is 0, 0.
CREATE TABLE commander_music_player (
	commander_id INTEGER NOT NULL PRIMARY KEY REFERENCES commanders (id),
	music_no     INTEGER NOT NULL,
	music_mode   INTEGER NOT NULL
);
