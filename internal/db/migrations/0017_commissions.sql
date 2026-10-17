-- commissions on offer or running (EventProxy, SC_13002 COLLECTIONINFO)
CREATE TABLE commander_commissions (
	commander_id  INTEGER NOT NULL REFERENCES commanders (id),
	collection_id INTEGER NOT NULL, -- collection_template id
	finish_time   INTEGER NOT NULL DEFAULT 0,
	over_time     INTEGER NOT NULL DEFAULT 0,
	PRIMARY KEY (commander_id, collection_id)
);
