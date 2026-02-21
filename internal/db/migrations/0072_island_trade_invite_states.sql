CREATE TABLE IF NOT EXISTS commander_island_trade_invite_states (
  commander_id bigint PRIMARY KEY REFERENCES commanders(commander_id) ON DELETE CASCADE,
  invited_commander_ids jsonb NOT NULL DEFAULT '[]'::jsonb,
  updated_at timestamptz NOT NULL DEFAULT NOW()
);
