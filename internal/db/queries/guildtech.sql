-- name: ListGuildTechnologies :many
SELECT * FROM guild_technologies WHERE guild_id = ? ORDER BY group_id;

-- name: GetGuildTechnology :one
SELECT * FROM guild_technologies WHERE guild_id = ? AND group_id = ?;

-- name: GetActiveGuildTechnology :one
SELECT * FROM guild_technologies WHERE guild_id = ? AND active;

-- name: StopGuildTechnologies :exec
UPDATE guild_technologies SET active = FALSE WHERE guild_id = ?;

-- name: StartGuildTechnology :exec
INSERT INTO guild_technologies (guild_id, group_id, tech_id, active, started_at)
VALUES (?, ?, ?, TRUE, ?)
ON CONFLICT (guild_id, group_id) DO UPDATE SET active = TRUE, started_at = excluded.started_at;

-- name: SetGuildTechnologyProgress :exec
UPDATE guild_technologies SET tech_id = ?, progress = ? WHERE guild_id = ? AND group_id = ?;

-- name: ListCommanderGuildTechnologies :many
SELECT * FROM commander_guild_technologies WHERE commander_id = ? ORDER BY group_id;

-- name: SetCommanderGuildTechnology :exec
INSERT INTO commander_guild_technologies (commander_id, group_id, tech_id) VALUES (?, ?, ?)
ON CONFLICT (commander_id, group_id) DO UPDATE SET tech_id = excluded.tech_id;
