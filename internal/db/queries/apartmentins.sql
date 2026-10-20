-- name: ListApartmentChats :many
SELECT ship_group, care_flag, cur_back, cur_comm_id FROM commander_apartment_chats
WHERE commander_id = ?;

-- name: SetApartmentChatCare :exec
INSERT INTO commander_apartment_chats (commander_id, ship_group, care_flag) VALUES (?, ?, ?)
ON CONFLICT (commander_id, ship_group) DO UPDATE SET care_flag = excluded.care_flag;

-- name: SetApartmentChatBack :exec
INSERT INTO commander_apartment_chats (commander_id, ship_group, cur_back) VALUES (?, ?, ?)
ON CONFLICT (commander_id, ship_group) DO UPDATE SET cur_back = excluded.cur_back;

-- name: SetApartmentChatTopic :exec
INSERT INTO commander_apartment_chats (commander_id, ship_group, cur_comm_id) VALUES (?, ?, ?)
ON CONFLICT (commander_id, ship_group) DO UPDATE SET cur_comm_id = excluded.cur_comm_id;

-- name: AddApartmentEvent :one
INSERT INTO commander_apartment_events (commander_id, ship_group, event_type, value) VALUES (?, ?, ?, ?)
ON CONFLICT (commander_id, ship_group, event_type) DO UPDATE SET value = value + excluded.value
RETURNING value;

-- name: MaxApartmentEvent :one
INSERT INTO commander_apartment_events (commander_id, ship_group, event_type, value) VALUES (?, ?, ?, ?)
ON CONFLICT (commander_id, ship_group, event_type) DO UPDATE SET value = MAX(value, excluded.value)
RETURNING value;

-- name: ListApartmentInsContents :many
SELECT type, content_id, ship_group, time, read_flag, good_flag, exit_time FROM commander_apartment_ins_contents
WHERE commander_id = ?
ORDER BY type, time, content_id;

-- name: GetApartmentInsContent :one
SELECT ship_group, time, read_flag, good_flag, exit_time FROM commander_apartment_ins_contents
WHERE commander_id = ? AND type = ? AND content_id = ?;

-- name: UnlockApartmentInsContent :execrows
INSERT INTO commander_apartment_ins_contents (commander_id, type, content_id, ship_group, time) VALUES (?, ?, ?, ?, ?)
ON CONFLICT DO NOTHING;

-- name: SetApartmentInsRead :exec
UPDATE commander_apartment_ins_contents SET read_flag = ?
WHERE commander_id = ? AND type = ? AND content_id = ?;

-- name: SetApartmentInsGood :exec
UPDATE commander_apartment_ins_contents SET good_flag = 1
WHERE commander_id = ? AND type = ? AND content_id = ?;

-- name: SetApartmentInsExit :exec
UPDATE commander_apartment_ins_contents SET exit_time = ?
WHERE commander_id = ? AND type = ? AND content_id = ?;

-- name: ListApartmentInsReplies :many
SELECT type, content_id, reply_key, value, time FROM commander_apartment_ins_replies
WHERE commander_id = ?
ORDER BY type, content_id, rowid;

-- name: AddApartmentInsReply :execrows
INSERT INTO commander_apartment_ins_replies (commander_id, type, content_id, reply_key, value, time) VALUES (?, ?, ?, ?, ?, ?)
ON CONFLICT DO NOTHING;
