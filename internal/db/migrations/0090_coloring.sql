-- the coloring activity (ActivityConst.ACTIVITY_TYPE_COLORING_ALPHA,
-- ColoringProxy, SC_26001): the cells the commander filled in each picture
-- (activity_coloring_template id) of an activity, color being the index in
-- the picture's colors (1 the first)
CREATE TABLE commander_coloring_cells (
	commander_id INTEGER NOT NULL REFERENCES commanders (id),
	activity_id  INTEGER NOT NULL,
	group_id     INTEGER NOT NULL,
	row          INTEGER NOT NULL,
	col          INTEGER NOT NULL,
	color        INTEGER NOT NULL,
	PRIMARY KEY (commander_id, activity_id, group_id, row, col)
);

-- the pictures whose reward the commander took, with the drops it gave
-- (SC_26001.award_list)
CREATE TABLE commander_coloring_awards (
	commander_id INTEGER NOT NULL REFERENCES commanders (id),
	activity_id  INTEGER NOT NULL,
	group_id     INTEGER NOT NULL,
	drops        TEXT    NOT NULL DEFAULT '[]',
	PRIMARY KEY (commander_id, activity_id, group_id)
);
