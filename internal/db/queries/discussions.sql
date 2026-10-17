-- name: ListShipDiscussions :many
SELECT d.id, c.name AS nick_name, d.context,
	CAST((SELECT COUNT(*) FROM ship_discussion_votes v WHERE v.discussion_id = d.id AND v.good_or_bad = 0) AS INTEGER) AS good_count,
	CAST((SELECT COUNT(*) FROM ship_discussion_votes v WHERE v.discussion_id = d.id AND v.good_or_bad = 1) AS INTEGER) AS bad_count
FROM ship_discussions d
JOIN commanders c ON c.id = d.commander_id
WHERE d.group_id = ?
ORDER BY d.id DESC;

-- name: CountShipDiscussions :one
SELECT COUNT(*) FROM ship_discussions WHERE group_id = ?;

-- name: CountGroupDiscussionsSince :one
SELECT COUNT(*) FROM ship_discussions
WHERE commander_id = sqlc.arg(commander_id) AND group_id = sqlc.arg(group_id) AND posted_at >= sqlc.arg(since);

-- name: CountDiscussionsSince :one
SELECT COUNT(*) FROM ship_discussions WHERE commander_id = sqlc.arg(commander_id) AND posted_at >= sqlc.arg(since);

-- name: CreateShipDiscussion :exec
INSERT INTO ship_discussions (group_id, commander_id, context, posted_at) VALUES (?, ?, ?, ?);

-- name: CountDiscussionInGroup :one
SELECT COUNT(*) FROM ship_discussions WHERE id = ? AND group_id = ?;

-- name: AddShipDiscussionVote :execrows
INSERT INTO ship_discussion_votes (discussion_id, commander_id, good_or_bad) VALUES (?, ?, ?)
ON CONFLICT DO NOTHING;

-- name: SetShipDiscussionReport :exec
INSERT INTO ship_discussion_reports (discussion_id, commander_id, reason, reported_at) VALUES (?, ?, ?, ?)
ON CONFLICT (discussion_id, commander_id) DO UPDATE SET reason = excluded.reason, reported_at = excluded.reported_at;

-- name: CountOwnedShipGroup :one
SELECT COUNT(*) FROM commander_ship_groups WHERE commander_id = ? AND group_id = ?;

-- name: SetShipGroupHeart :execrows
UPDATE commander_ship_groups SET heart_flag = 1 WHERE commander_id = ? AND group_id = ? AND heart_flag = 0;

-- name: CountShipHearts :one
SELECT COUNT(*) FROM commander_ship_groups WHERE group_id = ? AND heart_flag > 0;

-- name: ListShipHeartCounts :many
SELECT group_id, COUNT(*) AS hearts FROM commander_ship_groups WHERE heart_flag > 0 GROUP BY group_id;
