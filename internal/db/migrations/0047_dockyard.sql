-- the archives' best star (SHIP_STATISTICS_INFO.star): the highest
-- ship_data_template star any ship of the group reached, kept after the
-- ship is retired like the other best values (CollectionProxy.flushCollection
-- only raises it). 0 until a ship of the group is seen again: the base
-- template's star is the floor.
ALTER TABLE commander_ship_groups ADD COLUMN star INTEGER NOT NULL DEFAULT 0;

-- a ship's enhancement (SHIPINFO.strength_list, Ship.strengthList): the
-- exp fed into each ShipModAttr attribute (attr_id 2 cannon, 3 torpedo,
-- 4 anti-air, 5 air, 6 reload), capped by Ship:addModAttrExp
CREATE TABLE ship_strengths (
	ship_id INTEGER NOT NULL REFERENCES ships (id) ON DELETE CASCADE,
	attr_id INTEGER NOT NULL,
	exp     INTEGER NOT NULL,
	PRIMARY KEY (ship_id, attr_id)
);
