-- player_resource amounts (1 gold, 2 oil, 4 gems, ...)
CREATE TABLE commander_resources (
	commander_id INTEGER NOT NULL REFERENCES commanders (id),
	resource_id  INTEGER NOT NULL,
	amount       INTEGER NOT NULL,
	PRIMARY KEY (commander_id, resource_id)
);

-- bag items (item_data_statistics ids, e.g. 20001 wisdom cube)
CREATE TABLE commander_items (
	commander_id INTEGER NOT NULL REFERENCES commanders (id),
	item_id      INTEGER NOT NULL,
	count        INTEGER NOT NULL,
	PRIMARY KEY (commander_id, item_id)
);

-- equipment in the depot (not on a ship), stacked by equip_data_template id
CREATE TABLE commander_equipments (
	commander_id INTEGER NOT NULL REFERENCES commanders (id),
	equip_id     INTEGER NOT NULL,
	count        INTEGER NOT NULL,
	PRIMARY KEY (commander_id, equip_id)
);

-- virtual items (drop type 8) earned from tasks: 59012 weekly pt, ...; the
-- cruise pass pt (59599) are the TaskSubCruisePt task counter
CREATE TABLE commander_virtual_items (
	commander_id INTEGER NOT NULL REFERENCES commanders (id),
	item_id      INTEGER NOT NULL,
	amount       INTEGER NOT NULL,
	PRIMARY KEY (commander_id, item_id)
);
