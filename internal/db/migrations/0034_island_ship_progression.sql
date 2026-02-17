CREATE TABLE IF NOT EXISTS island_ships (
  commander_id bigint NOT NULL REFERENCES commanders(commander_id) ON DELETE CASCADE,
  ship_id bigint NOT NULL,
  level bigint NOT NULL DEFAULT 1,
  break_lv bigint NOT NULL DEFAULT 1,
  can_follow boolean NOT NULL DEFAULT true,
  PRIMARY KEY (commander_id, ship_id),
  CHECK (ship_id > 0),
  CHECK (level > 0),
  CHECK (break_lv > 0)
);

CREATE TABLE IF NOT EXISTS island_followers (
  commander_id bigint NOT NULL REFERENCES commanders(commander_id) ON DELETE CASCADE,
  ship_id bigint NOT NULL,
  order_idx bigint NOT NULL,
  PRIMARY KEY (commander_id, ship_id),
  CHECK (ship_id > 0),
  CHECK (order_idx >= 0)
);

CREATE INDEX IF NOT EXISTS island_followers_commander_order_idx
  ON island_followers(commander_id, order_idx);

CREATE TABLE IF NOT EXISTS island_ship_order_states (
  commander_id bigint PRIMARY KEY REFERENCES commanders(commander_id) ON DELETE CASCADE,
  refresh_at bigint NOT NULL DEFAULT 0,
  appoint_list jsonb NOT NULL DEFAULT '[]'::jsonb,
  CHECK (refresh_at >= 0)
);

CREATE TABLE IF NOT EXISTS island_ship_order_slots (
  commander_id bigint NOT NULL REFERENCES commanders(commander_id) ON DELETE CASCADE,
  slot_id bigint NOT NULL,
  state bigint NOT NULL DEFAULT 0,
  load_time bigint NOT NULL DEFAULT 0,
  get_time bigint NOT NULL DEFAULT 0,
  finish_num bigint NOT NULL DEFAULT 0,
  auto_time bigint NOT NULL DEFAULT 0,
  PRIMARY KEY (commander_id, slot_id),
  CHECK (slot_id > 0),
  CHECK (state >= 0),
  CHECK (load_time >= 0),
  CHECK (get_time >= 0),
  CHECK (finish_num >= 0),
  CHECK (auto_time >= 0)
);
