-- activities' key-value lists (ACTIVITYINFO date1_key_value_list, read by
-- the client as data1KeyValueList[list_key][item_key] = value): the
-- virtual bag's items (ACTIVITY_TYPE_VIRTUAL_BAG, list 1, by item id).
CREATE TABLE commander_activity_key_values (
	commander_id INTEGER NOT NULL REFERENCES commanders (id),
	activity_id  INTEGER NOT NULL,
	list_key     INTEGER NOT NULL,
	item_key     INTEGER NOT NULL,
	value        INTEGER NOT NULL,
	PRIMARY KEY (commander_id, activity_id, list_key, item_key)
);

-- the Feast (ACTIVITY_TYPE_FEAST, FeastProxy, SC_26157): when the party
-- ships were last drawn (unix seconds, 0 never).
CREATE TABLE commander_feasts (
	commander_id INTEGER NOT NULL REFERENCES commanders (id),
	activity_id  INTEGER NOT NULL,
	refresh_time BIGINT  NOT NULL,
	PRIMARY KEY (commander_id, activity_id)
);

-- the Feast's party ships (party_roles) by ship group: the bubble over
-- their head (FeastShip.BUBBLE_TYPE_*, 0 once interacted with) and the line
-- they say for it (speech_bubble).
CREATE TABLE commander_feast_ships (
	commander_id  INTEGER NOT NULL REFERENCES commanders (id),
	activity_id   INTEGER NOT NULL,
	group_id      INTEGER NOT NULL,
	bubble        INTEGER NOT NULL,
	speech_bubble INTEGER NOT NULL,
	PRIMARY KEY (commander_id, activity_id, group_id)
);

-- the Feast's invited ships (special_roles) the commander did something
-- with, by ship group: state is InvitedFeastShip.STATE_* (1 invitation
-- made, 2 given), gift 1 once the gift is given.
CREATE TABLE commander_feast_invitations (
	commander_id INTEGER NOT NULL REFERENCES commanders (id),
	activity_id  INTEGER NOT NULL,
	group_id     INTEGER NOT NULL,
	state        INTEGER NOT NULL,
	gift         INTEGER NOT NULL,
	PRIMARY KEY (commander_id, activity_id, group_id)
);
