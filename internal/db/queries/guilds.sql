-- name: CreateGuild :one
INSERT INTO guilds (name, policy, faction, manifesto, created_at)
VALUES (?, ?, ?, ?, ?)
RETURNING id;

-- name: GetGuild :one
SELECT * FROM guilds WHERE id = ?;

-- name: GuildNameTaken :one
SELECT EXISTS (SELECT 1 FROM guilds WHERE name = ? AND id != ?);

-- name: DeleteGuild :exec
DELETE FROM guilds WHERE id = ?;

-- name: GetGuildMember :one
SELECT * FROM guild_members WHERE commander_id = ?;

-- name: AddGuildMember :exec
INSERT INTO guild_members (commander_id, guild_id, duty, join_time) VALUES (?, ?, ?, ?);

-- name: DeleteGuildMember :exec
DELETE FROM guild_members WHERE commander_id = ?;

-- name: SetGuildMemberDuty :exec
UPDATE guild_members SET duty = ? WHERE commander_id = ?;

-- name: ListGuildMembers :many
SELECT sqlc.embed(guild_members), sqlc.embed(commanders) FROM guild_members
JOIN commanders ON commanders.id = guild_members.commander_id
WHERE guild_members.guild_id = ?
ORDER BY guild_members.duty, guild_members.join_time, guild_members.commander_id;

-- name: ListGuildMemberIDs :many
SELECT commander_id FROM guild_members WHERE guild_id = ? ORDER BY commander_id;

-- name: CountGuildMembers :one
SELECT COUNT(*) FROM guild_members WHERE guild_id = ?;

-- name: AddGuildLog :one
INSERT INTO guild_logs (guild_id, cmd, time, user_id, name, arg1)
VALUES (?, ?, ?, ?, ?, ?)
RETURNING *;

-- name: ListGuildLogs :many
SELECT * FROM (
	SELECT * FROM guild_logs WHERE guild_id = sqlc.arg(guild_id) ORDER BY id DESC LIMIT sqlc.arg(count)
) ORDER BY id;
