-- name: ListGuildRequests :many
SELECT sqlc.embed(guild_requests), sqlc.embed(commanders) FROM guild_requests
JOIN commanders ON commanders.id = guild_requests.commander_id
WHERE guild_requests.guild_id = ?
ORDER BY guild_requests.time, guild_requests.commander_id;

-- name: GuildRequestExists :one
SELECT EXISTS (SELECT 1 FROM guild_requests WHERE guild_id = ? AND commander_id = ?);

-- name: AddGuildRequest :exec
INSERT INTO guild_requests (guild_id, commander_id, content, time) VALUES (?, ?, ?, ?)
ON CONFLICT (guild_id, commander_id) DO UPDATE SET content = excluded.content, time = excluded.time;

-- name: DeleteGuildRequest :exec
DELETE FROM guild_requests WHERE guild_id = ? AND commander_id = ?;

-- name: DeleteCommanderGuildRequests :exec
DELETE FROM guild_requests WHERE commander_id = ?;

-- name: CountGuildRequests :one
SELECT COUNT(*) FROM guild_requests WHERE guild_id = ?;

-- name: GetGuildWaitTime :one
SELECT until FROM guild_wait_times WHERE commander_id = ?;

-- name: SetGuildWaitTime :exec
INSERT INTO guild_wait_times (commander_id, until) VALUES (?, ?)
ON CONFLICT (commander_id) DO UPDATE SET until = excluded.until;

-- name: GetGuildCommander :one
SELECT sqlc.embed(guild_members), sqlc.embed(commanders) FROM guild_members
JOIN commanders ON commanders.id = guild_members.commander_id
WHERE guild_members.guild_id = ? AND guild_members.duty = 1;

-- name: ListGuildsToJoin :many
SELECT sqlc.embed(guilds),
	(SELECT COUNT(*) FROM guild_members WHERE guild_members.guild_id = guilds.id) AS member_count
FROM guilds WHERE guilds.id != sqlc.arg(exclude)
ORDER BY RANDOM() LIMIT sqlc.arg(count);

-- name: SearchGuilds :many
SELECT sqlc.embed(guilds),
	(SELECT COUNT(*) FROM guild_members WHERE guild_members.guild_id = guilds.id) AS member_count
FROM guilds
WHERE guilds.id = sqlc.arg(id) OR instr(lower(guilds.name), lower(sqlc.arg(keyword))) > 0
ORDER BY guilds.id LIMIT sqlc.arg(count);

-- name: SetGuildName :exec
UPDATE guilds SET name = ? WHERE id = ?;

-- name: SetGuildManifesto :exec
UPDATE guilds SET manifesto = ? WHERE id = ?;

-- name: SetGuildAnnounce :exec
UPDATE guilds SET announce = ? WHERE id = ?;

-- name: SetGuildPolicy :exec
UPDATE guilds SET policy = ? WHERE id = ?;

-- name: SetGuildFaction :exec
UPDATE guilds SET faction = ?, change_faction_cd = ? WHERE id = ?;

-- name: SetGuildKickLeaderCd :exec
UPDATE guilds SET kick_leader_cd = ? WHERE id = ?;
