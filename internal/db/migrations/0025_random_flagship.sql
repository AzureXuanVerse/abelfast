-- the random secretary (PlayerVitaeShipsPage, MainRandomFlagShipSequence).
-- The client keeps the drawn ships itself (PlayerPrefs); the server keeps
-- the pool mode (CS_12206, SC_11003.random_ship_mode, 0 until picked: the
-- client then falls back to common flag 1000007) and the switch (CS_12204,
-- never sent back); no row is the new commander's: unset, off.
CREATE TABLE commander_random_flagship (
	commander_id INTEGER NOT NULL PRIMARY KEY REFERENCES commanders (id),
	mode         INTEGER NOT NULL DEFAULT 0, -- SettingsRandomFlagShipAndSkinPanel 1-3
	enabled      BOOLEAN NOT NULL DEFAULT 0
);

-- whether the ship is in the custom random pool (CS_12208,
-- SHIPINFO.char_random_flag); belfast keeps no phantoms, so only the ship
-- itself (shadow 0)
ALTER TABLE ships ADD COLUMN random_flag BOOLEAN NOT NULL DEFAULT 0;
