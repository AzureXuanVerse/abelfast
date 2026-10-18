-- the academy's oil field and merchant (player_resource oilField / goldField)
-- are filled up to resource_field_at (unix seconds), 0 when never since
-- registered_at
ALTER TABLE commanders ADD COLUMN resource_field_at BIGINT NOT NULL DEFAULT 0;
