-- name: AdminProfileCounts :one
SELECT
	CAST((SELECT COUNT(*) FROM ships s WHERE s.commander_id = sqlc.arg(id)) AS INTEGER) AS ships,
	CAST((SELECT COALESCE(SUM(e.count), 0) FROM commander_equipments e WHERE e.commander_id = sqlc.arg(id)) AS INTEGER) AS equipments,
	CAST((SELECT COUNT(*) FROM commander_skins k WHERE k.commander_id = sqlc.arg(id)) AS INTEGER) AS skins,
	CAST((SELECT COUNT(*) FROM meowfficers m WHERE m.commander_id = sqlc.arg(id)) AS INTEGER) AS meowfficers,
	CAST((SELECT COUNT(*) FROM friends f WHERE f.commander_id = sqlc.arg(id)) AS INTEGER) AS friends,
	CAST((SELECT COUNT(*) FROM spweapons w WHERE w.commander_id = sqlc.arg(id)) AS INTEGER) AS spweapons,
	CAST((SELECT COUNT(*) FROM commander_furniture u WHERE u.commander_id = sqlc.arg(id)) AS INTEGER) AS furniture,
	CAST((SELECT COUNT(*) FROM ships s WHERE s.commander_id = sqlc.arg(id) AND s.propose != 0) AS INTEGER) AS oaths,
	CAST((SELECT COUNT(*) FROM commander_secondary_passwords p WHERE p.commander_id = sqlc.arg(id)) AS INTEGER) AS secondary_password;

-- name: AdminCommanderGuild :one
SELECT g.id, g.name, m.duty FROM guild_members m JOIN guilds g ON g.id = m.guild_id WHERE m.commander_id = ?;

-- name: AdminSetItemCount :exec
INSERT INTO commander_items (commander_id, item_id, count) VALUES (?, ?, ?)
ON CONFLICT (commander_id, item_id) DO UPDATE SET count = excluded.count;

-- name: AdminDeleteItem :exec
DELETE FROM commander_items WHERE commander_id = ? AND item_id = ?;

-- name: AdminSetEquipmentCount :exec
INSERT INTO commander_equipments (commander_id, equip_id, count) VALUES (?, ?, ?)
ON CONFLICT (commander_id, equip_id) DO UPDATE SET count = excluded.count;

-- name: AdminDeleteEquipment :exec
DELETE FROM commander_equipments WHERE commander_id = ? AND equip_id = ?;

-- name: AdminSetEquipSkinCount :exec
INSERT INTO commander_equip_skins (commander_id, skin_id, count) VALUES (?, ?, ?)
ON CONFLICT (commander_id, skin_id) DO UPDATE SET count = excluded.count;

-- name: AdminDeleteEquipSkin :exec
DELETE FROM commander_equip_skins WHERE commander_id = ? AND skin_id = ?;

-- name: AdminListVirtualItems :many
SELECT item_id, amount FROM commander_virtual_items WHERE commander_id = ? ORDER BY item_id;

-- name: AdminSetVirtualItem :exec
UPDATE commander_virtual_items SET amount = ? WHERE commander_id = ? AND item_id = ?;

-- name: AdminListSkins :many
SELECT skin_id, end_time FROM commander_skins WHERE commander_id = ? ORDER BY skin_id;

-- name: AdminUnwearSkin :exec
UPDATE ships SET skin_id = 0 WHERE commander_id = ? AND skin_id = ?;

-- name: AdminDeleteSkin :exec
DELETE FROM commander_skins WHERE commander_id = ? AND skin_id = ?;

-- name: AdminListSpWeapons :many
SELECT w.id, w.template_id, w.attr_1, w.attr_2, w.attr_temp_1, w.attr_temp_2, w.pt,
	CAST(COALESCE(s.id, 0) AS INTEGER) AS ship_id, CAST(COALESCE(s.template_id, 0) AS INTEGER) AS ship_template_id,
	CAST(COALESCE(s.name, '') AS TEXT) AS ship_name
FROM spweapons w
LEFT JOIN ship_spweapons x ON x.spweapon_id = w.id
LEFT JOIN ships s ON s.id = x.ship_id
WHERE w.commander_id = ? ORDER BY w.id;

-- name: AdminListAllCompensations :many
SELECT * FROM compensations WHERE commander_id = ? ORDER BY id DESC;

-- name: AdminDeleteMail :execrows
DELETE FROM mails WHERE commander_id = ? AND id = ?;

-- name: AdminDeleteCompensation :execrows
DELETE FROM compensations WHERE commander_id = ? AND id = ?;

-- Generated from the schema (every table holding a commander's rows, see
-- game.commanderTables): one export and one delete per table, the rows
-- pointing at others first. Regenerate when a table is added.

-- name: AdminExportActivityFleetMeowfficers :one
SELECT CAST(json_group_array(json_object('commander_id', CASE WHEN typeof(commander_id) = 'blob' THEN (CASE WHEN json_valid(CAST(commander_id AS TEXT)) THEN CAST(commander_id AS TEXT) ELSE hex(commander_id) END) ELSE commander_id END, 'activity_id', CASE WHEN typeof(activity_id) = 'blob' THEN (CASE WHEN json_valid(CAST(activity_id AS TEXT)) THEN CAST(activity_id AS TEXT) ELSE hex(activity_id) END) ELSE activity_id END, 'fleet_id', CASE WHEN typeof(fleet_id) = 'blob' THEN (CASE WHEN json_valid(CAST(fleet_id AS TEXT)) THEN CAST(fleet_id AS TEXT) ELSE hex(fleet_id) END) ELSE fleet_id END, 'pos', CASE WHEN typeof(pos) = 'blob' THEN (CASE WHEN json_valid(CAST(pos AS TEXT)) THEN CAST(pos AS TEXT) ELSE hex(pos) END) ELSE pos END, 'meowfficer_id', CASE WHEN typeof(meowfficer_id) = 'blob' THEN (CASE WHEN json_valid(CAST(meowfficer_id AS TEXT)) THEN CAST(meowfficer_id AS TEXT) ELSE hex(meowfficer_id) END) ELSE meowfficer_id END)) AS TEXT) FROM activity_fleet_meowfficers WHERE activity_fleet_meowfficers.commander_id = sqlc.arg(id);

-- name: AdminDeleteActivityFleetMeowfficers :exec
DELETE FROM activity_fleet_meowfficers WHERE activity_fleet_meowfficers.commander_id = sqlc.arg(id);

-- name: AdminExportActivityFleetShips :one
SELECT CAST(json_group_array(json_object('commander_id', CASE WHEN typeof(commander_id) = 'blob' THEN (CASE WHEN json_valid(CAST(commander_id AS TEXT)) THEN CAST(commander_id AS TEXT) ELSE hex(commander_id) END) ELSE commander_id END, 'activity_id', CASE WHEN typeof(activity_id) = 'blob' THEN (CASE WHEN json_valid(CAST(activity_id AS TEXT)) THEN CAST(activity_id AS TEXT) ELSE hex(activity_id) END) ELSE activity_id END, 'fleet_id', CASE WHEN typeof(fleet_id) = 'blob' THEN (CASE WHEN json_valid(CAST(fleet_id AS TEXT)) THEN CAST(fleet_id AS TEXT) ELSE hex(fleet_id) END) ELSE fleet_id END, 'pos', CASE WHEN typeof(pos) = 'blob' THEN (CASE WHEN json_valid(CAST(pos AS TEXT)) THEN CAST(pos AS TEXT) ELSE hex(pos) END) ELSE pos END, 'ship_id', CASE WHEN typeof(ship_id) = 'blob' THEN (CASE WHEN json_valid(CAST(ship_id AS TEXT)) THEN CAST(ship_id AS TEXT) ELSE hex(ship_id) END) ELSE ship_id END)) AS TEXT) FROM activity_fleet_ships WHERE activity_fleet_ships.commander_id = sqlc.arg(id);

-- name: AdminDeleteActivityFleetShips :exec
DELETE FROM activity_fleet_ships WHERE activity_fleet_ships.commander_id = sqlc.arg(id);

-- name: AdminExportApartmentFurniture :one
SELECT CAST(json_group_array(json_object('commander_id', CASE WHEN typeof(commander_id) = 'blob' THEN (CASE WHEN json_valid(CAST(commander_id AS TEXT)) THEN CAST(commander_id AS TEXT) ELSE hex(commander_id) END) ELSE commander_id END, 'room_id', CASE WHEN typeof(room_id) = 'blob' THEN (CASE WHEN json_valid(CAST(room_id AS TEXT)) THEN CAST(room_id AS TEXT) ELSE hex(room_id) END) ELSE room_id END, 'furniture_id', CASE WHEN typeof(furniture_id) = 'blob' THEN (CASE WHEN json_valid(CAST(furniture_id AS TEXT)) THEN CAST(furniture_id AS TEXT) ELSE hex(furniture_id) END) ELSE furniture_id END, 'slot_id', CASE WHEN typeof(slot_id) = 'blob' THEN (CASE WHEN json_valid(CAST(slot_id AS TEXT)) THEN CAST(slot_id AS TEXT) ELSE hex(slot_id) END) ELSE slot_id END)) AS TEXT) FROM apartment_furniture WHERE apartment_furniture.commander_id = sqlc.arg(id);

-- name: AdminDeleteApartmentFurniture :exec
DELETE FROM apartment_furniture WHERE apartment_furniture.commander_id = sqlc.arg(id);

-- name: AdminExportApartmentGifts :one
SELECT CAST(json_group_array(json_object('commander_id', CASE WHEN typeof(commander_id) = 'blob' THEN (CASE WHEN json_valid(CAST(commander_id AS TEXT)) THEN CAST(commander_id AS TEXT) ELSE hex(commander_id) END) ELSE commander_id END, 'gift_id', CASE WHEN typeof(gift_id) = 'blob' THEN (CASE WHEN json_valid(CAST(gift_id AS TEXT)) THEN CAST(gift_id AS TEXT) ELSE hex(gift_id) END) ELSE gift_id END, 'number', CASE WHEN typeof(number) = 'blob' THEN (CASE WHEN json_valid(CAST(number AS TEXT)) THEN CAST(number AS TEXT) ELSE hex(number) END) ELSE number END, 'used_number', CASE WHEN typeof(used_number) = 'blob' THEN (CASE WHEN json_valid(CAST(used_number AS TEXT)) THEN CAST(used_number AS TEXT) ELSE hex(used_number) END) ELSE used_number END)) AS TEXT) FROM apartment_gifts WHERE apartment_gifts.commander_id = sqlc.arg(id);

-- name: AdminDeleteApartmentGifts :exec
DELETE FROM apartment_gifts WHERE apartment_gifts.commander_id = sqlc.arg(id);

-- name: AdminExportApartmentHiddenParts :one
SELECT CAST(json_group_array(json_object('commander_id', CASE WHEN typeof(commander_id) = 'blob' THEN (CASE WHEN json_valid(CAST(commander_id AS TEXT)) THEN CAST(commander_id AS TEXT) ELSE hex(commander_id) END) ELSE commander_id END, 'ship_group', CASE WHEN typeof(ship_group) = 'blob' THEN (CASE WHEN json_valid(CAST(ship_group AS TEXT)) THEN CAST(ship_group AS TEXT) ELSE hex(ship_group) END) ELSE ship_group END, 'skin_id', CASE WHEN typeof(skin_id) = 'blob' THEN (CASE WHEN json_valid(CAST(skin_id AS TEXT)) THEN CAST(skin_id AS TEXT) ELSE hex(skin_id) END) ELSE skin_id END, 'parts', CASE WHEN typeof(parts) = 'blob' THEN (CASE WHEN json_valid(CAST(parts AS TEXT)) THEN CAST(parts AS TEXT) ELSE hex(parts) END) ELSE parts END)) AS TEXT) FROM apartment_hidden_parts WHERE apartment_hidden_parts.commander_id = sqlc.arg(id);

-- name: AdminDeleteApartmentHiddenParts :exec
DELETE FROM apartment_hidden_parts WHERE apartment_hidden_parts.commander_id = sqlc.arg(id);

-- name: AdminExportApartmentRooms :one
SELECT CAST(json_group_array(json_object('commander_id', CASE WHEN typeof(commander_id) = 'blob' THEN (CASE WHEN json_valid(CAST(commander_id AS TEXT)) THEN CAST(commander_id AS TEXT) ELSE hex(commander_id) END) ELSE commander_id END, 'room_id', CASE WHEN typeof(room_id) = 'blob' THEN (CASE WHEN json_valid(CAST(room_id AS TEXT)) THEN CAST(room_id AS TEXT) ELSE hex(room_id) END) ELSE room_id END, 'collections', CASE WHEN typeof(collections) = 'blob' THEN (CASE WHEN json_valid(CAST(collections AS TEXT)) THEN CAST(collections AS TEXT) ELSE hex(collections) END) ELSE collections END, 'ships', CASE WHEN typeof(ships) = 'blob' THEN (CASE WHEN json_valid(CAST(ships AS TEXT)) THEN CAST(ships AS TEXT) ELSE hex(ships) END) ELSE ships END)) AS TEXT) FROM apartment_rooms WHERE apartment_rooms.commander_id = sqlc.arg(id);

-- name: AdminDeleteApartmentRooms :exec
DELETE FROM apartment_rooms WHERE apartment_rooms.commander_id = sqlc.arg(id);

-- name: AdminExportApartmentShips :one
SELECT CAST(json_group_array(json_object('commander_id', CASE WHEN typeof(commander_id) = 'blob' THEN (CASE WHEN json_valid(CAST(commander_id AS TEXT)) THEN CAST(commander_id AS TEXT) ELSE hex(commander_id) END) ELSE commander_id END, 'ship_group', CASE WHEN typeof(ship_group) = 'blob' THEN (CASE WHEN json_valid(CAST(ship_group AS TEXT)) THEN CAST(ship_group AS TEXT) ELSE hex(ship_group) END) ELSE ship_group END, 'favor_lv', CASE WHEN typeof(favor_lv) = 'blob' THEN (CASE WHEN json_valid(CAST(favor_lv AS TEXT)) THEN CAST(favor_lv AS TEXT) ELSE hex(favor_lv) END) ELSE favor_lv END, 'favor_exp', CASE WHEN typeof(favor_exp) = 'blob' THEN (CASE WHEN json_valid(CAST(favor_exp AS TEXT)) THEN CAST(favor_exp AS TEXT) ELSE hex(favor_exp) END) ELSE favor_exp END, 'daily_favor', CASE WHEN typeof(daily_favor) = 'blob' THEN (CASE WHEN json_valid(CAST(daily_favor AS TEXT)) THEN CAST(daily_favor AS TEXT) ELSE hex(daily_favor) END) ELSE daily_favor END, 'favor_day', CASE WHEN typeof(favor_day) = 'blob' THEN (CASE WHEN json_valid(CAST(favor_day AS TEXT)) THEN CAST(favor_day AS TEXT) ELSE hex(favor_day) END) ELSE favor_day END, 'triggers', CASE WHEN typeof(triggers) = 'blob' THEN (CASE WHEN json_valid(CAST(triggers AS TEXT)) THEN CAST(triggers AS TEXT) ELSE hex(triggers) END) ELSE triggers END, 'dialogues', CASE WHEN typeof(dialogues) = 'blob' THEN (CASE WHEN json_valid(CAST(dialogues AS TEXT)) THEN CAST(dialogues AS TEXT) ELSE hex(dialogues) END) ELSE dialogues END, 'skins', CASE WHEN typeof(skins) = 'blob' THEN (CASE WHEN json_valid(CAST(skins AS TEXT)) THEN CAST(skins AS TEXT) ELSE hex(skins) END) ELSE skins END, 'cur_skin', CASE WHEN typeof(cur_skin) = 'blob' THEN (CASE WHEN json_valid(CAST(cur_skin AS TEXT)) THEN CAST(cur_skin AS TEXT) ELSE hex(cur_skin) END) ELSE cur_skin END, 'name', CASE WHEN typeof(name) = 'blob' THEN (CASE WHEN json_valid(CAST(name AS TEXT)) THEN CAST(name AS TEXT) ELSE hex(name) END) ELSE name END, 'name_cd', CASE WHEN typeof(name_cd) = 'blob' THEN (CASE WHEN json_valid(CAST(name_cd AS TEXT)) THEN CAST(name_cd AS TEXT) ELSE hex(name_cd) END) ELSE name_cd END, 'visit_time', CASE WHEN typeof(visit_time) = 'blob' THEN (CASE WHEN json_valid(CAST(visit_time AS TEXT)) THEN CAST(visit_time AS TEXT) ELSE hex(visit_time) END) ELSE visit_time END)) AS TEXT) FROM apartment_ships WHERE apartment_ships.commander_id = sqlc.arg(id);

-- name: AdminDeleteApartmentShips :exec
DELETE FROM apartment_ships WHERE apartment_ships.commander_id = sqlc.arg(id);

-- name: AdminExportAttireFrames :one
SELECT CAST(json_group_array(json_object('commander_id', CASE WHEN typeof(commander_id) = 'blob' THEN (CASE WHEN json_valid(CAST(commander_id AS TEXT)) THEN CAST(commander_id AS TEXT) ELSE hex(commander_id) END) ELSE commander_id END, 'drop_type', CASE WHEN typeof(drop_type) = 'blob' THEN (CASE WHEN json_valid(CAST(drop_type AS TEXT)) THEN CAST(drop_type AS TEXT) ELSE hex(drop_type) END) ELSE drop_type END, 'frame_id', CASE WHEN typeof(frame_id) = 'blob' THEN (CASE WHEN json_valid(CAST(frame_id AS TEXT)) THEN CAST(frame_id AS TEXT) ELSE hex(frame_id) END) ELSE frame_id END)) AS TEXT) FROM attire_frames WHERE attire_frames.commander_id = sqlc.arg(id);

-- name: AdminDeleteAttireFrames :exec
DELETE FROM attire_frames WHERE attire_frames.commander_id = sqlc.arg(id);

-- name: AdminExportBlacklist :one
SELECT CAST(json_group_array(json_object('commander_id', CASE WHEN typeof(commander_id) = 'blob' THEN (CASE WHEN json_valid(CAST(commander_id AS TEXT)) THEN CAST(commander_id AS TEXT) ELSE hex(commander_id) END) ELSE commander_id END, 'blocked_id', CASE WHEN typeof(blocked_id) = 'blob' THEN (CASE WHEN json_valid(CAST(blocked_id AS TEXT)) THEN CAST(blocked_id AS TEXT) ELSE hex(blocked_id) END) ELSE blocked_id END, 'blocked_at', CASE WHEN typeof(blocked_at) = 'blob' THEN (CASE WHEN json_valid(CAST(blocked_at AS TEXT)) THEN CAST(blocked_at AS TEXT) ELSE hex(blocked_at) END) ELSE blocked_at END)) AS TEXT) FROM blacklist WHERE blacklist.blocked_id = sqlc.arg(id) OR blacklist.commander_id = sqlc.arg(id);

-- name: AdminDeleteBlacklist :exec
DELETE FROM blacklist WHERE blacklist.blocked_id = sqlc.arg(id) OR blacklist.commander_id = sqlc.arg(id);

-- name: AdminExportBuilds :one
SELECT CAST(json_group_array(json_object('id', CASE WHEN typeof(id) = 'blob' THEN (CASE WHEN json_valid(CAST(id AS TEXT)) THEN CAST(id AS TEXT) ELSE hex(id) END) ELSE id END, 'commander_id', CASE WHEN typeof(commander_id) = 'blob' THEN (CASE WHEN json_valid(CAST(commander_id AS TEXT)) THEN CAST(commander_id AS TEXT) ELSE hex(commander_id) END) ELSE commander_id END, 'pool_id', CASE WHEN typeof(pool_id) = 'blob' THEN (CASE WHEN json_valid(CAST(pool_id AS TEXT)) THEN CAST(pool_id AS TEXT) ELSE hex(pool_id) END) ELSE pool_id END, 'template_id', CASE WHEN typeof(template_id) = 'blob' THEN (CASE WHEN json_valid(CAST(template_id AS TEXT)) THEN CAST(template_id AS TEXT) ELSE hex(template_id) END) ELSE template_id END, 'duration', CASE WHEN typeof(duration) = 'blob' THEN (CASE WHEN json_valid(CAST(duration AS TEXT)) THEN CAST(duration AS TEXT) ELSE hex(duration) END) ELSE duration END, 'finish_time', CASE WHEN typeof(finish_time) = 'blob' THEN (CASE WHEN json_valid(CAST(finish_time AS TEXT)) THEN CAST(finish_time AS TEXT) ELSE hex(finish_time) END) ELSE finish_time END)) AS TEXT) FROM builds WHERE builds.commander_id = sqlc.arg(id);

-- name: AdminDeleteBuilds :exec
DELETE FROM builds WHERE builds.commander_id = sqlc.arg(id);

-- name: AdminExportChallengeMeowfficers :one
SELECT CAST(json_group_array(json_object('commander_id', CASE WHEN typeof(commander_id) = 'blob' THEN (CASE WHEN json_valid(CAST(commander_id AS TEXT)) THEN CAST(commander_id AS TEXT) ELSE hex(commander_id) END) ELSE commander_id END, 'activity_id', CASE WHEN typeof(activity_id) = 'blob' THEN (CASE WHEN json_valid(CAST(activity_id AS TEXT)) THEN CAST(activity_id AS TEXT) ELSE hex(activity_id) END) ELSE activity_id END, 'mode', CASE WHEN typeof(mode) = 'blob' THEN (CASE WHEN json_valid(CAST(mode AS TEXT)) THEN CAST(mode AS TEXT) ELSE hex(mode) END) ELSE mode END, 'fleet_id', CASE WHEN typeof(fleet_id) = 'blob' THEN (CASE WHEN json_valid(CAST(fleet_id AS TEXT)) THEN CAST(fleet_id AS TEXT) ELSE hex(fleet_id) END) ELSE fleet_id END, 'pos', CASE WHEN typeof(pos) = 'blob' THEN (CASE WHEN json_valid(CAST(pos AS TEXT)) THEN CAST(pos AS TEXT) ELSE hex(pos) END) ELSE pos END, 'meowfficer_id', CASE WHEN typeof(meowfficer_id) = 'blob' THEN (CASE WHEN json_valid(CAST(meowfficer_id AS TEXT)) THEN CAST(meowfficer_id AS TEXT) ELSE hex(meowfficer_id) END) ELSE meowfficer_id END)) AS TEXT) FROM challenge_meowfficers WHERE challenge_meowfficers.commander_id = sqlc.arg(id);

-- name: AdminDeleteChallengeMeowfficers :exec
DELETE FROM challenge_meowfficers WHERE challenge_meowfficers.commander_id = sqlc.arg(id);

-- name: AdminExportChallengeRecords :one
SELECT CAST(json_group_array(json_object('commander_id', CASE WHEN typeof(commander_id) = 'blob' THEN (CASE WHEN json_valid(CAST(commander_id AS TEXT)) THEN CAST(commander_id AS TEXT) ELSE hex(commander_id) END) ELSE commander_id END, 'activity_id', CASE WHEN typeof(activity_id) = 'blob' THEN (CASE WHEN json_valid(CAST(activity_id AS TEXT)) THEN CAST(activity_id AS TEXT) ELSE hex(activity_id) END) ELSE activity_id END, 'season', CASE WHEN typeof(season) = 'blob' THEN (CASE WHEN json_valid(CAST(season AS TEXT)) THEN CAST(season AS TEXT) ELSE hex(season) END) ELSE season END, 'max_score', CASE WHEN typeof(max_score) = 'blob' THEN (CASE WHEN json_valid(CAST(max_score AS TEXT)) THEN CAST(max_score AS TEXT) ELSE hex(max_score) END) ELSE max_score END, 'max_level', CASE WHEN typeof(max_level) = 'blob' THEN (CASE WHEN json_valid(CAST(max_level AS TEXT)) THEN CAST(max_level AS TEXT) ELSE hex(max_level) END) ELSE max_level END)) AS TEXT) FROM challenge_records WHERE challenge_records.commander_id = sqlc.arg(id);

-- name: AdminDeleteChallengeRecords :exec
DELETE FROM challenge_records WHERE challenge_records.commander_id = sqlc.arg(id);

-- name: AdminExportChallengeShips :one
SELECT CAST(json_group_array(json_object('commander_id', CASE WHEN typeof(commander_id) = 'blob' THEN (CASE WHEN json_valid(CAST(commander_id AS TEXT)) THEN CAST(commander_id AS TEXT) ELSE hex(commander_id) END) ELSE commander_id END, 'activity_id', CASE WHEN typeof(activity_id) = 'blob' THEN (CASE WHEN json_valid(CAST(activity_id AS TEXT)) THEN CAST(activity_id AS TEXT) ELSE hex(activity_id) END) ELSE activity_id END, 'mode', CASE WHEN typeof(mode) = 'blob' THEN (CASE WHEN json_valid(CAST(mode AS TEXT)) THEN CAST(mode AS TEXT) ELSE hex(mode) END) ELSE mode END, 'fleet_id', CASE WHEN typeof(fleet_id) = 'blob' THEN (CASE WHEN json_valid(CAST(fleet_id AS TEXT)) THEN CAST(fleet_id AS TEXT) ELSE hex(fleet_id) END) ELSE fleet_id END, 'pos', CASE WHEN typeof(pos) = 'blob' THEN (CASE WHEN json_valid(CAST(pos AS TEXT)) THEN CAST(pos AS TEXT) ELSE hex(pos) END) ELSE pos END, 'ship_id', CASE WHEN typeof(ship_id) = 'blob' THEN (CASE WHEN json_valid(CAST(ship_id AS TEXT)) THEN CAST(ship_id AS TEXT) ELSE hex(ship_id) END) ELSE ship_id END, 'hp_rant', CASE WHEN typeof(hp_rant) = 'blob' THEN (CASE WHEN json_valid(CAST(hp_rant AS TEXT)) THEN CAST(hp_rant AS TEXT) ELSE hex(hp_rant) END) ELSE hp_rant END)) AS TEXT) FROM challenge_ships WHERE challenge_ships.commander_id = sqlc.arg(id);

-- name: AdminDeleteChallengeShips :exec
DELETE FROM challenge_ships WHERE challenge_ships.commander_id = sqlc.arg(id);

-- name: AdminExportChallengeRuns :one
SELECT CAST(json_group_array(json_object('commander_id', CASE WHEN typeof(commander_id) = 'blob' THEN (CASE WHEN json_valid(CAST(commander_id AS TEXT)) THEN CAST(commander_id AS TEXT) ELSE hex(commander_id) END) ELSE commander_id END, 'activity_id', CASE WHEN typeof(activity_id) = 'blob' THEN (CASE WHEN json_valid(CAST(activity_id AS TEXT)) THEN CAST(activity_id AS TEXT) ELSE hex(activity_id) END) ELSE activity_id END, 'mode', CASE WHEN typeof(mode) = 'blob' THEN (CASE WHEN json_valid(CAST(mode AS TEXT)) THEN CAST(mode AS TEXT) ELSE hex(mode) END) ELSE mode END, 'season', CASE WHEN typeof(season) = 'blob' THEN (CASE WHEN json_valid(CAST(season AS TEXT)) THEN CAST(season AS TEXT) ELSE hex(season) END) ELSE season END, 'level', CASE WHEN typeof(level) = 'blob' THEN (CASE WHEN json_valid(CAST(level AS TEXT)) THEN CAST(level AS TEXT) ELSE hex(level) END) ELSE level END, 'score', CASE WHEN typeof(score) = 'blob' THEN (CASE WHEN json_valid(CAST(score AS TEXT)) THEN CAST(score AS TEXT) ELSE hex(score) END) ELSE score END, 'battling', CASE WHEN typeof(battling) = 'blob' THEN (CASE WHEN json_valid(CAST(battling AS TEXT)) THEN CAST(battling AS TEXT) ELSE hex(battling) END) ELSE battling END)) AS TEXT) FROM challenge_runs WHERE challenge_runs.commander_id = sqlc.arg(id);

-- name: AdminDeleteChallengeRuns :exec
DELETE FROM challenge_runs WHERE challenge_runs.commander_id = sqlc.arg(id);

-- name: AdminExportChapterAutoBattles :one
SELECT CAST(json_group_array(json_object('commander_id', CASE WHEN typeof(commander_id) = 'blob' THEN (CASE WHEN json_valid(CAST(commander_id AS TEXT)) THEN CAST(commander_id AS TEXT) ELSE hex(commander_id) END) ELSE commander_id END, 'seq', CASE WHEN typeof(seq) = 'blob' THEN (CASE WHEN json_valid(CAST(seq AS TEXT)) THEN CAST(seq AS TEXT) ELSE hex(seq) END) ELSE seq END, 'type', CASE WHEN typeof(type) = 'blob' THEN (CASE WHEN json_valid(CAST(type AS TEXT)) THEN CAST(type AS TEXT) ELSE hex(type) END) ELSE type END, 'chapter_id', CASE WHEN typeof(chapter_id) = 'blob' THEN (CASE WHEN json_valid(CAST(chapter_id AS TEXT)) THEN CAST(chapter_id AS TEXT) ELSE hex(chapter_id) END) ELSE chapter_id END, 'finish_at', CASE WHEN typeof(finish_at) = 'blob' THEN (CASE WHEN json_valid(CAST(finish_at AS TEXT)) THEN CAST(finish_at AS TEXT) ELSE hex(finish_at) END) ELSE finish_at END, 'ticket_expire_at', CASE WHEN typeof(ticket_expire_at) = 'blob' THEN (CASE WHEN json_valid(CAST(ticket_expire_at AS TEXT)) THEN CAST(ticket_expire_at AS TEXT) ELSE hex(ticket_expire_at) END) ELSE ticket_expire_at END, 'seconds', CASE WHEN typeof(seconds) = 'blob' THEN (CASE WHEN json_valid(CAST(seconds AS TEXT)) THEN CAST(seconds AS TEXT) ELSE hex(seconds) END) ELSE seconds END)) AS TEXT) FROM chapter_auto_battles WHERE chapter_auto_battles.commander_id = sqlc.arg(id);

-- name: AdminDeleteChapterAutoBattles :exec
DELETE FROM chapter_auto_battles WHERE chapter_auto_battles.commander_id = sqlc.arg(id);

-- name: AdminExportChapterAutoRecords :one
SELECT CAST(json_group_array(json_object('commander_id', CASE WHEN typeof(commander_id) = 'blob' THEN (CASE WHEN json_valid(CAST(commander_id AS TEXT)) THEN CAST(commander_id AS TEXT) ELSE hex(commander_id) END) ELSE commander_id END, 'type', CASE WHEN typeof(type) = 'blob' THEN (CASE WHEN json_valid(CAST(type AS TEXT)) THEN CAST(type AS TEXT) ELSE hex(type) END) ELSE type END, 'chapter_id', CASE WHEN typeof(chapter_id) = 'blob' THEN (CASE WHEN json_valid(CAST(chapter_id AS TEXT)) THEN CAST(chapter_id AS TEXT) ELSE hex(chapter_id) END) ELSE chapter_id END, 'seconds', CASE WHEN typeof(seconds) = 'blob' THEN (CASE WHEN json_valid(CAST(seconds AS TEXT)) THEN CAST(seconds AS TEXT) ELSE hex(seconds) END) ELSE seconds END)) AS TEXT) FROM chapter_auto_records WHERE chapter_auto_records.commander_id = sqlc.arg(id);

-- name: AdminDeleteChapterAutoRecords :exec
DELETE FROM chapter_auto_records WHERE chapter_auto_records.commander_id = sqlc.arg(id);

-- name: AdminExportChapterAutoStates :one
SELECT CAST(json_group_array(json_object('commander_id', CASE WHEN typeof(commander_id) = 'blob' THEN (CASE WHEN json_valid(CAST(commander_id AS TEXT)) THEN CAST(commander_id AS TEXT) ELSE hex(commander_id) END) ELSE commander_id END, 'oil', CASE WHEN typeof(oil) = 'blob' THEN (CASE WHEN json_valid(CAST(oil AS TEXT)) THEN CAST(oil AS TEXT) ELSE hex(oil) END) ELSE oil END, 'time_used', CASE WHEN typeof(time_used) = 'blob' THEN (CASE WHEN json_valid(CAST(time_used AS TEXT)) THEN CAST(time_used AS TEXT) ELSE hex(time_used) END) ELSE time_used END, 'extra_time', CASE WHEN typeof(extra_time) = 'blob' THEN (CASE WHEN json_valid(CAST(extra_time AS TEXT)) THEN CAST(extra_time AS TEXT) ELSE hex(extra_time) END) ELSE extra_time END, 'day', CASE WHEN typeof(day) = 'blob' THEN (CASE WHEN json_valid(CAST(day AS TEXT)) THEN CAST(day AS TEXT) ELSE hex(day) END) ELSE day END)) AS TEXT) FROM chapter_auto_states WHERE chapter_auto_states.commander_id = sqlc.arg(id);

-- name: AdminDeleteChapterAutoStates :exec
DELETE FROM chapter_auto_states WHERE chapter_auto_states.commander_id = sqlc.arg(id);

-- name: AdminExportChapterAutoTickets :one
SELECT CAST(json_group_array(json_object('commander_id', CASE WHEN typeof(commander_id) = 'blob' THEN (CASE WHEN json_valid(CAST(commander_id AS TEXT)) THEN CAST(commander_id AS TEXT) ELSE hex(commander_id) END) ELSE commander_id END, 'type', CASE WHEN typeof(type) = 'blob' THEN (CASE WHEN json_valid(CAST(type AS TEXT)) THEN CAST(type AS TEXT) ELSE hex(type) END) ELSE type END, 'expire_at', CASE WHEN typeof(expire_at) = 'blob' THEN (CASE WHEN json_valid(CAST(expire_at AS TEXT)) THEN CAST(expire_at AS TEXT) ELSE hex(expire_at) END) ELSE expire_at END, 'count', CASE WHEN typeof(count) = 'blob' THEN (CASE WHEN json_valid(CAST(count AS TEXT)) THEN CAST(count AS TEXT) ELSE hex(count) END) ELSE count END)) AS TEXT) FROM chapter_auto_tickets WHERE chapter_auto_tickets.commander_id = sqlc.arg(id);

-- name: AdminDeleteChapterAutoTickets :exec
DELETE FROM chapter_auto_tickets WHERE chapter_auto_tickets.commander_id = sqlc.arg(id);

-- name: AdminExportChapterDropShips :one
SELECT CAST(json_group_array(json_object('commander_id', CASE WHEN typeof(commander_id) = 'blob' THEN (CASE WHEN json_valid(CAST(commander_id AS TEXT)) THEN CAST(commander_id AS TEXT) ELSE hex(commander_id) END) ELSE commander_id END, 'chapter_id', CASE WHEN typeof(chapter_id) = 'blob' THEN (CASE WHEN json_valid(CAST(chapter_id AS TEXT)) THEN CAST(chapter_id AS TEXT) ELSE hex(chapter_id) END) ELSE chapter_id END, 'ship_id', CASE WHEN typeof(ship_id) = 'blob' THEN (CASE WHEN json_valid(CAST(ship_id AS TEXT)) THEN CAST(ship_id AS TEXT) ELSE hex(ship_id) END) ELSE ship_id END)) AS TEXT) FROM chapter_drop_ships WHERE chapter_drop_ships.commander_id = sqlc.arg(id);

-- name: AdminDeleteChapterDropShips :exec
DELETE FROM chapter_drop_ships WHERE chapter_drop_ships.commander_id = sqlc.arg(id);

-- name: AdminExportChapterEliteFleets :one
SELECT CAST(json_group_array(json_object('commander_id', CASE WHEN typeof(commander_id) = 'blob' THEN (CASE WHEN json_valid(CAST(commander_id AS TEXT)) THEN CAST(commander_id AS TEXT) ELSE hex(commander_id) END) ELSE commander_id END, 'formation', CASE WHEN typeof(formation) = 'blob' THEN (CASE WHEN json_valid(CAST(formation AS TEXT)) THEN CAST(formation AS TEXT) ELSE hex(formation) END) ELSE formation END, 'fleet', CASE WHEN typeof(fleet) = 'blob' THEN (CASE WHEN json_valid(CAST(fleet AS TEXT)) THEN CAST(fleet AS TEXT) ELSE hex(fleet) END) ELSE fleet END)) AS TEXT) FROM chapter_elite_fleets WHERE chapter_elite_fleets.commander_id = sqlc.arg(id);

-- name: AdminDeleteChapterEliteFleets :exec
DELETE FROM chapter_elite_fleets WHERE chapter_elite_fleets.commander_id = sqlc.arg(id);

-- name: AdminExportChapterProgress :one
SELECT CAST(json_group_array(json_object('commander_id', CASE WHEN typeof(commander_id) = 'blob' THEN (CASE WHEN json_valid(CAST(commander_id AS TEXT)) THEN CAST(commander_id AS TEXT) ELSE hex(commander_id) END) ELSE commander_id END, 'chapter_id', CASE WHEN typeof(chapter_id) = 'blob' THEN (CASE WHEN json_valid(CAST(chapter_id AS TEXT)) THEN CAST(chapter_id AS TEXT) ELSE hex(chapter_id) END) ELSE chapter_id END, 'progress', CASE WHEN typeof(progress) = 'blob' THEN (CASE WHEN json_valid(CAST(progress AS TEXT)) THEN CAST(progress AS TEXT) ELSE hex(progress) END) ELSE progress END, 'kill_boss_count', CASE WHEN typeof(kill_boss_count) = 'blob' THEN (CASE WHEN json_valid(CAST(kill_boss_count AS TEXT)) THEN CAST(kill_boss_count AS TEXT) ELSE hex(kill_boss_count) END) ELSE kill_boss_count END, 'kill_enemy_count', CASE WHEN typeof(kill_enemy_count) = 'blob' THEN (CASE WHEN json_valid(CAST(kill_enemy_count AS TEXT)) THEN CAST(kill_enemy_count AS TEXT) ELSE hex(kill_enemy_count) END) ELSE kill_enemy_count END, 'take_box_count', CASE WHEN typeof(take_box_count) = 'blob' THEN (CASE WHEN json_valid(CAST(take_box_count AS TEXT)) THEN CAST(take_box_count AS TEXT) ELSE hex(take_box_count) END) ELSE take_box_count END, 'defeat_count', CASE WHEN typeof(defeat_count) = 'blob' THEN (CASE WHEN json_valid(CAST(defeat_count AS TEXT)) THEN CAST(defeat_count AS TEXT) ELSE hex(defeat_count) END) ELSE defeat_count END, 'today_defeat_count', CASE WHEN typeof(today_defeat_count) = 'blob' THEN (CASE WHEN json_valid(CAST(today_defeat_count AS TEXT)) THEN CAST(today_defeat_count AS TEXT) ELSE hex(today_defeat_count) END) ELSE today_defeat_count END, 'pass_count', CASE WHEN typeof(pass_count) = 'blob' THEN (CASE WHEN json_valid(CAST(pass_count AS TEXT)) THEN CAST(pass_count AS TEXT) ELSE hex(pass_count) END) ELSE pass_count END)) AS TEXT) FROM chapter_progress WHERE chapter_progress.commander_id = sqlc.arg(id);

-- name: AdminDeleteChapterProgress :exec
DELETE FROM chapter_progress WHERE chapter_progress.commander_id = sqlc.arg(id);

-- name: AdminExportChapterStates :one
SELECT CAST(json_group_array(json_object('commander_id', CASE WHEN typeof(commander_id) = 'blob' THEN (CASE WHEN json_valid(CAST(commander_id AS TEXT)) THEN CAST(commander_id AS TEXT) ELSE hex(commander_id) END) ELSE commander_id END, 'state', CASE WHEN typeof(state) = 'blob' THEN (CASE WHEN json_valid(CAST(state AS TEXT)) THEN CAST(state AS TEXT) ELSE hex(state) END) ELSE state END, 'ambushed', CASE WHEN typeof(ambushed) = 'blob' THEN (CASE WHEN json_valid(CAST(ambushed AS TEXT)) THEN CAST(ambushed AS TEXT) ELSE hex(ambushed) END) ELSE ambushed END)) AS TEXT) FROM chapter_states WHERE chapter_states.commander_id = sqlc.arg(id);

-- name: AdminDeleteChapterStates :exec
DELETE FROM chapter_states WHERE chapter_states.commander_id = sqlc.arg(id);

-- name: AdminExportChargeExp :one
SELECT CAST(json_group_array(json_object('commander_id', CASE WHEN typeof(commander_id) = 'blob' THEN (CASE WHEN json_valid(CAST(commander_id AS TEXT)) THEN CAST(commander_id AS TEXT) ELSE hex(commander_id) END) ELSE commander_id END, 'exp', CASE WHEN typeof(exp) = 'blob' THEN (CASE WHEN json_valid(CAST(exp AS TEXT)) THEN CAST(exp AS TEXT) ELSE hex(exp) END) ELSE exp END, 'taps', CASE WHEN typeof(taps) = 'blob' THEN (CASE WHEN json_valid(CAST(taps AS TEXT)) THEN CAST(taps AS TEXT) ELSE hex(taps) END) ELSE taps END, 'tapped_at', CASE WHEN typeof(tapped_at) = 'blob' THEN (CASE WHEN json_valid(CAST(tapped_at AS TEXT)) THEN CAST(tapped_at AS TEXT) ELSE hex(tapped_at) END) ELSE tapped_at END)) AS TEXT) FROM charge_exp WHERE charge_exp.commander_id = sqlc.arg(id);

-- name: AdminDeleteChargeExp :exec
DELETE FROM charge_exp WHERE charge_exp.commander_id = sqlc.arg(id);

-- name: AdminExportCheaterMarks :one
SELECT CAST(json_group_array(json_object('id', CASE WHEN typeof(id) = 'blob' THEN (CASE WHEN json_valid(CAST(id AS TEXT)) THEN CAST(id AS TEXT) ELSE hex(id) END) ELSE id END, 'commander_id', CASE WHEN typeof(commander_id) = 'blob' THEN (CASE WHEN json_valid(CAST(commander_id AS TEXT)) THEN CAST(commander_id AS TEXT) ELSE hex(commander_id) END) ELSE commander_id END, 'account_id', CASE WHEN typeof(account_id) = 'blob' THEN (CASE WHEN json_valid(CAST(account_id AS TEXT)) THEN CAST(account_id AS TEXT) ELSE hex(account_id) END) ELSE account_id END, 'reason', CASE WHEN typeof(reason) = 'blob' THEN (CASE WHEN json_valid(CAST(reason AS TEXT)) THEN CAST(reason AS TEXT) ELSE hex(reason) END) ELSE reason END, 'marked_at', CASE WHEN typeof(marked_at) = 'blob' THEN (CASE WHEN json_valid(CAST(marked_at AS TEXT)) THEN CAST(marked_at AS TEXT) ELSE hex(marked_at) END) ELSE marked_at END)) AS TEXT) FROM cheater_marks WHERE cheater_marks.commander_id = sqlc.arg(id);

-- name: AdminDeleteCheaterMarks :exec
DELETE FROM cheater_marks WHERE cheater_marks.commander_id = sqlc.arg(id);

-- name: AdminExportCommanderActivities :one
SELECT CAST(json_group_array(json_object('commander_id', CASE WHEN typeof(commander_id) = 'blob' THEN (CASE WHEN json_valid(CAST(commander_id AS TEXT)) THEN CAST(commander_id AS TEXT) ELSE hex(commander_id) END) ELSE commander_id END, 'activity_id', CASE WHEN typeof(activity_id) = 'blob' THEN (CASE WHEN json_valid(CAST(activity_id AS TEXT)) THEN CAST(activity_id AS TEXT) ELSE hex(activity_id) END) ELSE activity_id END, 'data1', CASE WHEN typeof(data1) = 'blob' THEN (CASE WHEN json_valid(CAST(data1 AS TEXT)) THEN CAST(data1 AS TEXT) ELSE hex(data1) END) ELSE data1 END, 'data2', CASE WHEN typeof(data2) = 'blob' THEN (CASE WHEN json_valid(CAST(data2 AS TEXT)) THEN CAST(data2 AS TEXT) ELSE hex(data2) END) ELSE data2 END, 'data3', CASE WHEN typeof(data3) = 'blob' THEN (CASE WHEN json_valid(CAST(data3 AS TEXT)) THEN CAST(data3 AS TEXT) ELSE hex(data3) END) ELSE data3 END, 'data4', CASE WHEN typeof(data4) = 'blob' THEN (CASE WHEN json_valid(CAST(data4 AS TEXT)) THEN CAST(data4 AS TEXT) ELSE hex(data4) END) ELSE data4 END, 'data1_list', CASE WHEN typeof(data1_list) = 'blob' THEN (CASE WHEN json_valid(CAST(data1_list AS TEXT)) THEN CAST(data1_list AS TEXT) ELSE hex(data1_list) END) ELSE data1_list END, 'str_data1', CASE WHEN typeof(str_data1) = 'blob' THEN (CASE WHEN json_valid(CAST(str_data1 AS TEXT)) THEN CAST(str_data1 AS TEXT) ELSE hex(str_data1) END) ELSE str_data1 END)) AS TEXT) FROM commander_activities WHERE commander_activities.commander_id = sqlc.arg(id);

-- name: AdminDeleteCommanderActivities :exec
DELETE FROM commander_activities WHERE commander_activities.commander_id = sqlc.arg(id);

-- name: AdminExportCommanderActivityKeyValues :one
SELECT CAST(json_group_array(json_object('commander_id', CASE WHEN typeof(commander_id) = 'blob' THEN (CASE WHEN json_valid(CAST(commander_id AS TEXT)) THEN CAST(commander_id AS TEXT) ELSE hex(commander_id) END) ELSE commander_id END, 'activity_id', CASE WHEN typeof(activity_id) = 'blob' THEN (CASE WHEN json_valid(CAST(activity_id AS TEXT)) THEN CAST(activity_id AS TEXT) ELSE hex(activity_id) END) ELSE activity_id END, 'list_key', CASE WHEN typeof(list_key) = 'blob' THEN (CASE WHEN json_valid(CAST(list_key AS TEXT)) THEN CAST(list_key AS TEXT) ELSE hex(list_key) END) ELSE list_key END, 'item_key', CASE WHEN typeof(item_key) = 'blob' THEN (CASE WHEN json_valid(CAST(item_key AS TEXT)) THEN CAST(item_key AS TEXT) ELSE hex(item_key) END) ELSE item_key END, 'value', CASE WHEN typeof(value) = 'blob' THEN (CASE WHEN json_valid(CAST(value AS TEXT)) THEN CAST(value AS TEXT) ELSE hex(value) END) ELSE value END)) AS TEXT) FROM commander_activity_key_values WHERE commander_activity_key_values.commander_id = sqlc.arg(id);

-- name: AdminDeleteCommanderActivityKeyValues :exec
DELETE FROM commander_activity_key_values WHERE commander_activity_key_values.commander_id = sqlc.arg(id);

-- name: AdminExportCommanderActivityTasks :one
SELECT CAST(json_group_array(json_object('commander_id', CASE WHEN typeof(commander_id) = 'blob' THEN (CASE WHEN json_valid(CAST(commander_id AS TEXT)) THEN CAST(commander_id AS TEXT) ELSE hex(commander_id) END) ELSE commander_id END, 'act_id', CASE WHEN typeof(act_id) = 'blob' THEN (CASE WHEN json_valid(CAST(act_id AS TEXT)) THEN CAST(act_id AS TEXT) ELSE hex(act_id) END) ELSE act_id END, 'task_id', CASE WHEN typeof(task_id) = 'blob' THEN (CASE WHEN json_valid(CAST(task_id AS TEXT)) THEN CAST(task_id AS TEXT) ELSE hex(task_id) END) ELSE task_id END, 'progress', CASE WHEN typeof(progress) = 'blob' THEN (CASE WHEN json_valid(CAST(progress AS TEXT)) THEN CAST(progress AS TEXT) ELSE hex(progress) END) ELSE progress END, 'finished', CASE WHEN typeof(finished) = 'blob' THEN (CASE WHEN json_valid(CAST(finished AS TEXT)) THEN CAST(finished AS TEXT) ELSE hex(finished) END) ELSE finished END)) AS TEXT) FROM commander_activity_tasks WHERE commander_activity_tasks.commander_id = sqlc.arg(id);

-- name: AdminDeleteCommanderActivityTasks :exec
DELETE FROM commander_activity_tasks WHERE commander_activity_tasks.commander_id = sqlc.arg(id);

-- name: AdminExportCommanderApartment :one
SELECT CAST(json_group_array(json_object('commander_id', CASE WHEN typeof(commander_id) = 'blob' THEN (CASE WHEN json_valid(CAST(commander_id AS TEXT)) THEN CAST(commander_id AS TEXT) ELSE hex(commander_id) END) ELSE commander_id END, 'vigor_used', CASE WHEN typeof(vigor_used) = 'blob' THEN (CASE WHEN json_valid(CAST(vigor_used AS TEXT)) THEN CAST(vigor_used AS TEXT) ELSE hex(vigor_used) END) ELSE vigor_used END, 'vigor_day', CASE WHEN typeof(vigor_day) = 'blob' THEN (CASE WHEN json_valid(CAST(vigor_day AS TEXT)) THEN CAST(vigor_day AS TEXT) ELSE hex(vigor_day) END) ELSE vigor_day END, 'visit_day', CASE WHEN typeof(visit_day) = 'blob' THEN (CASE WHEN json_valid(CAST(visit_day AS TEXT)) THEN CAST(visit_day AS TEXT) ELSE hex(visit_day) END) ELSE visit_day END)) AS TEXT) FROM commander_apartment WHERE commander_apartment.commander_id = sqlc.arg(id);

-- name: AdminDeleteCommanderApartment :exec
DELETE FROM commander_apartment WHERE commander_apartment.commander_id = sqlc.arg(id);

-- name: AdminExportCommanderApartmentChats :one
SELECT CAST(json_group_array(json_object('commander_id', CASE WHEN typeof(commander_id) = 'blob' THEN (CASE WHEN json_valid(CAST(commander_id AS TEXT)) THEN CAST(commander_id AS TEXT) ELSE hex(commander_id) END) ELSE commander_id END, 'ship_group', CASE WHEN typeof(ship_group) = 'blob' THEN (CASE WHEN json_valid(CAST(ship_group AS TEXT)) THEN CAST(ship_group AS TEXT) ELSE hex(ship_group) END) ELSE ship_group END, 'care_flag', CASE WHEN typeof(care_flag) = 'blob' THEN (CASE WHEN json_valid(CAST(care_flag AS TEXT)) THEN CAST(care_flag AS TEXT) ELSE hex(care_flag) END) ELSE care_flag END, 'cur_back', CASE WHEN typeof(cur_back) = 'blob' THEN (CASE WHEN json_valid(CAST(cur_back AS TEXT)) THEN CAST(cur_back AS TEXT) ELSE hex(cur_back) END) ELSE cur_back END, 'cur_comm_id', CASE WHEN typeof(cur_comm_id) = 'blob' THEN (CASE WHEN json_valid(CAST(cur_comm_id AS TEXT)) THEN CAST(cur_comm_id AS TEXT) ELSE hex(cur_comm_id) END) ELSE cur_comm_id END)) AS TEXT) FROM commander_apartment_chats WHERE commander_apartment_chats.commander_id = sqlc.arg(id);

-- name: AdminDeleteCommanderApartmentChats :exec
DELETE FROM commander_apartment_chats WHERE commander_apartment_chats.commander_id = sqlc.arg(id);

-- name: AdminExportCommanderApartmentEvents :one
SELECT CAST(json_group_array(json_object('commander_id', CASE WHEN typeof(commander_id) = 'blob' THEN (CASE WHEN json_valid(CAST(commander_id AS TEXT)) THEN CAST(commander_id AS TEXT) ELSE hex(commander_id) END) ELSE commander_id END, 'ship_group', CASE WHEN typeof(ship_group) = 'blob' THEN (CASE WHEN json_valid(CAST(ship_group AS TEXT)) THEN CAST(ship_group AS TEXT) ELSE hex(ship_group) END) ELSE ship_group END, 'event_type', CASE WHEN typeof(event_type) = 'blob' THEN (CASE WHEN json_valid(CAST(event_type AS TEXT)) THEN CAST(event_type AS TEXT) ELSE hex(event_type) END) ELSE event_type END, 'value', CASE WHEN typeof(value) = 'blob' THEN (CASE WHEN json_valid(CAST(value AS TEXT)) THEN CAST(value AS TEXT) ELSE hex(value) END) ELSE value END)) AS TEXT) FROM commander_apartment_events WHERE commander_apartment_events.commander_id = sqlc.arg(id);

-- name: AdminDeleteCommanderApartmentEvents :exec
DELETE FROM commander_apartment_events WHERE commander_apartment_events.commander_id = sqlc.arg(id);

-- name: AdminExportCommanderApartmentInsContents :one
SELECT CAST(json_group_array(json_object('commander_id', CASE WHEN typeof(commander_id) = 'blob' THEN (CASE WHEN json_valid(CAST(commander_id AS TEXT)) THEN CAST(commander_id AS TEXT) ELSE hex(commander_id) END) ELSE commander_id END, 'type', CASE WHEN typeof(type) = 'blob' THEN (CASE WHEN json_valid(CAST(type AS TEXT)) THEN CAST(type AS TEXT) ELSE hex(type) END) ELSE type END, 'content_id', CASE WHEN typeof(content_id) = 'blob' THEN (CASE WHEN json_valid(CAST(content_id AS TEXT)) THEN CAST(content_id AS TEXT) ELSE hex(content_id) END) ELSE content_id END, 'ship_group', CASE WHEN typeof(ship_group) = 'blob' THEN (CASE WHEN json_valid(CAST(ship_group AS TEXT)) THEN CAST(ship_group AS TEXT) ELSE hex(ship_group) END) ELSE ship_group END, 'time', CASE WHEN typeof(time) = 'blob' THEN (CASE WHEN json_valid(CAST(time AS TEXT)) THEN CAST(time AS TEXT) ELSE hex(time) END) ELSE time END, 'read_flag', CASE WHEN typeof(read_flag) = 'blob' THEN (CASE WHEN json_valid(CAST(read_flag AS TEXT)) THEN CAST(read_flag AS TEXT) ELSE hex(read_flag) END) ELSE read_flag END, 'good_flag', CASE WHEN typeof(good_flag) = 'blob' THEN (CASE WHEN json_valid(CAST(good_flag AS TEXT)) THEN CAST(good_flag AS TEXT) ELSE hex(good_flag) END) ELSE good_flag END, 'exit_time', CASE WHEN typeof(exit_time) = 'blob' THEN (CASE WHEN json_valid(CAST(exit_time AS TEXT)) THEN CAST(exit_time AS TEXT) ELSE hex(exit_time) END) ELSE exit_time END)) AS TEXT) FROM commander_apartment_ins_contents WHERE commander_apartment_ins_contents.commander_id = sqlc.arg(id);

-- name: AdminDeleteCommanderApartmentInsContents :exec
DELETE FROM commander_apartment_ins_contents WHERE commander_apartment_ins_contents.commander_id = sqlc.arg(id);

-- name: AdminExportCommanderApartmentInsReplies :one
SELECT CAST(json_group_array(json_object('commander_id', CASE WHEN typeof(commander_id) = 'blob' THEN (CASE WHEN json_valid(CAST(commander_id AS TEXT)) THEN CAST(commander_id AS TEXT) ELSE hex(commander_id) END) ELSE commander_id END, 'type', CASE WHEN typeof(type) = 'blob' THEN (CASE WHEN json_valid(CAST(type AS TEXT)) THEN CAST(type AS TEXT) ELSE hex(type) END) ELSE type END, 'content_id', CASE WHEN typeof(content_id) = 'blob' THEN (CASE WHEN json_valid(CAST(content_id AS TEXT)) THEN CAST(content_id AS TEXT) ELSE hex(content_id) END) ELSE content_id END, 'reply_key', CASE WHEN typeof(reply_key) = 'blob' THEN (CASE WHEN json_valid(CAST(reply_key AS TEXT)) THEN CAST(reply_key AS TEXT) ELSE hex(reply_key) END) ELSE reply_key END, 'value', CASE WHEN typeof(value) = 'blob' THEN (CASE WHEN json_valid(CAST(value AS TEXT)) THEN CAST(value AS TEXT) ELSE hex(value) END) ELSE value END, 'time', CASE WHEN typeof(time) = 'blob' THEN (CASE WHEN json_valid(CAST(time AS TEXT)) THEN CAST(time AS TEXT) ELSE hex(time) END) ELSE time END)) AS TEXT) FROM commander_apartment_ins_replies WHERE commander_apartment_ins_replies.commander_id = sqlc.arg(id);

-- name: AdminDeleteCommanderApartmentInsReplies :exec
DELETE FROM commander_apartment_ins_replies WHERE commander_apartment_ins_replies.commander_id = sqlc.arg(id);

-- name: AdminExportCommanderAppearance :one
SELECT CAST(json_group_array(json_object('commander_id', CASE WHEN typeof(commander_id) = 'blob' THEN (CASE WHEN json_valid(CAST(commander_id AS TEXT)) THEN CAST(commander_id AS TEXT) ELSE hex(commander_id) END) ELSE commander_id END, 'icon_frame', CASE WHEN typeof(icon_frame) = 'blob' THEN (CASE WHEN json_valid(CAST(icon_frame AS TEXT)) THEN CAST(icon_frame AS TEXT) ELSE hex(icon_frame) END) ELSE icon_frame END, 'chat_frame', CASE WHEN typeof(chat_frame) = 'blob' THEN (CASE WHEN json_valid(CAST(chat_frame AS TEXT)) THEN CAST(chat_frame AS TEXT) ELSE hex(chat_frame) END) ELSE chat_frame END, 'battle_ui', CASE WHEN typeof(battle_ui) = 'blob' THEN (CASE WHEN json_valid(CAST(battle_ui AS TEXT)) THEN CAST(battle_ui AS TEXT) ELSE hex(battle_ui) END) ELSE battle_ui END, 'cover', CASE WHEN typeof(cover) = 'blob' THEN (CASE WHEN json_valid(CAST(cover AS TEXT)) THEN CAST(cover AS TEXT) ELSE hex(cover) END) ELSE cover END, 'loading_pic_open', CASE WHEN typeof(loading_pic_open) = 'blob' THEN (CASE WHEN json_valid(CAST(loading_pic_open AS TEXT)) THEN CAST(loading_pic_open AS TEXT) ELSE hex(loading_pic_open) END) ELSE loading_pic_open END, 'gallery_pics', CASE WHEN typeof(gallery_pics) = 'blob' THEN (CASE WHEN json_valid(CAST(gallery_pics AS TEXT)) THEN CAST(gallery_pics AS TEXT) ELSE hex(gallery_pics) END) ELSE gallery_pics END, 'manga_pics', CASE WHEN typeof(manga_pics) = 'blob' THEN (CASE WHEN json_valid(CAST(manga_pics AS TEXT)) THEN CAST(manga_pics AS TEXT) ELSE hex(manga_pics) END) ELSE manga_pics END)) AS TEXT) FROM commander_appearance WHERE commander_appearance.commander_id = sqlc.arg(id);

-- name: AdminDeleteCommanderAppearance :exec
DELETE FROM commander_appearance WHERE commander_appearance.commander_id = sqlc.arg(id);

-- name: AdminExportCommanderAppreciation :one
SELECT CAST(json_group_array(json_object('commander_id', CASE WHEN typeof(commander_id) = 'blob' THEN (CASE WHEN json_valid(CAST(commander_id AS TEXT)) THEN CAST(commander_id AS TEXT) ELSE hex(commander_id) END) ELSE commander_id END, 'kind', CASE WHEN typeof(kind) = 'blob' THEN (CASE WHEN json_valid(CAST(kind AS TEXT)) THEN CAST(kind AS TEXT) ELSE hex(kind) END) ELSE kind END, 'item_id', CASE WHEN typeof(item_id) = 'blob' THEN (CASE WHEN json_valid(CAST(item_id AS TEXT)) THEN CAST(item_id AS TEXT) ELSE hex(item_id) END) ELSE item_id END)) AS TEXT) FROM commander_appreciation WHERE commander_appreciation.commander_id = sqlc.arg(id);

-- name: AdminDeleteCommanderAppreciation :exec
DELETE FROM commander_appreciation WHERE commander_appreciation.commander_id = sqlc.arg(id);

-- name: AdminExportCommanderAtelierItems :one
SELECT CAST(json_group_array(json_object('commander_id', CASE WHEN typeof(commander_id) = 'blob' THEN (CASE WHEN json_valid(CAST(commander_id AS TEXT)) THEN CAST(commander_id AS TEXT) ELSE hex(commander_id) END) ELSE commander_id END, 'item_id', CASE WHEN typeof(item_id) = 'blob' THEN (CASE WHEN json_valid(CAST(item_id AS TEXT)) THEN CAST(item_id AS TEXT) ELSE hex(item_id) END) ELSE item_id END, 'count', CASE WHEN typeof(count) = 'blob' THEN (CASE WHEN json_valid(CAST(count AS TEXT)) THEN CAST(count AS TEXT) ELSE hex(count) END) ELSE count END)) AS TEXT) FROM commander_atelier_items WHERE commander_atelier_items.commander_id = sqlc.arg(id);

-- name: AdminDeleteCommanderAtelierItems :exec
DELETE FROM commander_atelier_items WHERE commander_atelier_items.commander_id = sqlc.arg(id);

-- name: AdminExportCommanderAtelierRecipes :one
SELECT CAST(json_group_array(json_object('commander_id', CASE WHEN typeof(commander_id) = 'blob' THEN (CASE WHEN json_valid(CAST(commander_id AS TEXT)) THEN CAST(commander_id AS TEXT) ELSE hex(commander_id) END) ELSE commander_id END, 'recipe_id', CASE WHEN typeof(recipe_id) = 'blob' THEN (CASE WHEN json_valid(CAST(recipe_id AS TEXT)) THEN CAST(recipe_id AS TEXT) ELSE hex(recipe_id) END) ELSE recipe_id END, 'used', CASE WHEN typeof(used) = 'blob' THEN (CASE WHEN json_valid(CAST(used AS TEXT)) THEN CAST(used AS TEXT) ELSE hex(used) END) ELSE used END)) AS TEXT) FROM commander_atelier_recipes WHERE commander_atelier_recipes.commander_id = sqlc.arg(id);

-- name: AdminDeleteCommanderAtelierRecipes :exec
DELETE FROM commander_atelier_recipes WHERE commander_atelier_recipes.commander_id = sqlc.arg(id);

-- name: AdminExportCommanderAtelierSlots :one
SELECT CAST(json_group_array(json_object('commander_id', CASE WHEN typeof(commander_id) = 'blob' THEN (CASE WHEN json_valid(CAST(commander_id AS TEXT)) THEN CAST(commander_id AS TEXT) ELSE hex(commander_id) END) ELSE commander_id END, 'pos', CASE WHEN typeof(pos) = 'blob' THEN (CASE WHEN json_valid(CAST(pos AS TEXT)) THEN CAST(pos AS TEXT) ELSE hex(pos) END) ELSE pos END, 'item_id', CASE WHEN typeof(item_id) = 'blob' THEN (CASE WHEN json_valid(CAST(item_id AS TEXT)) THEN CAST(item_id AS TEXT) ELSE hex(item_id) END) ELSE item_id END, 'item_num', CASE WHEN typeof(item_num) = 'blob' THEN (CASE WHEN json_valid(CAST(item_num AS TEXT)) THEN CAST(item_num AS TEXT) ELSE hex(item_num) END) ELSE item_num END)) AS TEXT) FROM commander_atelier_slots WHERE commander_atelier_slots.commander_id = sqlc.arg(id);

-- name: AdminDeleteCommanderAtelierSlots :exec
DELETE FROM commander_atelier_slots WHERE commander_atelier_slots.commander_id = sqlc.arg(id);

-- name: AdminExportCommanderAuction :one
SELECT CAST(json_group_array(json_object('commander_id', CASE WHEN typeof(commander_id) = 'blob' THEN (CASE WHEN json_valid(CAST(commander_id AS TEXT)) THEN CAST(commander_id AS TEXT) ELSE hex(commander_id) END) ELSE commander_id END, 'activity_id', CASE WHEN typeof(activity_id) = 'blob' THEN (CASE WHEN json_valid(CAST(activity_id AS TEXT)) THEN CAST(activity_id AS TEXT) ELSE hex(activity_id) END) ELSE activity_id END, 'gold', CASE WHEN typeof(gold) = 'blob' THEN (CASE WHEN json_valid(CAST(gold AS TEXT)) THEN CAST(gold AS TEXT) ELSE hex(gold) END) ELSE gold END, 'game_num', CASE WHEN typeof(game_num) = 'blob' THEN (CASE WHEN json_valid(CAST(game_num AS TEXT)) THEN CAST(game_num AS TEXT) ELSE hex(game_num) END) ELSE game_num END, 'buy_num', CASE WHEN typeof(buy_num) = 'blob' THEN (CASE WHEN json_valid(CAST(buy_num AS TEXT)) THEN CAST(buy_num AS TEXT) ELSE hex(buy_num) END) ELSE buy_num END, 'max_profit', CASE WHEN typeof(max_profit) = 'blob' THEN (CASE WHEN json_valid(CAST(max_profit AS TEXT)) THEN CAST(max_profit AS TEXT) ELSE hex(max_profit) END) ELSE max_profit END, 'acc_profit', CASE WHEN typeof(acc_profit) = 'blob' THEN (CASE WHEN json_valid(CAST(acc_profit AS TEXT)) THEN CAST(acc_profit AS TEXT) ELSE hex(acc_profit) END) ELSE acc_profit END, 'acc_buy_price', CASE WHEN typeof(acc_buy_price) = 'blob' THEN (CASE WHEN json_valid(CAST(acc_buy_price AS TEXT)) THEN CAST(acc_buy_price AS TEXT) ELSE hex(acc_buy_price) END) ELSE acc_buy_price END, 'acc_item_price', CASE WHEN typeof(acc_item_price) = 'blob' THEN (CASE WHEN json_valid(CAST(acc_item_price AS TEXT)) THEN CAST(acc_item_price AS TEXT) ELSE hex(acc_item_price) END) ELSE acc_item_price END, 'max_lot_value', CASE WHEN typeof(max_lot_value) = 'blob' THEN (CASE WHEN json_valid(CAST(max_lot_value AS TEXT)) THEN CAST(max_lot_value AS TEXT) ELSE hex(max_lot_value) END) ELSE max_lot_value END, 'item_list', CASE WHEN typeof(item_list) = 'blob' THEN (CASE WHEN json_valid(CAST(item_list AS TEXT)) THEN CAST(item_list AS TEXT) ELSE hex(item_list) END) ELSE item_list END, 'pre_buy_state', CASE WHEN typeof(pre_buy_state) = 'blob' THEN (CASE WHEN json_valid(CAST(pre_buy_state AS TEXT)) THEN CAST(pre_buy_state AS TEXT) ELSE hex(pre_buy_state) END) ELSE pre_buy_state END, 'pre_timestamp', CASE WHEN typeof(pre_timestamp) = 'blob' THEN (CASE WHEN json_valid(CAST(pre_timestamp AS TEXT)) THEN CAST(pre_timestamp AS TEXT) ELSE hex(pre_timestamp) END) ELSE pre_timestamp END, 'pre_box', CASE WHEN typeof(pre_box) = 'blob' THEN (CASE WHEN json_valid(CAST(pre_box AS TEXT)) THEN CAST(pre_box AS TEXT) ELSE hex(pre_box) END) ELSE pre_box END, 'pre_line', CASE WHEN typeof(pre_line) = 'blob' THEN (CASE WHEN json_valid(CAST(pre_line AS TEXT)) THEN CAST(pre_line AS TEXT) ELSE hex(pre_line) END) ELSE pre_line END, 'match_time', CASE WHEN typeof(match_time) = 'blob' THEN (CASE WHEN json_valid(CAST(match_time AS TEXT)) THEN CAST(match_time AS TEXT) ELSE hex(match_time) END) ELSE match_time END, 'is_forbidden', CASE WHEN typeof(is_forbidden) = 'blob' THEN (CASE WHEN json_valid(CAST(is_forbidden AS TEXT)) THEN CAST(is_forbidden AS TEXT) ELSE hex(is_forbidden) END) ELSE is_forbidden END, 'back_forbidden', CASE WHEN typeof(back_forbidden) = 'blob' THEN (CASE WHEN json_valid(CAST(back_forbidden AS TEXT)) THEN CAST(back_forbidden AS TEXT) ELSE hex(back_forbidden) END) ELSE back_forbidden END, 'inactive_num', CASE WHEN typeof(inactive_num) = 'blob' THEN (CASE WHEN json_valid(CAST(inactive_num AS TEXT)) THEN CAST(inactive_num AS TEXT) ELSE hex(inactive_num) END) ELSE inactive_num END, 'inactive_state', CASE WHEN typeof(inactive_state) = 'blob' THEN (CASE WHEN json_valid(CAST(inactive_state AS TEXT)) THEN CAST(inactive_state AS TEXT) ELSE hex(inactive_state) END) ELSE inactive_state END, 'get_relief_num', CASE WHEN typeof(get_relief_num) = 'blob' THEN (CASE WHEN json_valid(CAST(get_relief_num AS TEXT)) THEN CAST(get_relief_num AS TEXT) ELSE hex(get_relief_num) END) ELSE get_relief_num END, 'reset_day', CASE WHEN typeof(reset_day) = 'blob' THEN (CASE WHEN json_valid(CAST(reset_day AS TEXT)) THEN CAST(reset_day AS TEXT) ELSE hex(reset_day) END) ELSE reset_day END)) AS TEXT) FROM commander_auction WHERE commander_auction.commander_id = sqlc.arg(id);

-- name: AdminDeleteCommanderAuction :exec
DELETE FROM commander_auction WHERE commander_auction.commander_id = sqlc.arg(id);

-- name: AdminExportCommanderBlueprintState :one
SELECT CAST(json_group_array(json_object('commander_id', CASE WHEN typeof(commander_id) = 'blob' THEN (CASE WHEN json_valid(CAST(commander_id AS TEXT)) THEN CAST(commander_id AS TEXT) ELSE hex(commander_id) END) ELSE commander_id END, 'cold_time', CASE WHEN typeof(cold_time) = 'blob' THEN (CASE WHEN json_valid(CAST(cold_time AS TEXT)) THEN CAST(cold_time AS TEXT) ELSE hex(cold_time) END) ELSE cold_time END, 'pursue_day', CASE WHEN typeof(pursue_day) = 'blob' THEN (CASE WHEN json_valid(CAST(pursue_day AS TEXT)) THEN CAST(pursue_day AS TEXT) ELSE hex(pursue_day) END) ELSE pursue_day END, 'pursued', CASE WHEN typeof(pursued) = 'blob' THEN (CASE WHEN json_valid(CAST(pursued AS TEXT)) THEN CAST(pursued AS TEXT) ELSE hex(pursued) END) ELSE pursued END, 'pursued_ur', CASE WHEN typeof(pursued_ur) = 'blob' THEN (CASE WHEN json_valid(CAST(pursued_ur AS TEXT)) THEN CAST(pursued_ur AS TEXT) ELSE hex(pursued_ur) END) ELSE pursued_ur END)) AS TEXT) FROM commander_blueprint_state WHERE commander_blueprint_state.commander_id = sqlc.arg(id);

-- name: AdminDeleteCommanderBlueprintState :exec
DELETE FROM commander_blueprint_state WHERE commander_blueprint_state.commander_id = sqlc.arg(id);

-- name: AdminExportCommanderBlueprints :one
SELECT CAST(json_group_array(json_object('commander_id', CASE WHEN typeof(commander_id) = 'blob' THEN (CASE WHEN json_valid(CAST(commander_id AS TEXT)) THEN CAST(commander_id AS TEXT) ELSE hex(commander_id) END) ELSE commander_id END, 'blueprint_id', CASE WHEN typeof(blueprint_id) = 'blob' THEN (CASE WHEN json_valid(CAST(blueprint_id AS TEXT)) THEN CAST(blueprint_id AS TEXT) ELSE hex(blueprint_id) END) ELSE blueprint_id END, 'ship_id', CASE WHEN typeof(ship_id) = 'blob' THEN (CASE WHEN json_valid(CAST(ship_id AS TEXT)) THEN CAST(ship_id AS TEXT) ELSE hex(ship_id) END) ELSE ship_id END, 'start_time', CASE WHEN typeof(start_time) = 'blob' THEN (CASE WHEN json_valid(CAST(start_time AS TEXT)) THEN CAST(start_time AS TEXT) ELSE hex(start_time) END) ELSE start_time END, 'duration', CASE WHEN typeof(duration) = 'blob' THEN (CASE WHEN json_valid(CAST(duration AS TEXT)) THEN CAST(duration AS TEXT) ELSE hex(duration) END) ELSE duration END)) AS TEXT) FROM commander_blueprints WHERE commander_blueprints.commander_id = sqlc.arg(id);

-- name: AdminDeleteCommanderBlueprints :exec
DELETE FROM commander_blueprints WHERE commander_blueprints.commander_id = sqlc.arg(id);

-- name: AdminExportCommanderBuildStates :one
SELECT CAST(json_group_array(json_object('commander_id', CASE WHEN typeof(commander_id) = 'blob' THEN (CASE WHEN json_valid(CAST(commander_id AS TEXT)) THEN CAST(commander_id AS TEXT) ELSE hex(commander_id) END) ELSE commander_id END, 'exchange_count', CASE WHEN typeof(exchange_count) = 'blob' THEN (CASE WHEN json_valid(CAST(exchange_count AS TEXT)) THEN CAST(exchange_count AS TEXT) ELSE hex(exchange_count) END) ELSE exchange_count END)) AS TEXT) FROM commander_build_states WHERE commander_build_states.commander_id = sqlc.arg(id);

-- name: AdminDeleteCommanderBuildStates :exec
DELETE FROM commander_build_states WHERE commander_build_states.commander_id = sqlc.arg(id);

-- name: AdminExportCommanderCityRebuild :one
SELECT CAST(json_group_array(json_object('commander_id', CASE WHEN typeof(commander_id) = 'blob' THEN (CASE WHEN json_valid(CAST(commander_id AS TEXT)) THEN CAST(commander_id AS TEXT) ELSE hex(commander_id) END) ELSE commander_id END, 'activity_id', CASE WHEN typeof(activity_id) = 'blob' THEN (CASE WHEN json_valid(CAST(activity_id AS TEXT)) THEN CAST(activity_id AS TEXT) ELSE hex(activity_id) END) ELSE activity_id END, 'pt', CASE WHEN typeof(pt) = 'blob' THEN (CASE WHEN json_valid(CAST(pt AS TEXT)) THEN CAST(pt AS TEXT) ELSE hex(pt) END) ELSE pt END, 'summary_pt', CASE WHEN typeof(summary_pt) = 'blob' THEN (CASE WHEN json_valid(CAST(summary_pt AS TEXT)) THEN CAST(summary_pt AS TEXT) ELSE hex(summary_pt) END) ELSE summary_pt END, 'max_level', CASE WHEN typeof(max_level) = 'blob' THEN (CASE WHEN json_valid(CAST(max_level AS TEXT)) THEN CAST(max_level AS TEXT) ELSE hex(max_level) END) ELSE max_level END, 'cur_level', CASE WHEN typeof(cur_level) = 'blob' THEN (CASE WHEN json_valid(CAST(cur_level AS TEXT)) THEN CAST(cur_level AS TEXT) ELSE hex(cur_level) END) ELSE cur_level END, 'max_display', CASE WHEN typeof(max_display) = 'blob' THEN (CASE WHEN json_valid(CAST(max_display AS TEXT)) THEN CAST(max_display AS TEXT) ELSE hex(max_display) END) ELSE max_display END, 'fight_time', CASE WHEN typeof(fight_time) = 'blob' THEN (CASE WHEN json_valid(CAST(fight_time AS TEXT)) THEN CAST(fight_time AS TEXT) ELSE hex(fight_time) END) ELSE fight_time END, 'left_hp', CASE WHEN typeof(left_hp) = 'blob' THEN (CASE WHEN json_valid(CAST(left_hp AS TEXT)) THEN CAST(left_hp AS TEXT) ELSE hex(left_hp) END) ELSE left_hp END, 'awarded_level', CASE WHEN typeof(awarded_level) = 'blob' THEN (CASE WHEN json_valid(CAST(awarded_level AS TEXT)) THEN CAST(awarded_level AS TEXT) ELSE hex(awarded_level) END) ELSE awarded_level END)) AS TEXT) FROM commander_city_rebuild WHERE commander_city_rebuild.commander_id = sqlc.arg(id);

-- name: AdminDeleteCommanderCityRebuild :exec
DELETE FROM commander_city_rebuild WHERE commander_city_rebuild.commander_id = sqlc.arg(id);

-- name: AdminExportCommanderCityRebuildBuffs :one
SELECT CAST(json_group_array(json_object('commander_id', CASE WHEN typeof(commander_id) = 'blob' THEN (CASE WHEN json_valid(CAST(commander_id AS TEXT)) THEN CAST(commander_id AS TEXT) ELSE hex(commander_id) END) ELSE commander_id END, 'activity_id', CASE WHEN typeof(activity_id) = 'blob' THEN (CASE WHEN json_valid(CAST(activity_id AS TEXT)) THEN CAST(activity_id AS TEXT) ELSE hex(activity_id) END) ELSE activity_id END, 'buff_group', CASE WHEN typeof(buff_group) = 'blob' THEN (CASE WHEN json_valid(CAST(buff_group AS TEXT)) THEN CAST(buff_group AS TEXT) ELSE hex(buff_group) END) ELSE buff_group END, 'level', CASE WHEN typeof(level) = 'blob' THEN (CASE WHEN json_valid(CAST(level AS TEXT)) THEN CAST(level AS TEXT) ELSE hex(level) END) ELSE level END)) AS TEXT) FROM commander_city_rebuild_buffs WHERE commander_city_rebuild_buffs.commander_id = sqlc.arg(id);

-- name: AdminDeleteCommanderCityRebuildBuffs :exec
DELETE FROM commander_city_rebuild_buffs WHERE commander_city_rebuild_buffs.commander_id = sqlc.arg(id);

-- name: AdminExportCommanderCityRebuildUnits :one
SELECT CAST(json_group_array(json_object('commander_id', CASE WHEN typeof(commander_id) = 'blob' THEN (CASE WHEN json_valid(CAST(commander_id AS TEXT)) THEN CAST(commander_id AS TEXT) ELSE hex(commander_id) END) ELSE commander_id END, 'activity_id', CASE WHEN typeof(activity_id) = 'blob' THEN (CASE WHEN json_valid(CAST(activity_id AS TEXT)) THEN CAST(activity_id AS TEXT) ELSE hex(activity_id) END) ELSE activity_id END, 'unit_id', CASE WHEN typeof(unit_id) = 'blob' THEN (CASE WHEN json_valid(CAST(unit_id AS TEXT)) THEN CAST(unit_id AS TEXT) ELSE hex(unit_id) END) ELSE unit_id END, 'done', CASE WHEN typeof(done) = 'blob' THEN (CASE WHEN json_valid(CAST(done AS TEXT)) THEN CAST(done AS TEXT) ELSE hex(done) END) ELSE done END, 'start_time', CASE WHEN typeof(start_time) = 'blob' THEN (CASE WHEN json_valid(CAST(start_time AS TEXT)) THEN CAST(start_time AS TEXT) ELSE hex(start_time) END) ELSE start_time END)) AS TEXT) FROM commander_city_rebuild_units WHERE commander_city_rebuild_units.commander_id = sqlc.arg(id);

-- name: AdminDeleteCommanderCityRebuildUnits :exec
DELETE FROM commander_city_rebuild_units WHERE commander_city_rebuild_units.commander_id = sqlc.arg(id);

-- name: AdminExportCommanderCollectionAwards :one
SELECT CAST(json_group_array(json_object('commander_id', CASE WHEN typeof(commander_id) = 'blob' THEN (CASE WHEN json_valid(CAST(commander_id AS TEXT)) THEN CAST(commander_id AS TEXT) ELSE hex(commander_id) END) ELSE commander_id END, 'storeup_id', CASE WHEN typeof(storeup_id) = 'blob' THEN (CASE WHEN json_valid(CAST(storeup_id AS TEXT)) THEN CAST(storeup_id AS TEXT) ELSE hex(storeup_id) END) ELSE storeup_id END, 'award_index', CASE WHEN typeof(award_index) = 'blob' THEN (CASE WHEN json_valid(CAST(award_index AS TEXT)) THEN CAST(award_index AS TEXT) ELSE hex(award_index) END) ELSE award_index END)) AS TEXT) FROM commander_collection_awards WHERE commander_collection_awards.commander_id = sqlc.arg(id);

-- name: AdminDeleteCommanderCollectionAwards :exec
DELETE FROM commander_collection_awards WHERE commander_collection_awards.commander_id = sqlc.arg(id);

-- name: AdminExportCommanderColoringAwards :one
SELECT CAST(json_group_array(json_object('commander_id', CASE WHEN typeof(commander_id) = 'blob' THEN (CASE WHEN json_valid(CAST(commander_id AS TEXT)) THEN CAST(commander_id AS TEXT) ELSE hex(commander_id) END) ELSE commander_id END, 'activity_id', CASE WHEN typeof(activity_id) = 'blob' THEN (CASE WHEN json_valid(CAST(activity_id AS TEXT)) THEN CAST(activity_id AS TEXT) ELSE hex(activity_id) END) ELSE activity_id END, 'group_id', CASE WHEN typeof(group_id) = 'blob' THEN (CASE WHEN json_valid(CAST(group_id AS TEXT)) THEN CAST(group_id AS TEXT) ELSE hex(group_id) END) ELSE group_id END, 'drops', CASE WHEN typeof(drops) = 'blob' THEN (CASE WHEN json_valid(CAST(drops AS TEXT)) THEN CAST(drops AS TEXT) ELSE hex(drops) END) ELSE drops END)) AS TEXT) FROM commander_coloring_awards WHERE commander_coloring_awards.commander_id = sqlc.arg(id);

-- name: AdminDeleteCommanderColoringAwards :exec
DELETE FROM commander_coloring_awards WHERE commander_coloring_awards.commander_id = sqlc.arg(id);

-- name: AdminExportCommanderColoringCells :one
SELECT CAST(json_group_array(json_object('commander_id', CASE WHEN typeof(commander_id) = 'blob' THEN (CASE WHEN json_valid(CAST(commander_id AS TEXT)) THEN CAST(commander_id AS TEXT) ELSE hex(commander_id) END) ELSE commander_id END, 'activity_id', CASE WHEN typeof(activity_id) = 'blob' THEN (CASE WHEN json_valid(CAST(activity_id AS TEXT)) THEN CAST(activity_id AS TEXT) ELSE hex(activity_id) END) ELSE activity_id END, 'group_id', CASE WHEN typeof(group_id) = 'blob' THEN (CASE WHEN json_valid(CAST(group_id AS TEXT)) THEN CAST(group_id AS TEXT) ELSE hex(group_id) END) ELSE group_id END, 'row', CASE WHEN typeof(row) = 'blob' THEN (CASE WHEN json_valid(CAST(row AS TEXT)) THEN CAST(row AS TEXT) ELSE hex(row) END) ELSE row END, 'col', CASE WHEN typeof(col) = 'blob' THEN (CASE WHEN json_valid(CAST(col AS TEXT)) THEN CAST(col AS TEXT) ELSE hex(col) END) ELSE col END, 'color', CASE WHEN typeof(color) = 'blob' THEN (CASE WHEN json_valid(CAST(color AS TEXT)) THEN CAST(color AS TEXT) ELSE hex(color) END) ELSE color END)) AS TEXT) FROM commander_coloring_cells WHERE commander_coloring_cells.commander_id = sqlc.arg(id);

-- name: AdminDeleteCommanderColoringCells :exec
DELETE FROM commander_coloring_cells WHERE commander_coloring_cells.commander_id = sqlc.arg(id);

-- name: AdminExportCommanderCommissionRefresh :one
SELECT CAST(json_group_array(json_object('commander_id', CASE WHEN typeof(commander_id) = 'blob' THEN (CASE WHEN json_valid(CAST(commander_id AS TEXT)) THEN CAST(commander_id AS TEXT) ELSE hex(commander_id) END) ELSE commander_id END, 'refreshed_at', CASE WHEN typeof(refreshed_at) = 'blob' THEN (CASE WHEN json_valid(CAST(refreshed_at AS TEXT)) THEN CAST(refreshed_at AS TEXT) ELSE hex(refreshed_at) END) ELSE refreshed_at END, 'daily_drawn', CASE WHEN typeof(daily_drawn) = 'blob' THEN (CASE WHEN json_valid(CAST(daily_drawn AS TEXT)) THEN CAST(daily_drawn AS TEXT) ELSE hex(daily_drawn) END) ELSE daily_drawn END)) AS TEXT) FROM commander_commission_refresh WHERE commander_commission_refresh.commander_id = sqlc.arg(id);

-- name: AdminDeleteCommanderCommissionRefresh :exec
DELETE FROM commander_commission_refresh WHERE commander_commission_refresh.commander_id = sqlc.arg(id);

-- name: AdminExportCommanderCommissionShips :one
SELECT CAST(json_group_array(json_object('commander_id', CASE WHEN typeof(commander_id) = 'blob' THEN (CASE WHEN json_valid(CAST(commander_id AS TEXT)) THEN CAST(commander_id AS TEXT) ELSE hex(commander_id) END) ELSE commander_id END, 'collection_id', CASE WHEN typeof(collection_id) = 'blob' THEN (CASE WHEN json_valid(CAST(collection_id AS TEXT)) THEN CAST(collection_id AS TEXT) ELSE hex(collection_id) END) ELSE collection_id END, 'pos', CASE WHEN typeof(pos) = 'blob' THEN (CASE WHEN json_valid(CAST(pos AS TEXT)) THEN CAST(pos AS TEXT) ELSE hex(pos) END) ELSE pos END, 'ship_id', CASE WHEN typeof(ship_id) = 'blob' THEN (CASE WHEN json_valid(CAST(ship_id AS TEXT)) THEN CAST(ship_id AS TEXT) ELSE hex(ship_id) END) ELSE ship_id END)) AS TEXT) FROM commander_commission_ships WHERE commander_commission_ships.commander_id = sqlc.arg(id);

-- name: AdminDeleteCommanderCommissionShips :exec
DELETE FROM commander_commission_ships WHERE commander_commission_ships.commander_id = sqlc.arg(id);

-- name: AdminExportCommanderCommissions :one
SELECT CAST(json_group_array(json_object('commander_id', CASE WHEN typeof(commander_id) = 'blob' THEN (CASE WHEN json_valid(CAST(commander_id AS TEXT)) THEN CAST(commander_id AS TEXT) ELSE hex(commander_id) END) ELSE commander_id END, 'collection_id', CASE WHEN typeof(collection_id) = 'blob' THEN (CASE WHEN json_valid(CAST(collection_id AS TEXT)) THEN CAST(collection_id AS TEXT) ELSE hex(collection_id) END) ELSE collection_id END, 'finish_time', CASE WHEN typeof(finish_time) = 'blob' THEN (CASE WHEN json_valid(CAST(finish_time AS TEXT)) THEN CAST(finish_time AS TEXT) ELSE hex(finish_time) END) ELSE finish_time END, 'over_time', CASE WHEN typeof(over_time) = 'blob' THEN (CASE WHEN json_valid(CAST(over_time AS TEXT)) THEN CAST(over_time AS TEXT) ELSE hex(over_time) END) ELSE over_time END)) AS TEXT) FROM commander_commissions WHERE commander_commissions.commander_id = sqlc.arg(id);

-- name: AdminDeleteCommanderCommissions :exec
DELETE FROM commander_commissions WHERE commander_commissions.commander_id = sqlc.arg(id);

-- name: AdminExportCommanderCommonFlags :one
SELECT CAST(json_group_array(json_object('commander_id', CASE WHEN typeof(commander_id) = 'blob' THEN (CASE WHEN json_valid(CAST(commander_id AS TEXT)) THEN CAST(commander_id AS TEXT) ELSE hex(commander_id) END) ELSE commander_id END, 'flag_id', CASE WHEN typeof(flag_id) = 'blob' THEN (CASE WHEN json_valid(CAST(flag_id AS TEXT)) THEN CAST(flag_id AS TEXT) ELSE hex(flag_id) END) ELSE flag_id END)) AS TEXT) FROM commander_common_flags WHERE commander_common_flags.commander_id = sqlc.arg(id);

-- name: AdminDeleteCommanderCommonFlags :exec
DELETE FROM commander_common_flags WHERE commander_common_flags.commander_id = sqlc.arg(id);

-- name: AdminExportCommanderCooldowns :one
SELECT CAST(json_group_array(json_object('commander_id', CASE WHEN typeof(commander_id) = 'blob' THEN (CASE WHEN json_valid(CAST(commander_id AS TEXT)) THEN CAST(commander_id AS TEXT) ELSE hex(commander_id) END) ELSE commander_id END, 'key', CASE WHEN typeof(key) = 'blob' THEN (CASE WHEN json_valid(CAST(key AS TEXT)) THEN CAST(key AS TEXT) ELSE hex(key) END) ELSE key END, 'until', CASE WHEN typeof(until) = 'blob' THEN (CASE WHEN json_valid(CAST(until AS TEXT)) THEN CAST(until AS TEXT) ELSE hex(until) END) ELSE until END)) AS TEXT) FROM commander_cooldowns WHERE commander_cooldowns.commander_id = sqlc.arg(id);

-- name: AdminDeleteCommanderCooldowns :exec
DELETE FROM commander_cooldowns WHERE commander_cooldowns.commander_id = sqlc.arg(id);

-- name: AdminExportCommanderCryptolalia :one
SELECT CAST(json_group_array(json_object('commander_id', CASE WHEN typeof(commander_id) = 'blob' THEN (CASE WHEN json_valid(CAST(commander_id AS TEXT)) THEN CAST(commander_id AS TEXT) ELSE hex(commander_id) END) ELSE commander_id END, 'story_id', CASE WHEN typeof(story_id) = 'blob' THEN (CASE WHEN json_valid(CAST(story_id AS TEXT)) THEN CAST(story_id AS TEXT) ELSE hex(story_id) END) ELSE story_id END)) AS TEXT) FROM commander_cryptolalia WHERE commander_cryptolalia.commander_id = sqlc.arg(id);

-- name: AdminDeleteCommanderCryptolalia :exec
DELETE FROM commander_cryptolalia WHERE commander_cryptolalia.commander_id = sqlc.arg(id);

-- name: AdminExportCommanderDisplayMedals :one
SELECT CAST(json_group_array(json_object('commander_id', CASE WHEN typeof(commander_id) = 'blob' THEN (CASE WHEN json_valid(CAST(commander_id AS TEXT)) THEN CAST(commander_id AS TEXT) ELSE hex(commander_id) END) ELSE commander_id END, 'position', CASE WHEN typeof(position) = 'blob' THEN (CASE WHEN json_valid(CAST(position AS TEXT)) THEN CAST(position AS TEXT) ELSE hex(position) END) ELSE position END, 'medal_id', CASE WHEN typeof(medal_id) = 'blob' THEN (CASE WHEN json_valid(CAST(medal_id AS TEXT)) THEN CAST(medal_id AS TEXT) ELSE hex(medal_id) END) ELSE medal_id END)) AS TEXT) FROM commander_display_medals WHERE commander_display_medals.commander_id = sqlc.arg(id);

-- name: AdminDeleteCommanderDisplayMedals :exec
DELETE FROM commander_display_medals WHERE commander_display_medals.commander_id = sqlc.arg(id);

-- name: AdminExportCommanderDorms :one
SELECT CAST(json_group_array(json_object('commander_id', CASE WHEN typeof(commander_id) = 'blob' THEN (CASE WHEN json_valid(CAST(commander_id AS TEXT)) THEN CAST(commander_id AS TEXT) ELSE hex(commander_id) END) ELSE commander_id END, 'level', CASE WHEN typeof(level) = 'blob' THEN (CASE WHEN json_valid(CAST(level AS TEXT)) THEN CAST(level AS TEXT) ELSE hex(level) END) ELSE level END, 'floor_num', CASE WHEN typeof(floor_num) = 'blob' THEN (CASE WHEN json_valid(CAST(floor_num AS TEXT)) THEN CAST(floor_num AS TEXT) ELSE hex(floor_num) END) ELSE floor_num END, 'exp_pos', CASE WHEN typeof(exp_pos) = 'blob' THEN (CASE WHEN json_valid(CAST(exp_pos AS TEXT)) THEN CAST(exp_pos AS TEXT) ELSE hex(exp_pos) END) ELSE exp_pos END, 'food_max_increase', CASE WHEN typeof(food_max_increase) = 'blob' THEN (CASE WHEN json_valid(CAST(food_max_increase AS TEXT)) THEN CAST(food_max_increase AS TEXT) ELSE hex(food_max_increase) END) ELSE food_max_increase END, 'food_max_increase_count', CASE WHEN typeof(food_max_increase_count) = 'blob' THEN (CASE WHEN json_valid(CAST(food_max_increase_count AS TEXT)) THEN CAST(food_max_increase_count AS TEXT) ELSE hex(food_max_increase_count) END) ELSE food_max_increase_count END, 'name', CASE WHEN typeof(name) = 'blob' THEN (CASE WHEN json_valid(CAST(name AS TEXT)) THEN CAST(name AS TEXT) ELSE hex(name) END) ELSE name END, 'load_time', CASE WHEN typeof(load_time) = 'blob' THEN (CASE WHEN json_valid(CAST(load_time AS TEXT)) THEN CAST(load_time AS TEXT) ELSE hex(load_time) END) ELSE load_time END, 'next_timestamp', CASE WHEN typeof(next_timestamp) = 'blob' THEN (CASE WHEN json_valid(CAST(next_timestamp AS TEXT)) THEN CAST(next_timestamp AS TEXT) ELSE hex(next_timestamp) END) ELSE next_timestamp END)) AS TEXT) FROM commander_dorms WHERE commander_dorms.commander_id = sqlc.arg(id);

-- name: AdminDeleteCommanderDorms :exec
DELETE FROM commander_dorms WHERE commander_dorms.commander_id = sqlc.arg(id);

-- name: AdminExportCommanderEducate :one
SELECT CAST(json_group_array(json_object('commander_id', CASE WHEN typeof(commander_id) = 'blob' THEN (CASE WHEN json_valid(CAST(commander_id AS TEXT)) THEN CAST(commander_id AS TEXT) ELSE hex(commander_id) END) ELSE commander_id END, 'state', CASE WHEN typeof(state) = 'blob' THEN (CASE WHEN json_valid(CAST(state AS TEXT)) THEN CAST(state AS TEXT) ELSE hex(state) END) ELSE state END)) AS TEXT) FROM commander_educate WHERE commander_educate.commander_id = sqlc.arg(id);

-- name: AdminDeleteCommanderEducate :exec
DELETE FROM commander_educate WHERE commander_educate.commander_id = sqlc.arg(id);

-- name: AdminExportCommanderEquipSkins :one
SELECT CAST(json_group_array(json_object('commander_id', CASE WHEN typeof(commander_id) = 'blob' THEN (CASE WHEN json_valid(CAST(commander_id AS TEXT)) THEN CAST(commander_id AS TEXT) ELSE hex(commander_id) END) ELSE commander_id END, 'skin_id', CASE WHEN typeof(skin_id) = 'blob' THEN (CASE WHEN json_valid(CAST(skin_id AS TEXT)) THEN CAST(skin_id AS TEXT) ELSE hex(skin_id) END) ELSE skin_id END, 'count', CASE WHEN typeof(count) = 'blob' THEN (CASE WHEN json_valid(CAST(count AS TEXT)) THEN CAST(count AS TEXT) ELSE hex(count) END) ELSE count END)) AS TEXT) FROM commander_equip_skins WHERE commander_equip_skins.commander_id = sqlc.arg(id);

-- name: AdminDeleteCommanderEquipSkins :exec
DELETE FROM commander_equip_skins WHERE commander_equip_skins.commander_id = sqlc.arg(id);

-- name: AdminExportCommanderEquipments :one
SELECT CAST(json_group_array(json_object('commander_id', CASE WHEN typeof(commander_id) = 'blob' THEN (CASE WHEN json_valid(CAST(commander_id AS TEXT)) THEN CAST(commander_id AS TEXT) ELSE hex(commander_id) END) ELSE commander_id END, 'equip_id', CASE WHEN typeof(equip_id) = 'blob' THEN (CASE WHEN json_valid(CAST(equip_id AS TEXT)) THEN CAST(equip_id AS TEXT) ELSE hex(equip_id) END) ELSE equip_id END, 'count', CASE WHEN typeof(count) = 'blob' THEN (CASE WHEN json_valid(CAST(count AS TEXT)) THEN CAST(count AS TEXT) ELSE hex(count) END) ELSE count END)) AS TEXT) FROM commander_equipments WHERE commander_equipments.commander_id = sqlc.arg(id);

-- name: AdminDeleteCommanderEquipments :exec
DELETE FROM commander_equipments WHERE commander_equipments.commander_id = sqlc.arg(id);

-- name: AdminExportCommanderExercise :one
SELECT CAST(json_group_array(json_object('commander_id', CASE WHEN typeof(commander_id) = 'blob' THEN (CASE WHEN json_valid(CAST(commander_id AS TEXT)) THEN CAST(commander_id AS TEXT) ELSE hex(commander_id) END) ELSE commander_id END, 'season', CASE WHEN typeof(season) = 'blob' THEN (CASE WHEN json_valid(CAST(season AS TEXT)) THEN CAST(season AS TEXT) ELSE hex(season) END) ELSE season END, 'score', CASE WHEN typeof(score) = 'blob' THEN (CASE WHEN json_valid(CAST(score AS TEXT)) THEN CAST(score AS TEXT) ELSE hex(score) END) ELSE score END, 'best_rank', CASE WHEN typeof(best_rank) = 'blob' THEN (CASE WHEN json_valid(CAST(best_rank AS TEXT)) THEN CAST(best_rank AS TEXT) ELSE hex(best_rank) END) ELSE best_rank END, 'fight_count', CASE WHEN typeof(fight_count) = 'blob' THEN (CASE WHEN json_valid(CAST(fight_count AS TEXT)) THEN CAST(fight_count AS TEXT) ELSE hex(fight_count) END) ELSE fight_count END, 'refilled_at', CASE WHEN typeof(refilled_at) = 'blob' THEN (CASE WHEN json_valid(CAST(refilled_at AS TEXT)) THEN CAST(refilled_at AS TEXT) ELSE hex(refilled_at) END) ELSE refilled_at END, 'flash_count', CASE WHEN typeof(flash_count) = 'blob' THEN (CASE WHEN json_valid(CAST(flash_count AS TEXT)) THEN CAST(flash_count AS TEXT) ELSE hex(flash_count) END) ELSE flash_count END, 'flash_day', CASE WHEN typeof(flash_day) = 'blob' THEN (CASE WHEN json_valid(CAST(flash_day AS TEXT)) THEN CAST(flash_day AS TEXT) ELSE hex(flash_day) END) ELSE flash_day END, 'vanguard', CASE WHEN typeof(vanguard) = 'blob' THEN (CASE WHEN json_valid(CAST(vanguard AS TEXT)) THEN CAST(vanguard AS TEXT) ELSE hex(vanguard) END) ELSE vanguard END, 'main', CASE WHEN typeof(main) = 'blob' THEN (CASE WHEN json_valid(CAST(main AS TEXT)) THEN CAST(main AS TEXT) ELSE hex(main) END) ELSE main END, 'rivals', CASE WHEN typeof(rivals) = 'blob' THEN (CASE WHEN json_valid(CAST(rivals AS TEXT)) THEN CAST(rivals AS TEXT) ELSE hex(rivals) END) ELSE rivals END, 'shop_refreshes', CASE WHEN typeof(shop_refreshes) = 'blob' THEN (CASE WHEN json_valid(CAST(shop_refreshes AS TEXT)) THEN CAST(shop_refreshes AS TEXT) ELSE hex(shop_refreshes) END) ELSE shop_refreshes END, 'shop_day', CASE WHEN typeof(shop_day) = 'blob' THEN (CASE WHEN json_valid(CAST(shop_day AS TEXT)) THEN CAST(shop_day AS TEXT) ELSE hex(shop_day) END) ELSE shop_day END)) AS TEXT) FROM commander_exercise WHERE commander_exercise.commander_id = sqlc.arg(id);

-- name: AdminDeleteCommanderExercise :exec
DELETE FROM commander_exercise WHERE commander_exercise.commander_id = sqlc.arg(id);

-- name: AdminExportCommanderFeastInvitations :one
SELECT CAST(json_group_array(json_object('commander_id', CASE WHEN typeof(commander_id) = 'blob' THEN (CASE WHEN json_valid(CAST(commander_id AS TEXT)) THEN CAST(commander_id AS TEXT) ELSE hex(commander_id) END) ELSE commander_id END, 'activity_id', CASE WHEN typeof(activity_id) = 'blob' THEN (CASE WHEN json_valid(CAST(activity_id AS TEXT)) THEN CAST(activity_id AS TEXT) ELSE hex(activity_id) END) ELSE activity_id END, 'group_id', CASE WHEN typeof(group_id) = 'blob' THEN (CASE WHEN json_valid(CAST(group_id AS TEXT)) THEN CAST(group_id AS TEXT) ELSE hex(group_id) END) ELSE group_id END, 'state', CASE WHEN typeof(state) = 'blob' THEN (CASE WHEN json_valid(CAST(state AS TEXT)) THEN CAST(state AS TEXT) ELSE hex(state) END) ELSE state END, 'gift', CASE WHEN typeof(gift) = 'blob' THEN (CASE WHEN json_valid(CAST(gift AS TEXT)) THEN CAST(gift AS TEXT) ELSE hex(gift) END) ELSE gift END)) AS TEXT) FROM commander_feast_invitations WHERE commander_feast_invitations.commander_id = sqlc.arg(id);

-- name: AdminDeleteCommanderFeastInvitations :exec
DELETE FROM commander_feast_invitations WHERE commander_feast_invitations.commander_id = sqlc.arg(id);

-- name: AdminExportCommanderFeastShips :one
SELECT CAST(json_group_array(json_object('commander_id', CASE WHEN typeof(commander_id) = 'blob' THEN (CASE WHEN json_valid(CAST(commander_id AS TEXT)) THEN CAST(commander_id AS TEXT) ELSE hex(commander_id) END) ELSE commander_id END, 'activity_id', CASE WHEN typeof(activity_id) = 'blob' THEN (CASE WHEN json_valid(CAST(activity_id AS TEXT)) THEN CAST(activity_id AS TEXT) ELSE hex(activity_id) END) ELSE activity_id END, 'group_id', CASE WHEN typeof(group_id) = 'blob' THEN (CASE WHEN json_valid(CAST(group_id AS TEXT)) THEN CAST(group_id AS TEXT) ELSE hex(group_id) END) ELSE group_id END, 'bubble', CASE WHEN typeof(bubble) = 'blob' THEN (CASE WHEN json_valid(CAST(bubble AS TEXT)) THEN CAST(bubble AS TEXT) ELSE hex(bubble) END) ELSE bubble END, 'speech_bubble', CASE WHEN typeof(speech_bubble) = 'blob' THEN (CASE WHEN json_valid(CAST(speech_bubble AS TEXT)) THEN CAST(speech_bubble AS TEXT) ELSE hex(speech_bubble) END) ELSE speech_bubble END)) AS TEXT) FROM commander_feast_ships WHERE commander_feast_ships.commander_id = sqlc.arg(id);

-- name: AdminDeleteCommanderFeastShips :exec
DELETE FROM commander_feast_ships WHERE commander_feast_ships.commander_id = sqlc.arg(id);

-- name: AdminExportCommanderFeasts :one
SELECT CAST(json_group_array(json_object('commander_id', CASE WHEN typeof(commander_id) = 'blob' THEN (CASE WHEN json_valid(CAST(commander_id AS TEXT)) THEN CAST(commander_id AS TEXT) ELSE hex(commander_id) END) ELSE commander_id END, 'activity_id', CASE WHEN typeof(activity_id) = 'blob' THEN (CASE WHEN json_valid(CAST(activity_id AS TEXT)) THEN CAST(activity_id AS TEXT) ELSE hex(activity_id) END) ELSE activity_id END, 'refresh_time', CASE WHEN typeof(refresh_time) = 'blob' THEN (CASE WHEN json_valid(CAST(refresh_time AS TEXT)) THEN CAST(refresh_time AS TEXT) ELSE hex(refresh_time) END) ELSE refresh_time END)) AS TEXT) FROM commander_feasts WHERE commander_feasts.commander_id = sqlc.arg(id);

-- name: AdminDeleteCommanderFeasts :exec
DELETE FROM commander_feasts WHERE commander_feasts.commander_id = sqlc.arg(id);

-- name: AdminExportCommanderFurniture :one
SELECT CAST(json_group_array(json_object('commander_id', CASE WHEN typeof(commander_id) = 'blob' THEN (CASE WHEN json_valid(CAST(commander_id AS TEXT)) THEN CAST(commander_id AS TEXT) ELSE hex(commander_id) END) ELSE commander_id END, 'furniture_id', CASE WHEN typeof(furniture_id) = 'blob' THEN (CASE WHEN json_valid(CAST(furniture_id AS TEXT)) THEN CAST(furniture_id AS TEXT) ELSE hex(furniture_id) END) ELSE furniture_id END, 'count', CASE WHEN typeof(count) = 'blob' THEN (CASE WHEN json_valid(CAST(count AS TEXT)) THEN CAST(count AS TEXT) ELSE hex(count) END) ELSE count END)) AS TEXT) FROM commander_furniture WHERE commander_furniture.commander_id = sqlc.arg(id);

-- name: AdminDeleteCommanderFurniture :exec
DELETE FROM commander_furniture WHERE commander_furniture.commander_id = sqlc.arg(id);

-- name: AdminExportCommanderGameRoom :one
SELECT CAST(json_group_array(json_object('commander_id', CASE WHEN typeof(commander_id) = 'blob' THEN (CASE WHEN json_valid(CAST(commander_id AS TEXT)) THEN CAST(commander_id AS TEXT) ELSE hex(commander_id) END) ELSE commander_id END, 'first_enter', CASE WHEN typeof(first_enter) = 'blob' THEN (CASE WHEN json_valid(CAST(first_enter AS TEXT)) THEN CAST(first_enter AS TEXT) ELSE hex(first_enter) END) ELSE first_enter END, 'weekly_free_week', CASE WHEN typeof(weekly_free_week) = 'blob' THEN (CASE WHEN json_valid(CAST(weekly_free_week AS TEXT)) THEN CAST(weekly_free_week AS TEXT) ELSE hex(weekly_free_week) END) ELSE weekly_free_week END, 'pay_week', CASE WHEN typeof(pay_week) = 'blob' THEN (CASE WHEN json_valid(CAST(pay_week AS TEXT)) THEN CAST(pay_week AS TEXT) ELSE hex(pay_week) END) ELSE pay_week END, 'pay_coin_count', CASE WHEN typeof(pay_coin_count) = 'blob' THEN (CASE WHEN json_valid(CAST(pay_coin_count AS TEXT)) THEN CAST(pay_coin_count AS TEXT) ELSE hex(pay_coin_count) END) ELSE pay_coin_count END, 'ticket_month', CASE WHEN typeof(ticket_month) = 'blob' THEN (CASE WHEN json_valid(CAST(ticket_month AS TEXT)) THEN CAST(ticket_month AS TEXT) ELSE hex(ticket_month) END) ELSE ticket_month END, 'monthly_ticket', CASE WHEN typeof(monthly_ticket) = 'blob' THEN (CASE WHEN json_valid(CAST(monthly_ticket AS TEXT)) THEN CAST(monthly_ticket AS TEXT) ELSE hex(monthly_ticket) END) ELSE monthly_ticket END)) AS TEXT) FROM commander_game_room WHERE commander_game_room.commander_id = sqlc.arg(id);

-- name: AdminDeleteCommanderGameRoom :exec
DELETE FROM commander_game_room WHERE commander_game_room.commander_id = sqlc.arg(id);

-- name: AdminExportCommanderGameRoomScores :one
SELECT CAST(json_group_array(json_object('commander_id', CASE WHEN typeof(commander_id) = 'blob' THEN (CASE WHEN json_valid(CAST(commander_id AS TEXT)) THEN CAST(commander_id AS TEXT) ELSE hex(commander_id) END) ELSE commander_id END, 'room_id', CASE WHEN typeof(room_id) = 'blob' THEN (CASE WHEN json_valid(CAST(room_id AS TEXT)) THEN CAST(room_id AS TEXT) ELSE hex(room_id) END) ELSE room_id END, 'max_score', CASE WHEN typeof(max_score) = 'blob' THEN (CASE WHEN json_valid(CAST(max_score AS TEXT)) THEN CAST(max_score AS TEXT) ELSE hex(max_score) END) ELSE max_score END)) AS TEXT) FROM commander_game_room_scores WHERE commander_game_room_scores.commander_id = sqlc.arg(id);

-- name: AdminDeleteCommanderGameRoomScores :exec
DELETE FROM commander_game_room_scores WHERE commander_game_room_scores.commander_id = sqlc.arg(id);

-- name: AdminExportCommanderGuildShopPurchases :one
SELECT CAST(json_group_array(json_object('commander_id', CASE WHEN typeof(commander_id) = 'blob' THEN (CASE WHEN json_valid(CAST(commander_id AS TEXT)) THEN CAST(commander_id AS TEXT) ELSE hex(commander_id) END) ELSE commander_id END, 'slot', CASE WHEN typeof(slot) = 'blob' THEN (CASE WHEN json_valid(CAST(slot AS TEXT)) THEN CAST(slot AS TEXT) ELSE hex(slot) END) ELSE slot END, 'count', CASE WHEN typeof(count) = 'blob' THEN (CASE WHEN json_valid(CAST(count AS TEXT)) THEN CAST(count AS TEXT) ELSE hex(count) END) ELSE count END)) AS TEXT) FROM commander_guild_shop_purchases WHERE commander_guild_shop_purchases.commander_id = sqlc.arg(id);

-- name: AdminDeleteCommanderGuildShopPurchases :exec
DELETE FROM commander_guild_shop_purchases WHERE commander_guild_shop_purchases.commander_id = sqlc.arg(id);

-- name: AdminExportCommanderGuildShops :one
SELECT CAST(json_group_array(json_object('commander_id', CASE WHEN typeof(commander_id) = 'blob' THEN (CASE WHEN json_valid(CAST(commander_id AS TEXT)) THEN CAST(commander_id AS TEXT) ELSE hex(commander_id) END) ELSE commander_id END, 'next_refresh_time', CASE WHEN typeof(next_refresh_time) = 'blob' THEN (CASE WHEN json_valid(CAST(next_refresh_time AS TEXT)) THEN CAST(next_refresh_time AS TEXT) ELSE hex(next_refresh_time) END) ELSE next_refresh_time END, 'refresh_count', CASE WHEN typeof(refresh_count) = 'blob' THEN (CASE WHEN json_valid(CAST(refresh_count AS TEXT)) THEN CAST(refresh_count AS TEXT) ELSE hex(refresh_count) END) ELSE refresh_count END)) AS TEXT) FROM commander_guild_shops WHERE commander_guild_shops.commander_id = sqlc.arg(id);

-- name: AdminDeleteCommanderGuildShops :exec
DELETE FROM commander_guild_shops WHERE commander_guild_shops.commander_id = sqlc.arg(id);

-- name: AdminExportCommanderGuildTechnologies :one
SELECT CAST(json_group_array(json_object('commander_id', CASE WHEN typeof(commander_id) = 'blob' THEN (CASE WHEN json_valid(CAST(commander_id AS TEXT)) THEN CAST(commander_id AS TEXT) ELSE hex(commander_id) END) ELSE commander_id END, 'group_id', CASE WHEN typeof(group_id) = 'blob' THEN (CASE WHEN json_valid(CAST(group_id AS TEXT)) THEN CAST(group_id AS TEXT) ELSE hex(group_id) END) ELSE group_id END, 'tech_id', CASE WHEN typeof(tech_id) = 'blob' THEN (CASE WHEN json_valid(CAST(tech_id AS TEXT)) THEN CAST(tech_id AS TEXT) ELSE hex(tech_id) END) ELSE tech_id END)) AS TEXT) FROM commander_guild_technologies WHERE commander_guild_technologies.commander_id = sqlc.arg(id);

-- name: AdminDeleteCommanderGuildTechnologies :exec
DELETE FROM commander_guild_technologies WHERE commander_guild_technologies.commander_id = sqlc.arg(id);

-- name: AdminExportCommanderHandbookFinished :one
SELECT CAST(json_group_array(json_object('commander_id', CASE WHEN typeof(commander_id) = 'blob' THEN (CASE WHEN json_valid(CAST(commander_id AS TEXT)) THEN CAST(commander_id AS TEXT) ELSE hex(commander_id) END) ELSE commander_id END, 'page_id', CASE WHEN typeof(page_id) = 'blob' THEN (CASE WHEN json_valid(CAST(page_id AS TEXT)) THEN CAST(page_id AS TEXT) ELSE hex(page_id) END) ELSE page_id END, 'task_id', CASE WHEN typeof(task_id) = 'blob' THEN (CASE WHEN json_valid(CAST(task_id AS TEXT)) THEN CAST(task_id AS TEXT) ELSE hex(task_id) END) ELSE task_id END)) AS TEXT) FROM commander_handbook_finished WHERE commander_handbook_finished.commander_id = sqlc.arg(id);

-- name: AdminDeleteCommanderHandbookFinished :exec
DELETE FROM commander_handbook_finished WHERE commander_handbook_finished.commander_id = sqlc.arg(id);

-- name: AdminExportCommanderHandbookPages :one
SELECT CAST(json_group_array(json_object('commander_id', CASE WHEN typeof(commander_id) = 'blob' THEN (CASE WHEN json_valid(CAST(commander_id AS TEXT)) THEN CAST(commander_id AS TEXT) ELSE hex(commander_id) END) ELSE commander_id END, 'page_id', CASE WHEN typeof(page_id) = 'blob' THEN (CASE WHEN json_valid(CAST(page_id AS TEXT)) THEN CAST(page_id AS TEXT) ELSE hex(page_id) END) ELSE page_id END, 'award', CASE WHEN typeof(award) = 'blob' THEN (CASE WHEN json_valid(CAST(award AS TEXT)) THEN CAST(award AS TEXT) ELSE hex(award) END) ELSE award END)) AS TEXT) FROM commander_handbook_pages WHERE commander_handbook_pages.commander_id = sqlc.arg(id);

-- name: AdminDeleteCommanderHandbookPages :exec
DELETE FROM commander_handbook_pages WHERE commander_handbook_pages.commander_id = sqlc.arg(id);

-- name: AdminExportCommanderIslandNodes :one
SELECT CAST(json_group_array(json_object('commander_id', CASE WHEN typeof(commander_id) = 'blob' THEN (CASE WHEN json_valid(CAST(commander_id AS TEXT)) THEN CAST(commander_id AS TEXT) ELSE hex(commander_id) END) ELSE commander_id END, 'activity_id', CASE WHEN typeof(activity_id) = 'blob' THEN (CASE WHEN json_valid(CAST(activity_id AS TEXT)) THEN CAST(activity_id AS TEXT) ELSE hex(activity_id) END) ELSE activity_id END, 'node_id', CASE WHEN typeof(node_id) = 'blob' THEN (CASE WHEN json_valid(CAST(node_id AS TEXT)) THEN CAST(node_id AS TEXT) ELSE hex(node_id) END) ELSE node_id END, 'event_id', CASE WHEN typeof(event_id) = 'blob' THEN (CASE WHEN json_valid(CAST(event_id AS TEXT)) THEN CAST(event_id AS TEXT) ELSE hex(event_id) END) ELSE event_id END, 'is_new', CASE WHEN typeof(is_new) = 'blob' THEN (CASE WHEN json_valid(CAST(is_new AS TEXT)) THEN CAST(is_new AS TEXT) ELSE hex(is_new) END) ELSE is_new END, 'refreshed_at', CASE WHEN typeof(refreshed_at) = 'blob' THEN (CASE WHEN json_valid(CAST(refreshed_at AS TEXT)) THEN CAST(refreshed_at AS TEXT) ELSE hex(refreshed_at) END) ELSE refreshed_at END)) AS TEXT) FROM commander_island_nodes WHERE commander_island_nodes.commander_id = sqlc.arg(id);

-- name: AdminDeleteCommanderIslandNodes :exec
DELETE FROM commander_island_nodes WHERE commander_island_nodes.commander_id = sqlc.arg(id);

-- name: AdminExportCommanderIslands :one
SELECT CAST(json_group_array(json_object('commander_id', CASE WHEN typeof(commander_id) = 'blob' THEN (CASE WHEN json_valid(CAST(commander_id AS TEXT)) THEN CAST(commander_id AS TEXT) ELSE hex(commander_id) END) ELSE commander_id END, 'island', CASE WHEN typeof(island) = 'blob' THEN (CASE WHEN json_valid(CAST(island AS TEXT)) THEN CAST(island AS TEXT) ELSE hex(island) END) ELSE island END, 'position', CASE WHEN typeof(position) = 'blob' THEN (CASE WHEN json_valid(CAST(position AS TEXT)) THEN CAST(position AS TEXT) ELSE hex(position) END) ELSE position END, 'extra', CASE WHEN typeof(extra) = 'blob' THEN (CASE WHEN json_valid(CAST(extra AS TEXT)) THEN CAST(extra AS TEXT) ELSE hex(extra) END) ELSE extra END, 'invite_code', CASE WHEN typeof(invite_code) = 'blob' THEN (CASE WHEN json_valid(CAST(invite_code AS TEXT)) THEN CAST(invite_code AS TEXT) ELSE hex(invite_code) END) ELSE invite_code END)) AS TEXT) FROM commander_islands WHERE commander_islands.commander_id = sqlc.arg(id);

-- name: AdminDeleteCommanderIslands :exec
DELETE FROM commander_islands WHERE commander_islands.commander_id = sqlc.arg(id);

-- name: AdminExportCommanderItems :one
SELECT CAST(json_group_array(json_object('commander_id', CASE WHEN typeof(commander_id) = 'blob' THEN (CASE WHEN json_valid(CAST(commander_id AS TEXT)) THEN CAST(commander_id AS TEXT) ELSE hex(commander_id) END) ELSE commander_id END, 'item_id', CASE WHEN typeof(item_id) = 'blob' THEN (CASE WHEN json_valid(CAST(item_id AS TEXT)) THEN CAST(item_id AS TEXT) ELSE hex(item_id) END) ELSE item_id END, 'count', CASE WHEN typeof(count) = 'blob' THEN (CASE WHEN json_valid(CAST(count AS TEXT)) THEN CAST(count AS TEXT) ELSE hex(count) END) ELSE count END)) AS TEXT) FROM commander_items WHERE commander_items.commander_id = sqlc.arg(id);

-- name: AdminDeleteCommanderItems :exec
DELETE FROM commander_items WHERE commander_items.commander_id = sqlc.arg(id);

-- name: AdminExportCommanderJuusGroups :one
SELECT CAST(json_group_array(json_object('commander_id', CASE WHEN typeof(commander_id) = 'blob' THEN (CASE WHEN json_valid(CAST(commander_id AS TEXT)) THEN CAST(commander_id AS TEXT) ELSE hex(commander_id) END) ELSE commander_id END, 'group_id', CASE WHEN typeof(group_id) = 'blob' THEN (CASE WHEN json_valid(CAST(group_id AS TEXT)) THEN CAST(group_id AS TEXT) ELSE hex(group_id) END) ELSE group_id END, 'skin_id', CASE WHEN typeof(skin_id) = 'blob' THEN (CASE WHEN json_valid(CAST(skin_id AS TEXT)) THEN CAST(skin_id AS TEXT) ELSE hex(skin_id) END) ELSE skin_id END, 'favorite', CASE WHEN typeof(favorite) = 'blob' THEN (CASE WHEN json_valid(CAST(favorite AS TEXT)) THEN CAST(favorite AS TEXT) ELSE hex(favorite) END) ELSE favorite END, 'cur_topic', CASE WHEN typeof(cur_topic) = 'blob' THEN (CASE WHEN json_valid(CAST(cur_topic AS TEXT)) THEN CAST(cur_topic AS TEXT) ELSE hex(cur_topic) END) ELSE cur_topic END)) AS TEXT) FROM commander_juus_groups WHERE commander_juus_groups.commander_id = sqlc.arg(id);

-- name: AdminDeleteCommanderJuusGroups :exec
DELETE FROM commander_juus_groups WHERE commander_juus_groups.commander_id = sqlc.arg(id);

-- name: AdminExportCommanderJuusPosts :one
SELECT CAST(json_group_array(json_object('commander_id', CASE WHEN typeof(commander_id) = 'blob' THEN (CASE WHEN json_valid(CAST(commander_id AS TEXT)) THEN CAST(commander_id AS TEXT) ELSE hex(commander_id) END) ELSE commander_id END, 'post_id', CASE WHEN typeof(post_id) = 'blob' THEN (CASE WHEN json_valid(CAST(post_id AS TEXT)) THEN CAST(post_id AS TEXT) ELSE hex(post_id) END) ELSE post_id END, 'is_good', CASE WHEN typeof(is_good) = 'blob' THEN (CASE WHEN json_valid(CAST(is_good AS TEXT)) THEN CAST(is_good AS TEXT) ELSE hex(is_good) END) ELSE is_good END, 'is_read', CASE WHEN typeof(is_read) = 'blob' THEN (CASE WHEN json_valid(CAST(is_read AS TEXT)) THEN CAST(is_read AS TEXT) ELSE hex(is_read) END) ELSE is_read END, 'comment_discuss', CASE WHEN typeof(comment_discuss) = 'blob' THEN (CASE WHEN json_valid(CAST(comment_discuss AS TEXT)) THEN CAST(comment_discuss AS TEXT) ELSE hex(comment_discuss) END) ELSE comment_discuss END, 'comment_index', CASE WHEN typeof(comment_index) = 'blob' THEN (CASE WHEN json_valid(CAST(comment_index AS TEXT)) THEN CAST(comment_index AS TEXT) ELSE hex(comment_index) END) ELSE comment_index END, 'comment_time', CASE WHEN typeof(comment_time) = 'blob' THEN (CASE WHEN json_valid(CAST(comment_time AS TEXT)) THEN CAST(comment_time AS TEXT) ELSE hex(comment_time) END) ELSE comment_time END)) AS TEXT) FROM commander_juus_posts WHERE commander_juus_posts.commander_id = sqlc.arg(id);

-- name: AdminDeleteCommanderJuusPosts :exec
DELETE FROM commander_juus_posts WHERE commander_juus_posts.commander_id = sqlc.arg(id);

-- name: AdminExportCommanderJuusReplies :one
SELECT CAST(json_group_array(json_object('commander_id', CASE WHEN typeof(commander_id) = 'blob' THEN (CASE WHEN json_valid(CAST(commander_id AS TEXT)) THEN CAST(commander_id AS TEXT) ELSE hex(commander_id) END) ELSE commander_id END, 'topic_id', CASE WHEN typeof(topic_id) = 'blob' THEN (CASE WHEN json_valid(CAST(topic_id AS TEXT)) THEN CAST(topic_id AS TEXT) ELSE hex(topic_id) END) ELSE topic_id END, 'chat_id', CASE WHEN typeof(chat_id) = 'blob' THEN (CASE WHEN json_valid(CAST(chat_id AS TEXT)) THEN CAST(chat_id AS TEXT) ELSE hex(chat_id) END) ELSE chat_id END, 'value', CASE WHEN typeof(value) = 'blob' THEN (CASE WHEN json_valid(CAST(value AS TEXT)) THEN CAST(value AS TEXT) ELSE hex(value) END) ELSE value END)) AS TEXT) FROM commander_juus_replies WHERE commander_juus_replies.commander_id = sqlc.arg(id);

-- name: AdminDeleteCommanderJuusReplies :exec
DELETE FROM commander_juus_replies WHERE commander_juus_replies.commander_id = sqlc.arg(id);

-- name: AdminExportCommanderJuusTopics :one
SELECT CAST(json_group_array(json_object('commander_id', CASE WHEN typeof(commander_id) = 'blob' THEN (CASE WHEN json_valid(CAST(commander_id AS TEXT)) THEN CAST(commander_id AS TEXT) ELSE hex(commander_id) END) ELSE commander_id END, 'topic_id', CASE WHEN typeof(topic_id) = 'blob' THEN (CASE WHEN json_valid(CAST(topic_id AS TEXT)) THEN CAST(topic_id AS TEXT) ELSE hex(topic_id) END) ELSE topic_id END, 'op_time', CASE WHEN typeof(op_time) = 'blob' THEN (CASE WHEN json_valid(CAST(op_time AS TEXT)) THEN CAST(op_time AS TEXT) ELSE hex(op_time) END) ELSE op_time END, 'read_flag', CASE WHEN typeof(read_flag) = 'blob' THEN (CASE WHEN json_valid(CAST(read_flag AS TEXT)) THEN CAST(read_flag AS TEXT) ELSE hex(read_flag) END) ELSE read_flag END)) AS TEXT) FROM commander_juus_topics WHERE commander_juus_topics.commander_id = sqlc.arg(id);

-- name: AdminDeleteCommanderJuusTopics :exec
DELETE FROM commander_juus_topics WHERE commander_juus_topics.commander_id = sqlc.arg(id);

-- name: AdminExportCommanderLoveLetterGifts :one
SELECT CAST(json_group_array(json_object('commander_id', CASE WHEN typeof(commander_id) = 'blob' THEN (CASE WHEN json_valid(CAST(commander_id AS TEXT)) THEN CAST(commander_id AS TEXT) ELSE hex(commander_id) END) ELSE commander_id END, 'pos', CASE WHEN typeof(pos) = 'blob' THEN (CASE WHEN json_valid(CAST(pos AS TEXT)) THEN CAST(pos AS TEXT) ELSE hex(pos) END) ELSE pos END, 'item_id', CASE WHEN typeof(item_id) = 'blob' THEN (CASE WHEN json_valid(CAST(item_id AS TEXT)) THEN CAST(item_id AS TEXT) ELSE hex(item_id) END) ELSE item_id END, 'group_id', CASE WHEN typeof(group_id) = 'blob' THEN (CASE WHEN json_valid(CAST(group_id AS TEXT)) THEN CAST(group_id AS TEXT) ELSE hex(group_id) END) ELSE group_id END, 'year', CASE WHEN typeof(year) = 'blob' THEN (CASE WHEN json_valid(CAST(year AS TEXT)) THEN CAST(year AS TEXT) ELSE hex(year) END) ELSE year END)) AS TEXT) FROM commander_love_letter_gifts WHERE commander_love_letter_gifts.commander_id = sqlc.arg(id);

-- name: AdminDeleteCommanderLoveLetterGifts :exec
DELETE FROM commander_love_letter_gifts WHERE commander_love_letter_gifts.commander_id = sqlc.arg(id);

-- name: AdminExportCommanderLoveLetterMedals :one
SELECT CAST(json_group_array(json_object('commander_id', CASE WHEN typeof(commander_id) = 'blob' THEN (CASE WHEN json_valid(CAST(commander_id AS TEXT)) THEN CAST(commander_id AS TEXT) ELSE hex(commander_id) END) ELSE commander_id END, 'group_id', CASE WHEN typeof(group_id) = 'blob' THEN (CASE WHEN json_valid(CAST(group_id AS TEXT)) THEN CAST(group_id AS TEXT) ELSE hex(group_id) END) ELSE group_id END, 'exp', CASE WHEN typeof(exp) = 'blob' THEN (CASE WHEN json_valid(CAST(exp AS TEXT)) THEN CAST(exp AS TEXT) ELSE hex(exp) END) ELSE exp END, 'level', CASE WHEN typeof(level) = 'blob' THEN (CASE WHEN json_valid(CAST(level AS TEXT)) THEN CAST(level AS TEXT) ELSE hex(level) END) ELSE level END)) AS TEXT) FROM commander_love_letter_medals WHERE commander_love_letter_medals.commander_id = sqlc.arg(id);

-- name: AdminDeleteCommanderLoveLetterMedals :exec
DELETE FROM commander_love_letter_medals WHERE commander_love_letter_medals.commander_id = sqlc.arg(id);

-- name: AdminExportCommanderLoveLetterRewards :one
SELECT CAST(json_group_array(json_object('commander_id', CASE WHEN typeof(commander_id) = 'blob' THEN (CASE WHEN json_valid(CAST(commander_id AS TEXT)) THEN CAST(commander_id AS TEXT) ELSE hex(commander_id) END) ELSE commander_id END, 'reward_id', CASE WHEN typeof(reward_id) = 'blob' THEN (CASE WHEN json_valid(CAST(reward_id AS TEXT)) THEN CAST(reward_id AS TEXT) ELSE hex(reward_id) END) ELSE reward_id END)) AS TEXT) FROM commander_love_letter_rewards WHERE commander_love_letter_rewards.commander_id = sqlc.arg(id);

-- name: AdminDeleteCommanderLoveLetterRewards :exec
DELETE FROM commander_love_letter_rewards WHERE commander_love_letter_rewards.commander_id = sqlc.arg(id);

-- name: AdminExportCommanderLoveLetters :one
SELECT CAST(json_group_array(json_object('commander_id', CASE WHEN typeof(commander_id) = 'blob' THEN (CASE WHEN json_valid(CAST(commander_id AS TEXT)) THEN CAST(commander_id AS TEXT) ELSE hex(commander_id) END) ELSE commander_id END, 'letter_id', CASE WHEN typeof(letter_id) = 'blob' THEN (CASE WHEN json_valid(CAST(letter_id AS TEXT)) THEN CAST(letter_id AS TEXT) ELSE hex(letter_id) END) ELSE letter_id END)) AS TEXT) FROM commander_love_letters WHERE commander_love_letters.commander_id = sqlc.arg(id);

-- name: AdminDeleteCommanderLoveLetters :exec
DELETE FROM commander_love_letters WHERE commander_love_letters.commander_id = sqlc.arg(id);

-- name: AdminExportCommanderMedalShopPurchases :one
SELECT CAST(json_group_array(json_object('commander_id', CASE WHEN typeof(commander_id) = 'blob' THEN (CASE WHEN json_valid(CAST(commander_id AS TEXT)) THEN CAST(commander_id AS TEXT) ELSE hex(commander_id) END) ELSE commander_id END, 'goods_id', CASE WHEN typeof(goods_id) = 'blob' THEN (CASE WHEN json_valid(CAST(goods_id AS TEXT)) THEN CAST(goods_id AS TEXT) ELSE hex(goods_id) END) ELSE goods_id END, 'flash_time', CASE WHEN typeof(flash_time) = 'blob' THEN (CASE WHEN json_valid(CAST(flash_time AS TEXT)) THEN CAST(flash_time AS TEXT) ELSE hex(flash_time) END) ELSE flash_time END, 'count', CASE WHEN typeof(count) = 'blob' THEN (CASE WHEN json_valid(CAST(count AS TEXT)) THEN CAST(count AS TEXT) ELSE hex(count) END) ELSE count END)) AS TEXT) FROM commander_medal_shop_purchases WHERE commander_medal_shop_purchases.commander_id = sqlc.arg(id);

-- name: AdminDeleteCommanderMedalShopPurchases :exec
DELETE FROM commander_medal_shop_purchases WHERE commander_medal_shop_purchases.commander_id = sqlc.arg(id);

-- name: AdminExportCommanderMiniGameShopPurchases :one
SELECT CAST(json_group_array(json_object('commander_id', CASE WHEN typeof(commander_id) = 'blob' THEN (CASE WHEN json_valid(CAST(commander_id AS TEXT)) THEN CAST(commander_id AS TEXT) ELSE hex(commander_id) END) ELSE commander_id END, 'goods_id', CASE WHEN typeof(goods_id) = 'blob' THEN (CASE WHEN json_valid(CAST(goods_id AS TEXT)) THEN CAST(goods_id AS TEXT) ELSE hex(goods_id) END) ELSE goods_id END, 'month_start', CASE WHEN typeof(month_start) = 'blob' THEN (CASE WHEN json_valid(CAST(month_start AS TEXT)) THEN CAST(month_start AS TEXT) ELSE hex(month_start) END) ELSE month_start END, 'count', CASE WHEN typeof(count) = 'blob' THEN (CASE WHEN json_valid(CAST(count AS TEXT)) THEN CAST(count AS TEXT) ELSE hex(count) END) ELSE count END)) AS TEXT) FROM commander_mini_game_shop_purchases WHERE commander_mini_game_shop_purchases.commander_id = sqlc.arg(id);

-- name: AdminDeleteCommanderMiniGameShopPurchases :exec
DELETE FROM commander_mini_game_shop_purchases WHERE commander_mini_game_shop_purchases.commander_id = sqlc.arg(id);

-- name: AdminExportCommanderMinigameData :one
SELECT CAST(json_group_array(json_object('commander_id', CASE WHEN typeof(commander_id) = 'blob' THEN (CASE WHEN json_valid(CAST(commander_id AS TEXT)) THEN CAST(commander_id AS TEXT) ELSE hex(commander_id) END) ELSE commander_id END, 'game_id', CASE WHEN typeof(game_id) = 'blob' THEN (CASE WHEN json_valid(CAST(game_id AS TEXT)) THEN CAST(game_id AS TEXT) ELSE hex(game_id) END) ELSE game_id END, 'datas', CASE WHEN typeof(datas) = 'blob' THEN (CASE WHEN json_valid(CAST(datas AS TEXT)) THEN CAST(datas AS TEXT) ELSE hex(datas) END) ELSE datas END, 'count', CASE WHEN typeof(count) = 'blob' THEN (CASE WHEN json_valid(CAST(count AS TEXT)) THEN CAST(count AS TEXT) ELSE hex(count) END) ELSE count END, 'count_day', CASE WHEN typeof(count_day) = 'blob' THEN (CASE WHEN json_valid(CAST(count_day AS TEXT)) THEN CAST(count_day AS TEXT) ELSE hex(count_day) END) ELSE count_day END, 'gold', CASE WHEN typeof(gold) = 'blob' THEN (CASE WHEN json_valid(CAST(gold AS TEXT)) THEN CAST(gold AS TEXT) ELSE hex(gold) END) ELSE gold END)) AS TEXT) FROM commander_minigame_data WHERE commander_minigame_data.commander_id = sqlc.arg(id);

-- name: AdminDeleteCommanderMinigameData :exec
DELETE FROM commander_minigame_data WHERE commander_minigame_data.commander_id = sqlc.arg(id);

-- name: AdminExportCommanderMinigameHubs :one
SELECT CAST(json_group_array(json_object('commander_id', CASE WHEN typeof(commander_id) = 'blob' THEN (CASE WHEN json_valid(CAST(commander_id AS TEXT)) THEN CAST(commander_id AS TEXT) ELSE hex(commander_id) END) ELSE commander_id END, 'hub_id', CASE WHEN typeof(hub_id) = 'blob' THEN (CASE WHEN json_valid(CAST(hub_id AS TEXT)) THEN CAST(hub_id AS TEXT) ELSE hex(hub_id) END) ELSE hub_id END, 'used_cnt', CASE WHEN typeof(used_cnt) = 'blob' THEN (CASE WHEN json_valid(CAST(used_cnt AS TEXT)) THEN CAST(used_cnt AS TEXT) ELSE hex(used_cnt) END) ELSE used_cnt END, 'ultimate', CASE WHEN typeof(ultimate) = 'blob' THEN (CASE WHEN json_valid(CAST(ultimate AS TEXT)) THEN CAST(ultimate AS TEXT) ELSE hex(ultimate) END) ELSE ultimate END, 'opened_at', CASE WHEN typeof(opened_at) = 'blob' THEN (CASE WHEN json_valid(CAST(opened_at AS TEXT)) THEN CAST(opened_at AS TEXT) ELSE hex(opened_at) END) ELSE opened_at END)) AS TEXT) FROM commander_minigame_hubs WHERE commander_minigame_hubs.commander_id = sqlc.arg(id);

-- name: AdminDeleteCommanderMinigameHubs :exec
DELETE FROM commander_minigame_hubs WHERE commander_minigame_hubs.commander_id = sqlc.arg(id);

-- name: AdminExportCommanderMinigameScores :one
SELECT CAST(json_group_array(json_object('commander_id', CASE WHEN typeof(commander_id) = 'blob' THEN (CASE WHEN json_valid(CAST(commander_id AS TEXT)) THEN CAST(commander_id AS TEXT) ELSE hex(commander_id) END) ELSE commander_id END, 'game_id', CASE WHEN typeof(game_id) = 'blob' THEN (CASE WHEN json_valid(CAST(game_id AS TEXT)) THEN CAST(game_id AS TEXT) ELSE hex(game_id) END) ELSE game_id END, 'score', CASE WHEN typeof(score) = 'blob' THEN (CASE WHEN json_valid(CAST(score AS TEXT)) THEN CAST(score AS TEXT) ELSE hex(score) END) ELSE score END, 'extra', CASE WHEN typeof(extra) = 'blob' THEN (CASE WHEN json_valid(CAST(extra AS TEXT)) THEN CAST(extra AS TEXT) ELSE hex(extra) END) ELSE extra END)) AS TEXT) FROM commander_minigame_scores WHERE commander_minigame_scores.commander_id = sqlc.arg(id);

-- name: AdminDeleteCommanderMinigameScores :exec
DELETE FROM commander_minigame_scores WHERE commander_minigame_scores.commander_id = sqlc.arg(id);

-- name: AdminExportCommanderMinigameValues :one
SELECT CAST(json_group_array(json_object('commander_id', CASE WHEN typeof(commander_id) = 'blob' THEN (CASE WHEN json_valid(CAST(commander_id AS TEXT)) THEN CAST(commander_id AS TEXT) ELSE hex(commander_id) END) ELSE commander_id END, 'game_id', CASE WHEN typeof(game_id) = 'blob' THEN (CASE WHEN json_valid(CAST(game_id AS TEXT)) THEN CAST(game_id AS TEXT) ELSE hex(game_id) END) ELSE game_id END, 'list_key', CASE WHEN typeof(list_key) = 'blob' THEN (CASE WHEN json_valid(CAST(list_key AS TEXT)) THEN CAST(list_key AS TEXT) ELSE hex(list_key) END) ELSE list_key END, 'key', CASE WHEN typeof(key) = 'blob' THEN (CASE WHEN json_valid(CAST(key AS TEXT)) THEN CAST(key AS TEXT) ELSE hex(key) END) ELSE key END, 'value', CASE WHEN typeof(value) = 'blob' THEN (CASE WHEN json_valid(CAST(value AS TEXT)) THEN CAST(value AS TEXT) ELSE hex(value) END) ELSE value END, 'value2', CASE WHEN typeof(value2) = 'blob' THEN (CASE WHEN json_valid(CAST(value2 AS TEXT)) THEN CAST(value2 AS TEXT) ELSE hex(value2) END) ELSE value2 END)) AS TEXT) FROM commander_minigame_values WHERE commander_minigame_values.commander_id = sqlc.arg(id);

-- name: AdminDeleteCommanderMinigameValues :exec
DELETE FROM commander_minigame_values WHERE commander_minigame_values.commander_id = sqlc.arg(id);

-- name: AdminExportCommanderMonthShopPurchases :one
SELECT CAST(json_group_array(json_object('commander_id', CASE WHEN typeof(commander_id) = 'blob' THEN (CASE WHEN json_valid(CAST(commander_id AS TEXT)) THEN CAST(commander_id AS TEXT) ELSE hex(commander_id) END) ELSE commander_id END, 'goods_id', CASE WHEN typeof(goods_id) = 'blob' THEN (CASE WHEN json_valid(CAST(goods_id AS TEXT)) THEN CAST(goods_id AS TEXT) ELSE hex(goods_id) END) ELSE goods_id END, 'month', CASE WHEN typeof(month) = 'blob' THEN (CASE WHEN json_valid(CAST(month AS TEXT)) THEN CAST(month AS TEXT) ELSE hex(month) END) ELSE month END, 'count', CASE WHEN typeof(count) = 'blob' THEN (CASE WHEN json_valid(CAST(count AS TEXT)) THEN CAST(count AS TEXT) ELSE hex(count) END) ELSE count END)) AS TEXT) FROM commander_month_shop_purchases WHERE commander_month_shop_purchases.commander_id = sqlc.arg(id);

-- name: AdminDeleteCommanderMonthShopPurchases :exec
DELETE FROM commander_month_shop_purchases WHERE commander_month_shop_purchases.commander_id = sqlc.arg(id);

-- name: AdminExportCommanderMusicPlayer :one
SELECT CAST(json_group_array(json_object('commander_id', CASE WHEN typeof(commander_id) = 'blob' THEN (CASE WHEN json_valid(CAST(commander_id AS TEXT)) THEN CAST(commander_id AS TEXT) ELSE hex(commander_id) END) ELSE commander_id END, 'music_no', CASE WHEN typeof(music_no) = 'blob' THEN (CASE WHEN json_valid(CAST(music_no AS TEXT)) THEN CAST(music_no AS TEXT) ELSE hex(music_no) END) ELSE music_no END, 'music_mode', CASE WHEN typeof(music_mode) = 'blob' THEN (CASE WHEN json_valid(CAST(music_mode AS TEXT)) THEN CAST(music_mode AS TEXT) ELSE hex(music_mode) END) ELSE music_mode END)) AS TEXT) FROM commander_music_player WHERE commander_music_player.commander_id = sqlc.arg(id);

-- name: AdminDeleteCommanderMusicPlayer :exec
DELETE FROM commander_music_player WHERE commander_music_player.commander_id = sqlc.arg(id);

-- name: AdminExportCommanderNewEducate :one
SELECT CAST(json_group_array(json_object('commander_id', CASE WHEN typeof(commander_id) = 'blob' THEN (CASE WHEN json_valid(CAST(commander_id AS TEXT)) THEN CAST(commander_id AS TEXT) ELSE hex(commander_id) END) ELSE commander_id END, 'character_id', CASE WHEN typeof(character_id) = 'blob' THEN (CASE WHEN json_valid(CAST(character_id AS TEXT)) THEN CAST(character_id AS TEXT) ELSE hex(character_id) END) ELSE character_id END, 'state', CASE WHEN typeof(state) = 'blob' THEN (CASE WHEN json_valid(CAST(state AS TEXT)) THEN CAST(state AS TEXT) ELSE hex(state) END) ELSE state END, 'permanent', CASE WHEN typeof(permanent) = 'blob' THEN (CASE WHEN json_valid(CAST(permanent AS TEXT)) THEN CAST(permanent AS TEXT) ELSE hex(permanent) END) ELSE permanent END)) AS TEXT) FROM commander_new_educate WHERE commander_new_educate.commander_id = sqlc.arg(id);

-- name: AdminDeleteCommanderNewEducate :exec
DELETE FROM commander_new_educate WHERE commander_new_educate.commander_id = sqlc.arg(id);

-- name: AdminExportCommanderPermanentActivities :one
SELECT CAST(json_group_array(json_object('commander_id', CASE WHEN typeof(commander_id) = 'blob' THEN (CASE WHEN json_valid(CAST(commander_id AS TEXT)) THEN CAST(commander_id AS TEXT) ELSE hex(commander_id) END) ELSE commander_id END, 'activity_id', CASE WHEN typeof(activity_id) = 'blob' THEN (CASE WHEN json_valid(CAST(activity_id AS TEXT)) THEN CAST(activity_id AS TEXT) ELSE hex(activity_id) END) ELSE activity_id END, 'running', CASE WHEN typeof(running) = 'blob' THEN (CASE WHEN json_valid(CAST(running AS TEXT)) THEN CAST(running AS TEXT) ELSE hex(running) END) ELSE running END, 'finished', CASE WHEN typeof(finished) = 'blob' THEN (CASE WHEN json_valid(CAST(finished AS TEXT)) THEN CAST(finished AS TEXT) ELSE hex(finished) END) ELSE finished END)) AS TEXT) FROM commander_permanent_activities WHERE commander_permanent_activities.commander_id = sqlc.arg(id);

-- name: AdminDeleteCommanderPermanentActivities :exec
DELETE FROM commander_permanent_activities WHERE commander_permanent_activities.commander_id = sqlc.arg(id);

-- name: AdminExportCommanderPlayRoomScores :one
SELECT CAST(json_group_array(json_object('commander_id', CASE WHEN typeof(commander_id) = 'blob' THEN (CASE WHEN json_valid(CAST(commander_id AS TEXT)) THEN CAST(commander_id AS TEXT) ELSE hex(commander_id) END) ELSE commander_id END, 'game_type', CASE WHEN typeof(game_type) = 'blob' THEN (CASE WHEN json_valid(CAST(game_type AS TEXT)) THEN CAST(game_type AS TEXT) ELSE hex(game_type) END) ELSE game_type END, 'season', CASE WHEN typeof(season) = 'blob' THEN (CASE WHEN json_valid(CAST(season AS TEXT)) THEN CAST(season AS TEXT) ELSE hex(season) END) ELSE season END, 'score', CASE WHEN typeof(score) = 'blob' THEN (CASE WHEN json_valid(CAST(score AS TEXT)) THEN CAST(score AS TEXT) ELSE hex(score) END) ELSE score END, 'max_score', CASE WHEN typeof(max_score) = 'blob' THEN (CASE WHEN json_valid(CAST(max_score AS TEXT)) THEN CAST(max_score AS TEXT) ELSE hex(max_score) END) ELSE max_score END)) AS TEXT) FROM commander_play_room_scores WHERE commander_play_room_scores.commander_id = sqlc.arg(id);

-- name: AdminDeleteCommanderPlayRoomScores :exec
DELETE FROM commander_play_room_scores WHERE commander_play_room_scores.commander_id = sqlc.arg(id);

-- name: AdminExportCommanderPlayRoomViews :one
SELECT CAST(json_group_array(json_object('commander_id', CASE WHEN typeof(commander_id) = 'blob' THEN (CASE WHEN json_valid(CAST(commander_id AS TEXT)) THEN CAST(commander_id AS TEXT) ELSE hex(commander_id) END) ELSE commander_id END, 'game_type', CASE WHEN typeof(game_type) = 'blob' THEN (CASE WHEN json_valid(CAST(game_type AS TEXT)) THEN CAST(game_type AS TEXT) ELSE hex(game_type) END) ELSE game_type END, 'ship_id', CASE WHEN typeof(ship_id) = 'blob' THEN (CASE WHEN json_valid(CAST(ship_id AS TEXT)) THEN CAST(ship_id AS TEXT) ELSE hex(ship_id) END) ELSE ship_id END, 'skin_id', CASE WHEN typeof(skin_id) = 'blob' THEN (CASE WHEN json_valid(CAST(skin_id AS TEXT)) THEN CAST(skin_id AS TEXT) ELSE hex(skin_id) END) ELSE skin_id END, 'color', CASE WHEN typeof(color) = 'blob' THEN (CASE WHEN json_valid(CAST(color AS TEXT)) THEN CAST(color AS TEXT) ELSE hex(color) END) ELSE color END, 'dress_list', CASE WHEN typeof(dress_list) = 'blob' THEN (CASE WHEN json_valid(CAST(dress_list AS TEXT)) THEN CAST(dress_list AS TEXT) ELSE hex(dress_list) END) ELSE dress_list END)) AS TEXT) FROM commander_play_room_views WHERE commander_play_room_views.commander_id = sqlc.arg(id);

-- name: AdminDeleteCommanderPlayRoomViews :exec
DELETE FROM commander_play_room_views WHERE commander_play_room_views.commander_id = sqlc.arg(id);

-- name: AdminExportCommanderRandomFlagship :one
SELECT CAST(json_group_array(json_object('commander_id', CASE WHEN typeof(commander_id) = 'blob' THEN (CASE WHEN json_valid(CAST(commander_id AS TEXT)) THEN CAST(commander_id AS TEXT) ELSE hex(commander_id) END) ELSE commander_id END, 'mode', CASE WHEN typeof(mode) = 'blob' THEN (CASE WHEN json_valid(CAST(mode AS TEXT)) THEN CAST(mode AS TEXT) ELSE hex(mode) END) ELSE mode END, 'enabled', CASE WHEN typeof(enabled) = 'blob' THEN (CASE WHEN json_valid(CAST(enabled AS TEXT)) THEN CAST(enabled AS TEXT) ELSE hex(enabled) END) ELSE enabled END)) AS TEXT) FROM commander_random_flagship WHERE commander_random_flagship.commander_id = sqlc.arg(id);

-- name: AdminDeleteCommanderRandomFlagship :exec
DELETE FROM commander_random_flagship WHERE commander_random_flagship.commander_id = sqlc.arg(id);

-- name: AdminExportCommanderReflux :one
SELECT CAST(json_group_array(json_object('commander_id', CASE WHEN typeof(commander_id) = 'blob' THEN (CASE WHEN json_valid(CAST(commander_id AS TEXT)) THEN CAST(commander_id AS TEXT) ELSE hex(commander_id) END) ELSE commander_id END, 'return_lv', CASE WHEN typeof(return_lv) = 'blob' THEN (CASE WHEN json_valid(CAST(return_lv AS TEXT)) THEN CAST(return_lv AS TEXT) ELSE hex(return_lv) END) ELSE return_lv END, 'return_time', CASE WHEN typeof(return_time) = 'blob' THEN (CASE WHEN json_valid(CAST(return_time AS TEXT)) THEN CAST(return_time AS TEXT) ELSE hex(return_time) END) ELSE return_time END, 'ship_number', CASE WHEN typeof(ship_number) = 'blob' THEN (CASE WHEN json_valid(CAST(ship_number AS TEXT)) THEN CAST(ship_number AS TEXT) ELSE hex(ship_number) END) ELSE ship_number END, 'last_offline_time', CASE WHEN typeof(last_offline_time) = 'blob' THEN (CASE WHEN json_valid(CAST(last_offline_time AS TEXT)) THEN CAST(last_offline_time AS TEXT) ELSE hex(last_offline_time) END) ELSE last_offline_time END, 'pt', CASE WHEN typeof(pt) = 'blob' THEN (CASE WHEN json_valid(CAST(pt AS TEXT)) THEN CAST(pt AS TEXT) ELSE hex(pt) END) ELSE pt END, 'sign_cnt', CASE WHEN typeof(sign_cnt) = 'blob' THEN (CASE WHEN json_valid(CAST(sign_cnt AS TEXT)) THEN CAST(sign_cnt AS TEXT) ELSE hex(sign_cnt) END) ELSE sign_cnt END, 'sign_last_time', CASE WHEN typeof(sign_last_time) = 'blob' THEN (CASE WHEN json_valid(CAST(sign_last_time AS TEXT)) THEN CAST(sign_last_time AS TEXT) ELSE hex(sign_last_time) END) ELSE sign_last_time END, 'pt_stage', CASE WHEN typeof(pt_stage) = 'blob' THEN (CASE WHEN json_valid(CAST(pt_stage AS TEXT)) THEN CAST(pt_stage AS TEXT) ELSE hex(pt_stage) END) ELSE pt_stage END)) AS TEXT) FROM commander_reflux WHERE commander_reflux.commander_id = sqlc.arg(id);

-- name: AdminDeleteCommanderReflux :exec
DELETE FROM commander_reflux WHERE commander_reflux.commander_id = sqlc.arg(id);

-- name: AdminExportCommanderRemasters :one
SELECT CAST(json_group_array(json_object('commander_id', CASE WHEN typeof(commander_id) = 'blob' THEN (CASE WHEN json_valid(CAST(commander_id AS TEXT)) THEN CAST(commander_id AS TEXT) ELSE hex(commander_id) END) ELSE commander_id END, 'remaster_id', CASE WHEN typeof(remaster_id) = 'blob' THEN (CASE WHEN json_valid(CAST(remaster_id AS TEXT)) THEN CAST(remaster_id AS TEXT) ELSE hex(remaster_id) END) ELSE remaster_id END, 'timer_id', CASE WHEN typeof(timer_id) = 'blob' THEN (CASE WHEN json_valid(CAST(timer_id AS TEXT)) THEN CAST(timer_id AS TEXT) ELSE hex(timer_id) END) ELSE timer_id END, 'activated_at', CASE WHEN typeof(activated_at) = 'blob' THEN (CASE WHEN json_valid(CAST(activated_at AS TEXT)) THEN CAST(activated_at AS TEXT) ELSE hex(activated_at) END) ELSE activated_at END)) AS TEXT) FROM commander_remasters WHERE commander_remasters.commander_id = sqlc.arg(id);

-- name: AdminDeleteCommanderRemasters :exec
DELETE FROM commander_remasters WHERE commander_remasters.commander_id = sqlc.arg(id);

-- name: AdminExportCommanderRequisitions :one
SELECT CAST(json_group_array(json_object('commander_id', CASE WHEN typeof(commander_id) = 'blob' THEN (CASE WHEN json_valid(CAST(commander_id AS TEXT)) THEN CAST(commander_id AS TEXT) ELSE hex(commander_id) END) ELSE commander_id END, 'month', CASE WHEN typeof(month) = 'blob' THEN (CASE WHEN json_valid(CAST(month AS TEXT)) THEN CAST(month AS TEXT) ELSE hex(month) END) ELSE month END, 'count', CASE WHEN typeof(count) = 'blob' THEN (CASE WHEN json_valid(CAST(count AS TEXT)) THEN CAST(count AS TEXT) ELSE hex(count) END) ELSE count END)) AS TEXT) FROM commander_requisitions WHERE commander_requisitions.commander_id = sqlc.arg(id);

-- name: AdminDeleteCommanderRequisitions :exec
DELETE FROM commander_requisitions WHERE commander_requisitions.commander_id = sqlc.arg(id);

-- name: AdminExportCommanderResources :one
SELECT CAST(json_group_array(json_object('commander_id', CASE WHEN typeof(commander_id) = 'blob' THEN (CASE WHEN json_valid(CAST(commander_id AS TEXT)) THEN CAST(commander_id AS TEXT) ELSE hex(commander_id) END) ELSE commander_id END, 'resource_id', CASE WHEN typeof(resource_id) = 'blob' THEN (CASE WHEN json_valid(CAST(resource_id AS TEXT)) THEN CAST(resource_id AS TEXT) ELSE hex(resource_id) END) ELSE resource_id END, 'amount', CASE WHEN typeof(amount) = 'blob' THEN (CASE WHEN json_valid(CAST(amount AS TEXT)) THEN CAST(amount AS TEXT) ELSE hex(amount) END) ELSE amount END)) AS TEXT) FROM commander_resources WHERE commander_resources.commander_id = sqlc.arg(id);

-- name: AdminDeleteCommanderResources :exec
DELETE FROM commander_resources WHERE commander_resources.commander_id = sqlc.arg(id);

-- name: AdminExportCommanderSecondaryPasswords :one
SELECT CAST(json_group_array(json_object('commander_id', CASE WHEN typeof(commander_id) = 'blob' THEN (CASE WHEN json_valid(CAST(commander_id AS TEXT)) THEN CAST(commander_id AS TEXT) ELSE hex(commander_id) END) ELSE commander_id END, 'password_hash', CASE WHEN typeof(password_hash) = 'blob' THEN (CASE WHEN json_valid(CAST(password_hash AS TEXT)) THEN CAST(password_hash AS TEXT) ELSE hex(password_hash) END) ELSE password_hash END, 'notice', CASE WHEN typeof(notice) = 'blob' THEN (CASE WHEN json_valid(CAST(notice AS TEXT)) THEN CAST(notice AS TEXT) ELSE hex(notice) END) ELSE notice END, 'system_list', CASE WHEN typeof(system_list) = 'blob' THEN (CASE WHEN json_valid(CAST(system_list AS TEXT)) THEN CAST(system_list AS TEXT) ELSE hex(system_list) END) ELSE system_list END, 'fail_count', CASE WHEN typeof(fail_count) = 'blob' THEN (CASE WHEN json_valid(CAST(fail_count AS TEXT)) THEN CAST(fail_count AS TEXT) ELSE hex(fail_count) END) ELSE fail_count END, 'fail_cd', CASE WHEN typeof(fail_cd) = 'blob' THEN (CASE WHEN json_valid(CAST(fail_cd AS TEXT)) THEN CAST(fail_cd AS TEXT) ELSE hex(fail_cd) END) ELSE fail_cd END)) AS TEXT) FROM commander_secondary_passwords WHERE commander_secondary_passwords.commander_id = sqlc.arg(id);

-- name: AdminDeleteCommanderSecondaryPasswords :exec
DELETE FROM commander_secondary_passwords WHERE commander_secondary_passwords.commander_id = sqlc.arg(id);

-- name: AdminExportCommanderSecretaries :one
SELECT CAST(json_group_array(json_object('commander_id', CASE WHEN typeof(commander_id) = 'blob' THEN (CASE WHEN json_valid(CAST(commander_id AS TEXT)) THEN CAST(commander_id AS TEXT) ELSE hex(commander_id) END) ELSE commander_id END, 'position', CASE WHEN typeof(position) = 'blob' THEN (CASE WHEN json_valid(CAST(position AS TEXT)) THEN CAST(position AS TEXT) ELSE hex(position) END) ELSE position END, 'ship_id', CASE WHEN typeof(ship_id) = 'blob' THEN (CASE WHEN json_valid(CAST(ship_id AS TEXT)) THEN CAST(ship_id AS TEXT) ELSE hex(ship_id) END) ELSE ship_id END, 'phantom_id', CASE WHEN typeof(phantom_id) = 'blob' THEN (CASE WHEN json_valid(CAST(phantom_id AS TEXT)) THEN CAST(phantom_id AS TEXT) ELSE hex(phantom_id) END) ELSE phantom_id END)) AS TEXT) FROM commander_secretaries WHERE commander_secretaries.commander_id = sqlc.arg(id);

-- name: AdminDeleteCommanderSecretaries :exec
DELETE FROM commander_secretaries WHERE commander_secretaries.commander_id = sqlc.arg(id);

-- name: AdminExportCommanderShipGroups :one
SELECT CAST(json_group_array(json_object('commander_id', CASE WHEN typeof(commander_id) = 'blob' THEN (CASE WHEN json_valid(CAST(commander_id AS TEXT)) THEN CAST(commander_id AS TEXT) ELSE hex(commander_id) END) ELSE commander_id END, 'group_id', CASE WHEN typeof(group_id) = 'blob' THEN (CASE WHEN json_valid(CAST(group_id AS TEXT)) THEN CAST(group_id AS TEXT) ELSE hex(group_id) END) ELSE group_id END, 'intimacy_max', CASE WHEN typeof(intimacy_max) = 'blob' THEN (CASE WHEN json_valid(CAST(intimacy_max AS TEXT)) THEN CAST(intimacy_max AS TEXT) ELSE hex(intimacy_max) END) ELSE intimacy_max END, 'lv_max', CASE WHEN typeof(lv_max) = 'blob' THEN (CASE WHEN json_valid(CAST(lv_max AS TEXT)) THEN CAST(lv_max AS TEXT) ELSE hex(lv_max) END) ELSE lv_max END, 'marry_flag', CASE WHEN typeof(marry_flag) = 'blob' THEN (CASE WHEN json_valid(CAST(marry_flag AS TEXT)) THEN CAST(marry_flag AS TEXT) ELSE hex(marry_flag) END) ELSE marry_flag END, 'heart_flag', CASE WHEN typeof(heart_flag) = 'blob' THEN (CASE WHEN json_valid(CAST(heart_flag AS TEXT)) THEN CAST(heart_flag AS TEXT) ELSE hex(heart_flag) END) ELSE heart_flag END, 'star', CASE WHEN typeof(star) = 'blob' THEN (CASE WHEN json_valid(CAST(star AS TEXT)) THEN CAST(star AS TEXT) ELSE hex(star) END) ELSE star END, 'trans', CASE WHEN typeof(trans) = 'blob' THEN (CASE WHEN json_valid(CAST(trans AS TEXT)) THEN CAST(trans AS TEXT) ELSE hex(trans) END) ELSE trans END)) AS TEXT) FROM commander_ship_groups WHERE commander_ship_groups.commander_id = sqlc.arg(id);

-- name: AdminDeleteCommanderShipGroups :exec
DELETE FROM commander_ship_groups WHERE commander_ship_groups.commander_id = sqlc.arg(id);

-- name: AdminExportCommanderShopGroupPurchases :one
SELECT CAST(json_group_array(json_object('commander_id', CASE WHEN typeof(commander_id) = 'blob' THEN (CASE WHEN json_valid(CAST(commander_id AS TEXT)) THEN CAST(commander_id AS TEXT) ELSE hex(commander_id) END) ELSE commander_id END, 'group_id', CASE WHEN typeof(group_id) = 'blob' THEN (CASE WHEN json_valid(CAST(group_id AS TEXT)) THEN CAST(group_id AS TEXT) ELSE hex(group_id) END) ELSE group_id END, 'expires_at', CASE WHEN typeof(expires_at) = 'blob' THEN (CASE WHEN json_valid(CAST(expires_at AS TEXT)) THEN CAST(expires_at AS TEXT) ELSE hex(expires_at) END) ELSE expires_at END, 'count', CASE WHEN typeof(count) = 'blob' THEN (CASE WHEN json_valid(CAST(count AS TEXT)) THEN CAST(count AS TEXT) ELSE hex(count) END) ELSE count END)) AS TEXT) FROM commander_shop_group_purchases WHERE commander_shop_group_purchases.commander_id = sqlc.arg(id);

-- name: AdminDeleteCommanderShopGroupPurchases :exec
DELETE FROM commander_shop_group_purchases WHERE commander_shop_group_purchases.commander_id = sqlc.arg(id);

-- name: AdminExportCommanderShopPurchases :one
SELECT CAST(json_group_array(json_object('commander_id', CASE WHEN typeof(commander_id) = 'blob' THEN (CASE WHEN json_valid(CAST(commander_id AS TEXT)) THEN CAST(commander_id AS TEXT) ELSE hex(commander_id) END) ELSE commander_id END, 'shop_id', CASE WHEN typeof(shop_id) = 'blob' THEN (CASE WHEN json_valid(CAST(shop_id AS TEXT)) THEN CAST(shop_id AS TEXT) ELSE hex(shop_id) END) ELSE shop_id END, 'expires_at', CASE WHEN typeof(expires_at) = 'blob' THEN (CASE WHEN json_valid(CAST(expires_at AS TEXT)) THEN CAST(expires_at AS TEXT) ELSE hex(expires_at) END) ELSE expires_at END, 'count', CASE WHEN typeof(count) = 'blob' THEN (CASE WHEN json_valid(CAST(count AS TEXT)) THEN CAST(count AS TEXT) ELSE hex(count) END) ELSE count END)) AS TEXT) FROM commander_shop_purchases WHERE commander_shop_purchases.commander_id = sqlc.arg(id);

-- name: AdminDeleteCommanderShopPurchases :exec
DELETE FROM commander_shop_purchases WHERE commander_shop_purchases.commander_id = sqlc.arg(id);

-- name: AdminExportCommanderSkillClasses :one
SELECT CAST(json_group_array(json_object('commander_id', CASE WHEN typeof(commander_id) = 'blob' THEN (CASE WHEN json_valid(CAST(commander_id AS TEXT)) THEN CAST(commander_id AS TEXT) ELSE hex(commander_id) END) ELSE commander_id END, 'room_id', CASE WHEN typeof(room_id) = 'blob' THEN (CASE WHEN json_valid(CAST(room_id AS TEXT)) THEN CAST(room_id AS TEXT) ELSE hex(room_id) END) ELSE room_id END, 'ship_id', CASE WHEN typeof(ship_id) = 'blob' THEN (CASE WHEN json_valid(CAST(ship_id AS TEXT)) THEN CAST(ship_id AS TEXT) ELSE hex(ship_id) END) ELSE ship_id END, 'skill_id', CASE WHEN typeof(skill_id) = 'blob' THEN (CASE WHEN json_valid(CAST(skill_id AS TEXT)) THEN CAST(skill_id AS TEXT) ELSE hex(skill_id) END) ELSE skill_id END, 'item_id', CASE WHEN typeof(item_id) = 'blob' THEN (CASE WHEN json_valid(CAST(item_id AS TEXT)) THEN CAST(item_id AS TEXT) ELSE hex(item_id) END) ELSE item_id END, 'start_time', CASE WHEN typeof(start_time) = 'blob' THEN (CASE WHEN json_valid(CAST(start_time AS TEXT)) THEN CAST(start_time AS TEXT) ELSE hex(start_time) END) ELSE start_time END, 'finish_time', CASE WHEN typeof(finish_time) = 'blob' THEN (CASE WHEN json_valid(CAST(finish_time AS TEXT)) THEN CAST(finish_time AS TEXT) ELSE hex(finish_time) END) ELSE finish_time END, 'exp', CASE WHEN typeof(exp) = 'blob' THEN (CASE WHEN json_valid(CAST(exp AS TEXT)) THEN CAST(exp AS TEXT) ELSE hex(exp) END) ELSE exp END)) AS TEXT) FROM commander_skill_classes WHERE commander_skill_classes.commander_id = sqlc.arg(id) OR commander_skill_classes.ship_id IN (SELECT o.id FROM ships AS o WHERE o.commander_id = sqlc.arg(id));

-- name: AdminDeleteCommanderSkillClasses :exec
DELETE FROM commander_skill_classes WHERE commander_skill_classes.commander_id = sqlc.arg(id) OR commander_skill_classes.ship_id IN (SELECT o.id FROM ships AS o WHERE o.commander_id = sqlc.arg(id));

-- name: AdminExportCommanderSkins :one
SELECT CAST(json_group_array(json_object('commander_id', CASE WHEN typeof(commander_id) = 'blob' THEN (CASE WHEN json_valid(CAST(commander_id AS TEXT)) THEN CAST(commander_id AS TEXT) ELSE hex(commander_id) END) ELSE commander_id END, 'skin_id', CASE WHEN typeof(skin_id) = 'blob' THEN (CASE WHEN json_valid(CAST(skin_id AS TEXT)) THEN CAST(skin_id AS TEXT) ELSE hex(skin_id) END) ELSE skin_id END, 'end_time', CASE WHEN typeof(end_time) = 'blob' THEN (CASE WHEN json_valid(CAST(end_time AS TEXT)) THEN CAST(end_time AS TEXT) ELSE hex(end_time) END) ELSE end_time END)) AS TEXT) FROM commander_skins WHERE commander_skins.commander_id = sqlc.arg(id);

-- name: AdminDeleteCommanderSkins :exec
DELETE FROM commander_skins WHERE commander_skins.commander_id = sqlc.arg(id);

-- name: AdminExportCommanderStories :one
SELECT CAST(json_group_array(json_object('commander_id', CASE WHEN typeof(commander_id) = 'blob' THEN (CASE WHEN json_valid(CAST(commander_id AS TEXT)) THEN CAST(commander_id AS TEXT) ELSE hex(commander_id) END) ELSE commander_id END, 'story_id', CASE WHEN typeof(story_id) = 'blob' THEN (CASE WHEN json_valid(CAST(story_id AS TEXT)) THEN CAST(story_id AS TEXT) ELSE hex(story_id) END) ELSE story_id END)) AS TEXT) FROM commander_stories WHERE commander_stories.commander_id = sqlc.arg(id);

-- name: AdminDeleteCommanderStories :exec
DELETE FROM commander_stories WHERE commander_stories.commander_id = sqlc.arg(id);

-- name: AdminExportCommanderTaskCounters :one
SELECT CAST(json_group_array(json_object('commander_id', CASE WHEN typeof(commander_id) = 'blob' THEN (CASE WHEN json_valid(CAST(commander_id AS TEXT)) THEN CAST(commander_id AS TEXT) ELSE hex(commander_id) END) ELSE commander_id END, 'sub_type', CASE WHEN typeof(sub_type) = 'blob' THEN (CASE WHEN json_valid(CAST(sub_type AS TEXT)) THEN CAST(sub_type AS TEXT) ELSE hex(sub_type) END) ELSE sub_type END, 'target', CASE WHEN typeof(target) = 'blob' THEN (CASE WHEN json_valid(CAST(target AS TEXT)) THEN CAST(target AS TEXT) ELSE hex(target) END) ELSE target END, 'value', CASE WHEN typeof(value) = 'blob' THEN (CASE WHEN json_valid(CAST(value AS TEXT)) THEN CAST(value AS TEXT) ELSE hex(value) END) ELSE value END)) AS TEXT) FROM commander_task_counters WHERE commander_task_counters.commander_id = sqlc.arg(id);

-- name: AdminDeleteCommanderTaskCounters :exec
DELETE FROM commander_task_counters WHERE commander_task_counters.commander_id = sqlc.arg(id);

-- name: AdminExportCommanderTasks :one
SELECT CAST(json_group_array(json_object('commander_id', CASE WHEN typeof(commander_id) = 'blob' THEN (CASE WHEN json_valid(CAST(commander_id AS TEXT)) THEN CAST(commander_id AS TEXT) ELSE hex(commander_id) END) ELSE commander_id END, 'task_id', CASE WHEN typeof(task_id) = 'blob' THEN (CASE WHEN json_valid(CAST(task_id AS TEXT)) THEN CAST(task_id AS TEXT) ELSE hex(task_id) END) ELSE task_id END, 'progress', CASE WHEN typeof(progress) = 'blob' THEN (CASE WHEN json_valid(CAST(progress AS TEXT)) THEN CAST(progress AS TEXT) ELSE hex(progress) END) ELSE progress END, 'accept_time', CASE WHEN typeof(accept_time) = 'blob' THEN (CASE WHEN json_valid(CAST(accept_time AS TEXT)) THEN CAST(accept_time AS TEXT) ELSE hex(accept_time) END) ELSE accept_time END, 'submit_time', CASE WHEN typeof(submit_time) = 'blob' THEN (CASE WHEN json_valid(CAST(submit_time AS TEXT)) THEN CAST(submit_time AS TEXT) ELSE hex(submit_time) END) ELSE submit_time END)) AS TEXT) FROM commander_tasks WHERE commander_tasks.commander_id = sqlc.arg(id);

-- name: AdminDeleteCommanderTasks :exec
DELETE FROM commander_tasks WHERE commander_tasks.commander_id = sqlc.arg(id);

-- name: AdminExportCommanderTrophies :one
SELECT CAST(json_group_array(json_object('commander_id', CASE WHEN typeof(commander_id) = 'blob' THEN (CASE WHEN json_valid(CAST(commander_id AS TEXT)) THEN CAST(commander_id AS TEXT) ELSE hex(commander_id) END) ELSE commander_id END, 'trophy_id', CASE WHEN typeof(trophy_id) = 'blob' THEN (CASE WHEN json_valid(CAST(trophy_id AS TEXT)) THEN CAST(trophy_id AS TEXT) ELSE hex(trophy_id) END) ELSE trophy_id END, 'progress', CASE WHEN typeof(progress) = 'blob' THEN (CASE WHEN json_valid(CAST(progress AS TEXT)) THEN CAST(progress AS TEXT) ELSE hex(progress) END) ELSE progress END, 'timestamp', CASE WHEN typeof(timestamp) = 'blob' THEN (CASE WHEN json_valid(CAST(timestamp AS TEXT)) THEN CAST(timestamp AS TEXT) ELSE hex(timestamp) END) ELSE timestamp END)) AS TEXT) FROM commander_trophies WHERE commander_trophies.commander_id = sqlc.arg(id);

-- name: AdminDeleteCommanderTrophies :exec
DELETE FROM commander_trophies WHERE commander_trophies.commander_id = sqlc.arg(id);

-- name: AdminExportCommanderTrophyClaims :one
SELECT CAST(json_group_array(json_object('commander_id', CASE WHEN typeof(commander_id) = 'blob' THEN (CASE WHEN json_valid(CAST(commander_id AS TEXT)) THEN CAST(commander_id AS TEXT) ELSE hex(commander_id) END) ELSE commander_id END, 'trophy_id', CASE WHEN typeof(trophy_id) = 'blob' THEN (CASE WHEN json_valid(CAST(trophy_id AS TEXT)) THEN CAST(trophy_id AS TEXT) ELSE hex(trophy_id) END) ELSE trophy_id END, 'claimed_at', CASE WHEN typeof(claimed_at) = 'blob' THEN (CASE WHEN json_valid(CAST(claimed_at AS TEXT)) THEN CAST(claimed_at AS TEXT) ELSE hex(claimed_at) END) ELSE claimed_at END)) AS TEXT) FROM commander_trophy_claims WHERE commander_trophy_claims.commander_id = sqlc.arg(id);

-- name: AdminDeleteCommanderTrophyClaims :exec
DELETE FROM commander_trophy_claims WHERE commander_trophy_claims.commander_id = sqlc.arg(id);

-- name: AdminExportCommanderVirtualItems :one
SELECT CAST(json_group_array(json_object('commander_id', CASE WHEN typeof(commander_id) = 'blob' THEN (CASE WHEN json_valid(CAST(commander_id AS TEXT)) THEN CAST(commander_id AS TEXT) ELSE hex(commander_id) END) ELSE commander_id END, 'item_id', CASE WHEN typeof(item_id) = 'blob' THEN (CASE WHEN json_valid(CAST(item_id AS TEXT)) THEN CAST(item_id AS TEXT) ELSE hex(item_id) END) ELSE item_id END, 'amount', CASE WHEN typeof(amount) = 'blob' THEN (CASE WHEN json_valid(CAST(amount AS TEXT)) THEN CAST(amount AS TEXT) ELSE hex(amount) END) ELSE amount END)) AS TEXT) FROM commander_virtual_items WHERE commander_virtual_items.commander_id = sqlc.arg(id);

-- name: AdminDeleteCommanderVirtualItems :exec
DELETE FROM commander_virtual_items WHERE commander_virtual_items.commander_id = sqlc.arg(id);

-- name: AdminExportCommanderWeekly :one
SELECT CAST(json_group_array(json_object('commander_id', CASE WHEN typeof(commander_id) = 'blob' THEN (CASE WHEN json_valid(CAST(commander_id AS TEXT)) THEN CAST(commander_id AS TEXT) ELSE hex(commander_id) END) ELSE commander_id END, 'reward_lv', CASE WHEN typeof(reward_lv) = 'blob' THEN (CASE WHEN json_valid(CAST(reward_lv AS TEXT)) THEN CAST(reward_lv AS TEXT) ELSE hex(reward_lv) END) ELSE reward_lv END, 'week_start', CASE WHEN typeof(week_start) = 'blob' THEN (CASE WHEN json_valid(CAST(week_start AS TEXT)) THEN CAST(week_start AS TEXT) ELSE hex(week_start) END) ELSE week_start END)) AS TEXT) FROM commander_weekly WHERE commander_weekly.commander_id = sqlc.arg(id);

-- name: AdminDeleteCommanderWeekly :exec
DELETE FROM commander_weekly WHERE commander_weekly.commander_id = sqlc.arg(id);

-- name: AdminExportCommanderWeeklyTasks :one
SELECT CAST(json_group_array(json_object('commander_id', CASE WHEN typeof(commander_id) = 'blob' THEN (CASE WHEN json_valid(CAST(commander_id AS TEXT)) THEN CAST(commander_id AS TEXT) ELSE hex(commander_id) END) ELSE commander_id END, 'task_id', CASE WHEN typeof(task_id) = 'blob' THEN (CASE WHEN json_valid(CAST(task_id AS TEXT)) THEN CAST(task_id AS TEXT) ELSE hex(task_id) END) ELSE task_id END, 'progress', CASE WHEN typeof(progress) = 'blob' THEN (CASE WHEN json_valid(CAST(progress AS TEXT)) THEN CAST(progress AS TEXT) ELSE hex(progress) END) ELSE progress END, 'claimed', CASE WHEN typeof(claimed) = 'blob' THEN (CASE WHEN json_valid(CAST(claimed AS TEXT)) THEN CAST(claimed AS TEXT) ELSE hex(claimed) END) ELSE claimed END)) AS TEXT) FROM commander_weekly_tasks WHERE commander_weekly_tasks.commander_id = sqlc.arg(id);

-- name: AdminDeleteCommanderWeeklyTasks :exec
DELETE FROM commander_weekly_tasks WHERE commander_weekly_tasks.commander_id = sqlc.arg(id);

-- name: AdminExportCompensations :one
SELECT CAST(json_group_array(json_object('id', CASE WHEN typeof(id) = 'blob' THEN (CASE WHEN json_valid(CAST(id AS TEXT)) THEN CAST(id AS TEXT) ELSE hex(id) END) ELSE id END, 'commander_id', CASE WHEN typeof(commander_id) = 'blob' THEN (CASE WHEN json_valid(CAST(commander_id AS TEXT)) THEN CAST(commander_id AS TEXT) ELSE hex(commander_id) END) ELSE commander_id END, 'date', CASE WHEN typeof(date) = 'blob' THEN (CASE WHEN json_valid(CAST(date AS TEXT)) THEN CAST(date AS TEXT) ELSE hex(date) END) ELSE date END, 'expires', CASE WHEN typeof(expires) = 'blob' THEN (CASE WHEN json_valid(CAST(expires AS TEXT)) THEN CAST(expires AS TEXT) ELSE hex(expires) END) ELSE expires END, 'title', CASE WHEN typeof(title) = 'blob' THEN (CASE WHEN json_valid(CAST(title AS TEXT)) THEN CAST(title AS TEXT) ELSE hex(title) END) ELSE title END, 'sender', CASE WHEN typeof(sender) = 'blob' THEN (CASE WHEN json_valid(CAST(sender AS TEXT)) THEN CAST(sender AS TEXT) ELSE hex(sender) END) ELSE sender END, 'content', CASE WHEN typeof(content) = 'blob' THEN (CASE WHEN json_valid(CAST(content AS TEXT)) THEN CAST(content AS TEXT) ELSE hex(content) END) ELSE content END, 'attachments', CASE WHEN typeof(attachments) = 'blob' THEN (CASE WHEN json_valid(CAST(attachments AS TEXT)) THEN CAST(attachments AS TEXT) ELSE hex(attachments) END) ELSE attachments END, 'is_claimed', CASE WHEN typeof(is_claimed) = 'blob' THEN (CASE WHEN json_valid(CAST(is_claimed AS TEXT)) THEN CAST(is_claimed AS TEXT) ELSE hex(is_claimed) END) ELSE is_claimed END)) AS TEXT) FROM compensations WHERE compensations.commander_id = sqlc.arg(id);

-- name: AdminDeleteCompensations :exec
DELETE FROM compensations WHERE compensations.commander_id = sqlc.arg(id);

-- name: AdminExportDailyLevelCounts :one
SELECT CAST(json_group_array(json_object('commander_id', CASE WHEN typeof(commander_id) = 'blob' THEN (CASE WHEN json_valid(CAST(commander_id AS TEXT)) THEN CAST(commander_id AS TEXT) ELSE hex(commander_id) END) ELSE commander_id END, 'daily_id', CASE WHEN typeof(daily_id) = 'blob' THEN (CASE WHEN json_valid(CAST(daily_id AS TEXT)) THEN CAST(daily_id AS TEXT) ELSE hex(daily_id) END) ELSE daily_id END, 'count', CASE WHEN typeof(count) = 'blob' THEN (CASE WHEN json_valid(CAST(count AS TEXT)) THEN CAST(count AS TEXT) ELSE hex(count) END) ELSE count END, 'period', CASE WHEN typeof(period) = 'blob' THEN (CASE WHEN json_valid(CAST(period AS TEXT)) THEN CAST(period AS TEXT) ELSE hex(period) END) ELSE period END)) AS TEXT) FROM daily_level_counts WHERE daily_level_counts.commander_id = sqlc.arg(id);

-- name: AdminDeleteDailyLevelCounts :exec
DELETE FROM daily_level_counts WHERE daily_level_counts.commander_id = sqlc.arg(id);

-- name: AdminExportDailyQuickStages :one
SELECT CAST(json_group_array(json_object('commander_id', CASE WHEN typeof(commander_id) = 'blob' THEN (CASE WHEN json_valid(CAST(commander_id AS TEXT)) THEN CAST(commander_id AS TEXT) ELSE hex(commander_id) END) ELSE commander_id END, 'stage_id', CASE WHEN typeof(stage_id) = 'blob' THEN (CASE WHEN json_valid(CAST(stage_id AS TEXT)) THEN CAST(stage_id AS TEXT) ELSE hex(stage_id) END) ELSE stage_id END)) AS TEXT) FROM daily_quick_stages WHERE daily_quick_stages.commander_id = sqlc.arg(id);

-- name: AdminDeleteDailyQuickStages :exec
DELETE FROM daily_quick_stages WHERE daily_quick_stages.commander_id = sqlc.arg(id);

-- name: AdminExportDormFloors :one
SELECT CAST(json_group_array(json_object('commander_id', CASE WHEN typeof(commander_id) = 'blob' THEN (CASE WHEN json_valid(CAST(commander_id AS TEXT)) THEN CAST(commander_id AS TEXT) ELSE hex(commander_id) END) ELSE commander_id END, 'floor', CASE WHEN typeof(floor) = 'blob' THEN (CASE WHEN json_valid(CAST(floor AS TEXT)) THEN CAST(floor AS TEXT) ELSE hex(floor) END) ELSE floor END, 'layout', CASE WHEN typeof(layout) = 'blob' THEN (CASE WHEN json_valid(CAST(layout AS TEXT)) THEN CAST(layout AS TEXT) ELSE hex(layout) END) ELSE layout END)) AS TEXT) FROM dorm_floors WHERE dorm_floors.commander_id = sqlc.arg(id);

-- name: AdminDeleteDormFloors :exec
DELETE FROM dorm_floors WHERE dorm_floors.commander_id = sqlc.arg(id);

-- name: AdminExportDormShips :one
SELECT CAST(json_group_array(json_object('ship_id', CASE WHEN typeof(ship_id) = 'blob' THEN (CASE WHEN json_valid(CAST(ship_id AS TEXT)) THEN CAST(ship_id AS TEXT) ELSE hex(ship_id) END) ELSE ship_id END, 'commander_id', CASE WHEN typeof(commander_id) = 'blob' THEN (CASE WHEN json_valid(CAST(commander_id AS TEXT)) THEN CAST(commander_id AS TEXT) ELSE hex(commander_id) END) ELSE commander_id END, 'floor', CASE WHEN typeof(floor) = 'blob' THEN (CASE WHEN json_valid(CAST(floor AS TEXT)) THEN CAST(floor AS TEXT) ELSE hex(floor) END) ELSE floor END, 'pop_icon', CASE WHEN typeof(pop_icon) = 'blob' THEN (CASE WHEN json_valid(CAST(pop_icon AS TEXT)) THEN CAST(pop_icon AS TEXT) ELSE hex(pop_icon) END) ELSE pop_icon END, 'pop_intimacy', CASE WHEN typeof(pop_intimacy) = 'blob' THEN (CASE WHEN json_valid(CAST(pop_intimacy AS TEXT)) THEN CAST(pop_intimacy AS TEXT) ELSE hex(pop_intimacy) END) ELSE pop_intimacy END, 'pop_at', CASE WHEN typeof(pop_at) = 'blob' THEN (CASE WHEN json_valid(CAST(pop_at AS TEXT)) THEN CAST(pop_at AS TEXT) ELSE hex(pop_at) END) ELSE pop_at END)) AS TEXT) FROM dorm_ships WHERE dorm_ships.commander_id = sqlc.arg(id) OR dorm_ships.ship_id IN (SELECT o.id FROM ships AS o WHERE o.commander_id = sqlc.arg(id));

-- name: AdminDeleteDormShips :exec
DELETE FROM dorm_ships WHERE dorm_ships.commander_id = sqlc.arg(id) OR dorm_ships.ship_id IN (SELECT o.id FROM ships AS o WHERE o.commander_id = sqlc.arg(id));

-- name: AdminExportDormThemeFavs :one
SELECT CAST(json_group_array(json_object('commander_id', CASE WHEN typeof(commander_id) = 'blob' THEN (CASE WHEN json_valid(CAST(commander_id AS TEXT)) THEN CAST(commander_id AS TEXT) ELSE hex(commander_id) END) ELSE commander_id END, 'theme_id', CASE WHEN typeof(theme_id) = 'blob' THEN (CASE WHEN json_valid(CAST(theme_id AS TEXT)) THEN CAST(theme_id AS TEXT) ELSE hex(theme_id) END) ELSE theme_id END, 'upload_time', CASE WHEN typeof(upload_time) = 'blob' THEN (CASE WHEN json_valid(CAST(upload_time AS TEXT)) THEN CAST(upload_time AS TEXT) ELSE hex(upload_time) END) ELSE upload_time END, 'faved_at', CASE WHEN typeof(faved_at) = 'blob' THEN (CASE WHEN json_valid(CAST(faved_at AS TEXT)) THEN CAST(faved_at AS TEXT) ELSE hex(faved_at) END) ELSE faved_at END)) AS TEXT) FROM dorm_theme_favs WHERE dorm_theme_favs.commander_id = sqlc.arg(id);

-- name: AdminDeleteDormThemeFavs :exec
DELETE FROM dorm_theme_favs WHERE dorm_theme_favs.commander_id = sqlc.arg(id);

-- name: AdminExportDormThemeLikes :one
SELECT CAST(json_group_array(json_object('theme_id', CASE WHEN typeof(theme_id) = 'blob' THEN (CASE WHEN json_valid(CAST(theme_id AS TEXT)) THEN CAST(theme_id AS TEXT) ELSE hex(theme_id) END) ELSE theme_id END, 'commander_id', CASE WHEN typeof(commander_id) = 'blob' THEN (CASE WHEN json_valid(CAST(commander_id AS TEXT)) THEN CAST(commander_id AS TEXT) ELSE hex(commander_id) END) ELSE commander_id END)) AS TEXT) FROM dorm_theme_likes WHERE dorm_theme_likes.commander_id = sqlc.arg(id);

-- name: AdminDeleteDormThemeLikes :exec
DELETE FROM dorm_theme_likes WHERE dorm_theme_likes.commander_id = sqlc.arg(id);

-- name: AdminExportDormThemes :one
SELECT CAST(json_group_array(json_object('commander_id', CASE WHEN typeof(commander_id) = 'blob' THEN (CASE WHEN json_valid(CAST(commander_id AS TEXT)) THEN CAST(commander_id AS TEXT) ELSE hex(commander_id) END) ELSE commander_id END, 'pos', CASE WHEN typeof(pos) = 'blob' THEN (CASE WHEN json_valid(CAST(pos AS TEXT)) THEN CAST(pos AS TEXT) ELSE hex(pos) END) ELSE pos END, 'theme', CASE WHEN typeof(theme) = 'blob' THEN (CASE WHEN json_valid(CAST(theme AS TEXT)) THEN CAST(theme AS TEXT) ELSE hex(theme) END) ELSE theme END, 'theme_id', CASE WHEN typeof(theme_id) = 'blob' THEN (CASE WHEN json_valid(CAST(theme_id AS TEXT)) THEN CAST(theme_id AS TEXT) ELSE hex(theme_id) END) ELSE theme_id END, 'upload_time', CASE WHEN typeof(upload_time) = 'blob' THEN (CASE WHEN json_valid(CAST(upload_time AS TEXT)) THEN CAST(upload_time AS TEXT) ELSE hex(upload_time) END) ELSE upload_time END)) AS TEXT) FROM dorm_themes WHERE dorm_themes.commander_id = sqlc.arg(id);

-- name: AdminDeleteDormThemes :exec
DELETE FROM dorm_themes WHERE dorm_themes.commander_id = sqlc.arg(id);

-- name: AdminExportEquipCodeLikes :one
SELECT CAST(json_group_array(json_object('share_id', CASE WHEN typeof(share_id) = 'blob' THEN (CASE WHEN json_valid(CAST(share_id AS TEXT)) THEN CAST(share_id AS TEXT) ELSE hex(share_id) END) ELSE share_id END, 'commander_id', CASE WHEN typeof(commander_id) = 'blob' THEN (CASE WHEN json_valid(CAST(commander_id AS TEXT)) THEN CAST(commander_id AS TEXT) ELSE hex(commander_id) END) ELSE commander_id END, 'liked_at', CASE WHEN typeof(liked_at) = 'blob' THEN (CASE WHEN json_valid(CAST(liked_at AS TEXT)) THEN CAST(liked_at AS TEXT) ELSE hex(liked_at) END) ELSE liked_at END)) AS TEXT) FROM equip_code_likes WHERE equip_code_likes.commander_id = sqlc.arg(id) OR equip_code_likes.share_id IN (SELECT o.id FROM equip_codes AS o WHERE o.commander_id = sqlc.arg(id));

-- name: AdminDeleteEquipCodeLikes :exec
DELETE FROM equip_code_likes WHERE equip_code_likes.commander_id = sqlc.arg(id) OR equip_code_likes.share_id IN (SELECT o.id FROM equip_codes AS o WHERE o.commander_id = sqlc.arg(id));

-- name: AdminExportEquipCodeReports :one
SELECT CAST(json_group_array(json_object('share_id', CASE WHEN typeof(share_id) = 'blob' THEN (CASE WHEN json_valid(CAST(share_id AS TEXT)) THEN CAST(share_id AS TEXT) ELSE hex(share_id) END) ELSE share_id END, 'commander_id', CASE WHEN typeof(commander_id) = 'blob' THEN (CASE WHEN json_valid(CAST(commander_id AS TEXT)) THEN CAST(commander_id AS TEXT) ELSE hex(commander_id) END) ELSE commander_id END, 'report_type', CASE WHEN typeof(report_type) = 'blob' THEN (CASE WHEN json_valid(CAST(report_type AS TEXT)) THEN CAST(report_type AS TEXT) ELSE hex(report_type) END) ELSE report_type END, 'reported_at', CASE WHEN typeof(reported_at) = 'blob' THEN (CASE WHEN json_valid(CAST(reported_at AS TEXT)) THEN CAST(reported_at AS TEXT) ELSE hex(reported_at) END) ELSE reported_at END)) AS TEXT) FROM equip_code_reports WHERE equip_code_reports.commander_id = sqlc.arg(id) OR equip_code_reports.share_id IN (SELECT o.id FROM equip_codes AS o WHERE o.commander_id = sqlc.arg(id));

-- name: AdminDeleteEquipCodeReports :exec
DELETE FROM equip_code_reports WHERE equip_code_reports.commander_id = sqlc.arg(id) OR equip_code_reports.share_id IN (SELECT o.id FROM equip_codes AS o WHERE o.commander_id = sqlc.arg(id));

-- name: AdminExportEquipCodes :one
SELECT CAST(json_group_array(json_object('id', CASE WHEN typeof(id) = 'blob' THEN (CASE WHEN json_valid(CAST(id AS TEXT)) THEN CAST(id AS TEXT) ELSE hex(id) END) ELSE id END, 'group_id', CASE WHEN typeof(group_id) = 'blob' THEN (CASE WHEN json_valid(CAST(group_id AS TEXT)) THEN CAST(group_id AS TEXT) ELSE hex(group_id) END) ELSE group_id END, 'commander_id', CASE WHEN typeof(commander_id) = 'blob' THEN (CASE WHEN json_valid(CAST(commander_id AS TEXT)) THEN CAST(commander_id AS TEXT) ELSE hex(commander_id) END) ELSE commander_id END, 'eqcode', CASE WHEN typeof(eqcode) = 'blob' THEN (CASE WHEN json_valid(CAST(eqcode AS TEXT)) THEN CAST(eqcode AS TEXT) ELSE hex(eqcode) END) ELSE eqcode END, 'likes', CASE WHEN typeof(likes) = 'blob' THEN (CASE WHEN json_valid(CAST(likes AS TEXT)) THEN CAST(likes AS TEXT) ELSE hex(likes) END) ELSE likes END, 'shared_at', CASE WHEN typeof(shared_at) = 'blob' THEN (CASE WHEN json_valid(CAST(shared_at AS TEXT)) THEN CAST(shared_at AS TEXT) ELSE hex(shared_at) END) ELSE shared_at END)) AS TEXT) FROM equip_codes WHERE equip_codes.commander_id = sqlc.arg(id);

-- name: AdminDeleteEquipCodes :exec
DELETE FROM equip_codes WHERE equip_codes.commander_id = sqlc.arg(id);

-- name: AdminExportExchangeCodeRedemptions :one
SELECT CAST(json_group_array(json_object('commander_id', CASE WHEN typeof(commander_id) = 'blob' THEN (CASE WHEN json_valid(CAST(commander_id AS TEXT)) THEN CAST(commander_id AS TEXT) ELSE hex(commander_id) END) ELSE commander_id END, 'code', CASE WHEN typeof(code) = 'blob' THEN (CASE WHEN json_valid(CAST(code AS TEXT)) THEN CAST(code AS TEXT) ELSE hex(code) END) ELSE code END, 'redeemed_at', CASE WHEN typeof(redeemed_at) = 'blob' THEN (CASE WHEN json_valid(CAST(redeemed_at AS TEXT)) THEN CAST(redeemed_at AS TEXT) ELSE hex(redeemed_at) END) ELSE redeemed_at END)) AS TEXT) FROM exchange_code_redemptions WHERE exchange_code_redemptions.commander_id = sqlc.arg(id);

-- name: AdminDeleteExchangeCodeRedemptions :exec
DELETE FROM exchange_code_redemptions WHERE exchange_code_redemptions.commander_id = sqlc.arg(id);

-- name: AdminExportFleetMeowfficers :one
SELECT CAST(json_group_array(json_object('commander_id', CASE WHEN typeof(commander_id) = 'blob' THEN (CASE WHEN json_valid(CAST(commander_id AS TEXT)) THEN CAST(commander_id AS TEXT) ELSE hex(commander_id) END) ELSE commander_id END, 'fleet_id', CASE WHEN typeof(fleet_id) = 'blob' THEN (CASE WHEN json_valid(CAST(fleet_id AS TEXT)) THEN CAST(fleet_id AS TEXT) ELSE hex(fleet_id) END) ELSE fleet_id END, 'pos', CASE WHEN typeof(pos) = 'blob' THEN (CASE WHEN json_valid(CAST(pos AS TEXT)) THEN CAST(pos AS TEXT) ELSE hex(pos) END) ELSE pos END, 'meowfficer_id', CASE WHEN typeof(meowfficer_id) = 'blob' THEN (CASE WHEN json_valid(CAST(meowfficer_id AS TEXT)) THEN CAST(meowfficer_id AS TEXT) ELSE hex(meowfficer_id) END) ELSE meowfficer_id END)) AS TEXT) FROM fleet_meowfficers WHERE fleet_meowfficers.commander_id = sqlc.arg(id) OR fleet_meowfficers.meowfficer_id IN (SELECT o.id FROM meowfficers AS o WHERE o.commander_id = sqlc.arg(id));

-- name: AdminDeleteFleetMeowfficers :exec
DELETE FROM fleet_meowfficers WHERE fleet_meowfficers.commander_id = sqlc.arg(id) OR fleet_meowfficers.meowfficer_id IN (SELECT o.id FROM meowfficers AS o WHERE o.commander_id = sqlc.arg(id));

-- name: AdminExportFleetShips :one
SELECT CAST(json_group_array(json_object('commander_id', CASE WHEN typeof(commander_id) = 'blob' THEN (CASE WHEN json_valid(CAST(commander_id AS TEXT)) THEN CAST(commander_id AS TEXT) ELSE hex(commander_id) END) ELSE commander_id END, 'fleet_id', CASE WHEN typeof(fleet_id) = 'blob' THEN (CASE WHEN json_valid(CAST(fleet_id AS TEXT)) THEN CAST(fleet_id AS TEXT) ELSE hex(fleet_id) END) ELSE fleet_id END, 'position', CASE WHEN typeof(position) = 'blob' THEN (CASE WHEN json_valid(CAST(position AS TEXT)) THEN CAST(position AS TEXT) ELSE hex(position) END) ELSE position END, 'ship_id', CASE WHEN typeof(ship_id) = 'blob' THEN (CASE WHEN json_valid(CAST(ship_id AS TEXT)) THEN CAST(ship_id AS TEXT) ELSE hex(ship_id) END) ELSE ship_id END)) AS TEXT) FROM fleet_ships WHERE fleet_ships.commander_id = sqlc.arg(id);

-- name: AdminDeleteFleetShips :exec
DELETE FROM fleet_ships WHERE fleet_ships.commander_id = sqlc.arg(id);

-- name: AdminExportFleetTechAttrs :one
SELECT CAST(json_group_array(json_object('commander_id', CASE WHEN typeof(commander_id) = 'blob' THEN (CASE WHEN json_valid(CAST(commander_id AS TEXT)) THEN CAST(commander_id AS TEXT) ELSE hex(commander_id) END) ELSE commander_id END, 'ship_type', CASE WHEN typeof(ship_type) = 'blob' THEN (CASE WHEN json_valid(CAST(ship_type AS TEXT)) THEN CAST(ship_type AS TEXT) ELSE hex(ship_type) END) ELSE ship_type END, 'attr_type', CASE WHEN typeof(attr_type) = 'blob' THEN (CASE WHEN json_valid(CAST(attr_type AS TEXT)) THEN CAST(attr_type AS TEXT) ELSE hex(attr_type) END) ELSE attr_type END, 'set_value', CASE WHEN typeof(set_value) = 'blob' THEN (CASE WHEN json_valid(CAST(set_value AS TEXT)) THEN CAST(set_value AS TEXT) ELSE hex(set_value) END) ELSE set_value END)) AS TEXT) FROM fleet_tech_attrs WHERE fleet_tech_attrs.commander_id = sqlc.arg(id);

-- name: AdminDeleteFleetTechAttrs :exec
DELETE FROM fleet_tech_attrs WHERE fleet_tech_attrs.commander_id = sqlc.arg(id);

-- name: AdminExportFleetTechs :one
SELECT CAST(json_group_array(json_object('commander_id', CASE WHEN typeof(commander_id) = 'blob' THEN (CASE WHEN json_valid(CAST(commander_id AS TEXT)) THEN CAST(commander_id AS TEXT) ELSE hex(commander_id) END) ELSE commander_id END, 'group_id', CASE WHEN typeof(group_id) = 'blob' THEN (CASE WHEN json_valid(CAST(group_id AS TEXT)) THEN CAST(group_id AS TEXT) ELSE hex(group_id) END) ELSE group_id END, 'effect_tech_id', CASE WHEN typeof(effect_tech_id) = 'blob' THEN (CASE WHEN json_valid(CAST(effect_tech_id AS TEXT)) THEN CAST(effect_tech_id AS TEXT) ELSE hex(effect_tech_id) END) ELSE effect_tech_id END, 'study_tech_id', CASE WHEN typeof(study_tech_id) = 'blob' THEN (CASE WHEN json_valid(CAST(study_tech_id AS TEXT)) THEN CAST(study_tech_id AS TEXT) ELSE hex(study_tech_id) END) ELSE study_tech_id END, 'study_finish_at', CASE WHEN typeof(study_finish_at) = 'blob' THEN (CASE WHEN json_valid(CAST(study_finish_at AS TEXT)) THEN CAST(study_finish_at AS TEXT) ELSE hex(study_finish_at) END) ELSE study_finish_at END, 'rewarded_tech_id', CASE WHEN typeof(rewarded_tech_id) = 'blob' THEN (CASE WHEN json_valid(CAST(rewarded_tech_id AS TEXT)) THEN CAST(rewarded_tech_id AS TEXT) ELSE hex(rewarded_tech_id) END) ELSE rewarded_tech_id END)) AS TEXT) FROM fleet_techs WHERE fleet_techs.commander_id = sqlc.arg(id);

-- name: AdminDeleteFleetTechs :exec
DELETE FROM fleet_techs WHERE fleet_techs.commander_id = sqlc.arg(id);

-- name: AdminExportFleets :one
SELECT CAST(json_group_array(json_object('commander_id', CASE WHEN typeof(commander_id) = 'blob' THEN (CASE WHEN json_valid(CAST(commander_id AS TEXT)) THEN CAST(commander_id AS TEXT) ELSE hex(commander_id) END) ELSE commander_id END, 'fleet_id', CASE WHEN typeof(fleet_id) = 'blob' THEN (CASE WHEN json_valid(CAST(fleet_id AS TEXT)) THEN CAST(fleet_id AS TEXT) ELSE hex(fleet_id) END) ELSE fleet_id END, 'name', CASE WHEN typeof(name) = 'blob' THEN (CASE WHEN json_valid(CAST(name AS TEXT)) THEN CAST(name AS TEXT) ELSE hex(name) END) ELSE name END)) AS TEXT) FROM fleets WHERE fleets.commander_id = sqlc.arg(id);

-- name: AdminDeleteFleets :exec
DELETE FROM fleets WHERE fleets.commander_id = sqlc.arg(id);

-- name: AdminExportFriendRequests :one
SELECT CAST(json_group_array(json_object('commander_id', CASE WHEN typeof(commander_id) = 'blob' THEN (CASE WHEN json_valid(CAST(commander_id AS TEXT)) THEN CAST(commander_id AS TEXT) ELSE hex(commander_id) END) ELSE commander_id END, 'sender_id', CASE WHEN typeof(sender_id) = 'blob' THEN (CASE WHEN json_valid(CAST(sender_id AS TEXT)) THEN CAST(sender_id AS TEXT) ELSE hex(sender_id) END) ELSE sender_id END, 'content', CASE WHEN typeof(content) = 'blob' THEN (CASE WHEN json_valid(CAST(content AS TEXT)) THEN CAST(content AS TEXT) ELSE hex(content) END) ELSE content END, 'sent_at', CASE WHEN typeof(sent_at) = 'blob' THEN (CASE WHEN json_valid(CAST(sent_at AS TEXT)) THEN CAST(sent_at AS TEXT) ELSE hex(sent_at) END) ELSE sent_at END)) AS TEXT) FROM friend_requests WHERE friend_requests.commander_id = sqlc.arg(id) OR friend_requests.sender_id = sqlc.arg(id);

-- name: AdminDeleteFriendRequests :exec
DELETE FROM friend_requests WHERE friend_requests.commander_id = sqlc.arg(id) OR friend_requests.sender_id = sqlc.arg(id);

-- name: AdminExportFriends :one
SELECT CAST(json_group_array(json_object('commander_id', CASE WHEN typeof(commander_id) = 'blob' THEN (CASE WHEN json_valid(CAST(commander_id AS TEXT)) THEN CAST(commander_id AS TEXT) ELSE hex(commander_id) END) ELSE commander_id END, 'friend_id', CASE WHEN typeof(friend_id) = 'blob' THEN (CASE WHEN json_valid(CAST(friend_id AS TEXT)) THEN CAST(friend_id AS TEXT) ELSE hex(friend_id) END) ELSE friend_id END, 'since', CASE WHEN typeof(since) = 'blob' THEN (CASE WHEN json_valid(CAST(since AS TEXT)) THEN CAST(since AS TEXT) ELSE hex(since) END) ELSE since END)) AS TEXT) FROM friends WHERE friends.commander_id = sqlc.arg(id) OR friends.friend_id = sqlc.arg(id);

-- name: AdminDeleteFriends :exec
DELETE FROM friends WHERE friends.commander_id = sqlc.arg(id) OR friends.friend_id = sqlc.arg(id);

-- name: AdminExportGuildAssaultShips :one
SELECT CAST(json_group_array(json_object('commander_id', CASE WHEN typeof(commander_id) = 'blob' THEN (CASE WHEN json_valid(CAST(commander_id AS TEXT)) THEN CAST(commander_id AS TEXT) ELSE hex(commander_id) END) ELSE commander_id END, 'pos', CASE WHEN typeof(pos) = 'blob' THEN (CASE WHEN json_valid(CAST(pos AS TEXT)) THEN CAST(pos AS TEXT) ELSE hex(pos) END) ELSE pos END, 'ship_id', CASE WHEN typeof(ship_id) = 'blob' THEN (CASE WHEN json_valid(CAST(ship_id AS TEXT)) THEN CAST(ship_id AS TEXT) ELSE hex(ship_id) END) ELSE ship_id END, 'recommended', CASE WHEN typeof(recommended) = 'blob' THEN (CASE WHEN json_valid(CAST(recommended AS TEXT)) THEN CAST(recommended AS TEXT) ELSE hex(recommended) END) ELSE recommended END, 'changed_at', CASE WHEN typeof(changed_at) = 'blob' THEN (CASE WHEN json_valid(CAST(changed_at AS TEXT)) THEN CAST(changed_at AS TEXT) ELSE hex(changed_at) END) ELSE changed_at END)) AS TEXT) FROM guild_assault_ships WHERE guild_assault_ships.commander_id = sqlc.arg(id) OR guild_assault_ships.ship_id IN (SELECT o.id FROM ships AS o WHERE o.commander_id = sqlc.arg(id));

-- name: AdminDeleteGuildAssaultShips :exec
DELETE FROM guild_assault_ships WHERE guild_assault_ships.commander_id = sqlc.arg(id) OR guild_assault_ships.ship_id IN (SELECT o.id FROM ships AS o WHERE o.commander_id = sqlc.arg(id));

-- name: AdminExportGuildBossFleetMeowfficers :one
SELECT CAST(json_group_array(json_object('operation_id', CASE WHEN typeof(operation_id) = 'blob' THEN (CASE WHEN json_valid(CAST(operation_id AS TEXT)) THEN CAST(operation_id AS TEXT) ELSE hex(operation_id) END) ELSE operation_id END, 'commander_id', CASE WHEN typeof(commander_id) = 'blob' THEN (CASE WHEN json_valid(CAST(commander_id AS TEXT)) THEN CAST(commander_id AS TEXT) ELSE hex(commander_id) END) ELSE commander_id END, 'fleet_id', CASE WHEN typeof(fleet_id) = 'blob' THEN (CASE WHEN json_valid(CAST(fleet_id AS TEXT)) THEN CAST(fleet_id AS TEXT) ELSE hex(fleet_id) END) ELSE fleet_id END, 'pos', CASE WHEN typeof(pos) = 'blob' THEN (CASE WHEN json_valid(CAST(pos AS TEXT)) THEN CAST(pos AS TEXT) ELSE hex(pos) END) ELSE pos END, 'meowfficer_id', CASE WHEN typeof(meowfficer_id) = 'blob' THEN (CASE WHEN json_valid(CAST(meowfficer_id AS TEXT)) THEN CAST(meowfficer_id AS TEXT) ELSE hex(meowfficer_id) END) ELSE meowfficer_id END)) AS TEXT) FROM guild_boss_fleet_meowfficers WHERE guild_boss_fleet_meowfficers.commander_id = sqlc.arg(id);

-- name: AdminDeleteGuildBossFleetMeowfficers :exec
DELETE FROM guild_boss_fleet_meowfficers WHERE guild_boss_fleet_meowfficers.commander_id = sqlc.arg(id);

-- name: AdminExportGuildBossFleetShips :one
SELECT CAST(json_group_array(json_object('operation_id', CASE WHEN typeof(operation_id) = 'blob' THEN (CASE WHEN json_valid(CAST(operation_id AS TEXT)) THEN CAST(operation_id AS TEXT) ELSE hex(operation_id) END) ELSE operation_id END, 'commander_id', CASE WHEN typeof(commander_id) = 'blob' THEN (CASE WHEN json_valid(CAST(commander_id AS TEXT)) THEN CAST(commander_id AS TEXT) ELSE hex(commander_id) END) ELSE commander_id END, 'fleet_id', CASE WHEN typeof(fleet_id) = 'blob' THEN (CASE WHEN json_valid(CAST(fleet_id AS TEXT)) THEN CAST(fleet_id AS TEXT) ELSE hex(fleet_id) END) ELSE fleet_id END, 'slot', CASE WHEN typeof(slot) = 'blob' THEN (CASE WHEN json_valid(CAST(slot AS TEXT)) THEN CAST(slot AS TEXT) ELSE hex(slot) END) ELSE slot END, 'user_id', CASE WHEN typeof(user_id) = 'blob' THEN (CASE WHEN json_valid(CAST(user_id AS TEXT)) THEN CAST(user_id AS TEXT) ELSE hex(user_id) END) ELSE user_id END, 'ship_id', CASE WHEN typeof(ship_id) = 'blob' THEN (CASE WHEN json_valid(CAST(ship_id AS TEXT)) THEN CAST(ship_id AS TEXT) ELSE hex(ship_id) END) ELSE ship_id END)) AS TEXT) FROM guild_boss_fleet_ships WHERE guild_boss_fleet_ships.commander_id = sqlc.arg(id) OR guild_boss_fleet_ships.user_id = sqlc.arg(id);

-- name: AdminDeleteGuildBossFleetShips :exec
DELETE FROM guild_boss_fleet_ships WHERE guild_boss_fleet_ships.commander_id = sqlc.arg(id) OR guild_boss_fleet_ships.user_id = sqlc.arg(id);

-- name: AdminExportGuildChat :one
SELECT CAST(json_group_array(json_object('id', CASE WHEN typeof(id) = 'blob' THEN (CASE WHEN json_valid(CAST(id AS TEXT)) THEN CAST(id AS TEXT) ELSE hex(id) END) ELSE id END, 'guild_id', CASE WHEN typeof(guild_id) = 'blob' THEN (CASE WHEN json_valid(CAST(guild_id AS TEXT)) THEN CAST(guild_id AS TEXT) ELSE hex(guild_id) END) ELSE guild_id END, 'commander_id', CASE WHEN typeof(commander_id) = 'blob' THEN (CASE WHEN json_valid(CAST(commander_id AS TEXT)) THEN CAST(commander_id AS TEXT) ELSE hex(commander_id) END) ELSE commander_id END, 'content', CASE WHEN typeof(content) = 'blob' THEN (CASE WHEN json_valid(CAST(content AS TEXT)) THEN CAST(content AS TEXT) ELSE hex(content) END) ELSE content END, 'time', CASE WHEN typeof(time) = 'blob' THEN (CASE WHEN json_valid(CAST(time AS TEXT)) THEN CAST(time AS TEXT) ELSE hex(time) END) ELSE time END)) AS TEXT) FROM guild_chat WHERE guild_chat.commander_id = sqlc.arg(id);

-- name: AdminDeleteGuildChat :exec
DELETE FROM guild_chat WHERE guild_chat.commander_id = sqlc.arg(id);

-- name: AdminExportGuildMembers :one
SELECT CAST(json_group_array(json_object('commander_id', CASE WHEN typeof(commander_id) = 'blob' THEN (CASE WHEN json_valid(CAST(commander_id AS TEXT)) THEN CAST(commander_id AS TEXT) ELSE hex(commander_id) END) ELSE commander_id END, 'guild_id', CASE WHEN typeof(guild_id) = 'blob' THEN (CASE WHEN json_valid(CAST(guild_id AS TEXT)) THEN CAST(guild_id AS TEXT) ELSE hex(guild_id) END) ELSE guild_id END, 'duty', CASE WHEN typeof(duty) = 'blob' THEN (CASE WHEN json_valid(CAST(duty AS TEXT)) THEN CAST(duty AS TEXT) ELSE hex(duty) END) ELSE duty END, 'liveness', CASE WHEN typeof(liveness) = 'blob' THEN (CASE WHEN json_valid(CAST(liveness AS TEXT)) THEN CAST(liveness AS TEXT) ELSE hex(liveness) END) ELSE liveness END, 'join_time', CASE WHEN typeof(join_time) = 'blob' THEN (CASE WHEN json_valid(CAST(join_time AS TEXT)) THEN CAST(join_time AS TEXT) ELSE hex(join_time) END) ELSE join_time END)) AS TEXT) FROM guild_members WHERE guild_members.commander_id = sqlc.arg(id);

-- name: AdminDeleteGuildMembers :exec
DELETE FROM guild_members WHERE guild_members.commander_id = sqlc.arg(id);

-- name: AdminExportGuildOperationMembers :one
SELECT CAST(json_group_array(json_object('operation_id', CASE WHEN typeof(operation_id) = 'blob' THEN (CASE WHEN json_valid(CAST(operation_id AS TEXT)) THEN CAST(operation_id AS TEXT) ELSE hex(operation_id) END) ELSE operation_id END, 'commander_id', CASE WHEN typeof(commander_id) = 'blob' THEN (CASE WHEN json_valid(CAST(commander_id AS TEXT)) THEN CAST(commander_id AS TEXT) ELSE hex(commander_id) END) ELSE commander_id END, 'joined_at', CASE WHEN typeof(joined_at) = 'blob' THEN (CASE WHEN json_valid(CAST(joined_at AS TEXT)) THEN CAST(joined_at AS TEXT) ELSE hex(joined_at) END) ELSE joined_at END, 'extra', CASE WHEN typeof(extra) = 'blob' THEN (CASE WHEN json_valid(CAST(extra AS TEXT)) THEN CAST(extra AS TEXT) ELSE hex(extra) END) ELSE extra END, 'boss_day', CASE WHEN typeof(boss_day) = 'blob' THEN (CASE WHEN json_valid(CAST(boss_day AS TEXT)) THEN CAST(boss_day AS TEXT) ELSE hex(boss_day) END) ELSE boss_day END, 'boss_count', CASE WHEN typeof(boss_count) = 'blob' THEN (CASE WHEN json_valid(CAST(boss_count AS TEXT)) THEN CAST(boss_count AS TEXT) ELSE hex(boss_count) END) ELSE boss_count END, 'boss_damage', CASE WHEN typeof(boss_damage) = 'blob' THEN (CASE WHEN json_valid(CAST(boss_damage AS TEXT)) THEN CAST(boss_damage AS TEXT) ELSE hex(boss_damage) END) ELSE boss_damage END, 'boss_battle_at', CASE WHEN typeof(boss_battle_at) = 'blob' THEN (CASE WHEN json_valid(CAST(boss_battle_at AS TEXT)) THEN CAST(boss_battle_at AS TEXT) ELSE hex(boss_battle_at) END) ELSE boss_battle_at END)) AS TEXT) FROM guild_operation_members WHERE guild_operation_members.commander_id = sqlc.arg(id);

-- name: AdminDeleteGuildOperationMembers :exec
DELETE FROM guild_operation_members WHERE guild_operation_members.commander_id = sqlc.arg(id);

-- name: AdminExportGuildOperationPerfs :one
SELECT CAST(json_group_array(json_object('operation_id', CASE WHEN typeof(operation_id) = 'blob' THEN (CASE WHEN json_valid(CAST(operation_id AS TEXT)) THEN CAST(operation_id AS TEXT) ELSE hex(operation_id) END) ELSE operation_id END, 'commander_id', CASE WHEN typeof(commander_id) = 'blob' THEN (CASE WHEN json_valid(CAST(commander_id AS TEXT)) THEN CAST(commander_id AS TEXT) ELSE hex(commander_id) END) ELSE commander_id END, 'event_id', CASE WHEN typeof(event_id) = 'blob' THEN (CASE WHEN json_valid(CAST(event_id AS TEXT)) THEN CAST(event_id AS TEXT) ELSE hex(event_id) END) ELSE event_id END, 'idx', CASE WHEN typeof(idx) = 'blob' THEN (CASE WHEN json_valid(CAST(idx AS TEXT)) THEN CAST(idx AS TEXT) ELSE hex(idx) END) ELSE idx END)) AS TEXT) FROM guild_operation_perfs WHERE guild_operation_perfs.commander_id = sqlc.arg(id);

-- name: AdminDeleteGuildOperationPerfs :exec
DELETE FROM guild_operation_perfs WHERE guild_operation_perfs.commander_id = sqlc.arg(id);

-- name: AdminExportGuildOperationShips :one
SELECT CAST(json_group_array(json_object('operation_id', CASE WHEN typeof(operation_id) = 'blob' THEN (CASE WHEN json_valid(CAST(operation_id AS TEXT)) THEN CAST(operation_id AS TEXT) ELSE hex(operation_id) END) ELSE operation_id END, 'event_id', CASE WHEN typeof(event_id) = 'blob' THEN (CASE WHEN json_valid(CAST(event_id AS TEXT)) THEN CAST(event_id AS TEXT) ELSE hex(event_id) END) ELSE event_id END, 'commander_id', CASE WHEN typeof(commander_id) = 'blob' THEN (CASE WHEN json_valid(CAST(commander_id AS TEXT)) THEN CAST(commander_id AS TEXT) ELSE hex(commander_id) END) ELSE commander_id END, 'fleet', CASE WHEN typeof(fleet) = 'blob' THEN (CASE WHEN json_valid(CAST(fleet AS TEXT)) THEN CAST(fleet AS TEXT) ELSE hex(fleet) END) ELSE fleet END, 'slot', CASE WHEN typeof(slot) = 'blob' THEN (CASE WHEN json_valid(CAST(slot AS TEXT)) THEN CAST(slot AS TEXT) ELSE hex(slot) END) ELSE slot END, 'ship_id', CASE WHEN typeof(ship_id) = 'blob' THEN (CASE WHEN json_valid(CAST(ship_id AS TEXT)) THEN CAST(ship_id AS TEXT) ELSE hex(ship_id) END) ELSE ship_id END, 'template_id', CASE WHEN typeof(template_id) = 'blob' THEN (CASE WHEN json_valid(CAST(template_id AS TEXT)) THEN CAST(template_id AS TEXT) ELSE hex(template_id) END) ELSE template_id END, 'skin_id', CASE WHEN typeof(skin_id) = 'blob' THEN (CASE WHEN json_valid(CAST(skin_id AS TEXT)) THEN CAST(skin_id AS TEXT) ELSE hex(skin_id) END) ELSE skin_id END, 'level', CASE WHEN typeof(level) = 'blob' THEN (CASE WHEN json_valid(CAST(level AS TEXT)) THEN CAST(level AS TEXT) ELSE hex(level) END) ELSE level END, 'efficiency', CASE WHEN typeof(efficiency) = 'blob' THEN (CASE WHEN json_valid(CAST(efficiency AS TEXT)) THEN CAST(efficiency AS TEXT) ELSE hex(efficiency) END) ELSE efficiency END, 'dispatched_at', CASE WHEN typeof(dispatched_at) = 'blob' THEN (CASE WHEN json_valid(CAST(dispatched_at AS TEXT)) THEN CAST(dispatched_at AS TEXT) ELSE hex(dispatched_at) END) ELSE dispatched_at END)) AS TEXT) FROM guild_operation_ships WHERE guild_operation_ships.commander_id = sqlc.arg(id);

-- name: AdminDeleteGuildOperationShips :exec
DELETE FROM guild_operation_ships WHERE guild_operation_ships.commander_id = sqlc.arg(id);

-- name: AdminExportGuildRankScores :one
SELECT CAST(json_group_array(json_object('guild_id', CASE WHEN typeof(guild_id) = 'blob' THEN (CASE WHEN json_valid(CAST(guild_id AS TEXT)) THEN CAST(guild_id AS TEXT) ELSE hex(guild_id) END) ELSE guild_id END, 'commander_id', CASE WHEN typeof(commander_id) = 'blob' THEN (CASE WHEN json_valid(CAST(commander_id AS TEXT)) THEN CAST(commander_id AS TEXT) ELSE hex(commander_id) END) ELSE commander_id END, 'kind', CASE WHEN typeof(kind) = 'blob' THEN (CASE WHEN json_valid(CAST(kind AS TEXT)) THEN CAST(kind AS TEXT) ELSE hex(kind) END) ELSE kind END, 'week_start', CASE WHEN typeof(week_start) = 'blob' THEN (CASE WHEN json_valid(CAST(week_start AS TEXT)) THEN CAST(week_start AS TEXT) ELSE hex(week_start) END) ELSE week_start END, 'week', CASE WHEN typeof(week) = 'blob' THEN (CASE WHEN json_valid(CAST(week AS TEXT)) THEN CAST(week AS TEXT) ELSE hex(week) END) ELSE week END, 'month_start', CASE WHEN typeof(month_start) = 'blob' THEN (CASE WHEN json_valid(CAST(month_start AS TEXT)) THEN CAST(month_start AS TEXT) ELSE hex(month_start) END) ELSE month_start END, 'month', CASE WHEN typeof(month) = 'blob' THEN (CASE WHEN json_valid(CAST(month AS TEXT)) THEN CAST(month AS TEXT) ELSE hex(month) END) ELSE month END, 'total', CASE WHEN typeof(total) = 'blob' THEN (CASE WHEN json_valid(CAST(total AS TEXT)) THEN CAST(total AS TEXT) ELSE hex(total) END) ELSE total END)) AS TEXT) FROM guild_rank_scores WHERE guild_rank_scores.commander_id = sqlc.arg(id);

-- name: AdminDeleteGuildRankScores :exec
DELETE FROM guild_rank_scores WHERE guild_rank_scores.commander_id = sqlc.arg(id);

-- name: AdminExportGuildReportClaims :one
SELECT CAST(json_group_array(json_object('report_id', CASE WHEN typeof(report_id) = 'blob' THEN (CASE WHEN json_valid(CAST(report_id AS TEXT)) THEN CAST(report_id AS TEXT) ELSE hex(report_id) END) ELSE report_id END, 'commander_id', CASE WHEN typeof(commander_id) = 'blob' THEN (CASE WHEN json_valid(CAST(commander_id AS TEXT)) THEN CAST(commander_id AS TEXT) ELSE hex(commander_id) END) ELSE commander_id END, 'status', CASE WHEN typeof(status) = 'blob' THEN (CASE WHEN json_valid(CAST(status AS TEXT)) THEN CAST(status AS TEXT) ELSE hex(status) END) ELSE status END)) AS TEXT) FROM guild_report_claims WHERE guild_report_claims.commander_id = sqlc.arg(id);

-- name: AdminDeleteGuildReportClaims :exec
DELETE FROM guild_report_claims WHERE guild_report_claims.commander_id = sqlc.arg(id);

-- name: AdminExportGuildRequests :one
SELECT CAST(json_group_array(json_object('guild_id', CASE WHEN typeof(guild_id) = 'blob' THEN (CASE WHEN json_valid(CAST(guild_id AS TEXT)) THEN CAST(guild_id AS TEXT) ELSE hex(guild_id) END) ELSE guild_id END, 'commander_id', CASE WHEN typeof(commander_id) = 'blob' THEN (CASE WHEN json_valid(CAST(commander_id AS TEXT)) THEN CAST(commander_id AS TEXT) ELSE hex(commander_id) END) ELSE commander_id END, 'content', CASE WHEN typeof(content) = 'blob' THEN (CASE WHEN json_valid(CAST(content AS TEXT)) THEN CAST(content AS TEXT) ELSE hex(content) END) ELSE content END, 'time', CASE WHEN typeof(time) = 'blob' THEN (CASE WHEN json_valid(CAST(time AS TEXT)) THEN CAST(time AS TEXT) ELSE hex(time) END) ELSE time END)) AS TEXT) FROM guild_requests WHERE guild_requests.commander_id = sqlc.arg(id);

-- name: AdminDeleteGuildRequests :exec
DELETE FROM guild_requests WHERE guild_requests.commander_id = sqlc.arg(id);

-- name: AdminExportGuildUsers :one
SELECT CAST(json_group_array(json_object('commander_id', CASE WHEN typeof(commander_id) = 'blob' THEN (CASE WHEN json_valid(CAST(commander_id AS TEXT)) THEN CAST(commander_id AS TEXT) ELSE hex(commander_id) END) ELSE commander_id END, 'donate_day', CASE WHEN typeof(donate_day) = 'blob' THEN (CASE WHEN json_valid(CAST(donate_day AS TEXT)) THEN CAST(donate_day AS TEXT) ELSE hex(donate_day) END) ELSE donate_day END, 'donate_count', CASE WHEN typeof(donate_count) = 'blob' THEN (CASE WHEN json_valid(CAST(donate_count AS TEXT)) THEN CAST(donate_count AS TEXT) ELSE hex(donate_count) END) ELSE donate_count END, 'donate_tasks', CASE WHEN typeof(donate_tasks) = 'blob' THEN (CASE WHEN json_valid(CAST(donate_tasks AS TEXT)) THEN CAST(donate_tasks AS TEXT) ELSE hex(donate_tasks) END) ELSE donate_tasks END, 'extra_donate', CASE WHEN typeof(extra_donate) = 'blob' THEN (CASE WHEN json_valid(CAST(extra_donate AS TEXT)) THEN CAST(extra_donate AS TEXT) ELSE hex(extra_donate) END) ELSE extra_donate END, 'extra_operation', CASE WHEN typeof(extra_operation) = 'blob' THEN (CASE WHEN json_valid(CAST(extra_operation AS TEXT)) THEN CAST(extra_operation AS TEXT) ELSE hex(extra_operation) END) ELSE extra_operation END, 'benefit_time', CASE WHEN typeof(benefit_time) = 'blob' THEN (CASE WHEN json_valid(CAST(benefit_time AS TEXT)) THEN CAST(benefit_time AS TEXT) ELSE hex(benefit_time) END) ELSE benefit_time END, 'weekly_task_week', CASE WHEN typeof(weekly_task_week) = 'blob' THEN (CASE WHEN json_valid(CAST(weekly_task_week AS TEXT)) THEN CAST(weekly_task_week AS TEXT) ELSE hex(weekly_task_week) END) ELSE weekly_task_week END, 'operation_month', CASE WHEN typeof(operation_month) = 'blob' THEN (CASE WHEN json_valid(CAST(operation_month AS TEXT)) THEN CAST(operation_month AS TEXT) ELSE hex(operation_month) END) ELSE operation_month END, 'operation_joins', CASE WHEN typeof(operation_joins) = 'blob' THEN (CASE WHEN json_valid(CAST(operation_joins AS TEXT)) THEN CAST(operation_joins AS TEXT) ELSE hex(operation_joins) END) ELSE operation_joins END)) AS TEXT) FROM guild_users WHERE guild_users.commander_id = sqlc.arg(id);

-- name: AdminDeleteGuildUsers :exec
DELETE FROM guild_users WHERE guild_users.commander_id = sqlc.arg(id);

-- name: AdminExportGuildWaitTimes :one
SELECT CAST(json_group_array(json_object('commander_id', CASE WHEN typeof(commander_id) = 'blob' THEN (CASE WHEN json_valid(CAST(commander_id AS TEXT)) THEN CAST(commander_id AS TEXT) ELSE hex(commander_id) END) ELSE commander_id END, 'until', CASE WHEN typeof(until) = 'blob' THEN (CASE WHEN json_valid(CAST(until AS TEXT)) THEN CAST(until AS TEXT) ELSE hex(until) END) ELSE until END)) AS TEXT) FROM guild_wait_times WHERE guild_wait_times.commander_id = sqlc.arg(id);

-- name: AdminDeleteGuildWaitTimes :exec
DELETE FROM guild_wait_times WHERE guild_wait_times.commander_id = sqlc.arg(id);

-- name: AdminExportLimitChallenges :one
SELECT CAST(json_group_array(json_object('commander_id', CASE WHEN typeof(commander_id) = 'blob' THEN (CASE WHEN json_valid(CAST(commander_id AS TEXT)) THEN CAST(commander_id AS TEXT) ELSE hex(commander_id) END) ELSE commander_id END, 'challenge_id', CASE WHEN typeof(challenge_id) = 'blob' THEN (CASE WHEN json_valid(CAST(challenge_id AS TEXT)) THEN CAST(challenge_id AS TEXT) ELSE hex(challenge_id) END) ELSE challenge_id END, 'best_time', CASE WHEN typeof(best_time) = 'blob' THEN (CASE WHEN json_valid(CAST(best_time AS TEXT)) THEN CAST(best_time AS TEXT) ELSE hex(best_time) END) ELSE best_time END, 'pass_month', CASE WHEN typeof(pass_month) = 'blob' THEN (CASE WHEN json_valid(CAST(pass_month AS TEXT)) THEN CAST(pass_month AS TEXT) ELSE hex(pass_month) END) ELSE pass_month END, 'awarded', CASE WHEN typeof(awarded) = 'blob' THEN (CASE WHEN json_valid(CAST(awarded AS TEXT)) THEN CAST(awarded AS TEXT) ELSE hex(awarded) END) ELSE awarded END)) AS TEXT) FROM limit_challenges WHERE limit_challenges.commander_id = sqlc.arg(id);

-- name: AdminDeleteLimitChallenges :exec
DELETE FROM limit_challenges WHERE limit_challenges.commander_id = sqlc.arg(id);

-- name: AdminExportMails :one
SELECT CAST(json_group_array(json_object('id', CASE WHEN typeof(id) = 'blob' THEN (CASE WHEN json_valid(CAST(id AS TEXT)) THEN CAST(id AS TEXT) ELSE hex(id) END) ELSE id END, 'commander_id', CASE WHEN typeof(commander_id) = 'blob' THEN (CASE WHEN json_valid(CAST(commander_id AS TEXT)) THEN CAST(commander_id AS TEXT) ELSE hex(commander_id) END) ELSE commander_id END, 'date', CASE WHEN typeof(date) = 'blob' THEN (CASE WHEN json_valid(CAST(date AS TEXT)) THEN CAST(date AS TEXT) ELSE hex(date) END) ELSE date END, 'title', CASE WHEN typeof(title) = 'blob' THEN (CASE WHEN json_valid(CAST(title AS TEXT)) THEN CAST(title AS TEXT) ELSE hex(title) END) ELSE title END, 'sender', CASE WHEN typeof(sender) = 'blob' THEN (CASE WHEN json_valid(CAST(sender AS TEXT)) THEN CAST(sender AS TEXT) ELSE hex(sender) END) ELSE sender END, 'content', CASE WHEN typeof(content) = 'blob' THEN (CASE WHEN json_valid(CAST(content AS TEXT)) THEN CAST(content AS TEXT) ELSE hex(content) END) ELSE content END, 'attachments', CASE WHEN typeof(attachments) = 'blob' THEN (CASE WHEN json_valid(CAST(attachments AS TEXT)) THEN CAST(attachments AS TEXT) ELSE hex(attachments) END) ELSE attachments END, 'is_read', CASE WHEN typeof(is_read) = 'blob' THEN (CASE WHEN json_valid(CAST(is_read AS TEXT)) THEN CAST(is_read AS TEXT) ELSE hex(is_read) END) ELSE is_read END, 'is_claimed', CASE WHEN typeof(is_claimed) = 'blob' THEN (CASE WHEN json_valid(CAST(is_claimed AS TEXT)) THEN CAST(is_claimed AS TEXT) ELSE hex(is_claimed) END) ELSE is_claimed END, 'is_important', CASE WHEN typeof(is_important) = 'blob' THEN (CASE WHEN json_valid(CAST(is_important AS TEXT)) THEN CAST(is_important AS TEXT) ELSE hex(is_important) END) ELSE is_important END, 'archived', CASE WHEN typeof(archived) = 'blob' THEN (CASE WHEN json_valid(CAST(archived AS TEXT)) THEN CAST(archived AS TEXT) ELSE hex(archived) END) ELSE archived END)) AS TEXT) FROM mails WHERE mails.commander_id = sqlc.arg(id);

-- name: AdminDeleteMails :exec
DELETE FROM mails WHERE mails.commander_id = sqlc.arg(id);

-- name: AdminExportMeowfficerBoxes :one
SELECT CAST(json_group_array(json_object('commander_id', CASE WHEN typeof(commander_id) = 'blob' THEN (CASE WHEN json_valid(CAST(commander_id AS TEXT)) THEN CAST(commander_id AS TEXT) ELSE hex(commander_id) END) ELSE commander_id END, 'box_id', CASE WHEN typeof(box_id) = 'blob' THEN (CASE WHEN json_valid(CAST(box_id AS TEXT)) THEN CAST(box_id AS TEXT) ELSE hex(box_id) END) ELSE box_id END, 'pool_id', CASE WHEN typeof(pool_id) = 'blob' THEN (CASE WHEN json_valid(CAST(pool_id AS TEXT)) THEN CAST(pool_id AS TEXT) ELSE hex(pool_id) END) ELSE pool_id END, 'template_id', CASE WHEN typeof(template_id) = 'blob' THEN (CASE WHEN json_valid(CAST(template_id AS TEXT)) THEN CAST(template_id AS TEXT) ELSE hex(template_id) END) ELSE template_id END, 'begin_time', CASE WHEN typeof(begin_time) = 'blob' THEN (CASE WHEN json_valid(CAST(begin_time AS TEXT)) THEN CAST(begin_time AS TEXT) ELSE hex(begin_time) END) ELSE begin_time END, 'finish_time', CASE WHEN typeof(finish_time) = 'blob' THEN (CASE WHEN json_valid(CAST(finish_time AS TEXT)) THEN CAST(finish_time AS TEXT) ELSE hex(finish_time) END) ELSE finish_time END)) AS TEXT) FROM meowfficer_boxes WHERE meowfficer_boxes.commander_id = sqlc.arg(id);

-- name: AdminDeleteMeowfficerBoxes :exec
DELETE FROM meowfficer_boxes WHERE meowfficer_boxes.commander_id = sqlc.arg(id);

-- name: AdminExportMeowfficerCatteries :one
SELECT CAST(json_group_array(json_object('commander_id', CASE WHEN typeof(commander_id) = 'blob' THEN (CASE WHEN json_valid(CAST(commander_id AS TEXT)) THEN CAST(commander_id AS TEXT) ELSE hex(commander_id) END) ELSE commander_id END, 'slot_id', CASE WHEN typeof(slot_id) = 'blob' THEN (CASE WHEN json_valid(CAST(slot_id AS TEXT)) THEN CAST(slot_id AS TEXT) ELSE hex(slot_id) END) ELSE slot_id END, 'meowfficer_id', CASE WHEN typeof(meowfficer_id) = 'blob' THEN (CASE WHEN json_valid(CAST(meowfficer_id AS TEXT)) THEN CAST(meowfficer_id AS TEXT) ELSE hex(meowfficer_id) END) ELSE meowfficer_id END, 'style', CASE WHEN typeof(style) = 'blob' THEN (CASE WHEN json_valid(CAST(style AS TEXT)) THEN CAST(style AS TEXT) ELSE hex(style) END) ELSE style END, 'exp_time', CASE WHEN typeof(exp_time) = 'blob' THEN (CASE WHEN json_valid(CAST(exp_time AS TEXT)) THEN CAST(exp_time AS TEXT) ELSE hex(exp_time) END) ELSE exp_time END, 'clean_time', CASE WHEN typeof(clean_time) = 'blob' THEN (CASE WHEN json_valid(CAST(clean_time AS TEXT)) THEN CAST(clean_time AS TEXT) ELSE hex(clean_time) END) ELSE clean_time END, 'feed_time', CASE WHEN typeof(feed_time) = 'blob' THEN (CASE WHEN json_valid(CAST(feed_time AS TEXT)) THEN CAST(feed_time AS TEXT) ELSE hex(feed_time) END) ELSE feed_time END, 'play_time', CASE WHEN typeof(play_time) = 'blob' THEN (CASE WHEN json_valid(CAST(play_time AS TEXT)) THEN CAST(play_time AS TEXT) ELSE hex(play_time) END) ELSE play_time END)) AS TEXT) FROM meowfficer_catteries WHERE meowfficer_catteries.commander_id = sqlc.arg(id);

-- name: AdminDeleteMeowfficerCatteries :exec
DELETE FROM meowfficer_catteries WHERE meowfficer_catteries.commander_id = sqlc.arg(id);

-- name: AdminExportMeowfficerPresetSlots :one
SELECT CAST(json_group_array(json_object('commander_id', CASE WHEN typeof(commander_id) = 'blob' THEN (CASE WHEN json_valid(CAST(commander_id AS TEXT)) THEN CAST(commander_id AS TEXT) ELSE hex(commander_id) END) ELSE commander_id END, 'preset_id', CASE WHEN typeof(preset_id) = 'blob' THEN (CASE WHEN json_valid(CAST(preset_id AS TEXT)) THEN CAST(preset_id AS TEXT) ELSE hex(preset_id) END) ELSE preset_id END, 'pos', CASE WHEN typeof(pos) = 'blob' THEN (CASE WHEN json_valid(CAST(pos AS TEXT)) THEN CAST(pos AS TEXT) ELSE hex(pos) END) ELSE pos END, 'meowfficer_id', CASE WHEN typeof(meowfficer_id) = 'blob' THEN (CASE WHEN json_valid(CAST(meowfficer_id AS TEXT)) THEN CAST(meowfficer_id AS TEXT) ELSE hex(meowfficer_id) END) ELSE meowfficer_id END)) AS TEXT) FROM meowfficer_preset_slots WHERE meowfficer_preset_slots.commander_id = sqlc.arg(id) OR meowfficer_preset_slots.meowfficer_id IN (SELECT o.id FROM meowfficers AS o WHERE o.commander_id = sqlc.arg(id));

-- name: AdminDeleteMeowfficerPresetSlots :exec
DELETE FROM meowfficer_preset_slots WHERE meowfficer_preset_slots.commander_id = sqlc.arg(id) OR meowfficer_preset_slots.meowfficer_id IN (SELECT o.id FROM meowfficers AS o WHERE o.commander_id = sqlc.arg(id));

-- name: AdminExportMeowfficerPresets :one
SELECT CAST(json_group_array(json_object('commander_id', CASE WHEN typeof(commander_id) = 'blob' THEN (CASE WHEN json_valid(CAST(commander_id AS TEXT)) THEN CAST(commander_id AS TEXT) ELSE hex(commander_id) END) ELSE commander_id END, 'preset_id', CASE WHEN typeof(preset_id) = 'blob' THEN (CASE WHEN json_valid(CAST(preset_id AS TEXT)) THEN CAST(preset_id AS TEXT) ELSE hex(preset_id) END) ELSE preset_id END, 'name', CASE WHEN typeof(name) = 'blob' THEN (CASE WHEN json_valid(CAST(name AS TEXT)) THEN CAST(name AS TEXT) ELSE hex(name) END) ELSE name END, 'rename_time', CASE WHEN typeof(rename_time) = 'blob' THEN (CASE WHEN json_valid(CAST(rename_time AS TEXT)) THEN CAST(rename_time AS TEXT) ELSE hex(rename_time) END) ELSE rename_time END)) AS TEXT) FROM meowfficer_presets WHERE meowfficer_presets.commander_id = sqlc.arg(id);

-- name: AdminDeleteMeowfficerPresets :exec
DELETE FROM meowfficer_presets WHERE meowfficer_presets.commander_id = sqlc.arg(id);

-- name: AdminExportMeowfficerStates :one
SELECT CAST(json_group_array(json_object('commander_id', CASE WHEN typeof(commander_id) = 'blob' THEN (CASE WHEN json_valid(CAST(commander_id AS TEXT)) THEN CAST(commander_id AS TEXT) ELSE hex(commander_id) END) ELSE commander_id END, 'reserve_count', CASE WHEN typeof(reserve_count) = 'blob' THEN (CASE WHEN json_valid(CAST(reserve_count AS TEXT)) THEN CAST(reserve_count AS TEXT) ELSE hex(reserve_count) END) ELSE reserve_count END, 'reserve_day', CASE WHEN typeof(reserve_day) = 'blob' THEN (CASE WHEN json_valid(CAST(reserve_day AS TEXT)) THEN CAST(reserve_day AS TEXT) ELSE hex(reserve_day) END) ELSE reserve_day END, 'reserve_total', CASE WHEN typeof(reserve_total) = 'blob' THEN (CASE WHEN json_valid(CAST(reserve_total AS TEXT)) THEN CAST(reserve_total AS TEXT) ELSE hex(reserve_total) END) ELSE reserve_total END, 'home_level', CASE WHEN typeof(home_level) = 'blob' THEN (CASE WHEN json_valid(CAST(home_level AS TEXT)) THEN CAST(home_level AS TEXT) ELSE hex(home_level) END) ELSE home_level END, 'home_exp', CASE WHEN typeof(home_exp) = 'blob' THEN (CASE WHEN json_valid(CAST(home_exp AS TEXT)) THEN CAST(home_exp AS TEXT) ELSE hex(home_exp) END) ELSE home_exp END, 'home_clean', CASE WHEN typeof(home_clean) = 'blob' THEN (CASE WHEN json_valid(CAST(home_clean AS TEXT)) THEN CAST(home_clean AS TEXT) ELSE hex(home_clean) END) ELSE home_clean END, 'clean_time', CASE WHEN typeof(clean_time) = 'blob' THEN (CASE WHEN json_valid(CAST(clean_time AS TEXT)) THEN CAST(clean_time AS TEXT) ELSE hex(clean_time) END) ELSE clean_time END)) AS TEXT) FROM meowfficer_states WHERE meowfficer_states.commander_id = sqlc.arg(id);

-- name: AdminDeleteMeowfficerStates :exec
DELETE FROM meowfficer_states WHERE meowfficer_states.commander_id = sqlc.arg(id);

-- name: AdminExportMeowfficers :one
SELECT CAST(json_group_array(json_object('id', CASE WHEN typeof(id) = 'blob' THEN (CASE WHEN json_valid(CAST(id AS TEXT)) THEN CAST(id AS TEXT) ELSE hex(id) END) ELSE id END, 'commander_id', CASE WHEN typeof(commander_id) = 'blob' THEN (CASE WHEN json_valid(CAST(commander_id AS TEXT)) THEN CAST(commander_id AS TEXT) ELSE hex(commander_id) END) ELSE commander_id END, 'template_id', CASE WHEN typeof(template_id) = 'blob' THEN (CASE WHEN json_valid(CAST(template_id AS TEXT)) THEN CAST(template_id AS TEXT) ELSE hex(template_id) END) ELSE template_id END, 'level', CASE WHEN typeof(level) = 'blob' THEN (CASE WHEN json_valid(CAST(level AS TEXT)) THEN CAST(level AS TEXT) ELSE hex(level) END) ELSE level END, 'exp', CASE WHEN typeof(exp) = 'blob' THEN (CASE WHEN json_valid(CAST(exp AS TEXT)) THEN CAST(exp AS TEXT) ELSE hex(exp) END) ELSE exp END, 'locked', CASE WHEN typeof(locked) = 'blob' THEN (CASE WHEN json_valid(CAST(locked AS TEXT)) THEN CAST(locked AS TEXT) ELSE hex(locked) END) ELSE locked END, 'name', CASE WHEN typeof(name) = 'blob' THEN (CASE WHEN json_valid(CAST(name AS TEXT)) THEN CAST(name AS TEXT) ELSE hex(name) END) ELSE name END, 'rename_time', CASE WHEN typeof(rename_time) = 'blob' THEN (CASE WHEN json_valid(CAST(rename_time AS TEXT)) THEN CAST(rename_time AS TEXT) ELSE hex(rename_time) END) ELSE rename_time END, 'talents', CASE WHEN typeof(talents) = 'blob' THEN (CASE WHEN json_valid(CAST(talents AS TEXT)) THEN CAST(talents AS TEXT) ELSE hex(talents) END) ELSE talents END, 'talent_origins', CASE WHEN typeof(talent_origins) = 'blob' THEN (CASE WHEN json_valid(CAST(talent_origins AS TEXT)) THEN CAST(talent_origins AS TEXT) ELSE hex(talent_origins) END) ELSE talent_origins END, 'talent_choices', CASE WHEN typeof(talent_choices) = 'blob' THEN (CASE WHEN json_valid(CAST(talent_choices AS TEXT)) THEN CAST(talent_choices AS TEXT) ELSE hex(talent_choices) END) ELSE talent_choices END, 'talent_time', CASE WHEN typeof(talent_time) = 'blob' THEN (CASE WHEN json_valid(CAST(talent_time AS TEXT)) THEN CAST(talent_time AS TEXT) ELSE hex(talent_time) END) ELSE talent_time END, 'used_pt', CASE WHEN typeof(used_pt) = 'blob' THEN (CASE WHEN json_valid(CAST(used_pt AS TEXT)) THEN CAST(used_pt AS TEXT) ELSE hex(used_pt) END) ELSE used_pt END, 'skill_id', CASE WHEN typeof(skill_id) = 'blob' THEN (CASE WHEN json_valid(CAST(skill_id AS TEXT)) THEN CAST(skill_id AS TEXT) ELSE hex(skill_id) END) ELSE skill_id END, 'skill_exp', CASE WHEN typeof(skill_exp) = 'blob' THEN (CASE WHEN json_valid(CAST(skill_exp AS TEXT)) THEN CAST(skill_exp AS TEXT) ELSE hex(skill_exp) END) ELSE skill_exp END, 'home_clean_time', CASE WHEN typeof(home_clean_time) = 'blob' THEN (CASE WHEN json_valid(CAST(home_clean_time AS TEXT)) THEN CAST(home_clean_time AS TEXT) ELSE hex(home_clean_time) END) ELSE home_clean_time END, 'home_play_time', CASE WHEN typeof(home_play_time) = 'blob' THEN (CASE WHEN json_valid(CAST(home_play_time AS TEXT)) THEN CAST(home_play_time AS TEXT) ELSE hex(home_play_time) END) ELSE home_play_time END, 'home_feed_time', CASE WHEN typeof(home_feed_time) = 'blob' THEN (CASE WHEN json_valid(CAST(home_feed_time AS TEXT)) THEN CAST(home_feed_time AS TEXT) ELSE hex(home_feed_time) END) ELSE home_feed_time END)) AS TEXT) FROM meowfficers WHERE meowfficers.commander_id = sqlc.arg(id);

-- name: AdminDeleteMeowfficers :exec
DELETE FROM meowfficers WHERE meowfficers.commander_id = sqlc.arg(id);

-- name: AdminExportMetaPtAwards :one
SELECT CAST(json_group_array(json_object('commander_id', CASE WHEN typeof(commander_id) = 'blob' THEN (CASE WHEN json_valid(CAST(commander_id AS TEXT)) THEN CAST(commander_id AS TEXT) ELSE hex(commander_id) END) ELSE commander_id END, 'group_id', CASE WHEN typeof(group_id) = 'blob' THEN (CASE WHEN json_valid(CAST(group_id AS TEXT)) THEN CAST(group_id AS TEXT) ELSE hex(group_id) END) ELSE group_id END, 'target', CASE WHEN typeof(target) = 'blob' THEN (CASE WHEN json_valid(CAST(target AS TEXT)) THEN CAST(target AS TEXT) ELSE hex(target) END) ELSE target END)) AS TEXT) FROM meta_pt_awards WHERE meta_pt_awards.commander_id = sqlc.arg(id);

-- name: AdminDeleteMetaPtAwards :exec
DELETE FROM meta_pt_awards WHERE meta_pt_awards.commander_id = sqlc.arg(id);

-- name: AdminExportMetaPts :one
SELECT CAST(json_group_array(json_object('commander_id', CASE WHEN typeof(commander_id) = 'blob' THEN (CASE WHEN json_valid(CAST(commander_id AS TEXT)) THEN CAST(commander_id AS TEXT) ELSE hex(commander_id) END) ELSE commander_id END, 'group_id', CASE WHEN typeof(group_id) = 'blob' THEN (CASE WHEN json_valid(CAST(group_id AS TEXT)) THEN CAST(group_id AS TEXT) ELSE hex(group_id) END) ELSE group_id END, 'pt', CASE WHEN typeof(pt) = 'blob' THEN (CASE WHEN json_valid(CAST(pt AS TEXT)) THEN CAST(pt AS TEXT) ELSE hex(pt) END) ELSE pt END)) AS TEXT) FROM meta_pts WHERE meta_pts.commander_id = sqlc.arg(id);

-- name: AdminDeleteMetaPts :exec
DELETE FROM meta_pts WHERE meta_pts.commander_id = sqlc.arg(id);

-- name: AdminExportNewServerShopGoods :one
SELECT CAST(json_group_array(json_object('commander_id', CASE WHEN typeof(commander_id) = 'blob' THEN (CASE WHEN json_valid(CAST(commander_id AS TEXT)) THEN CAST(commander_id AS TEXT) ELSE hex(commander_id) END) ELSE commander_id END, 'activity_id', CASE WHEN typeof(activity_id) = 'blob' THEN (CASE WHEN json_valid(CAST(activity_id AS TEXT)) THEN CAST(activity_id AS TEXT) ELSE hex(activity_id) END) ELSE activity_id END, 'goods_id', CASE WHEN typeof(goods_id) = 'blob' THEN (CASE WHEN json_valid(CAST(goods_id AS TEXT)) THEN CAST(goods_id AS TEXT) ELSE hex(goods_id) END) ELSE goods_id END, 'bought', CASE WHEN typeof(bought) = 'blob' THEN (CASE WHEN json_valid(CAST(bought AS TEXT)) THEN CAST(bought AS TEXT) ELSE hex(bought) END) ELSE bought END)) AS TEXT) FROM new_server_shop_goods WHERE new_server_shop_goods.commander_id = sqlc.arg(id);

-- name: AdminDeleteNewServerShopGoods :exec
DELETE FROM new_server_shop_goods WHERE new_server_shop_goods.commander_id = sqlc.arg(id);

-- name: AdminExportNewServerShopSubGoods :one
SELECT CAST(json_group_array(json_object('commander_id', CASE WHEN typeof(commander_id) = 'blob' THEN (CASE WHEN json_valid(CAST(commander_id AS TEXT)) THEN CAST(commander_id AS TEXT) ELSE hex(commander_id) END) ELSE commander_id END, 'activity_id', CASE WHEN typeof(activity_id) = 'blob' THEN (CASE WHEN json_valid(CAST(activity_id AS TEXT)) THEN CAST(activity_id AS TEXT) ELSE hex(activity_id) END) ELSE activity_id END, 'goods_id', CASE WHEN typeof(goods_id) = 'blob' THEN (CASE WHEN json_valid(CAST(goods_id AS TEXT)) THEN CAST(goods_id AS TEXT) ELSE hex(goods_id) END) ELSE goods_id END, 'item_id', CASE WHEN typeof(item_id) = 'blob' THEN (CASE WHEN json_valid(CAST(item_id AS TEXT)) THEN CAST(item_id AS TEXT) ELSE hex(item_id) END) ELSE item_id END)) AS TEXT) FROM new_server_shop_sub_goods WHERE new_server_shop_sub_goods.commander_id = sqlc.arg(id);

-- name: AdminDeleteNewServerShopSubGoods :exec
DELETE FROM new_server_shop_sub_goods WHERE new_server_shop_sub_goods.commander_id = sqlc.arg(id);

-- name: AdminExportRemasterAwards :one
SELECT CAST(json_group_array(json_object('commander_id', CASE WHEN typeof(commander_id) = 'blob' THEN (CASE WHEN json_valid(CAST(commander_id AS TEXT)) THEN CAST(commander_id AS TEXT) ELSE hex(commander_id) END) ELSE commander_id END, 'act_id', CASE WHEN typeof(act_id) = 'blob' THEN (CASE WHEN json_valid(CAST(act_id AS TEXT)) THEN CAST(act_id AS TEXT) ELSE hex(act_id) END) ELSE act_id END, 'chapter_id', CASE WHEN typeof(chapter_id) = 'blob' THEN (CASE WHEN json_valid(CAST(chapter_id AS TEXT)) THEN CAST(chapter_id AS TEXT) ELSE hex(chapter_id) END) ELSE chapter_id END, 'pos', CASE WHEN typeof(pos) = 'blob' THEN (CASE WHEN json_valid(CAST(pos AS TEXT)) THEN CAST(pos AS TEXT) ELSE hex(pos) END) ELSE pos END, 'count', CASE WHEN typeof(count) = 'blob' THEN (CASE WHEN json_valid(CAST(count AS TEXT)) THEN CAST(count AS TEXT) ELSE hex(count) END) ELSE count END, 'received', CASE WHEN typeof(received) = 'blob' THEN (CASE WHEN json_valid(CAST(received AS TEXT)) THEN CAST(received AS TEXT) ELSE hex(received) END) ELSE received END)) AS TEXT) FROM remaster_awards WHERE remaster_awards.commander_id = sqlc.arg(id);

-- name: AdminDeleteRemasterAwards :exec
DELETE FROM remaster_awards WHERE remaster_awards.commander_id = sqlc.arg(id);

-- name: AdminExportRemasterStates :one
SELECT CAST(json_group_array(json_object('commander_id', CASE WHEN typeof(commander_id) = 'blob' THEN (CASE WHEN json_valid(CAST(commander_id AS TEXT)) THEN CAST(commander_id AS TEXT) ELSE hex(commander_id) END) ELSE commander_id END, 'tickets', CASE WHEN typeof(tickets) = 'blob' THEN (CASE WHEN json_valid(CAST(tickets AS TEXT)) THEN CAST(tickets AS TEXT) ELSE hex(tickets) END) ELSE tickets END, 'claimed_day', CASE WHEN typeof(claimed_day) = 'blob' THEN (CASE WHEN json_valid(CAST(claimed_day AS TEXT)) THEN CAST(claimed_day AS TEXT) ELSE hex(claimed_day) END) ELSE claimed_day END)) AS TEXT) FROM remaster_states WHERE remaster_states.commander_id = sqlc.arg(id);

-- name: AdminDeleteRemasterStates :exec
DELETE FROM remaster_states WHERE remaster_states.commander_id = sqlc.arg(id);

-- name: AdminExportShipDiscussionReports :one
SELECT CAST(json_group_array(json_object('discussion_id', CASE WHEN typeof(discussion_id) = 'blob' THEN (CASE WHEN json_valid(CAST(discussion_id AS TEXT)) THEN CAST(discussion_id AS TEXT) ELSE hex(discussion_id) END) ELSE discussion_id END, 'commander_id', CASE WHEN typeof(commander_id) = 'blob' THEN (CASE WHEN json_valid(CAST(commander_id AS TEXT)) THEN CAST(commander_id AS TEXT) ELSE hex(commander_id) END) ELSE commander_id END, 'reason', CASE WHEN typeof(reason) = 'blob' THEN (CASE WHEN json_valid(CAST(reason AS TEXT)) THEN CAST(reason AS TEXT) ELSE hex(reason) END) ELSE reason END, 'reported_at', CASE WHEN typeof(reported_at) = 'blob' THEN (CASE WHEN json_valid(CAST(reported_at AS TEXT)) THEN CAST(reported_at AS TEXT) ELSE hex(reported_at) END) ELSE reported_at END)) AS TEXT) FROM ship_discussion_reports WHERE ship_discussion_reports.commander_id = sqlc.arg(id) OR ship_discussion_reports.discussion_id IN (SELECT o.id FROM ship_discussions AS o WHERE o.commander_id = sqlc.arg(id));

-- name: AdminDeleteShipDiscussionReports :exec
DELETE FROM ship_discussion_reports WHERE ship_discussion_reports.commander_id = sqlc.arg(id) OR ship_discussion_reports.discussion_id IN (SELECT o.id FROM ship_discussions AS o WHERE o.commander_id = sqlc.arg(id));

-- name: AdminExportShipDiscussionVotes :one
SELECT CAST(json_group_array(json_object('discussion_id', CASE WHEN typeof(discussion_id) = 'blob' THEN (CASE WHEN json_valid(CAST(discussion_id AS TEXT)) THEN CAST(discussion_id AS TEXT) ELSE hex(discussion_id) END) ELSE discussion_id END, 'commander_id', CASE WHEN typeof(commander_id) = 'blob' THEN (CASE WHEN json_valid(CAST(commander_id AS TEXT)) THEN CAST(commander_id AS TEXT) ELSE hex(commander_id) END) ELSE commander_id END, 'good_or_bad', CASE WHEN typeof(good_or_bad) = 'blob' THEN (CASE WHEN json_valid(CAST(good_or_bad AS TEXT)) THEN CAST(good_or_bad AS TEXT) ELSE hex(good_or_bad) END) ELSE good_or_bad END)) AS TEXT) FROM ship_discussion_votes WHERE ship_discussion_votes.commander_id = sqlc.arg(id) OR ship_discussion_votes.discussion_id IN (SELECT o.id FROM ship_discussions AS o WHERE o.commander_id = sqlc.arg(id));

-- name: AdminDeleteShipDiscussionVotes :exec
DELETE FROM ship_discussion_votes WHERE ship_discussion_votes.commander_id = sqlc.arg(id) OR ship_discussion_votes.discussion_id IN (SELECT o.id FROM ship_discussions AS o WHERE o.commander_id = sqlc.arg(id));

-- name: AdminExportShipDiscussions :one
SELECT CAST(json_group_array(json_object('id', CASE WHEN typeof(id) = 'blob' THEN (CASE WHEN json_valid(CAST(id AS TEXT)) THEN CAST(id AS TEXT) ELSE hex(id) END) ELSE id END, 'group_id', CASE WHEN typeof(group_id) = 'blob' THEN (CASE WHEN json_valid(CAST(group_id AS TEXT)) THEN CAST(group_id AS TEXT) ELSE hex(group_id) END) ELSE group_id END, 'commander_id', CASE WHEN typeof(commander_id) = 'blob' THEN (CASE WHEN json_valid(CAST(commander_id AS TEXT)) THEN CAST(commander_id AS TEXT) ELSE hex(commander_id) END) ELSE commander_id END, 'context', CASE WHEN typeof(context) = 'blob' THEN (CASE WHEN json_valid(CAST(context AS TEXT)) THEN CAST(context AS TEXT) ELSE hex(context) END) ELSE context END, 'posted_at', CASE WHEN typeof(posted_at) = 'blob' THEN (CASE WHEN json_valid(CAST(posted_at AS TEXT)) THEN CAST(posted_at AS TEXT) ELSE hex(posted_at) END) ELSE posted_at END)) AS TEXT) FROM ship_discussions WHERE ship_discussions.commander_id = sqlc.arg(id);

-- name: AdminDeleteShipDiscussions :exec
DELETE FROM ship_discussions WHERE ship_discussions.commander_id = sqlc.arg(id);

-- name: AdminExportShipEquipments :one
SELECT CAST(json_group_array(json_object('ship_id', CASE WHEN typeof(ship_id) = 'blob' THEN (CASE WHEN json_valid(CAST(ship_id AS TEXT)) THEN CAST(ship_id AS TEXT) ELSE hex(ship_id) END) ELSE ship_id END, 'pos', CASE WHEN typeof(pos) = 'blob' THEN (CASE WHEN json_valid(CAST(pos AS TEXT)) THEN CAST(pos AS TEXT) ELSE hex(pos) END) ELSE pos END, 'equip_id', CASE WHEN typeof(equip_id) = 'blob' THEN (CASE WHEN json_valid(CAST(equip_id AS TEXT)) THEN CAST(equip_id AS TEXT) ELSE hex(equip_id) END) ELSE equip_id END, 'skin_id', CASE WHEN typeof(skin_id) = 'blob' THEN (CASE WHEN json_valid(CAST(skin_id AS TEXT)) THEN CAST(skin_id AS TEXT) ELSE hex(skin_id) END) ELSE skin_id END)) AS TEXT) FROM ship_equipments WHERE ship_equipments.ship_id IN (SELECT o.id FROM ships AS o WHERE o.commander_id = sqlc.arg(id));

-- name: AdminDeleteShipEquipments :exec
DELETE FROM ship_equipments WHERE ship_equipments.ship_id IN (SELECT o.id FROM ships AS o WHERE o.commander_id = sqlc.arg(id));

-- name: AdminExportShipMetaRepairs :one
SELECT CAST(json_group_array(json_object('ship_id', CASE WHEN typeof(ship_id) = 'blob' THEN (CASE WHEN json_valid(CAST(ship_id AS TEXT)) THEN CAST(ship_id AS TEXT) ELSE hex(ship_id) END) ELSE ship_id END, 'repair_id', CASE WHEN typeof(repair_id) = 'blob' THEN (CASE WHEN json_valid(CAST(repair_id AS TEXT)) THEN CAST(repair_id AS TEXT) ELSE hex(repair_id) END) ELSE repair_id END)) AS TEXT) FROM ship_meta_repairs WHERE ship_meta_repairs.ship_id IN (SELECT o.id FROM ships AS o WHERE o.commander_id = sqlc.arg(id));

-- name: AdminDeleteShipMetaRepairs :exec
DELETE FROM ship_meta_repairs WHERE ship_meta_repairs.ship_id IN (SELECT o.id FROM ships AS o WHERE o.commander_id = sqlc.arg(id));

-- name: AdminExportShipMetaTactics :one
SELECT CAST(json_group_array(json_object('ship_id', CASE WHEN typeof(ship_id) = 'blob' THEN (CASE WHEN json_valid(CAST(ship_id AS TEXT)) THEN CAST(ship_id AS TEXT) ELSE hex(ship_id) END) ELSE ship_id END, 'skill_id', CASE WHEN typeof(skill_id) = 'blob' THEN (CASE WHEN json_valid(CAST(skill_id AS TEXT)) THEN CAST(skill_id AS TEXT) ELSE hex(skill_id) END) ELSE skill_id END, 'day', CASE WHEN typeof(day) = 'blob' THEN (CASE WHEN json_valid(CAST(day AS TEXT)) THEN CAST(day AS TEXT) ELSE hex(day) END) ELSE day END, 'day_exp', CASE WHEN typeof(day_exp) = 'blob' THEN (CASE WHEN json_valid(CAST(day_exp AS TEXT)) THEN CAST(day_exp AS TEXT) ELSE hex(day_exp) END) ELSE day_exp END, 'double_exp', CASE WHEN typeof(double_exp) = 'blob' THEN (CASE WHEN json_valid(CAST(double_exp AS TEXT)) THEN CAST(double_exp AS TEXT) ELSE hex(double_exp) END) ELSE double_exp END, 'switches', CASE WHEN typeof(switches) = 'blob' THEN (CASE WHEN json_valid(CAST(switches AS TEXT)) THEN CAST(switches AS TEXT) ELSE hex(switches) END) ELSE switches END)) AS TEXT) FROM ship_meta_tactics WHERE ship_meta_tactics.ship_id IN (SELECT o.id FROM ships AS o WHERE o.commander_id = sqlc.arg(id));

-- name: AdminDeleteShipMetaTactics :exec
DELETE FROM ship_meta_tactics WHERE ship_meta_tactics.ship_id IN (SELECT o.id FROM ships AS o WHERE o.commander_id = sqlc.arg(id));

-- name: AdminExportShipSkills :one
SELECT CAST(json_group_array(json_object('ship_id', CASE WHEN typeof(ship_id) = 'blob' THEN (CASE WHEN json_valid(CAST(ship_id AS TEXT)) THEN CAST(ship_id AS TEXT) ELSE hex(ship_id) END) ELSE ship_id END, 'skill_id', CASE WHEN typeof(skill_id) = 'blob' THEN (CASE WHEN json_valid(CAST(skill_id AS TEXT)) THEN CAST(skill_id AS TEXT) ELSE hex(skill_id) END) ELSE skill_id END, 'level', CASE WHEN typeof(level) = 'blob' THEN (CASE WHEN json_valid(CAST(level AS TEXT)) THEN CAST(level AS TEXT) ELSE hex(level) END) ELSE level END, 'exp', CASE WHEN typeof(exp) = 'blob' THEN (CASE WHEN json_valid(CAST(exp AS TEXT)) THEN CAST(exp AS TEXT) ELSE hex(exp) END) ELSE exp END)) AS TEXT) FROM ship_skills WHERE ship_skills.ship_id IN (SELECT o.id FROM ships AS o WHERE o.commander_id = sqlc.arg(id));

-- name: AdminDeleteShipSkills :exec
DELETE FROM ship_skills WHERE ship_skills.ship_id IN (SELECT o.id FROM ships AS o WHERE o.commander_id = sqlc.arg(id));

-- name: AdminExportShipSkinShadows :one
SELECT CAST(json_group_array(json_object('ship_id', CASE WHEN typeof(ship_id) = 'blob' THEN (CASE WHEN json_valid(CAST(ship_id AS TEXT)) THEN CAST(ship_id AS TEXT) ELSE hex(ship_id) END) ELSE ship_id END, 'shadow_id', CASE WHEN typeof(shadow_id) = 'blob' THEN (CASE WHEN json_valid(CAST(shadow_id AS TEXT)) THEN CAST(shadow_id AS TEXT) ELSE hex(shadow_id) END) ELSE shadow_id END, 'skin_id', CASE WHEN typeof(skin_id) = 'blob' THEN (CASE WHEN json_valid(CAST(skin_id AS TEXT)) THEN CAST(skin_id AS TEXT) ELSE hex(skin_id) END) ELSE skin_id END)) AS TEXT) FROM ship_skin_shadows WHERE ship_skin_shadows.ship_id IN (SELECT o.id FROM ships AS o WHERE o.commander_id = sqlc.arg(id));

-- name: AdminDeleteShipSkinShadows :exec
DELETE FROM ship_skin_shadows WHERE ship_skin_shadows.ship_id IN (SELECT o.id FROM ships AS o WHERE o.commander_id = sqlc.arg(id));

-- name: AdminExportShipSpweapons :one
SELECT CAST(json_group_array(json_object('ship_id', CASE WHEN typeof(ship_id) = 'blob' THEN (CASE WHEN json_valid(CAST(ship_id AS TEXT)) THEN CAST(ship_id AS TEXT) ELSE hex(ship_id) END) ELSE ship_id END, 'spweapon_id', CASE WHEN typeof(spweapon_id) = 'blob' THEN (CASE WHEN json_valid(CAST(spweapon_id AS TEXT)) THEN CAST(spweapon_id AS TEXT) ELSE hex(spweapon_id) END) ELSE spweapon_id END)) AS TEXT) FROM ship_spweapons WHERE ship_spweapons.spweapon_id IN (SELECT o.id FROM spweapons AS o WHERE o.commander_id = sqlc.arg(id)) OR ship_spweapons.ship_id IN (SELECT o.id FROM ships AS o WHERE o.commander_id = sqlc.arg(id));

-- name: AdminDeleteShipSpweapons :exec
DELETE FROM ship_spweapons WHERE ship_spweapons.spweapon_id IN (SELECT o.id FROM spweapons AS o WHERE o.commander_id = sqlc.arg(id)) OR ship_spweapons.ship_id IN (SELECT o.id FROM ships AS o WHERE o.commander_id = sqlc.arg(id));

-- name: AdminExportShipStrengths :one
SELECT CAST(json_group_array(json_object('ship_id', CASE WHEN typeof(ship_id) = 'blob' THEN (CASE WHEN json_valid(CAST(ship_id AS TEXT)) THEN CAST(ship_id AS TEXT) ELSE hex(ship_id) END) ELSE ship_id END, 'attr_id', CASE WHEN typeof(attr_id) = 'blob' THEN (CASE WHEN json_valid(CAST(attr_id AS TEXT)) THEN CAST(attr_id AS TEXT) ELSE hex(attr_id) END) ELSE attr_id END, 'exp', CASE WHEN typeof(exp) = 'blob' THEN (CASE WHEN json_valid(CAST(exp AS TEXT)) THEN CAST(exp AS TEXT) ELSE hex(exp) END) ELSE exp END)) AS TEXT) FROM ship_strengths WHERE ship_strengths.ship_id IN (SELECT o.id FROM ships AS o WHERE o.commander_id = sqlc.arg(id));

-- name: AdminDeleteShipStrengths :exec
DELETE FROM ship_strengths WHERE ship_strengths.ship_id IN (SELECT o.id FROM ships AS o WHERE o.commander_id = sqlc.arg(id));

-- name: AdminExportShipTransforms :one
SELECT CAST(json_group_array(json_object('ship_id', CASE WHEN typeof(ship_id) = 'blob' THEN (CASE WHEN json_valid(CAST(ship_id AS TEXT)) THEN CAST(ship_id AS TEXT) ELSE hex(ship_id) END) ELSE ship_id END, 'transform_id', CASE WHEN typeof(transform_id) = 'blob' THEN (CASE WHEN json_valid(CAST(transform_id AS TEXT)) THEN CAST(transform_id AS TEXT) ELSE hex(transform_id) END) ELSE transform_id END, 'level', CASE WHEN typeof(level) = 'blob' THEN (CASE WHEN json_valid(CAST(level AS TEXT)) THEN CAST(level AS TEXT) ELSE hex(level) END) ELSE level END)) AS TEXT) FROM ship_transforms WHERE ship_transforms.ship_id IN (SELECT o.id FROM ships AS o WHERE o.commander_id = sqlc.arg(id));

-- name: AdminDeleteShipTransforms :exec
DELETE FROM ship_transforms WHERE ship_transforms.ship_id IN (SELECT o.id FROM ships AS o WHERE o.commander_id = sqlc.arg(id));

-- name: AdminExportShips :one
SELECT CAST(json_group_array(json_object('id', CASE WHEN typeof(id) = 'blob' THEN (CASE WHEN json_valid(CAST(id AS TEXT)) THEN CAST(id AS TEXT) ELSE hex(id) END) ELSE id END, 'commander_id', CASE WHEN typeof(commander_id) = 'blob' THEN (CASE WHEN json_valid(CAST(commander_id AS TEXT)) THEN CAST(commander_id AS TEXT) ELSE hex(commander_id) END) ELSE commander_id END, 'template_id', CASE WHEN typeof(template_id) = 'blob' THEN (CASE WHEN json_valid(CAST(template_id AS TEXT)) THEN CAST(template_id AS TEXT) ELSE hex(template_id) END) ELSE template_id END, 'level', CASE WHEN typeof(level) = 'blob' THEN (CASE WHEN json_valid(CAST(level AS TEXT)) THEN CAST(level AS TEXT) ELSE hex(level) END) ELSE level END, 'exp', CASE WHEN typeof(exp) = 'blob' THEN (CASE WHEN json_valid(CAST(exp AS TEXT)) THEN CAST(exp AS TEXT) ELSE hex(exp) END) ELSE exp END, 'energy', CASE WHEN typeof(energy) = 'blob' THEN (CASE WHEN json_valid(CAST(energy AS TEXT)) THEN CAST(energy AS TEXT) ELSE hex(energy) END) ELSE energy END, 'intimacy', CASE WHEN typeof(intimacy) = 'blob' THEN (CASE WHEN json_valid(CAST(intimacy AS TEXT)) THEN CAST(intimacy AS TEXT) ELSE hex(intimacy) END) ELSE intimacy END, 'max_level', CASE WHEN typeof(max_level) = 'blob' THEN (CASE WHEN json_valid(CAST(max_level AS TEXT)) THEN CAST(max_level AS TEXT) ELSE hex(max_level) END) ELSE max_level END, 'is_locked', CASE WHEN typeof(is_locked) = 'blob' THEN (CASE WHEN json_valid(CAST(is_locked AS TEXT)) THEN CAST(is_locked AS TEXT) ELSE hex(is_locked) END) ELSE is_locked END, 'created_at', CASE WHEN typeof(created_at) = 'blob' THEN (CASE WHEN json_valid(CAST(created_at AS TEXT)) THEN CAST(created_at AS TEXT) ELSE hex(created_at) END) ELSE created_at END, 'state', CASE WHEN typeof(state) = 'blob' THEN (CASE WHEN json_valid(CAST(state AS TEXT)) THEN CAST(state AS TEXT) ELSE hex(state) END) ELSE state END, 'proficiency', CASE WHEN typeof(proficiency) = 'blob' THEN (CASE WHEN json_valid(CAST(proficiency AS TEXT)) THEN CAST(proficiency AS TEXT) ELSE hex(proficiency) END) ELSE proficiency END, 'skin_id', CASE WHEN typeof(skin_id) = 'blob' THEN (CASE WHEN json_valid(CAST(skin_id AS TEXT)) THEN CAST(skin_id AS TEXT) ELSE hex(skin_id) END) ELSE skin_id END, 'propose', CASE WHEN typeof(propose) = 'blob' THEN (CASE WHEN json_valid(CAST(propose AS TEXT)) THEN CAST(propose AS TEXT) ELSE hex(propose) END) ELSE propose END, 'name', CASE WHEN typeof(name) = 'blob' THEN (CASE WHEN json_valid(CAST(name AS TEXT)) THEN CAST(name AS TEXT) ELSE hex(name) END) ELSE name END, 'change_name_timestamp', CASE WHEN typeof(change_name_timestamp) = 'blob' THEN (CASE WHEN json_valid(CAST(change_name_timestamp AS TEXT)) THEN CAST(change_name_timestamp AS TEXT) ELSE hex(change_name_timestamp) END) ELSE change_name_timestamp END, 'common_flag', CASE WHEN typeof(common_flag) = 'blob' THEN (CASE WHEN json_valid(CAST(common_flag AS TEXT)) THEN CAST(common_flag AS TEXT) ELSE hex(common_flag) END) ELSE common_flag END, 'random_flag', CASE WHEN typeof(random_flag) = 'blob' THEN (CASE WHEN json_valid(CAST(random_flag AS TEXT)) THEN CAST(random_flag AS TEXT) ELSE hex(random_flag) END) ELSE random_flag END)) AS TEXT) FROM ships WHERE ships.commander_id = sqlc.arg(id);

-- name: AdminDeleteShips :exec
DELETE FROM ships WHERE ships.commander_id = sqlc.arg(id);

-- name: AdminExportSpweapons :one
SELECT CAST(json_group_array(json_object('id', CASE WHEN typeof(id) = 'blob' THEN (CASE WHEN json_valid(CAST(id AS TEXT)) THEN CAST(id AS TEXT) ELSE hex(id) END) ELSE id END, 'commander_id', CASE WHEN typeof(commander_id) = 'blob' THEN (CASE WHEN json_valid(CAST(commander_id AS TEXT)) THEN CAST(commander_id AS TEXT) ELSE hex(commander_id) END) ELSE commander_id END, 'template_id', CASE WHEN typeof(template_id) = 'blob' THEN (CASE WHEN json_valid(CAST(template_id AS TEXT)) THEN CAST(template_id AS TEXT) ELSE hex(template_id) END) ELSE template_id END, 'attr_1', CASE WHEN typeof(attr_1) = 'blob' THEN (CASE WHEN json_valid(CAST(attr_1 AS TEXT)) THEN CAST(attr_1 AS TEXT) ELSE hex(attr_1) END) ELSE attr_1 END, 'attr_2', CASE WHEN typeof(attr_2) = 'blob' THEN (CASE WHEN json_valid(CAST(attr_2 AS TEXT)) THEN CAST(attr_2 AS TEXT) ELSE hex(attr_2) END) ELSE attr_2 END, 'attr_temp_1', CASE WHEN typeof(attr_temp_1) = 'blob' THEN (CASE WHEN json_valid(CAST(attr_temp_1 AS TEXT)) THEN CAST(attr_temp_1 AS TEXT) ELSE hex(attr_temp_1) END) ELSE attr_temp_1 END, 'attr_temp_2', CASE WHEN typeof(attr_temp_2) = 'blob' THEN (CASE WHEN json_valid(CAST(attr_temp_2 AS TEXT)) THEN CAST(attr_temp_2 AS TEXT) ELSE hex(attr_temp_2) END) ELSE attr_temp_2 END, 'pt', CASE WHEN typeof(pt) = 'blob' THEN (CASE WHEN json_valid(CAST(pt AS TEXT)) THEN CAST(pt AS TEXT) ELSE hex(pt) END) ELSE pt END)) AS TEXT) FROM spweapons WHERE spweapons.commander_id = sqlc.arg(id);

-- name: AdminDeleteSpweapons :exec
DELETE FROM spweapons WHERE spweapons.commander_id = sqlc.arg(id);

-- name: AdminExportSurveysDone :one
SELECT CAST(json_group_array(json_object('commander_id', CASE WHEN typeof(commander_id) = 'blob' THEN (CASE WHEN json_valid(CAST(commander_id AS TEXT)) THEN CAST(commander_id AS TEXT) ELSE hex(commander_id) END) ELSE commander_id END, 'survey_id', CASE WHEN typeof(survey_id) = 'blob' THEN (CASE WHEN json_valid(CAST(survey_id AS TEXT)) THEN CAST(survey_id AS TEXT) ELSE hex(survey_id) END) ELSE survey_id END, 'done_at', CASE WHEN typeof(done_at) = 'blob' THEN (CASE WHEN json_valid(CAST(done_at AS TEXT)) THEN CAST(done_at AS TEXT) ELSE hex(done_at) END) ELSE done_at END)) AS TEXT) FROM surveys_done WHERE surveys_done.commander_id = sqlc.arg(id);

-- name: AdminDeleteSurveysDone :exec
DELETE FROM surveys_done WHERE surveys_done.commander_id = sqlc.arg(id);

-- name: AdminExportTechnologyCatchups :one
SELECT CAST(json_group_array(json_object('commander_id', CASE WHEN typeof(commander_id) = 'blob' THEN (CASE WHEN json_valid(CAST(commander_id AS TEXT)) THEN CAST(commander_id AS TEXT) ELSE hex(commander_id) END) ELSE commander_id END, 'version', CASE WHEN typeof(version) = 'blob' THEN (CASE WHEN json_valid(CAST(version AS TEXT)) THEN CAST(version AS TEXT) ELSE hex(version) END) ELSE version END, 'ship_group', CASE WHEN typeof(ship_group) = 'blob' THEN (CASE WHEN json_valid(CAST(ship_group AS TEXT)) THEN CAST(ship_group AS TEXT) ELSE hex(ship_group) END) ELSE ship_group END, 'number', CASE WHEN typeof(number) = 'blob' THEN (CASE WHEN json_valid(CAST(number AS TEXT)) THEN CAST(number AS TEXT) ELSE hex(number) END) ELSE number END)) AS TEXT) FROM technology_catchups WHERE technology_catchups.commander_id = sqlc.arg(id);

-- name: AdminDeleteTechnologyCatchups :exec
DELETE FROM technology_catchups WHERE technology_catchups.commander_id = sqlc.arg(id);

-- name: AdminExportTechnologyOffers :one
SELECT CAST(json_group_array(json_object('commander_id', CASE WHEN typeof(commander_id) = 'blob' THEN (CASE WHEN json_valid(CAST(commander_id AS TEXT)) THEN CAST(commander_id AS TEXT) ELSE hex(commander_id) END) ELSE commander_id END, 'pos', CASE WHEN typeof(pos) = 'blob' THEN (CASE WHEN json_valid(CAST(pos AS TEXT)) THEN CAST(pos AS TEXT) ELSE hex(pos) END) ELSE pos END, 'pool_id', CASE WHEN typeof(pool_id) = 'blob' THEN (CASE WHEN json_valid(CAST(pool_id AS TEXT)) THEN CAST(pool_id AS TEXT) ELSE hex(pool_id) END) ELSE pool_id END, 'tech_id', CASE WHEN typeof(tech_id) = 'blob' THEN (CASE WHEN json_valid(CAST(tech_id AS TEXT)) THEN CAST(tech_id AS TEXT) ELSE hex(tech_id) END) ELSE tech_id END, 'finish_at', CASE WHEN typeof(finish_at) = 'blob' THEN (CASE WHEN json_valid(CAST(finish_at AS TEXT)) THEN CAST(finish_at AS TEXT) ELSE hex(finish_at) END) ELSE finish_at END)) AS TEXT) FROM technology_offers WHERE technology_offers.commander_id = sqlc.arg(id);

-- name: AdminDeleteTechnologyOffers :exec
DELETE FROM technology_offers WHERE technology_offers.commander_id = sqlc.arg(id);

-- name: AdminExportTechnologyQueue :one
SELECT CAST(json_group_array(json_object('id', CASE WHEN typeof(id) = 'blob' THEN (CASE WHEN json_valid(CAST(id AS TEXT)) THEN CAST(id AS TEXT) ELSE hex(id) END) ELSE id END, 'commander_id', CASE WHEN typeof(commander_id) = 'blob' THEN (CASE WHEN json_valid(CAST(commander_id AS TEXT)) THEN CAST(commander_id AS TEXT) ELSE hex(commander_id) END) ELSE commander_id END, 'tech_id', CASE WHEN typeof(tech_id) = 'blob' THEN (CASE WHEN json_valid(CAST(tech_id AS TEXT)) THEN CAST(tech_id AS TEXT) ELSE hex(tech_id) END) ELSE tech_id END, 'finish_at', CASE WHEN typeof(finish_at) = 'blob' THEN (CASE WHEN json_valid(CAST(finish_at AS TEXT)) THEN CAST(finish_at AS TEXT) ELSE hex(finish_at) END) ELSE finish_at END)) AS TEXT) FROM technology_queue WHERE technology_queue.commander_id = sqlc.arg(id);

-- name: AdminDeleteTechnologyQueue :exec
DELETE FROM technology_queue WHERE technology_queue.commander_id = sqlc.arg(id);

-- name: AdminExportTechnologyStates :one
SELECT CAST(json_group_array(json_object('commander_id', CASE WHEN typeof(commander_id) = 'blob' THEN (CASE WHEN json_valid(CAST(commander_id AS TEXT)) THEN CAST(commander_id AS TEXT) ELSE hex(commander_id) END) ELSE commander_id END, 'refreshed_at', CASE WHEN typeof(refreshed_at) = 'blob' THEN (CASE WHEN json_valid(CAST(refreshed_at AS TEXT)) THEN CAST(refreshed_at AS TEXT) ELSE hex(refreshed_at) END) ELSE refreshed_at END, 'focus', CASE WHEN typeof(focus) = 'blob' THEN (CASE WHEN json_valid(CAST(focus AS TEXT)) THEN CAST(focus AS TEXT) ELSE hex(focus) END) ELSE focus END, 'catchup_version', CASE WHEN typeof(catchup_version) = 'blob' THEN (CASE WHEN json_valid(CAST(catchup_version AS TEXT)) THEN CAST(catchup_version AS TEXT) ELSE hex(catchup_version) END) ELSE catchup_version END, 'catchup_target', CASE WHEN typeof(catchup_target) = 'blob' THEN (CASE WHEN json_valid(CAST(catchup_target AS TEXT)) THEN CAST(catchup_target AS TEXT) ELSE hex(catchup_target) END) ELSE catchup_target END)) AS TEXT) FROM technology_states WHERE technology_states.commander_id = sqlc.arg(id);

-- name: AdminDeleteTechnologyStates :exec
DELETE FROM technology_states WHERE technology_states.commander_id = sqlc.arg(id);

-- name: AdminExportWorldBossShips :one
SELECT CAST(json_group_array(json_object('commander_id', CASE WHEN typeof(commander_id) = 'blob' THEN (CASE WHEN json_valid(CAST(commander_id AS TEXT)) THEN CAST(commander_id AS TEXT) ELSE hex(commander_id) END) ELSE commander_id END, 'pos', CASE WHEN typeof(pos) = 'blob' THEN (CASE WHEN json_valid(CAST(pos AS TEXT)) THEN CAST(pos AS TEXT) ELSE hex(pos) END) ELSE pos END, 'ship_id', CASE WHEN typeof(ship_id) = 'blob' THEN (CASE WHEN json_valid(CAST(ship_id AS TEXT)) THEN CAST(ship_id AS TEXT) ELSE hex(ship_id) END) ELSE ship_id END)) AS TEXT) FROM world_boss_ships WHERE world_boss_ships.commander_id = sqlc.arg(id);

-- name: AdminDeleteWorldBossShips :exec
DELETE FROM world_boss_ships WHERE world_boss_ships.commander_id = sqlc.arg(id);

-- name: AdminExportWorldBosses :one
SELECT CAST(json_group_array(json_object('commander_id', CASE WHEN typeof(commander_id) = 'blob' THEN (CASE WHEN json_valid(CAST(commander_id AS TEXT)) THEN CAST(commander_id AS TEXT) ELSE hex(commander_id) END) ELSE commander_id END, 'summon_pt', CASE WHEN typeof(summon_pt) = 'blob' THEN (CASE WHEN json_valid(CAST(summon_pt AS TEXT)) THEN CAST(summon_pt AS TEXT) ELSE hex(summon_pt) END) ELSE summon_pt END, 'summon_pt_old', CASE WHEN typeof(summon_pt_old) = 'blob' THEN (CASE WHEN json_valid(CAST(summon_pt_old AS TEXT)) THEN CAST(summon_pt_old AS TEXT) ELSE hex(summon_pt_old) END) ELSE summon_pt_old END, 'summon_pt_daily_acc', CASE WHEN typeof(summon_pt_daily_acc) = 'blob' THEN (CASE WHEN json_valid(CAST(summon_pt_daily_acc AS TEXT)) THEN CAST(summon_pt_daily_acc AS TEXT) ELSE hex(summon_pt_daily_acc) END) ELSE summon_pt_daily_acc END, 'summon_pt_old_daily_acc', CASE WHEN typeof(summon_pt_old_daily_acc) = 'blob' THEN (CASE WHEN json_valid(CAST(summon_pt_old_daily_acc AS TEXT)) THEN CAST(summon_pt_old_daily_acc AS TEXT) ELSE hex(summon_pt_old_daily_acc) END) ELSE summon_pt_old_daily_acc END, 'acc_day', CASE WHEN typeof(acc_day) = 'blob' THEN (CASE WHEN json_valid(CAST(acc_day AS TEXT)) THEN CAST(acc_day AS TEXT) ELSE hex(acc_day) END) ELSE acc_day END, 'summon_free', CASE WHEN typeof(summon_free) = 'blob' THEN (CASE WHEN json_valid(CAST(summon_free AS TEXT)) THEN CAST(summon_free AS TEXT) ELSE hex(summon_free) END) ELSE summon_free END, 'archives_id', CASE WHEN typeof(archives_id) = 'blob' THEN (CASE WHEN json_valid(CAST(archives_id AS TEXT)) THEN CAST(archives_id AS TEXT) ELSE hex(archives_id) END) ELSE archives_id END, 'self_boss_lv', CASE WHEN typeof(self_boss_lv) = 'blob' THEN (CASE WHEN json_valid(CAST(self_boss_lv AS TEXT)) THEN CAST(self_boss_lv AS TEXT) ELSE hex(self_boss_lv) END) ELSE self_boss_lv END, 'self_boss_template_id', CASE WHEN typeof(self_boss_template_id) = 'blob' THEN (CASE WHEN json_valid(CAST(self_boss_template_id AS TEXT)) THEN CAST(self_boss_template_id AS TEXT) ELSE hex(self_boss_template_id) END) ELSE self_boss_template_id END, 'boss_template_id', CASE WHEN typeof(boss_template_id) = 'blob' THEN (CASE WHEN json_valid(CAST(boss_template_id AS TEXT)) THEN CAST(boss_template_id AS TEXT) ELSE hex(boss_template_id) END) ELSE boss_template_id END, 'boss_lv', CASE WHEN typeof(boss_lv) = 'blob' THEN (CASE WHEN json_valid(CAST(boss_lv AS TEXT)) THEN CAST(boss_lv AS TEXT) ELSE hex(boss_lv) END) ELSE boss_lv END, 'boss_hp', CASE WHEN typeof(boss_hp) = 'blob' THEN (CASE WHEN json_valid(CAST(boss_hp AS TEXT)) THEN CAST(boss_hp AS TEXT) ELSE hex(boss_hp) END) ELSE boss_hp END, 'boss_last_time', CASE WHEN typeof(boss_last_time) = 'blob' THEN (CASE WHEN json_valid(CAST(boss_last_time AS TEXT)) THEN CAST(boss_last_time AS TEXT) ELSE hex(boss_last_time) END) ELSE boss_last_time END, 'boss_kill_time', CASE WHEN typeof(boss_kill_time) = 'blob' THEN (CASE WHEN json_valid(CAST(boss_kill_time AS TEXT)) THEN CAST(boss_kill_time AS TEXT) ELSE hex(boss_kill_time) END) ELSE boss_kill_time END, 'boss_fight_count', CASE WHEN typeof(boss_fight_count) = 'blob' THEN (CASE WHEN json_valid(CAST(boss_fight_count AS TEXT)) THEN CAST(boss_fight_count AS TEXT) ELSE hex(boss_fight_count) END) ELSE boss_fight_count END, 'boss_damage', CASE WHEN typeof(boss_damage) = 'blob' THEN (CASE WHEN json_valid(CAST(boss_damage AS TEXT)) THEN CAST(boss_damage AS TEXT) ELSE hex(boss_damage) END) ELSE boss_damage END, 'auto_fight_finish_time', CASE WHEN typeof(auto_fight_finish_time) = 'blob' THEN (CASE WHEN json_valid(CAST(auto_fight_finish_time AS TEXT)) THEN CAST(auto_fight_finish_time AS TEXT) ELSE hex(auto_fight_finish_time) END) ELSE auto_fight_finish_time END, 'auto_fight_max_damage', CASE WHEN typeof(auto_fight_max_damage) = 'blob' THEN (CASE WHEN json_valid(CAST(auto_fight_max_damage AS TEXT)) THEN CAST(auto_fight_max_damage AS TEXT) ELSE hex(auto_fight_max_damage) END) ELSE auto_fight_max_damage END, 'guild_support', CASE WHEN typeof(guild_support) = 'blob' THEN (CASE WHEN json_valid(CAST(guild_support AS TEXT)) THEN CAST(guild_support AS TEXT) ELSE hex(guild_support) END) ELSE guild_support END, 'friend_support', CASE WHEN typeof(friend_support) = 'blob' THEN (CASE WHEN json_valid(CAST(friend_support AS TEXT)) THEN CAST(friend_support AS TEXT) ELSE hex(friend_support) END) ELSE friend_support END, 'world_support', CASE WHEN typeof(world_support) = 'blob' THEN (CASE WHEN json_valid(CAST(world_support AS TEXT)) THEN CAST(world_support AS TEXT) ELSE hex(world_support) END) ELSE world_support END)) AS TEXT) FROM world_bosses WHERE world_bosses.commander_id = sqlc.arg(id);

-- name: AdminDeleteWorldBosses :exec
DELETE FROM world_bosses WHERE world_bosses.commander_id = sqlc.arg(id);

-- name: AdminExportWorldMaps :one
SELECT CAST(json_group_array(json_object('commander_id', CASE WHEN typeof(commander_id) = 'blob' THEN (CASE WHEN json_valid(CAST(commander_id AS TEXT)) THEN CAST(commander_id AS TEXT) ELSE hex(commander_id) END) ELSE commander_id END, 'random_id', CASE WHEN typeof(random_id) = 'blob' THEN (CASE WHEN json_valid(CAST(random_id AS TEXT)) THEN CAST(random_id AS TEXT) ELSE hex(random_id) END) ELSE random_id END, 'state', CASE WHEN typeof(state) = 'blob' THEN (CASE WHEN json_valid(CAST(state AS TEXT)) THEN CAST(state AS TEXT) ELSE hex(state) END) ELSE state END)) AS TEXT) FROM world_maps WHERE world_maps.commander_id = sqlc.arg(id);

-- name: AdminDeleteWorldMaps :exec
DELETE FROM world_maps WHERE world_maps.commander_id = sqlc.arg(id);

-- name: AdminExportWorldStates :one
SELECT CAST(json_group_array(json_object('commander_id', CASE WHEN typeof(commander_id) = 'blob' THEN (CASE WHEN json_valid(CAST(commander_id AS TEXT)) THEN CAST(commander_id AS TEXT) ELSE hex(commander_id) END) ELSE commander_id END, 'state', CASE WHEN typeof(state) = 'blob' THEN (CASE WHEN json_valid(CAST(state AS TEXT)) THEN CAST(state AS TEXT) ELSE hex(state) END) ELSE state END)) AS TEXT) FROM world_states WHERE world_states.commander_id = sqlc.arg(id);

-- name: AdminDeleteWorldStates :exec
DELETE FROM world_states WHERE world_states.commander_id = sqlc.arg(id);

-- name: AdminExportCommanders :one
SELECT CAST(json_group_array(json_object('id', CASE WHEN typeof(id) = 'blob' THEN (CASE WHEN json_valid(CAST(id AS TEXT)) THEN CAST(id AS TEXT) ELSE hex(id) END) ELSE id END, 'account_id', CASE WHEN typeof(account_id) = 'blob' THEN (CASE WHEN json_valid(CAST(account_id AS TEXT)) THEN CAST(account_id AS TEXT) ELSE hex(account_id) END) ELSE account_id END, 'name', CASE WHEN typeof(name) = 'blob' THEN (CASE WHEN json_valid(CAST(name AS TEXT)) THEN CAST(name AS TEXT) ELSE hex(name) END) ELSE name END, 'starter_ship', CASE WHEN typeof(starter_ship) = 'blob' THEN (CASE WHEN json_valid(CAST(starter_ship AS TEXT)) THEN CAST(starter_ship AS TEXT) ELSE hex(starter_ship) END) ELSE starter_ship END, 'registered_at', CASE WHEN typeof(registered_at) = 'blob' THEN (CASE WHEN json_valid(CAST(registered_at AS TEXT)) THEN CAST(registered_at AS TEXT) ELSE hex(registered_at) END) ELSE registered_at END, 'guide_index', CASE WHEN typeof(guide_index) = 'blob' THEN (CASE WHEN json_valid(CAST(guide_index AS TEXT)) THEN CAST(guide_index AS TEXT) ELSE hex(guide_index) END) ELSE guide_index END, 'new_guide_index', CASE WHEN typeof(new_guide_index) = 'blob' THEN (CASE WHEN json_valid(CAST(new_guide_index AS TEXT)) THEN CAST(new_guide_index AS TEXT) ELSE hex(new_guide_index) END) ELSE new_guide_index END, 'level', CASE WHEN typeof(level) = 'blob' THEN (CASE WHEN json_valid(CAST(level AS TEXT)) THEN CAST(level AS TEXT) ELSE hex(level) END) ELSE level END, 'exp', CASE WHEN typeof(exp) = 'blob' THEN (CASE WHEN json_valid(CAST(exp AS TEXT)) THEN CAST(exp AS TEXT) ELSE hex(exp) END) ELSE exp END, 'ship_bag_max', CASE WHEN typeof(ship_bag_max) = 'blob' THEN (CASE WHEN json_valid(CAST(ship_bag_max AS TEXT)) THEN CAST(ship_bag_max AS TEXT) ELSE hex(ship_bag_max) END) ELSE ship_bag_max END, 'equip_bag_max', CASE WHEN typeof(equip_bag_max) = 'blob' THEN (CASE WHEN json_valid(CAST(equip_bag_max AS TEXT)) THEN CAST(equip_bag_max AS TEXT) ELSE hex(equip_bag_max) END) ELSE equip_bag_max END, 'commander_bag_max', CASE WHEN typeof(commander_bag_max) = 'blob' THEN (CASE WHEN json_valid(CAST(commander_bag_max AS TEXT)) THEN CAST(commander_bag_max AS TEXT) ELSE hex(commander_bag_max) END) ELSE commander_bag_max END, 'rank', CASE WHEN typeof(rank) = 'blob' THEN (CASE WHEN json_valid(CAST(rank AS TEXT)) THEN CAST(rank AS TEXT) ELSE hex(rank) END) ELSE rank END, 'chat_room_id', CASE WHEN typeof(chat_room_id) = 'blob' THEN (CASE WHEN json_valid(CAST(chat_room_id AS TEXT)) THEN CAST(chat_room_id AS TEXT) ELSE hex(chat_room_id) END) ELSE chat_room_id END, 'mail_storeroom_lv', CASE WHEN typeof(mail_storeroom_lv) = 'blob' THEN (CASE WHEN json_valid(CAST(mail_storeroom_lv AS TEXT)) THEN CAST(mail_storeroom_lv AS TEXT) ELSE hex(mail_storeroom_lv) END) ELSE mail_storeroom_lv END, 'manifesto', CASE WHEN typeof(manifesto) = 'blob' THEN (CASE WHEN json_valid(CAST(manifesto AS TEXT)) THEN CAST(manifesto AS TEXT) ELSE hex(manifesto) END) ELSE manifesto END, 'dorm_food', CASE WHEN typeof(dorm_food) = 'blob' THEN (CASE WHEN json_valid(CAST(dorm_food AS TEXT)) THEN CAST(dorm_food AS TEXT) ELSE hex(dorm_food) END) ELSE dorm_food END, 'skill_class_num', CASE WHEN typeof(skill_class_num) = 'blob' THEN (CASE WHEN json_valid(CAST(skill_class_num AS TEXT)) THEN CAST(skill_class_num AS TEXT) ELSE hex(skill_class_num) END) ELSE skill_class_num END, 'quick_finish_day', CASE WHEN typeof(quick_finish_day) = 'blob' THEN (CASE WHEN json_valid(CAST(quick_finish_day AS TEXT)) THEN CAST(quick_finish_day AS TEXT) ELSE hex(quick_finish_day) END) ELSE quick_finish_day END, 'quick_finishes', CASE WHEN typeof(quick_finishes) = 'blob' THEN (CASE WHEN json_valid(CAST(quick_finishes AS TEXT)) THEN CAST(quick_finishes AS TEXT) ELSE hex(quick_finishes) END) ELSE quick_finishes END, 'oil_field_at', CASE WHEN typeof(oil_field_at) = 'blob' THEN (CASE WHEN json_valid(CAST(oil_field_at AS TEXT)) THEN CAST(oil_field_at AS TEXT) ELSE hex(oil_field_at) END) ELSE oil_field_at END, 'gold_field_at', CASE WHEN typeof(gold_field_at) = 'blob' THEN (CASE WHEN json_valid(CAST(gold_field_at AS TEXT)) THEN CAST(gold_field_at AS TEXT) ELSE hex(gold_field_at) END) ELSE gold_field_at END, 'oil_field_level', CASE WHEN typeof(oil_field_level) = 'blob' THEN (CASE WHEN json_valid(CAST(oil_field_level AS TEXT)) THEN CAST(oil_field_level AS TEXT) ELSE hex(oil_field_level) END) ELSE oil_field_level END, 'gold_field_level', CASE WHEN typeof(gold_field_level) = 'blob' THEN (CASE WHEN json_valid(CAST(gold_field_level AS TEXT)) THEN CAST(gold_field_level AS TEXT) ELSE hex(gold_field_level) END) ELSE gold_field_level END, 'class_field_level', CASE WHEN typeof(class_field_level) = 'blob' THEN (CASE WHEN json_valid(CAST(class_field_level AS TEXT)) THEN CAST(class_field_level AS TEXT) ELSE hex(class_field_level) END) ELSE class_field_level END, 'class_proficiency', CASE WHEN typeof(class_proficiency) = 'blob' THEN (CASE WHEN json_valid(CAST(class_proficiency AS TEXT)) THEN CAST(class_proficiency AS TEXT) ELSE hex(class_proficiency) END) ELSE class_proficiency END, 'class_field_at', CASE WHEN typeof(class_field_at) = 'blob' THEN (CASE WHEN json_valid(CAST(class_field_at AS TEXT)) THEN CAST(class_field_at AS TEXT) ELSE hex(class_field_at) END) ELSE class_field_at END, 'oil_field_upgrade_at', CASE WHEN typeof(oil_field_upgrade_at) = 'blob' THEN (CASE WHEN json_valid(CAST(oil_field_upgrade_at AS TEXT)) THEN CAST(oil_field_upgrade_at AS TEXT) ELSE hex(oil_field_upgrade_at) END) ELSE oil_field_upgrade_at END, 'gold_field_upgrade_at', CASE WHEN typeof(gold_field_upgrade_at) = 'blob' THEN (CASE WHEN json_valid(CAST(gold_field_upgrade_at AS TEXT)) THEN CAST(gold_field_upgrade_at AS TEXT) ELSE hex(gold_field_upgrade_at) END) ELSE gold_field_upgrade_at END, 'class_field_upgrade_at', CASE WHEN typeof(class_field_upgrade_at) = 'blob' THEN (CASE WHEN json_valid(CAST(class_field_upgrade_at AS TEXT)) THEN CAST(class_field_upgrade_at AS TEXT) ELSE hex(class_field_upgrade_at) END) ELSE class_field_upgrade_at END, 'last_online', CASE WHEN typeof(last_online) = 'blob' THEN (CASE WHEN json_valid(CAST(last_online AS TEXT)) THEN CAST(last_online AS TEXT) ELSE hex(last_online) END) ELSE last_online END, 'marry_ship', CASE WHEN typeof(marry_ship) = 'blob' THEN (CASE WHEN json_valid(CAST(marry_ship AS TEXT)) THEN CAST(marry_ship AS TEXT) ELSE hex(marry_ship) END) ELSE marry_ship END, 'child_display', CASE WHEN typeof(child_display) = 'blob' THEN (CASE WHEN json_valid(CAST(child_display AS TEXT)) THEN CAST(child_display AS TEXT) ELSE hex(child_display) END) ELSE child_display END)) AS TEXT) FROM commanders WHERE commanders.id = sqlc.arg(id);

-- name: AdminDeleteCommanders :exec
DELETE FROM commanders WHERE commanders.id = sqlc.arg(id);
