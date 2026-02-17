CREATE TABLE IF NOT EXISTS commander_island_social_states (
  commander_id bigint PRIMARY KEY REFERENCES commanders(commander_id) ON DELETE CASCADE,
  invite_code text NOT NULL DEFAULT '',
  invite_code_refresh_day bigint NOT NULL DEFAULT 0,
  invited_commander_ids jsonb NOT NULL DEFAULT '[]'::jsonb,
  gift_count bigint NOT NULL DEFAULT 0,
  gift_timestamp bigint NOT NULL DEFAULT 0,
  gift_visitors jsonb NOT NULL DEFAULT '[]'::jsonb,
  updated_at timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE UNIQUE INDEX IF NOT EXISTS commander_island_social_states_invite_code_uq
ON commander_island_social_states (invite_code)
WHERE invite_code <> '';
