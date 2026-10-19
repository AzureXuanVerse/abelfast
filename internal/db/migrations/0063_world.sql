-- Operation Siren (protocol module p33): a commander's world, the WORLDINFO
-- of SC_33001 and everything around it (camp, stamina, fleets, atlas,
-- inventory, tasks, ports, achievements, counters), as the JSON of
-- game.worldState. No row is a world never activated.
CREATE TABLE world_states (
	commander_id INTEGER PRIMARY KEY REFERENCES commanders (id),
	state BLOB NOT NULL
);

-- the maps of a world the commander has entered (MAPINFO of SC_33107): the
-- template drawn for the random map id, the attachments generated on it and
-- what happened to them since, as the JSON of game.worldMap
CREATE TABLE world_maps (
	commander_id INTEGER NOT NULL REFERENCES commanders (id),
	random_id INTEGER NOT NULL,
	state BLOB NOT NULL,
	PRIMARY KEY (commander_id, random_id)
);
