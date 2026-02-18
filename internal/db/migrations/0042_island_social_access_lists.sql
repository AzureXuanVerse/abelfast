ALTER TABLE commander_island_social_states
  ADD COLUMN IF NOT EXISTS white_list JSONB NOT NULL DEFAULT '[]'::jsonb,
  ADD COLUMN IF NOT EXISTS black_list JSONB NOT NULL DEFAULT '[]'::jsonb;
