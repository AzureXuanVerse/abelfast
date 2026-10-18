-- the Arcade (GameRoomProxy, SC_26120): whether the commander took the
-- first visit's coins (GameRoomFirstCoinCommand, CS_26128)
CREATE TABLE commander_game_room (
	commander_id INTEGER NOT NULL PRIMARY KEY REFERENCES commanders (id),
	first_enter  INTEGER NOT NULL DEFAULT 0
);
