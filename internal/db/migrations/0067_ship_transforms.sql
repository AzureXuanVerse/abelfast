-- a ship's retrofit nodes (SHIPINFO.transform_list, Ship.transforms): the
-- transform_data_template id and the level it reached. A node another
-- one's edit_trans removes loses its row.
CREATE TABLE ship_transforms (
	ship_id      INTEGER NOT NULL REFERENCES ships (id) ON DELETE CASCADE,
	transform_id INTEGER NOT NULL,
	level        INTEGER NOT NULL,
	PRIMARY KEY (ship_id, transform_id)
);

-- the archives' retrofit flag (SC_17001.transform_list, ShipGroup.trans):
-- a ship of the group finished a retrofit node that gives a skin, which
-- unlocks the retrofit skin in the archives. Kept after the ship is retired.
ALTER TABLE commander_ship_groups ADD COLUMN trans BOOLEAN NOT NULL DEFAULT FALSE;
