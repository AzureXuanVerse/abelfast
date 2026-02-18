ALTER TABLE island_ships
  ADD COLUMN IF NOT EXISTS exp bigint NOT NULL DEFAULT 0,
  ADD COLUMN IF NOT EXISTS skill_lv bigint NOT NULL DEFAULT 1,
  ADD COLUMN IF NOT EXISTS power bigint NOT NULL DEFAULT 0,
  ADD COLUMN IF NOT EXISTS recover_time bigint NOT NULL DEFAULT 0,
  ADD COLUMN IF NOT EXISTS up_limit_state bigint NOT NULL DEFAULT 0,
  ADD COLUMN IF NOT EXISTS cur_skin_id bigint NOT NULL DEFAULT 0,
  ADD COLUMN IF NOT EXISTS extra_attr jsonb NOT NULL DEFAULT '[]'::jsonb,
  ADD COLUMN IF NOT EXISTS buffs jsonb NOT NULL DEFAULT '[]'::jsonb;

CREATE TABLE IF NOT EXISTS island_ship_invites (
  commander_id bigint NOT NULL REFERENCES commanders(commander_id) ON DELETE CASCADE,
  ship_id bigint NOT NULL,
  PRIMARY KEY (commander_id, ship_id),
  CHECK (ship_id > 0)
);

CREATE TABLE IF NOT EXISTS island_role_dresses (
  commander_id bigint NOT NULL REFERENCES commanders(commander_id) ON DELETE CASCADE,
  dress_id bigint NOT NULL,
  num bigint NOT NULL DEFAULT 0,
  read bigint NOT NULL DEFAULT 0,
  time bigint NOT NULL DEFAULT 0,
  updated_at timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (commander_id, dress_id),
  CHECK (dress_id > 0),
  CHECK (num >= 0)
);

CREATE TABLE IF NOT EXISTS island_ship_dresses (
  commander_id bigint NOT NULL REFERENCES commanders(commander_id) ON DELETE CASCADE,
  ship_id bigint NOT NULL,
  dress_id bigint NOT NULL,
  PRIMARY KEY (commander_id, ship_id, dress_id),
  CHECK (ship_id > 0),
  CHECK (dress_id > 0)
);

CREATE INDEX IF NOT EXISTS island_ship_dresses_commander_dress_idx
  ON island_ship_dresses(commander_id, dress_id);

CREATE TABLE IF NOT EXISTS island_ship_skins (
  commander_id bigint NOT NULL REFERENCES commanders(commander_id) ON DELETE CASCADE,
  ship_id bigint NOT NULL,
  skin_id bigint NOT NULL,
  color_id bigint NOT NULL DEFAULT 0,
  color_list jsonb NOT NULL DEFAULT '[]'::jsonb,
  updated_at timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (commander_id, ship_id, skin_id),
  CHECK (ship_id > 0),
  CHECK (skin_id > 0)
);

CREATE TABLE IF NOT EXISTS island_commander_dress_profiles (
  commander_id bigint PRIMARY KEY REFERENCES commanders(commander_id) ON DELETE CASCADE,
  island_id bigint NOT NULL DEFAULT 0,
  cur_dress jsonb NOT NULL DEFAULT '[]'::jsonb,
  cap_list jsonb NOT NULL DEFAULT '[]'::jsonb,
  updated_at timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS island_npc_feedback_states (
  commander_id bigint PRIMARY KEY REFERENCES commanders(commander_id) ON DELETE CASCADE,
  day_start_unix bigint NOT NULL DEFAULT 0,
  claimed_npc_ids jsonb NOT NULL DEFAULT '[]'::jsonb,
  updated_at timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP
);
