-- name: GetChargeExp :one
SELECT * FROM charge_exp WHERE commander_id = ?;

-- name: SaveChargeExp :exec
INSERT INTO charge_exp (commander_id, exp, taps, tapped_at) VALUES (?, ?, ?, ?)
ON CONFLICT (commander_id) DO UPDATE SET
	exp = excluded.exp,
	taps = excluded.taps,
	tapped_at = excluded.tapped_at;

-- name: CountExchangeCodeRedemptions :one
SELECT COUNT(*) FROM exchange_code_redemptions WHERE commander_id = ? AND code = ?;

-- name: AddExchangeCodeRedemption :exec
INSERT INTO exchange_code_redemptions (commander_id, code, redeemed_at) VALUES (?, ?, ?);

-- name: CountSurveysDone :one
SELECT COUNT(*) FROM surveys_done WHERE commander_id = ? AND survey_id = ?;

-- name: AddSurveyDone :exec
INSERT INTO surveys_done (commander_id, survey_id, done_at) VALUES (?, ?, ?)
ON CONFLICT (commander_id, survey_id) DO NOTHING;
