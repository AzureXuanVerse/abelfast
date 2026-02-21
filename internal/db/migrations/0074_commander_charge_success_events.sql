CREATE TABLE IF NOT EXISTS commander_charge_success_events (
  commander_id bigint NOT NULL REFERENCES commanders(commander_id) ON DELETE CASCADE,
  pay_id text NOT NULL,
  created_at timestamptz NOT NULL DEFAULT NOW(),
  PRIMARY KEY (commander_id, pay_id)
);

CREATE INDEX IF NOT EXISTS idx_commander_charge_success_events_commander_created
  ON commander_charge_success_events (commander_id, created_at DESC);
