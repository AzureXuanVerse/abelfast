-- name: GetDormTheme :one
SELECT theme_id, upload_time FROM dorm_themes WHERE commander_id = ? AND pos = ?;

-- name: GetSharedDormTheme :one
SELECT t.commander_id, t.pos, t.theme, t.upload_time,
	(SELECT COUNT(*) FROM dorm_theme_likes l WHERE l.theme_id = t.theme_id) AS like_count,
	(SELECT COUNT(*) FROM dorm_theme_favs f WHERE f.theme_id = t.theme_id) AS fav_count
FROM dorm_themes t WHERE t.theme_id = ? AND t.upload_time > 0;

-- name: ListSharedDormThemes :many
SELECT t.theme_id, t.upload_time,
	(SELECT COUNT(*) FROM dorm_theme_likes l WHERE l.theme_id = t.theme_id) AS like_count,
	(SELECT COUNT(*) FROM dorm_theme_favs f WHERE f.theme_id = t.theme_id) AS fav_count
FROM dorm_themes t WHERE t.upload_time > 0;

-- name: CountSharedDormThemes :one
SELECT COUNT(*) FROM dorm_themes WHERE commander_id = ? AND upload_time > 0;

-- name: SetDormThemeUploadTime :execrows
UPDATE dorm_themes SET upload_time = ? WHERE commander_id = ? AND pos = ?;

-- name: DeleteDormThemeLikes :exec
DELETE FROM dorm_theme_likes WHERE theme_id = ?;

-- name: DeleteDormThemeFavs :exec
DELETE FROM dorm_theme_favs WHERE theme_id = ?;

-- name: AddDormThemeLike :execrows
INSERT INTO dorm_theme_likes (theme_id, commander_id) VALUES (?, ?) ON CONFLICT DO NOTHING;

-- name: CountDormThemeLike :one
SELECT COUNT(*) FROM dorm_theme_likes WHERE theme_id = ? AND commander_id = ?;

-- name: AddDormThemeFav :execrows
INSERT INTO dorm_theme_favs (commander_id, theme_id, upload_time, faved_at) VALUES (?, ?, ?, ?) ON CONFLICT DO NOTHING;

-- name: DeleteDormThemeFav :execrows
DELETE FROM dorm_theme_favs WHERE commander_id = ? AND theme_id = ?;

-- name: CountDormThemeFav :one
SELECT COUNT(*) FROM dorm_theme_favs WHERE commander_id = ? AND theme_id = ?;

-- name: CountDormThemeFavs :one
SELECT COUNT(*) FROM dorm_theme_favs WHERE commander_id = ?;

-- name: ListDormThemeFavs :many
SELECT f.theme_id, f.upload_time FROM dorm_theme_favs f
JOIN dorm_themes t ON t.theme_id = f.theme_id AND t.upload_time > 0
WHERE f.commander_id = ? ORDER BY f.faved_at, f.theme_id;
