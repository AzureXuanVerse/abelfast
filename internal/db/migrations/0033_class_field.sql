-- the academy's lecture hall (class_upgrade_template): class_proficiency is
-- the proficiency waiting to be turned into player_resource expField, which
-- it is up to class_field_at (unix seconds, 0 when never since
-- registered_at); sent as SC_22001 class_lv / class.proficiency
ALTER TABLE commanders ADD COLUMN class_field_level INTEGER NOT NULL DEFAULT 1;
ALTER TABLE commanders ADD COLUMN class_proficiency INTEGER NOT NULL DEFAULT 0;
ALTER TABLE commanders ADD COLUMN class_field_at BIGINT NOT NULL DEFAULT 0;
