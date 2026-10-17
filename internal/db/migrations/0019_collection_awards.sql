-- the archives' milestone rewards claimed (CollectionGetAwardCommand,
-- CS_17005): one row per storeup_data_template id and award_display index
-- (1-based), reported in SC_17001 ship_award_list
CREATE TABLE commander_collection_awards (
	commander_id INTEGER NOT NULL REFERENCES commanders (id),
	storeup_id   INTEGER NOT NULL, -- storeup_data_template id
	award_index  INTEGER NOT NULL,
	PRIMARY KEY (commander_id, storeup_id, award_index)
);
