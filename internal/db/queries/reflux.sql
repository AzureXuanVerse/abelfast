-- name: GetReflux :one
SELECT * FROM commander_reflux WHERE commander_id = ?;

-- name: StartReflux :exec
INSERT INTO commander_reflux (commander_id, return_lv, return_time, ship_number, last_offline_time) VALUES (?, ?, ?, ?, ?)
ON CONFLICT (commander_id) DO UPDATE SET
	return_lv = excluded.return_lv, return_time = excluded.return_time, ship_number = excluded.ship_number,
	last_offline_time = excluded.last_offline_time, pt = 0, sign_cnt = 0, sign_last_time = 0, pt_stage = 0;

-- name: SignReflux :execrows
UPDATE commander_reflux SET sign_cnt = sign_cnt + 1, sign_last_time = sqlc.arg(now)
WHERE commander_id = sqlc.arg(commander_id) AND sign_cnt = sqlc.arg(sign_cnt);

-- name: AddRefluxPt :exec
UPDATE commander_reflux SET pt = pt + ? WHERE commander_id = ?;

-- name: ClaimRefluxPtStage :execrows
UPDATE commander_reflux SET pt_stage = pt_stage + 1
WHERE commander_id = sqlc.arg(commander_id) AND pt_stage = sqlc.arg(pt_stage);
