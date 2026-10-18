-- a guild member's assault fleet (Battle Support, GuildAssaultFleet): the
-- ships lent to the guild's boss battles, at pos 1 and 2. changed_at is when
-- the position was last set (SHIPID_POS_INFO.last_time, the start of its
-- guildset operation_assault_team_cd cooldown), recommended whether an
-- administrator recommended the ship (TEAM_CELL). Leaving the guild or
-- retiring the ship empties the position.
CREATE TABLE guild_assault_ships (
	commander_id INTEGER NOT NULL REFERENCES guild_members (commander_id) ON DELETE CASCADE,
	pos          INTEGER NOT NULL,
	ship_id      INTEGER NOT NULL REFERENCES ships (id) ON DELETE CASCADE,
	recommended  BOOLEAN NOT NULL DEFAULT 0,
	changed_at   INTEGER NOT NULL,
	PRIMARY KEY (commander_id, pos)
);
CREATE INDEX guild_assault_ships_ship ON guild_assault_ships (ship_id);
