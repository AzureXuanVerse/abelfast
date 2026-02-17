CREATE TABLE IF NOT EXISTS island_order_states (
  commander_id bigint PRIMARY KEY REFERENCES commanders(commander_id) ON DELETE CASCADE,
  favor bigint NOT NULL DEFAULT 0,
  daily_select bigint NOT NULL DEFAULT 0,
  daily_slot_num bigint NOT NULL DEFAULT 0,
  time_slot_num bigint NOT NULL DEFAULT 0,
  urgency_finish_count bigint NOT NULL DEFAULT 0,
  ship_refresh bigint NOT NULL DEFAULT 0,
  CHECK (favor >= 0),
  CHECK (daily_select >= 0),
  CHECK (daily_slot_num >= 0),
  CHECK (time_slot_num >= 0),
  CHECK (urgency_finish_count >= 0),
  CHECK (ship_refresh >= 0)
);

CREATE TABLE IF NOT EXISTS island_order_favor_claims (
  commander_id bigint NOT NULL REFERENCES commanders(commander_id) ON DELETE CASCADE,
  level bigint NOT NULL,
  PRIMARY KEY (commander_id, level),
  CHECK (level > 0)
);

CREATE TABLE IF NOT EXISTS island_order_slots (
  commander_id bigint NOT NULL REFERENCES commanders(commander_id) ON DELETE CASCADE,
  slot_id bigint NOT NULL,
  slot_data bytea NOT NULL,
  PRIMARY KEY (commander_id, slot_id),
  CHECK (slot_id > 0)
);

CREATE TABLE IF NOT EXISTS island_ship_order_slots (
  commander_id bigint NOT NULL REFERENCES commanders(commander_id) ON DELETE CASCADE,
  slot_id bigint NOT NULL,
  slot_data bytea NOT NULL,
  PRIMARY KEY (commander_id, slot_id),
  CHECK (slot_id > 0)
);

CREATE TABLE IF NOT EXISTS island_ship_order_appoints (
  commander_id bigint NOT NULL REFERENCES commanders(commander_id) ON DELETE CASCADE,
  appoint_id bigint NOT NULL,
  appoint_data bytea NOT NULL,
  PRIMARY KEY (commander_id, appoint_id),
  CHECK (appoint_id > 0)
);

CREATE TABLE IF NOT EXISTS island_season_reward_claims (
  commander_id bigint NOT NULL REFERENCES commanders(commander_id) ON DELETE CASCADE,
  target_pt bigint NOT NULL,
  PRIMARY KEY (commander_id, target_pt),
  CHECK (target_pt > 0)
);

CREATE TABLE IF NOT EXISTS island_prosperity_states (
  commander_id bigint PRIMARY KEY REFERENCES commanders(commander_id) ON DELETE CASCADE,
  prosperity bigint NOT NULL DEFAULT 0,
  claimed_levels jsonb NOT NULL DEFAULT '[]'::jsonb,
  CHECK (prosperity >= 0)
);
