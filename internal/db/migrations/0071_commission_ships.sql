-- The ships sent on a running commission (COLLECTIONINFO.ship_id_list,
-- EventInfo.shipIds: the client's "inEvent" ships), in the order sent.
CREATE TABLE commander_commission_ships (
	commander_id  INTEGER NOT NULL REFERENCES commanders (id),
	collection_id INTEGER NOT NULL, -- collection_template id
	pos           INTEGER NOT NULL, -- index in CS_13003.ship_id_list
	ship_id       INTEGER NOT NULL,
	PRIMARY KEY (commander_id, collection_id, pos)
);

-- When the commissions on offer were last refreshed for the server day
-- (the daily ones drawn, the previous day's given an over_time).
CREATE TABLE commander_commission_refresh (
	commander_id INTEGER PRIMARY KEY REFERENCES commanders (id),
	refreshed_at BIGINT NOT NULL -- unix time
);
