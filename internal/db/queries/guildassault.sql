-- name: ListGuildAssaultShips :many
SELECT * FROM guild_assault_ships WHERE commander_id = ? ORDER BY pos;

-- name: ListGuildAssaultShipsByGuild :many
SELECT sqlc.embed(guild_assault_ships) FROM guild_assault_ships
JOIN guild_members ON guild_members.commander_id = guild_assault_ships.commander_id
WHERE guild_members.guild_id = ?
ORDER BY guild_assault_ships.commander_id, guild_assault_ships.pos;

-- name: GetGuildAssaultShip :one
SELECT sqlc.embed(guild_assault_ships) FROM guild_assault_ships
JOIN guild_members ON guild_members.commander_id = guild_assault_ships.commander_id
WHERE guild_members.guild_id = ? AND guild_assault_ships.commander_id = ? AND guild_assault_ships.ship_id = ?;

-- name: SetGuildAssaultShip :exec
-- a new ship at the position loses the recommendation of the one before
INSERT INTO guild_assault_ships (commander_id, pos, ship_id, changed_at) VALUES (?, ?, ?, ?)
ON CONFLICT (commander_id, pos) DO UPDATE SET
	ship_id = excluded.ship_id,
	changed_at = excluded.changed_at,
	recommended = guild_assault_ships.recommended AND guild_assault_ships.ship_id = excluded.ship_id;

-- name: SetGuildAssaultShipRecommended :exec
UPDATE guild_assault_ships SET recommended = ? WHERE commander_id = ? AND ship_id = ?;

-- name: CountGuildRecommendedShips :one
SELECT COUNT(*) FROM guild_assault_ships
JOIN guild_members ON guild_members.commander_id = guild_assault_ships.commander_id
WHERE guild_members.guild_id = ? AND guild_assault_ships.recommended;

-- name: ListGuildRecommendedShips :many
SELECT guild_assault_ships.commander_id, guild_assault_ships.ship_id FROM guild_assault_ships
JOIN guild_members ON guild_members.commander_id = guild_assault_ships.commander_id
WHERE guild_members.guild_id = ? AND guild_assault_ships.recommended
ORDER BY guild_assault_ships.commander_id, guild_assault_ships.pos;
