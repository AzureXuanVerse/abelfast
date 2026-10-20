-- The Island life-sim (IslandProxy, protocol module 21): one row per
-- commander, created the first time the island is entered or given
-- something. island is the PB_ISLAND the client gets in SC_21201 (public
-- and private data) as the server keeps it; position the
-- PB_PLAYER_POS_RECORD where the commander last left it (CS_21229, empty
-- until then); extra the server-only state of each subsystem as JSON
-- ({"<subsystem>": {...}}, islandState.extra in internal/game). invite_code
-- copies private_data.invite_code for the visits by code (CS_21202).
CREATE TABLE commander_islands (
	commander_id INTEGER PRIMARY KEY REFERENCES commanders (id),
	island       BLOB    NOT NULL,
	position     BLOB    NOT NULL DEFAULT x'',
	extra        TEXT    NOT NULL DEFAULT '{}',
	invite_code  TEXT    NOT NULL DEFAULT ''
);
CREATE INDEX commander_islands_invite_code ON commander_islands (invite_code);
