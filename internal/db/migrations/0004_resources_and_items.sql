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
