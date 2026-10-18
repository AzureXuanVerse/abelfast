-- name: GetRequisitionCount :one
SELECT count FROM commander_requisitions
WHERE commander_id = ? AND month = ?;

-- name: AddRequisitions :exec
INSERT INTO commander_requisitions (commander_id, month, count) VALUES (?, ?, ?)
ON CONFLICT (commander_id, month) DO UPDATE SET count = count + excluded.count;
