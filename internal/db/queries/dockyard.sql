-- name: UpdateShipGrowth :exec
UPDATE ships SET template_id = ?, level = ?, exp = ?, max_level = ? WHERE id = ? AND commander_id = ?;

-- name: DeleteShipSkill :exec
DELETE FROM ship_skills WHERE ship_id = ? AND skill_id = ?;

-- name: ListShipStrengths :many
SELECT * FROM ship_strengths WHERE ship_id = ? ORDER BY attr_id;

-- name: ListShipStrengthsByCommander :many
SELECT t.ship_id, t.attr_id, t.exp FROM ship_strengths t
JOIN ships s ON s.id = t.ship_id
WHERE s.commander_id = ? ORDER BY t.ship_id, t.attr_id;

-- name: SetShipStrength :exec
INSERT INTO ship_strengths (ship_id, attr_id, exp) VALUES (?, ?, ?)
ON CONFLICT (ship_id, attr_id) DO UPDATE SET exp = excluded.exp;
