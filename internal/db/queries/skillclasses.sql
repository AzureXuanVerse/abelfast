-- name: ListSkillClasses :many
SELECT * FROM commander_skill_classes WHERE commander_id = ? ORDER BY room_id;

-- name: GetSkillClass :one
SELECT * FROM commander_skill_classes WHERE commander_id = ? AND room_id = ?;

-- name: AddSkillClass :exec
INSERT INTO commander_skill_classes (commander_id, room_id, ship_id, skill_id, item_id, start_time, finish_time, exp)
VALUES (?, ?, ?, ?, ?, ?, ?, ?);

-- name: DeleteSkillClass :exec
DELETE FROM commander_skill_classes WHERE commander_id = ? AND room_id = ?;

-- name: GetShipSkill :one
SELECT k.ship_id, k.skill_id, k.level, k.exp FROM ship_skills k
JOIN ships s ON s.id = k.ship_id
WHERE s.commander_id = ? AND k.ship_id = ? AND k.skill_id = ?;

-- name: SetShipSkillExp :exec
UPDATE ship_skills SET level = ?, exp = ? WHERE ship_id = ? AND skill_id = ?;

-- name: AddSkillClassNum :exec
UPDATE commanders SET skill_class_num = skill_class_num + 1 WHERE id = ?;

-- name: SetQuickFinishes :exec
UPDATE commanders SET quick_finish_day = ?, quick_finishes = ? WHERE id = ?;
