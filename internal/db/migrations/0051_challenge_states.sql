CREATE TABLE IF NOT EXISTS challenge_mode_states (
  commander_id bigint NOT NULL REFERENCES commanders(commander_id) ON DELETE CASCADE,
  activity_id bigint NOT NULL,
  mode bigint NOT NULL,
  season_id bigint NOT NULL DEFAULT 1,
  level bigint NOT NULL DEFAULT 1,
  current_score bigint NOT NULL DEFAULT 0,
  issl bigint NOT NULL DEFAULT 0,
  regular_group_id bigint NOT NULL DEFAULT 0,
  submarine_group_id bigint NOT NULL DEFAULT 0,
  regular_ship_ids jsonb NOT NULL DEFAULT '[]'::jsonb,
  submarine_ship_ids jsonb NOT NULL DEFAULT '[]'::jsonb,
  regular_commanders jsonb NOT NULL DEFAULT '[]'::jsonb,
  submarine_commanders jsonb NOT NULL DEFAULT '[]'::jsonb,
  created_at timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (commander_id, activity_id, mode)
);

CREATE TABLE IF NOT EXISTS limit_challenge_states (
  commander_id bigint NOT NULL REFERENCES commanders(commander_id) ON DELETE CASCADE,
  month_bucket bigint NOT NULL,
  best_times jsonb NOT NULL DEFAULT '{}'::jsonb,
  awarded jsonb NOT NULL DEFAULT '{}'::jsonb,
  pass_ids jsonb NOT NULL DEFAULT '[]'::jsonb,
  created_at timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (commander_id)
);
