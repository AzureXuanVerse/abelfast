-- name: ListRecentEquipCodes :many
SELECT e.id, e.eqcode, e.likes,
	CAST((SELECT COUNT(*) FROM equip_code_likes l WHERE l.share_id = e.id AND l.liked_at >= sqlc.arg(liked_since)) AS INTEGER) AS recent_likes,
	CAST((SELECT COUNT(*) FROM equip_code_reports r WHERE r.share_id = e.id) AS INTEGER) AS reports
FROM equip_codes e
WHERE e.group_id = sqlc.arg(group_id) AND e.shared_at >= sqlc.arg(shared_since)
ORDER BY e.id DESC
LIMIT sqlc.arg(max_rows);

-- name: ListOlderEquipCodes :many
SELECT e.id, e.eqcode, e.likes,
	CAST((SELECT COUNT(*) FROM equip_code_likes l WHERE l.share_id = e.id AND l.liked_at >= sqlc.arg(liked_since)) AS INTEGER) AS recent_likes,
	CAST((SELECT COUNT(*) FROM equip_code_reports r WHERE r.share_id = e.id) AS INTEGER) AS reports
FROM equip_codes e
WHERE e.group_id = sqlc.arg(group_id) AND e.shared_at < sqlc.arg(shared_before)
ORDER BY e.likes DESC, e.id DESC
LIMIT sqlc.arg(max_rows);

-- name: CountGroupEquipCodesSince :one
SELECT COUNT(*) FROM equip_codes
WHERE commander_id = sqlc.arg(commander_id) AND group_id = sqlc.arg(group_id) AND shared_at >= sqlc.arg(since);

-- name: CountEquipCodesSince :one
SELECT COUNT(*) FROM equip_codes WHERE commander_id = sqlc.arg(commander_id) AND shared_at >= sqlc.arg(since);

-- name: CreateEquipCode :exec
INSERT INTO equip_codes (group_id, commander_id, eqcode, shared_at) VALUES (?, ?, ?, ?);

-- name: CountEquipCodeInGroup :one
SELECT COUNT(*) FROM equip_codes WHERE id = ? AND group_id = ?;

-- name: CountEquipCodeLikesSince :one
SELECT COUNT(*) FROM equip_code_likes
WHERE share_id = sqlc.arg(share_id) AND commander_id = sqlc.arg(commander_id) AND liked_at >= sqlc.arg(since);

-- name: SetEquipCodeLike :exec
INSERT INTO equip_code_likes (share_id, commander_id, liked_at) VALUES (?, ?, ?)
ON CONFLICT (share_id, commander_id) DO UPDATE SET liked_at = excluded.liked_at;

-- name: AddEquipCodeLike :exec
UPDATE equip_codes SET likes = likes + 1 WHERE id = ?;

-- name: CountEquipCodeReportsSince :one
SELECT COUNT(*) FROM equip_code_reports WHERE commander_id = sqlc.arg(commander_id) AND reported_at >= sqlc.arg(since);

-- name: SetEquipCodeReport :exec
INSERT INTO equip_code_reports (share_id, commander_id, report_type, reported_at) VALUES (?, ?, ?, ?)
ON CONFLICT (share_id, commander_id) DO UPDATE SET report_type = excluded.report_type, reported_at = excluded.reported_at;
