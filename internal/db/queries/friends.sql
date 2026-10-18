-- name: SetLastOnline :exec
UPDATE commanders SET last_online = ? WHERE id = ?;

-- name: GetCommanderByName :one
SELECT * FROM commanders WHERE name = ? ORDER BY id LIMIT 1;

-- name: ListCommandersByID :many
SELECT * FROM commanders WHERE id IN (sqlc.slice(ids)) ORDER BY id;

-- name: ListRecommendedCommanders :many
SELECT * FROM commanders
WHERE id != sqlc.arg(commander_id)
	AND id NOT IN (SELECT friend_id FROM friends WHERE commander_id = sqlc.arg(commander_id))
	AND id NOT IN (SELECT blocked_id FROM blacklist WHERE commander_id = sqlc.arg(commander_id))
ORDER BY last_online DESC, id
LIMIT sqlc.arg(count);

-- name: CountShips :one
SELECT COUNT(*) FROM ships WHERE commander_id = ?;

-- name: ListFriends :many
SELECT sqlc.embed(commanders) FROM friends
JOIN commanders ON commanders.id = friends.friend_id
WHERE friends.commander_id = ?
ORDER BY friends.since, friends.friend_id;

-- name: ListFriendIDs :many
SELECT friend_id FROM friends WHERE commander_id = ? ORDER BY friend_id;

-- name: CountFriends :one
SELECT COUNT(*) FROM friends WHERE commander_id = ?;

-- name: CountFriend :one
SELECT COUNT(*) FROM friends WHERE commander_id = ? AND friend_id = ?;

-- name: AddFriend :exec
INSERT INTO friends (commander_id, friend_id, since) VALUES (?, ?, ?) ON CONFLICT DO NOTHING;

-- name: DeleteFriendship :execrows
DELETE FROM friends
WHERE (commander_id = sqlc.arg(commander_id) AND friend_id = sqlc.arg(friend_id))
	OR (commander_id = sqlc.arg(friend_id) AND friend_id = sqlc.arg(commander_id));

-- name: ListFriendRequests :many
SELECT sqlc.embed(commanders), friend_requests.content, friend_requests.sent_at FROM friend_requests
JOIN commanders ON commanders.id = friend_requests.sender_id
WHERE friend_requests.commander_id = ?
ORDER BY friend_requests.sent_at, friend_requests.sender_id;

-- name: CountFriendRequest :one
SELECT COUNT(*) FROM friend_requests WHERE commander_id = ? AND sender_id = ?;

-- name: SetFriendRequest :exec
INSERT INTO friend_requests (commander_id, sender_id, content, sent_at) VALUES (?, ?, ?, ?)
ON CONFLICT (commander_id, sender_id) DO UPDATE SET content = excluded.content, sent_at = excluded.sent_at;

-- name: DeleteFriendRequest :execrows
DELETE FROM friend_requests WHERE commander_id = ? AND sender_id = ?;

-- name: DeleteFriendRequests :exec
DELETE FROM friend_requests WHERE commander_id = ?;

-- name: ListBlacklist :many
SELECT sqlc.embed(commanders) FROM blacklist
JOIN commanders ON commanders.id = blacklist.blocked_id
WHERE blacklist.commander_id = ?
ORDER BY blacklist.blocked_at, blacklist.blocked_id;

-- name: CountBlacklist :one
SELECT COUNT(*) FROM blacklist WHERE commander_id = ?;

-- name: AddBlacklist :exec
INSERT INTO blacklist (commander_id, blocked_id, blocked_at) VALUES (?, ?, ?) ON CONFLICT DO NOTHING;

-- name: DeleteBlacklist :execrows
DELETE FROM blacklist WHERE commander_id = ? AND blocked_id = ?;
