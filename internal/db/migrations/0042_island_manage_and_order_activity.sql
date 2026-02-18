CREATE TABLE IF NOT EXISTS island_manage_trades (
  commander_id bigint NOT NULL REFERENCES commanders(commander_id) ON DELETE CASCADE,
  trade_id bigint NOT NULL,
  trade_data bytea NOT NULL,
  presell_data bytea NOT NULL,
  total_sales bigint NOT NULL DEFAULT 0,
  PRIMARY KEY (commander_id, trade_id),
  CHECK (trade_id > 0),
  CHECK (total_sales >= 0)
);

CREATE TABLE IF NOT EXISTS island_order_act_groups (
  commander_id bigint NOT NULL REFERENCES commanders(commander_id) ON DELETE CASCADE,
  act_id bigint NOT NULL,
  group_id bigint NOT NULL,
  PRIMARY KEY (commander_id, act_id, group_id),
  CHECK (act_id > 0),
  CHECK (group_id > 0)
);
