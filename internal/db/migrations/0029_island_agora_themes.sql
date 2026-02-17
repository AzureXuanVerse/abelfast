-- 0029_island_agora_themes.sql

CREATE TABLE IF NOT EXISTS island_agora_themes (
  commander_id bigint NOT NULL REFERENCES commanders(commander_id) ON DELETE CASCADE,
  theme_slot_id bigint NOT NULL,
  name text NOT NULL DEFAULT '',
  placed_data bytea NOT NULL DEFAULT '\x'::bytea,
  PRIMARY KEY (commander_id, theme_slot_id)
);
