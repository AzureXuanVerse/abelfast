-- the commander's ship skins (ShipSkinProxy, SC_12201 skin_list), by
-- ship_skin_template id: end_time 0 for a permanent skin, else when the
-- time-limited one runs out (unix seconds)
CREATE TABLE commander_skins (
	commander_id INTEGER NOT NULL REFERENCES commanders (id),
	skin_id      INTEGER NOT NULL,
	end_time     BIGINT  NOT NULL DEFAULT 0,
	PRIMARY KEY (commander_id, skin_id)
);
