-- formation fleets (FleetProxy): 1-6 surface, 11-12 submarine
CREATE TABLE fleets (
	commander_id INTEGER NOT NULL REFERENCES commanders (id),
	fleet_id     INTEGER NOT NULL,
	name         TEXT    NOT NULL DEFAULT '',
	PRIMARY KEY (commander_id, fleet_id)
);

-- ships of a fleet in the client's order (vanguard, main, submarine);
-- ship_id is a ships.id
CREATE TABLE fleet_ships (
	commander_id INTEGER NOT NULL,
	fleet_id     INTEGER NOT NULL,
	position     INTEGER NOT NULL,
	ship_id      INTEGER NOT NULL,
	PRIMARY KEY (commander_id, fleet_id, position),
	FOREIGN KEY (commander_id, fleet_id) REFERENCES fleets (commander_id, fleet_id)
);
