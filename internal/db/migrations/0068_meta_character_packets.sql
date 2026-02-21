CREATE TABLE IF NOT EXISTS owned_ship_meta_repairs (
  owner_id bigint NOT NULL REFERENCES commanders(commander_id) ON DELETE CASCADE,
  ship_id bigint NOT NULL REFERENCES owned_ships(id) ON DELETE CASCADE,
  repair_id bigint NOT NULL,
  created_at timestamptz NOT NULL DEFAULT NOW(),
  PRIMARY KEY (owner_id, ship_id, repair_id)
);

CREATE TABLE IF NOT EXISTS commander_meta_tactics_states (
  commander_id bigint NOT NULL REFERENCES commanders(commander_id) ON DELETE CASCADE,
  ship_id bigint NOT NULL REFERENCES owned_ships(id) ON DELETE CASCADE,
  current_skill_id bigint NOT NULL DEFAULT 0,
  daily_exp bigint NOT NULL DEFAULT 0,
  double_exp bigint NOT NULL DEFAULT 0,
  switch_cnt bigint NOT NULL DEFAULT 3,
  updated_at timestamptz NOT NULL DEFAULT NOW(),
  PRIMARY KEY (commander_id, ship_id)
);

CREATE TABLE IF NOT EXISTS commander_meta_tactics_skill_states (
  commander_id bigint NOT NULL REFERENCES commanders(commander_id) ON DELETE CASCADE,
  ship_id bigint NOT NULL REFERENCES owned_ships(id) ON DELETE CASCADE,
  skill_id bigint NOT NULL,
  skill_pos bigint NOT NULL,
  level bigint NOT NULL DEFAULT 0,
  exp bigint NOT NULL DEFAULT 0,
  updated_at timestamptz NOT NULL DEFAULT NOW(),
  PRIMARY KEY (commander_id, ship_id, skill_id),
  CONSTRAINT commander_meta_tactics_skill_states_unique_pos UNIQUE (commander_id, ship_id, skill_pos)
);

CREATE TABLE IF NOT EXISTS commander_meta_tactics_task_progress (
  commander_id bigint NOT NULL REFERENCES commanders(commander_id) ON DELETE CASCADE,
  ship_id bigint NOT NULL REFERENCES owned_ships(id) ON DELETE CASCADE,
  skill_id bigint NOT NULL,
  task_id bigint NOT NULL,
  finish_cnt bigint NOT NULL DEFAULT 0,
  updated_at timestamptz NOT NULL DEFAULT NOW(),
  PRIMARY KEY (commander_id, ship_id, skill_id, task_id)
);
