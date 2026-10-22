-- The back-office's moderation screens (internal/admin area_moderation.go):
-- reports, cheater marks, shared content and guilds.

-- name: ModerationListDiscussionReports :many
SELECT r.discussion_id, r.commander_id, rc.name AS reporter, r.reason, r.reported_at,
	d.group_id, d.commander_id AS author_id, ac.name AS author, d.context, d.posted_at
FROM ship_discussion_reports r
JOIN ship_discussions d ON d.id = r.discussion_id
JOIN commanders rc ON rc.id = r.commander_id
JOIN commanders ac ON ac.id = d.commander_id
ORDER BY r.reported_at DESC;

-- name: ModerationListEquipCodeReports :many
SELECT r.share_id, r.commander_id, rc.name AS reporter, r.report_type, r.reported_at,
	e.group_id, e.commander_id AS author_id, ac.name AS author, e.eqcode, e.shared_at
FROM equip_code_reports r
JOIN equip_codes e ON e.id = r.share_id
JOIN commanders rc ON rc.id = r.commander_id
JOIN commanders ac ON ac.id = e.commander_id
ORDER BY r.reported_at DESC;

-- name: ModerationDeleteDiscussionReports :execrows
DELETE FROM ship_discussion_reports WHERE discussion_id = ?;

-- name: ModerationDeleteEquipCodeReports :execrows
DELETE FROM equip_code_reports WHERE share_id = ?;

-- name: ModerationDeleteDiscussionVotes :exec
DELETE FROM ship_discussion_votes WHERE discussion_id = ?;

-- name: ModerationDeleteDiscussion :execrows
DELETE FROM ship_discussions WHERE id = ?;

-- name: ModerationDeleteEquipCodeLikes :exec
DELETE FROM equip_code_likes WHERE share_id = ?;

-- name: ModerationDeleteEquipCode :execrows
DELETE FROM equip_codes WHERE id = ?;

-- name: ModerationListDiscussions :many
SELECT d.id, d.group_id, d.commander_id, c.name AS author, d.context, d.posted_at,
	CAST((SELECT COUNT(*) FROM ship_discussion_votes v WHERE v.discussion_id = d.id AND v.good_or_bad = 0) AS INTEGER) AS good_count,
	CAST((SELECT COUNT(*) FROM ship_discussion_votes v WHERE v.discussion_id = d.id AND v.good_or_bad = 1) AS INTEGER) AS bad_count,
	CAST((SELECT COUNT(*) FROM ship_discussion_reports r WHERE r.discussion_id = d.id) AS INTEGER) AS reports
FROM ship_discussions d
JOIN commanders c ON c.id = d.commander_id
ORDER BY d.id DESC;

-- name: ModerationListEquipCodes :many
SELECT e.id, e.group_id, e.commander_id, c.name AS author, e.eqcode, e.likes, e.shared_at,
	CAST((SELECT COUNT(*) FROM equip_code_reports r WHERE r.share_id = e.id AND r.report_type = 1) AS INTEGER) AS nonsense,
	CAST((SELECT COUNT(*) FROM equip_code_reports r WHERE r.share_id = e.id AND r.report_type = 2) AS INTEGER) AS outdated
FROM equip_codes e
JOIN commanders c ON c.id = e.commander_id
ORDER BY e.id DESC;

-- name: ModerationListSharedDormThemes :many
SELECT t.commander_id, c.name AS author, t.pos, t.theme, t.theme_id, t.upload_time,
	CAST((SELECT COUNT(*) FROM dorm_theme_likes l WHERE l.theme_id = t.theme_id) AS INTEGER) AS like_count,
	CAST((SELECT COUNT(*) FROM dorm_theme_favs f WHERE f.theme_id = t.theme_id) AS INTEGER) AS fav_count
FROM dorm_themes t
JOIN commanders c ON c.id = t.commander_id
WHERE t.upload_time > 0
ORDER BY t.upload_time DESC;

-- name: ModerationListCheaterMarks :many
SELECT m.id, m.commander_id, m.account_id, m.reason, m.marked_at, COALESCE(c.name, '') AS name
FROM cheater_marks m
LEFT JOIN commanders c ON c.id = m.commander_id
ORDER BY m.marked_at DESC, m.id DESC;

-- name: ModerationDismissCheaterMarks :execrows
DELETE FROM cheater_marks WHERE commander_id = ?;

-- name: ModerationListGuilds :many
SELECT sqlc.embed(guilds),
	CAST((SELECT COUNT(*) FROM guild_members m WHERE m.guild_id = guilds.id) AS INTEGER) AS member_count,
	CAST((SELECT COUNT(*) FROM guild_requests r WHERE r.guild_id = guilds.id) AS INTEGER) AS request_count,
	CAST(COALESCE((SELECT c.name FROM guild_members m JOIN commanders c ON c.id = m.commander_id
		WHERE m.guild_id = guilds.id AND m.duty = 1), '') AS TEXT) AS leader
FROM guilds
ORDER BY guilds.level DESC, guilds.exp DESC, guilds.id;

-- name: ModerationLatestGuildOperation :one
SELECT * FROM guild_operations WHERE guild_id = ? ORDER BY id DESC LIMIT 1;

-- name: ModerationEndGuildOperation :execrows
UPDATE guild_operations SET ended = 1 WHERE id = ? AND NOT ended;

-- name: ModerationRestoreGuildOperationMember :exec
INSERT INTO guild_operation_members (operation_id, commander_id, joined_at, extra) VALUES (?, ?, ?, ?);

-- name: ModerationCountGuildReports :one
SELECT COUNT(*) FROM guild_reports WHERE operation_id = ?;

-- name: ModerationListGuildCapitalLogs :many
SELECT * FROM guild_capital_logs WHERE guild_id = sqlc.arg(guild_id) ORDER BY id DESC LIMIT sqlc.arg(count);

-- name: ModerationListGuildLogs :many
SELECT * FROM guild_logs WHERE guild_id = sqlc.arg(guild_id) ORDER BY id DESC LIMIT sqlc.arg(count);

-- name: ModerationListGuildRankScores :many
SELECT s.commander_id, COALESCE(c.name, '') AS name, s.kind, s.week_start, s.week, s.month_start, s.month, s.total
FROM guild_rank_scores s
LEFT JOIN commanders c ON c.id = s.commander_id
WHERE s.guild_id = ?
ORDER BY s.total DESC, s.commander_id;
