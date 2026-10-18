-- name: ListLimitChallenges :many
SELECT * FROM limit_challenges WHERE commander_id = ? ORDER BY challenge_id;

-- name: PassLimitChallenge :exec
INSERT INTO limit_challenges (commander_id, challenge_id, best_time, pass_month) VALUES (?, ?, ?, ?)
ON CONFLICT (commander_id, challenge_id) DO UPDATE SET
	best_time = CASE WHEN best_time = 0 OR excluded.best_time < best_time THEN excluded.best_time ELSE best_time END,
	pass_month = excluded.pass_month;

-- name: AwardLimitChallenge :exec
UPDATE limit_challenges SET awarded = TRUE WHERE commander_id = ? AND challenge_id = ?;
