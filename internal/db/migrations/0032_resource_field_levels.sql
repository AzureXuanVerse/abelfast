-- each academy resource field fills on its own clock and level: the oil
-- field (oilfield_template) and the merchant (tradingport_template) are
-- filled up to oil_field_at / gold_field_at (unix seconds, 0 when never
-- since registered_at), sent as SC_22001 oil_well_level / gold_well_level
ALTER TABLE commanders RENAME COLUMN resource_field_at TO oil_field_at;
ALTER TABLE commanders ADD COLUMN gold_field_at BIGINT NOT NULL DEFAULT 0;
UPDATE commanders SET gold_field_at = oil_field_at;
ALTER TABLE commanders ADD COLUMN oil_field_level INTEGER NOT NULL DEFAULT 1;
ALTER TABLE commanders ADD COLUMN gold_field_level INTEGER NOT NULL DEFAULT 1;
