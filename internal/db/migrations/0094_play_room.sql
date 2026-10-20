-- the play room games (PlayRoomProxy, mode_room): the look a commander
-- picked for a game type (CS_23029, PTGC_GAME_USER_VIEW, the island
-- character seated at the cheat bar table)
CREATE TABLE commander_play_room_views (
	commander_id INTEGER NOT NULL REFERENCES commanders (id),
	game_type    INTEGER NOT NULL,
	ship_id      INTEGER NOT NULL,
	skin_id      INTEGER NOT NULL DEFAULT 0,
	color        INTEGER NOT NULL DEFAULT 0,
	dress_list   TEXT    NOT NULL DEFAULT '[]',
	PRIMARY KEY (commander_id, game_type)
);

-- the ranked matchmaking score of a game type (PTGC_PLAYER_RANK.score,
-- SC_23108 / SC_23117 cur_score) in a season, the id of the
-- mode_room.activity_type activity it was played in (0 outside one);
-- max_score is the season's best (the activity's data2)
CREATE TABLE commander_play_room_scores (
	commander_id INTEGER NOT NULL REFERENCES commanders (id),
	game_type    INTEGER NOT NULL,
	season       INTEGER NOT NULL,
	score        INTEGER NOT NULL DEFAULT 0,
	max_score    INTEGER NOT NULL DEFAULT 0,
	PRIMARY KEY (commander_id, game_type, season)
);
