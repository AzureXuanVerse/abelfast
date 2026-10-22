-- name: ArchivesSetShipGroup :execrows
UPDATE commander_ship_groups SET lv_max = ?, intimacy_max = ?, star = ?, marry_flag = ?, heart_flag = ?, trans = ?
WHERE commander_id = ? AND group_id = ?;

-- name: ArchivesDeleteShipGroup :execrows
DELETE FROM commander_ship_groups WHERE commander_id = ? AND group_id = ?;

-- name: ArchivesUnclaimCollection :exec
DELETE FROM commander_collection_awards WHERE commander_id = ? AND storeup_id = ? AND award_index >= ?;

-- name: ArchivesDeleteAttireFrame :execrows
DELETE FROM attire_frames WHERE commander_id = ? AND drop_type = ? AND frame_id = ?;

-- name: ArchivesDeleteStory :exec
DELETE FROM commander_stories WHERE commander_id = ? AND story_id = ?;
