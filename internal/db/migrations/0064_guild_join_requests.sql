CREATE TABLE IF NOT EXISTS guild_join_requests (
    guild_id bigint NOT NULL REFERENCES guilds(id) ON DELETE CASCADE,
    applicant_commander_id bigint NOT NULL REFERENCES commanders(commander_id) ON DELETE CASCADE,
    content text NOT NULL DEFAULT '',
    requested_at timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (guild_id, applicant_commander_id)
);

CREATE INDEX IF NOT EXISTS idx_guild_join_requests_guild_requested_at
ON guild_join_requests (guild_id, requested_at, applicant_commander_id);

CREATE INDEX IF NOT EXISTS idx_guild_join_requests_applicant
ON guild_join_requests (applicant_commander_id);
