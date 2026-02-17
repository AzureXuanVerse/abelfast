CREATE TABLE IF NOT EXISTS island_hand_plants (
  commander_id bigint NOT NULL REFERENCES commanders(commander_id) ON DELETE CASCADE,
  build_id bigint NOT NULL,
  slot_id bigint NOT NULL,
  state bigint NOT NULL DEFAULT 0,
  formula_id bigint NOT NULL DEFAULT 0,
  start_time bigint NOT NULL DEFAULT 0,
  end_time bigint NOT NULL DEFAULT 0,
  PRIMARY KEY (commander_id, slot_id)
);

CREATE INDEX IF NOT EXISTS idx_island_hand_plants_commander_build
  ON island_hand_plants(commander_id, build_id);
