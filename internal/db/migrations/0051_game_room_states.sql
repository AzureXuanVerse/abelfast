CREATE TABLE IF NOT EXISTS game_room_states (
  commander_id bigint PRIMARY KEY REFERENCES commanders(commander_id) ON DELETE CASCADE,
  week_start_unix bigint NOT NULL DEFAULT 0,
  weekly_claimed boolean NOT NULL DEFAULT false,
  pay_coin_count bigint NOT NULL DEFAULT 0,
  first_enter_claimed boolean NOT NULL DEFAULT false,
  month_key bigint NOT NULL DEFAULT 0,
  monthly_ticket bigint NOT NULL DEFAULT 0,
  updated_at timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS game_room_scores (
  commander_id bigint NOT NULL REFERENCES commanders(commander_id) ON DELETE CASCADE,
  room_id bigint NOT NULL,
  max_score bigint NOT NULL DEFAULT 0,
  updated_at timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (commander_id, room_id)
);
