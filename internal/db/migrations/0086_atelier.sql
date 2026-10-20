-- the Atelier (ACTIVITY_TYPE_ATELIER_LINK, AtelierActivity, SC_26052): the
-- materials the commander holds (activity_ryza_item, DROP_TYPE_RYZA_DROP),
-- how many times each recipe was synthesized (activity_ryza_recipe) and the
-- five buff slots (pos 1-5: a STRENGTHEN material and how many of its
-- benefit_buff levels are used).
CREATE TABLE commander_atelier_items (
	commander_id INTEGER NOT NULL REFERENCES commanders (id),
	item_id      INTEGER NOT NULL,
	count        INTEGER NOT NULL,
	PRIMARY KEY (commander_id, item_id)
);

CREATE TABLE commander_atelier_recipes (
	commander_id INTEGER NOT NULL REFERENCES commanders (id),
	recipe_id    INTEGER NOT NULL,
	used         INTEGER NOT NULL,
	PRIMARY KEY (commander_id, recipe_id)
);

CREATE TABLE commander_atelier_slots (
	commander_id INTEGER NOT NULL REFERENCES commanders (id),
	pos          INTEGER NOT NULL,
	item_id      INTEGER NOT NULL,
	item_num     INTEGER NOT NULL,
	PRIMARY KEY (commander_id, pos)
);
