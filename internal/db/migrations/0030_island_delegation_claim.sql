CREATE TABLE IF NOT EXISTS island_delegations (
  commander_id bigint NOT NULL REFERENCES commanders(commander_id) ON DELETE CASCADE,
  build_id bigint NOT NULL,
  area_id bigint NOT NULL,
  has_role boolean NOT NULL DEFAULT false,
  reward_ready boolean NOT NULL DEFAULT false,
  formula_id bigint NOT NULL DEFAULT 0,
  main_num bigint NOT NULL DEFAULT 0,
  other_num bigint NOT NULL DEFAULT 0,
  extra_main_num bigint NOT NULL DEFAULT 0,
  extra_other_num bigint NOT NULL DEFAULT 0,
  get_times bigint NOT NULL DEFAULT 0,
  pt_award bigint NOT NULL DEFAULT 0,
  PRIMARY KEY (commander_id, build_id, area_id),
  CHECK (build_id > 0),
  CHECK (area_id > 0)
);

CREATE TABLE IF NOT EXISTS island_inventories (
  commander_id bigint NOT NULL REFERENCES commanders(commander_id) ON DELETE CASCADE,
  item_id bigint NOT NULL,
  count bigint NOT NULL DEFAULT 0,
  PRIMARY KEY (commander_id, item_id),
  CHECK (item_id > 0),
  CHECK (count >= 0)
);

CREATE TABLE IF NOT EXISTS island_seasons (
  commander_id bigint PRIMARY KEY REFERENCES commanders(commander_id) ON DELETE CASCADE,
  pt bigint NOT NULL DEFAULT 0,
  CHECK (pt >= 0)
);
