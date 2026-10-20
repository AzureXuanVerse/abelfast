-- name: ListShipTransforms :many
SELECT * FROM ship_transforms WHERE ship_id = ? ORDER BY transform_id;

-- name: ListShipTransformsByCommander :many
SELECT t.ship_id, t.transform_id, t.level FROM ship_transforms t
JOIN ships s ON s.id = t.ship_id
WHERE s.commander_id = ? ORDER BY t.ship_id, t.transform_id;

-- name: SetShipTransform :exec
INSERT INTO ship_transforms (ship_id, transform_id, level) VALUES (?, ?, ?)
ON CONFLICT (ship_id, transform_id) DO UPDATE SET level = excluded.level;

-- name: DeleteShipTransform :exec
DELETE FROM ship_transforms WHERE ship_id = ? AND transform_id = ?;

-- name: RenameShipSkill :exec
UPDATE ship_skills SET skill_id = sqlc.arg(new_skill_id) WHERE ship_id = sqlc.arg(ship_id) AND skill_id = sqlc.arg(skill_id);

-- name: RenameSkillClassSkill :exec
UPDATE commander_skill_classes SET skill_id = sqlc.arg(new_skill_id)
WHERE commander_id = sqlc.arg(commander_id) AND ship_id = sqlc.arg(ship_id) AND skill_id = sqlc.arg(skill_id);

-- name: SetShipGroupTrans :exec
UPDATE commander_ship_groups SET trans = TRUE WHERE commander_id = ? AND group_id = ?;
