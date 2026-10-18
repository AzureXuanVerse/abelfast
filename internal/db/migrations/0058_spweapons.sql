-- the commander's augment modules (SPWEAPONINFO): template_id a
-- spweapon_data_statistics id, attr_1/attr_2 what was rolled on top of its
-- value_1/value_2, attr_temp_1/attr_temp_2 a reforge waiting to be kept or
-- discarded (0 none), pt the upgrade points past the current level
CREATE TABLE spweapons (
	id           INTEGER PRIMARY KEY AUTOINCREMENT,
	commander_id INTEGER NOT NULL REFERENCES commanders (id),
	template_id  INTEGER NOT NULL,
	attr_1       INTEGER NOT NULL DEFAULT 0,
	attr_2       INTEGER NOT NULL DEFAULT 0,
	attr_temp_1  INTEGER NOT NULL DEFAULT 0,
	attr_temp_2  INTEGER NOT NULL DEFAULT 0,
	pt           INTEGER NOT NULL DEFAULT 0
);

CREATE INDEX spweapons_commander_id ON spweapons (commander_id);

-- the augment a ship wears (SHIPINFO.spweapon); the others are in the bag
-- (SC_14001.spweapon_list). A ship retired or consumed gives it back.
CREATE TABLE ship_spweapons (
	ship_id     INTEGER PRIMARY KEY REFERENCES ships (id) ON DELETE CASCADE,
	spweapon_id INTEGER NOT NULL UNIQUE REFERENCES spweapons (id) ON DELETE CASCADE
);
