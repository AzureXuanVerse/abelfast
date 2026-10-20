-- Project Identity (Oceana, Explorer...), the newer child-raising game
-- (NewEducateProxy, protocol module 29): one row per commander and
-- child2_data character. state is the running game as JSON (neState in
-- internal/game/neweducate.go: the round, attributes, resources, buffs,
-- the FSM and its caches, plus what only the server knows); permanent is
-- what outlives a game (games played, endings, polaroids, collection).
-- No row until the character is first requested (SC_29002).
CREATE TABLE commander_new_educate (
	commander_id INTEGER NOT NULL REFERENCES commanders (id),
	character_id INTEGER NOT NULL,
	state        TEXT    NOT NULL,
	permanent    TEXT    NOT NULL,
	PRIMARY KEY (commander_id, character_id)
);
