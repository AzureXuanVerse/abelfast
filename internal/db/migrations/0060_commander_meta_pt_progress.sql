CREATE TABLE IF NOT EXISTS commander_meta_pt_progress (
  commander_id BIGINT NOT NULL,
  group_id BIGINT NOT NULL,
  pt BIGINT NOT NULL DEFAULT 0,
  fetch_list JSONB NOT NULL DEFAULT '[]'::jsonb,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  PRIMARY KEY (commander_id, group_id)
);
