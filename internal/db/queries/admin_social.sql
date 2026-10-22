-- name: AdminSocialWorldMaps :many
-- the maps a commander's world has entered, for the back-office
SELECT random_id, state FROM world_maps WHERE commander_id = ? ORDER BY random_id;

-- name: AdminSocialDormThemes :many
-- a commander's theme templates with their likes and collections
SELECT t.pos, t.theme, t.upload_time,
	(SELECT COUNT(*) FROM dorm_theme_likes l WHERE l.theme_id = t.theme_id) AS like_count,
	(SELECT COUNT(*) FROM dorm_theme_favs f WHERE f.theme_id = t.theme_id) AS fav_count
FROM dorm_themes t WHERE t.commander_id = ? ORDER BY t.pos;

-- name: AdminSocialFriends :many
SELECT c.id, c.name, c.level, c.last_online, f.since FROM friends f
JOIN commanders c ON c.id = f.friend_id
WHERE f.commander_id = ?
ORDER BY f.since, f.friend_id;

-- name: AdminSocialBlacklist :many
SELECT c.id, c.name, c.level, b.blocked_at FROM blacklist b
JOIN commanders c ON c.id = b.blocked_id
WHERE b.commander_id = ?
ORDER BY b.blocked_at, b.blocked_id;

-- name: AdminSocialSetMetaPt :exec
INSERT INTO meta_pts (commander_id, group_id, pt) VALUES (?, ?, ?)
ON CONFLICT (commander_id, group_id) DO UPDATE SET pt = excluded.pt;
