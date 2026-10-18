-- name: AddGuildChat :one
INSERT INTO guild_chat (guild_id, commander_id, content, time) VALUES (?, ?, ?, ?)
RETURNING *;

-- name: TrimGuildChat :exec
DELETE FROM guild_chat WHERE guild_chat.guild_id = sqlc.arg(guild_id) AND guild_chat.id NOT IN (
	SELECT kept.id FROM guild_chat AS kept WHERE kept.guild_id = sqlc.arg(guild_id) ORDER BY kept.id DESC LIMIT sqlc.arg(keep)
);

-- name: ListGuildChat :many
SELECT sqlc.embed(guild_chat), sqlc.embed(commanders) FROM guild_chat
JOIN guild_members ON guild_members.commander_id = guild_chat.commander_id AND guild_members.guild_id = guild_chat.guild_id
JOIN commanders ON commanders.id = guild_chat.commander_id
WHERE guild_chat.guild_id = sqlc.arg(guild_id)
ORDER BY guild_chat.id DESC
LIMIT sqlc.arg(count);
