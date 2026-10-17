-- ship comments (ShipEvaluationLayer, SHIP_DISCUSS_INFO): shared by every
-- commander of the server, keyed by ship group. A comment's poster shows
-- under their current name. The ship's hearts are commander_ship_groups
-- heart_flag, counted over every commander.
CREATE TABLE ship_discussions (
	id           INTEGER PRIMARY KEY AUTOINCREMENT, -- DISCUSS_INFO id
	group_id     INTEGER NOT NULL, -- ship_data_template.group_type
	commander_id INTEGER NOT NULL REFERENCES commanders (id),
	context      TEXT    NOT NULL,
	posted_at    BIGINT  NOT NULL -- unix seconds
);

CREATE INDEX ship_discussions_group ON ship_discussions (group_id);
CREATE INDEX ship_discussions_commander ON ship_discussions (commander_id, posted_at);

-- one review per commander and comment (CS_17105 good_or_bad: 0 good,
-- 1 bad), DISCUSS_INFO good_count / bad_count count them
CREATE TABLE ship_discussion_votes (
	discussion_id INTEGER NOT NULL REFERENCES ship_discussions (id),
	commander_id  INTEGER NOT NULL REFERENCES commanders (id),
	good_or_bad   INTEGER NOT NULL CHECK (good_or_bad IN (0, 1)),
	PRIMARY KEY (discussion_id, commander_id)
);

-- reports (CS_17109), kept for whoever runs the server: a commander's
-- latest reason per comment
CREATE TABLE ship_discussion_reports (
	discussion_id INTEGER NOT NULL REFERENCES ship_discussions (id),
	commander_id  INTEGER NOT NULL REFERENCES commanders (id),
	reason        TEXT    NOT NULL,
	reported_at   BIGINT  NOT NULL, -- unix seconds
	PRIMARY KEY (discussion_id, commander_id)
);
