-- an academy resource field's running upgrade (shop_template 7, 8, 20) ends
-- at *_upgrade_at (unix seconds, 0 when none): its level is the one before
-- until then, sent as SC_22001 oil_well_lv_up_time / gold_well_lv_up_time /
-- class_lv_up_time
ALTER TABLE commanders ADD COLUMN oil_field_upgrade_at BIGINT NOT NULL DEFAULT 0;
ALTER TABLE commanders ADD COLUMN gold_field_upgrade_at BIGINT NOT NULL DEFAULT 0;
ALTER TABLE commanders ADD COLUMN class_field_upgrade_at BIGINT NOT NULL DEFAULT 0;
