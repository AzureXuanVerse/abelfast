ALTER TABLE island_delegations
  ADD COLUMN IF NOT EXISTS ship_id bigint NOT NULL DEFAULT 0,
  ADD COLUMN IF NOT EXISTS max_times bigint NOT NULL DEFAULT 0,
  ADD COLUMN IF NOT EXISTS start_time bigint NOT NULL DEFAULT 0,
  ADD COLUMN IF NOT EXISTS cost_time_list jsonb NOT NULL DEFAULT '[]'::jsonb,
  ADD COLUMN IF NOT EXISTS speed_time bigint NOT NULL DEFAULT 0,
  ADD COLUMN IF NOT EXISTS times_extra jsonb NOT NULL DEFAULT '[]'::jsonb,
  ADD COLUMN IF NOT EXISTS recover_time bigint NOT NULL DEFAULT 0,
  ADD COLUMN IF NOT EXISTS add_exp bigint NOT NULL DEFAULT 0,
  ADD COLUMN IF NOT EXISTS return_num bigint NOT NULL DEFAULT 0;

CREATE TABLE IF NOT EXISTS island_speedup_tickets (
  commander_id bigint NOT NULL REFERENCES commanders(commander_id) ON DELETE CASCADE,
  speed_id bigint NOT NULL,
  end_time bigint NOT NULL,
  count bigint NOT NULL DEFAULT 0,
  PRIMARY KEY (commander_id, speed_id, end_time),
  CHECK (speed_id > 0),
  CHECK (count >= 0)
);

CREATE TABLE IF NOT EXISTS island_overflow_inventories (
  commander_id bigint NOT NULL REFERENCES commanders(commander_id) ON DELETE CASCADE,
  item_id bigint NOT NULL,
  count bigint NOT NULL DEFAULT 0,
  PRIMARY KEY (commander_id, item_id),
  CHECK (item_id > 0),
  CHECK (count >= 0)
);

CREATE TABLE IF NOT EXISTS island_speedup_targets (
  commander_id bigint NOT NULL REFERENCES commanders(commander_id) ON DELETE CASCADE,
  target_type bigint NOT NULL,
  target_id bigint NOT NULL,
  end_time bigint NOT NULL DEFAULT 0,
  PRIMARY KEY (commander_id, target_type, target_id),
  CHECK (target_type > 0),
  CHECK (end_time >= 0)
);

CREATE TABLE IF NOT EXISTS island_ship_order_slots (
  commander_id bigint NOT NULL REFERENCES commanders(commander_id) ON DELETE CASCADE,
  ship_slot_id bigint NOT NULL,
  state bigint NOT NULL DEFAULT 0,
  get_time bigint NOT NULL DEFAULT 0,
  end_time bigint NOT NULL DEFAULT 0,
  cost_list jsonb NOT NULL DEFAULT '[]'::jsonb,
  PRIMARY KEY (commander_id, ship_slot_id),
  CHECK (ship_slot_id > 0),
  CHECK (state >= 0),
  CHECK (get_time >= 0),
  CHECK (end_time >= 0)
);

ALTER TABLE island_ship_order_slots
  ADD COLUMN IF NOT EXISTS ship_slot_id bigint,
  ADD COLUMN IF NOT EXISTS state bigint NOT NULL DEFAULT 0,
  ADD COLUMN IF NOT EXISTS get_time bigint NOT NULL DEFAULT 0,
  ADD COLUMN IF NOT EXISTS end_time bigint NOT NULL DEFAULT 0,
  ADD COLUMN IF NOT EXISTS cost_list jsonb NOT NULL DEFAULT '[]'::jsonb;

DO $$
BEGIN
  IF EXISTS (
    SELECT 1
    FROM information_schema.columns
    WHERE table_schema = 'public'
      AND table_name = 'island_ship_order_slots'
      AND column_name = 'slot_id'
  ) THEN
    UPDATE island_ship_order_slots
    SET ship_slot_id = slot_id
    WHERE ship_slot_id IS NULL;
  END IF;
END $$;

CREATE UNIQUE INDEX IF NOT EXISTS island_ship_order_slots_commander_ship_slot_idx
  ON island_ship_order_slots(commander_id, ship_slot_id);
