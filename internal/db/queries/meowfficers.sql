-- name: ListMeowfficers :many
SELECT * FROM meowfficers WHERE commander_id = ? ORDER BY id;

-- name: GetMeowfficer :one
SELECT * FROM meowfficers WHERE commander_id = ? AND id = ?;

-- name: CountMeowfficers :one
SELECT COUNT(*) FROM meowfficers WHERE commander_id = ?;

-- name: CreateMeowfficer :one
INSERT INTO meowfficers (commander_id, template_id, skill_id, talents, talent_origins)
VALUES (?, ?, ?, ?, ?)
RETURNING *;

-- name: SetMeowfficerLevel :exec
UPDATE meowfficers SET level = ?, exp = ? WHERE commander_id = ? AND id = ?;

-- name: SetMeowfficerHomeCleanTime :exec
UPDATE meowfficers SET home_clean_time = ? WHERE commander_id = ? AND id = ?;

-- name: SetMeowfficerHomeFeedTime :exec
UPDATE meowfficers SET home_feed_time = ? WHERE commander_id = ? AND id = ?;

-- name: SetMeowfficerHomePlayTime :exec
UPDATE meowfficers SET home_play_time = ? WHERE commander_id = ? AND id = ?;

-- name: ListMeowfficerBoxes :many
SELECT * FROM meowfficer_boxes WHERE commander_id = ? ORDER BY box_id;

-- name: CreateMeowfficerBox :exec
INSERT INTO meowfficer_boxes (commander_id, box_id, pool_id, template_id, begin_time, finish_time)
VALUES (?, ?, ?, ?, ?, ?);

-- name: SetMeowfficerBoxTimes :exec
UPDATE meowfficer_boxes SET begin_time = ?, finish_time = ? WHERE commander_id = ? AND box_id = ?;

-- name: DeleteMeowfficerBox :exec
DELETE FROM meowfficer_boxes WHERE commander_id = ? AND box_id = ?;

-- name: GetMeowfficerState :one
SELECT * FROM meowfficer_states WHERE commander_id = ?;

-- name: CreateMeowfficerState :exec
INSERT INTO meowfficer_states (commander_id) VALUES (?) ON CONFLICT DO NOTHING;

-- name: SetMeowfficerReserve :exec
UPDATE meowfficer_states SET reserve_count = ?, reserve_day = ?, reserve_total = ?
WHERE commander_id = ?;

-- name: SetMeowfficerHome :exec
UPDATE meowfficer_states SET home_level = ?, home_exp = ?, home_clean = ?
WHERE commander_id = ?;

-- name: ListMeowfficerCatteries :many
SELECT * FROM meowfficer_catteries WHERE commander_id = ? ORDER BY slot_id;

-- name: SaveMeowfficerCattery :exec
INSERT INTO meowfficer_catteries (commander_id, slot_id, meowfficer_id, style, exp_time, clean_time, feed_time, play_time)
VALUES (?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT (commander_id, slot_id) DO UPDATE SET
	meowfficer_id = excluded.meowfficer_id,
	style = excluded.style,
	exp_time = excluded.exp_time,
	clean_time = excluded.clean_time,
	feed_time = excluded.feed_time,
	play_time = excluded.play_time;

-- name: ListFleetMeowfficers :many
SELECT fleet_id, pos, meowfficer_id FROM fleet_meowfficers WHERE commander_id = ? ORDER BY fleet_id, pos;

-- name: SetFleetMeowfficer :exec
INSERT INTO fleet_meowfficers (commander_id, fleet_id, pos, meowfficer_id) VALUES (?, ?, ?, ?)
ON CONFLICT (commander_id, fleet_id, pos) DO UPDATE SET meowfficer_id = excluded.meowfficer_id;

-- name: ClearFleetMeowfficer :exec
DELETE FROM fleet_meowfficers WHERE commander_id = ? AND fleet_id = ? AND pos = ?;

-- name: ListMeowfficerPresets :many
SELECT * FROM meowfficer_presets WHERE commander_id = ? ORDER BY preset_id;

-- name: ListMeowfficerPresetSlots :many
SELECT preset_id, pos, meowfficer_id FROM meowfficer_preset_slots WHERE commander_id = ? ORDER BY preset_id, pos;

-- name: SetMeowfficerLocked :exec
UPDATE meowfficers SET locked = ? WHERE commander_id = ? AND id = ?;

-- name: RenameMeowfficer :exec
UPDATE meowfficers SET name = ?, rename_time = ? WHERE commander_id = ? AND id = ?;

-- name: SetMeowfficerSkill :exec
UPDATE meowfficers SET skill_id = ?, skill_exp = ? WHERE commander_id = ? AND id = ?;

-- name: DeleteMeowfficer :exec
DELETE FROM meowfficers WHERE commander_id = ? AND id = ?;

-- name: DeleteMeowfficerFromPresets :exec
DELETE FROM meowfficer_preset_slots WHERE commander_id = ? AND meowfficer_id = ?;
-- name: GetMeowfficerPreset :one
SELECT * FROM meowfficer_presets WHERE commander_id = ? AND preset_id = ?;

-- name: CreateMeowfficerPreset :exec
INSERT INTO meowfficer_presets (commander_id, preset_id) VALUES (?, ?) ON CONFLICT DO NOTHING;

-- name: RenameMeowfficerPreset :exec
UPDATE meowfficer_presets SET name = ?, rename_time = ? WHERE commander_id = ? AND preset_id = ?;

-- name: ClearMeowfficerPresetSlots :exec
DELETE FROM meowfficer_preset_slots WHERE commander_id = ? AND preset_id = ?;

-- name: SetMeowfficerPresetSlot :exec
INSERT INTO meowfficer_preset_slots (commander_id, preset_id, pos, meowfficer_id) VALUES (?, ?, ?, ?);

-- name: SetMeowfficerHomeClean :exec
UPDATE meowfficer_states SET home_clean = ?, clean_time = ? WHERE commander_id = ?;
-- name: SetMeowfficerTalents :exec
UPDATE meowfficers SET talents = ?, talent_choices = ?, used_pt = ?, talent_time = ?
WHERE commander_id = ? AND id = ?;

-- name: SetMeowfficerTalentChoices :exec
UPDATE meowfficers SET talent_choices = ? WHERE commander_id = ? AND id = ?;
