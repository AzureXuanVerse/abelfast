-- equipment skins in the gear skin box (EquipmentProxy equipmentSkinIds, sent
-- in SC_14101), stacked by equip_skin_template id; the ones worn sit in
-- ship_equipments.skin_id instead
CREATE TABLE commander_equip_skins (
	commander_id INTEGER NOT NULL REFERENCES commanders (id),
	skin_id      INTEGER NOT NULL,
	count        INTEGER NOT NULL,
	PRIMARY KEY (commander_id, skin_id)
);
