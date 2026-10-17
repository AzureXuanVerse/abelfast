-- name: AddCommission :exec
INSERT INTO commander_commissions (commander_id, collection_id, finish_time, over_time) VALUES (?, ?, 0, 0) ON CONFLICT DO NOTHING;

-- name: ListCommissions :many
SELECT * FROM commander_commissions WHERE commander_id = ? ORDER BY rowid;
