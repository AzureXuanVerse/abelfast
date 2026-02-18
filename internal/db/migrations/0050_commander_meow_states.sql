CREATE TABLE IF NOT EXISTS commander_meows (
  id bigserial PRIMARY KEY,
  commander_id bigint NOT NULL REFERENCES commanders(commander_id) ON DELETE CASCADE,
  template_id bigint NOT NULL,
  level bigint NOT NULL DEFAULT 1,
  exp bigint NOT NULL DEFAULT 0,
  is_locked bigint NOT NULL DEFAULT 0,
  used_pt bigint NOT NULL DEFAULT 0,
  created_at timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_commander_meows_commander_id ON commander_meows(commander_id);

CREATE TABLE IF NOT EXISTS commander_boxes (
  commander_id bigint NOT NULL REFERENCES commanders(commander_id) ON DELETE CASCADE,
  box_id bigint NOT NULL,
  pool_id bigint NOT NULL DEFAULT 0,
  begin_time bigint NOT NULL DEFAULT 0,
  finish_time bigint NOT NULL DEFAULT 0,
  PRIMARY KEY (commander_id, box_id)
);
