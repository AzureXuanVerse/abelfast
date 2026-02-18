CREATE TABLE IF NOT EXISTS commander_homes (
  commander_id bigint PRIMARY KEY REFERENCES commanders(commander_id) ON DELETE CASCADE,
  level bigint NOT NULL DEFAULT 1,
  exp bigint NOT NULL DEFAULT 0,
  clean bigint NOT NULL DEFAULT 0,
  scene_open boolean NOT NULL DEFAULT false
);

CREATE TABLE IF NOT EXISTS commander_home_slots (
  commander_id bigint NOT NULL REFERENCES commanders(commander_id) ON DELETE CASCADE,
  slot_id bigint NOT NULL,
  op_flag bigint NOT NULL DEFAULT 7,
  exp_time bigint NOT NULL DEFAULT 0,
  assigned_commander_id bigint NOT NULL DEFAULT 0,
  style bigint NOT NULL DEFAULT 1,
  cache_exp bigint NOT NULL DEFAULT 0,
  PRIMARY KEY (commander_id, slot_id)
);
