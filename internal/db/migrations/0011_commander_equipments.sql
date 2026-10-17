-- equipment in the depot (not on a ship), stacked by equip_data_template id
CREATE TABLE commander_equipments (
	commander_id INTEGER NOT NULL REFERENCES commanders (id),
	equip_id     INTEGER NOT NULL,
	count        INTEGER NOT NULL,
	PRIMARY KEY (commander_id, equip_id)
);
