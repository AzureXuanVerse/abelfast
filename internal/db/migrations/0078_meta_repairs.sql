-- META ships' Rigging Fortification (MetaCharacterRepairCommand, CS_63301):
-- the ship_meta_repair rows done on each ship, sent back as
-- SHIPINFO.meta_repair_list (MetaCharacter reads each attribute's level from
-- the furthest of its ship_strengthen_meta repair_<attr> ids listed).
CREATE TABLE ship_meta_repairs (
	ship_id   INTEGER NOT NULL REFERENCES ships (id) ON DELETE CASCADE,
	repair_id INTEGER NOT NULL,
	PRIMARY KEY (ship_id, repair_id)
);
