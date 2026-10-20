-- name: GetTechnologyState :one
SELECT * FROM technology_states WHERE commander_id = ?;

-- name: SaveTechnologyState :exec
INSERT INTO technology_states (commander_id, refreshed_at, focus, catchup_version, catchup_target)
VALUES (?, ?, ?, ?, ?)
ON CONFLICT (commander_id) DO UPDATE SET
	refreshed_at = excluded.refreshed_at,
	focus = excluded.focus,
	catchup_version = excluded.catchup_version,
	catchup_target = excluded.catchup_target;

-- name: ListTechnologyOffers :many
SELECT * FROM technology_offers WHERE commander_id = ? ORDER BY pos;

-- name: DeleteTechnologyOffers :exec
DELETE FROM technology_offers WHERE commander_id = ?;

-- name: AddTechnologyOffer :exec
INSERT INTO technology_offers (commander_id, pos, pool_id, tech_id) VALUES (?, ?, ?, ?);

-- name: SetTechnologyOfferFinish :exec
UPDATE technology_offers SET finish_at = ? WHERE commander_id = ? AND pos = ?;

-- name: ListTechnologyQueue :many
SELECT * FROM technology_queue WHERE commander_id = ? ORDER BY finish_at, id;

-- name: AddTechnologyQueue :exec
INSERT INTO technology_queue (commander_id, tech_id, finish_at) VALUES (?, ?, ?);

-- name: DeleteTechnologyQueue :exec
DELETE FROM technology_queue WHERE commander_id = ? AND id = ?;

-- name: ListTechnologyCatchups :many
SELECT * FROM technology_catchups WHERE commander_id = ? ORDER BY version, ship_group;

-- name: AddTechnologyCatchup :exec
INSERT INTO technology_catchups (commander_id, version, ship_group, number) VALUES (?, ?, ?, ?)
ON CONFLICT (commander_id, version, ship_group) DO UPDATE SET number = number + excluded.number;
