CREATE TABLE IF NOT EXISTS commander_packet_states (
  owner_commander_id bigint NOT NULL REFERENCES commanders(commander_id) ON DELETE CASCADE,
  commander_id bigint NOT NULL,
  level bigint NOT NULL DEFAULT 1,
  name text NOT NULL DEFAULT '',
  is_locked boolean NOT NULL DEFAULT false,
  used_pt bigint NOT NULL DEFAULT 0,
  ability_ids jsonb NOT NULL DEFAULT '[]'::jsonb,
  ability_origin_ids jsonb NOT NULL DEFAULT '[]'::jsonb,
  pending_ability_ids jsonb NOT NULL DEFAULT '[]'::jsonb,
  ability_reset_at timestamptz NOT NULL DEFAULT '1970-01-01 00:00:00+00',
  rename_cooldown_at timestamptz NOT NULL DEFAULT '1970-01-01 00:00:00+00',
  created_at timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (owner_commander_id, commander_id)
);

CREATE TABLE IF NOT EXISTS commander_prefab_fleets (
  owner_commander_id bigint NOT NULL REFERENCES commanders(commander_id) ON DELETE CASCADE,
  prefab_id bigint NOT NULL,
  name text NOT NULL DEFAULT '',
  rename_cooldown_at timestamptz NOT NULL DEFAULT '1970-01-01 00:00:00+00',
  commander_slots jsonb NOT NULL DEFAULT '[]'::jsonb,
  created_at timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (owner_commander_id, prefab_id)
);
