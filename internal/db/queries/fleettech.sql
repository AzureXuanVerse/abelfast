-- name: ListFleetTechs :many
SELECT * FROM fleet_techs WHERE commander_id = ? ORDER BY group_id;

-- name: SaveFleetTech :exec
INSERT INTO fleet_techs (commander_id, group_id, effect_tech_id, study_tech_id, study_finish_at, rewarded_tech_id)
VALUES (?, ?, ?, ?, ?, ?)
ON CONFLICT (commander_id, group_id) DO UPDATE SET
	effect_tech_id = excluded.effect_tech_id,
	study_tech_id = excluded.study_tech_id,
	study_finish_at = excluded.study_finish_at,
	rewarded_tech_id = excluded.rewarded_tech_id;

-- name: ListFleetTechAttrs :many
SELECT * FROM fleet_tech_attrs WHERE commander_id = ? ORDER BY ship_type, attr_type;

-- name: DeleteFleetTechAttrs :exec
DELETE FROM fleet_tech_attrs WHERE commander_id = ?;

-- name: AddFleetTechAttr :exec
INSERT INTO fleet_tech_attrs (commander_id, ship_type, attr_type, set_value) VALUES (?, ?, ?, ?);
