-- The ships a chapter has dropped for the commander (Chapter.dropShipIdList,
-- CS_13109): its display_icon ships not in the list show as unknown.
CREATE TABLE chapter_drop_ships (
	commander_id INTEGER NOT NULL REFERENCES commanders (id),
	chapter_id   INTEGER NOT NULL, -- chapter_template id
	ship_id      INTEGER NOT NULL, -- ship_data_template id, as dropped
	PRIMARY KEY (commander_id, chapter_id, ship_id)
);
