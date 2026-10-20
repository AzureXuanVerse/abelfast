-- the oathed ship the commander shows as theirs (SC_11003.marry_ship,
-- Player.proposeShipId), set by CS_12032 (ProposeCommand and
-- ProposeRegiesterShipCommand); a ship id, 0 for none
ALTER TABLE commanders ADD COLUMN marry_ship INTEGER NOT NULL DEFAULT 0;
