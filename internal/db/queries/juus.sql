-- name: ListJuusPosts :many
SELECT * FROM commander_juus_posts WHERE commander_id = ? ORDER BY post_id;

-- name: GetJuusPost :one
SELECT * FROM commander_juus_posts WHERE commander_id = ? AND post_id = ?;

-- name: CreateJuusPost :exec
INSERT INTO commander_juus_posts (commander_id, post_id) VALUES (?, ?) ON CONFLICT DO NOTHING;

-- name: LikeJuusPost :one
UPDATE commander_juus_posts SET is_good = 1 WHERE commander_id = ? AND post_id = ?
RETURNING *;

-- name: ReadJuusPost :one
UPDATE commander_juus_posts SET is_read = 1 WHERE commander_id = ? AND post_id = ?
RETURNING *;

-- name: CommentJuusPost :exec
UPDATE commander_juus_posts SET comment_discuss = ?, comment_index = ?, comment_time = ?
WHERE commander_id = ? AND post_id = ?;

-- name: ListJuusTopics :many
SELECT topic_id, op_time, read_flag FROM commander_juus_topics WHERE commander_id = ? ORDER BY rowid;

-- name: ActivateJuusTopic :execrows
INSERT INTO commander_juus_topics (commander_id, topic_id, op_time) VALUES (?, ?, ?)
ON CONFLICT DO NOTHING;

-- name: TouchJuusTopic :execrows
UPDATE commander_juus_topics SET op_time = ?, read_flag = 0 WHERE commander_id = ? AND topic_id = ?;

-- name: ReadJuusTopic :exec
UPDATE commander_juus_topics SET read_flag = 1 WHERE commander_id = ? AND topic_id = ?;

-- name: ReadAllJuusTopics :exec
UPDATE commander_juus_topics SET read_flag = 1 WHERE commander_id = ?;

-- name: AddJuusReply :exec
INSERT INTO commander_juus_replies (commander_id, topic_id, chat_id, value) VALUES (?, ?, ?, ?)
ON CONFLICT (commander_id, topic_id, chat_id) DO UPDATE SET value = excluded.value;

-- name: ListJuusReplies :many
SELECT topic_id, chat_id, value FROM commander_juus_replies WHERE commander_id = ? ORDER BY rowid;

-- name: ListJuusGroups :many
SELECT * FROM commander_juus_groups WHERE commander_id = ?;

-- name: SetJuusGroupSkin :exec
INSERT INTO commander_juus_groups (commander_id, group_id, skin_id) VALUES (?, ?, ?)
ON CONFLICT (commander_id, group_id) DO UPDATE SET skin_id = excluded.skin_id;

-- name: SetJuusGroupFavorite :exec
INSERT INTO commander_juus_groups (commander_id, group_id, favorite) VALUES (?, ?, ?)
ON CONFLICT (commander_id, group_id) DO UPDATE SET favorite = excluded.favorite;

-- name: SetJuusGroupTopic :exec
INSERT INTO commander_juus_groups (commander_id, group_id, cur_topic) VALUES (?, ?, ?)
ON CONFLICT (commander_id, group_id) DO UPDATE SET cur_topic = excluded.cur_topic;
