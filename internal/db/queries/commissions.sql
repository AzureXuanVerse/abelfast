-- name: AddCommission :exec
INSERT INTO commander_commissions (commander_id, collection_id, finish_time, over_time) VALUES (?, ?, 0, 0) ON CONFLICT DO NOTHING;

-- name: ListCommissions :many
SELECT * FROM commander_commissions WHERE commander_id = ? ORDER BY rowid;

-- name: OfferCommission :exec
INSERT INTO commander_commissions (commander_id, collection_id, finish_time, over_time) VALUES (?, ?, 0, ?) ON CONFLICT DO NOTHING;

-- name: GetCommission :one
SELECT * FROM commander_commissions WHERE commander_id = ? AND collection_id = ?;

-- name: SetCommissionFinishTime :exec
UPDATE commander_commissions SET finish_time = ? WHERE commander_id = ? AND collection_id = ?;

-- name: SetCommissionOverTime :exec
UPDATE commander_commissions SET over_time = ? WHERE commander_id = ? AND collection_id = ?;

-- name: DeleteCommission :exec
DELETE FROM commander_commissions WHERE commander_id = ? AND collection_id = ?;

-- name: ListCommissionShips :many
SELECT collection_id, ship_id FROM commander_commission_ships WHERE commander_id = ? ORDER BY collection_id, pos;

-- name: AddCommissionShip :exec
INSERT INTO commander_commission_ships (commander_id, collection_id, pos, ship_id) VALUES (?, ?, ?, ?);

-- name: DeleteCommissionShips :exec
DELETE FROM commander_commission_ships WHERE commander_id = ? AND collection_id = ?;

-- name: GetCommissionRefresh :one
SELECT refreshed_at FROM commander_commission_refresh WHERE commander_id = ?;

-- name: SetCommissionRefresh :exec
INSERT INTO commander_commission_refresh (commander_id, refreshed_at) VALUES (?, ?)
ON CONFLICT (commander_id) DO UPDATE SET refreshed_at = excluded.refreshed_at;
