CREATE TABLE IF NOT EXISTS guild_assault_fleet_slots (
    guild_id bigint NOT NULL REFERENCES guilds(id) ON DELETE CASCADE,
    commander_id bigint NOT NULL REFERENCES commanders(commander_id) ON DELETE CASCADE,
    pos bigint NOT NULL,
    ship_id bigint NOT NULL,
    last_time bigint NOT NULL DEFAULT 0,
    created_at timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (guild_id, commander_id, pos)
);

CREATE INDEX IF NOT EXISTS idx_guild_assault_fleet_slots_guild_commander
    ON guild_assault_fleet_slots (guild_id, commander_id, pos);

CREATE TABLE IF NOT EXISTS guild_assault_recommendations (
    guild_id bigint NOT NULL REFERENCES guilds(id) ON DELETE CASCADE,
    commander_id bigint NOT NULL REFERENCES commanders(commander_id) ON DELETE CASCADE,
    ship_id bigint NOT NULL,
    created_at timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (guild_id, commander_id, ship_id)
);

CREATE INDEX IF NOT EXISTS idx_guild_assault_recommendations_guild
    ON guild_assault_recommendations (guild_id, commander_id, ship_id);

CREATE TABLE IF NOT EXISTS guild_boss_mission_fleets (
    guild_id bigint NOT NULL REFERENCES guilds(id) ON DELETE CASCADE,
    operation_id bigint NOT NULL,
    fleet_id bigint NOT NULL,
    ships jsonb NOT NULL DEFAULT '[]'::jsonb,
    commanders jsonb NOT NULL DEFAULT '[]'::jsonb,
    updated_at timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (guild_id, operation_id, fleet_id)
);

CREATE INDEX IF NOT EXISTS idx_guild_boss_mission_fleets_guild_operation
    ON guild_boss_mission_fleets (guild_id, operation_id, fleet_id);
