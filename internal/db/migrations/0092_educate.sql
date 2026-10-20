-- Project Identity: TB, the first child-raising game (EducateProxy,
-- protocol module 27): the commander's running game as JSON (educateState
-- in internal/game/educate.go: TB's time, attributes, resources, tasks,
-- shops, plans... and the collections kept from one game to the next).
-- No row until the game is first requested (SC_27001).
CREATE TABLE commander_educate (
	commander_id INTEGER NOT NULL PRIMARY KEY REFERENCES commanders (id),
	state        TEXT    NOT NULL
);

-- the special secretary (secretary_special_ship id, 0 none) shown in the
-- commander's profile and as flagship (SC_11003.child_display, CS_27041)
ALTER TABLE commanders ADD COLUMN child_display INTEGER NOT NULL DEFAULT 0;
