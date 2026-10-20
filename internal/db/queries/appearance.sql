-- name: GetAppearance :one
SELECT * FROM commander_appearance WHERE commander_id = ?;

-- name: SetIconFrame :exec
INSERT INTO commander_appearance (commander_id, icon_frame) VALUES (?, ?)
ON CONFLICT (commander_id) DO UPDATE SET icon_frame = excluded.icon_frame;

-- name: SetChatFrame :exec
INSERT INTO commander_appearance (commander_id, chat_frame) VALUES (?, ?)
ON CONFLICT (commander_id) DO UPDATE SET chat_frame = excluded.chat_frame;

-- name: SetBattleUI :exec
INSERT INTO commander_appearance (commander_id, battle_ui) VALUES (?, ?)
ON CONFLICT (commander_id) DO UPDATE SET battle_ui = excluded.battle_ui;

-- name: SetCover :exec
INSERT INTO commander_appearance (commander_id, cover) VALUES (?, ?)
ON CONFLICT (commander_id) DO UPDATE SET cover = excluded.cover;

-- name: SetLoadingPics :exec
INSERT INTO commander_appearance (commander_id, loading_pic_open, gallery_pics, manga_pics) VALUES (?, ?, ?, ?)
ON CONFLICT (commander_id) DO UPDATE SET loading_pic_open = excluded.loading_pic_open,
	gallery_pics = excluded.gallery_pics, manga_pics = excluded.manga_pics;

-- name: GetSecondaryPassword :one
SELECT * FROM commander_secondary_passwords WHERE commander_id = ?;

-- name: CreateSecondaryPassword :exec
INSERT INTO commander_secondary_passwords (commander_id, password_hash, notice, system_list) VALUES (?, ?, ?, ?);

-- name: SetSecondaryPasswordSystems :exec
UPDATE commander_secondary_passwords SET system_list = ? WHERE commander_id = ?;

-- name: SetSecondaryPasswordFails :exec
UPDATE commander_secondary_passwords SET fail_count = ?, fail_cd = ? WHERE commander_id = ?;

-- name: DeleteSecondaryPassword :exec
DELETE FROM commander_secondary_passwords WHERE commander_id = ?;
