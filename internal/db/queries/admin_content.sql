-- name: ListExchangeCodeRedemptions :many
SELECT r.commander_id, c.name, r.code, r.redeemed_at
FROM exchange_code_redemptions r JOIN commanders c ON c.id = r.commander_id
ORDER BY r.redeemed_at DESC LIMIT ?;

-- name: CountExchangeCodeUses :many
SELECT code, COUNT(*) AS uses FROM exchange_code_redemptions GROUP BY code;

-- name: ListAllActivityBossMarks :many
SELECT * FROM activity_boss_marks ORDER BY activity_id, boss_id, mark DESC;

-- name: DeleteActivityBossMark :execrows
DELETE FROM activity_boss_marks WHERE activity_id = ? AND boss_id = ? AND mark = ?;

-- name: ListSurveysTaken :many
SELECT survey_id, COUNT(*) AS taken, CAST(MAX(done_at) AS INTEGER) AS latest
FROM surveys_done GROUP BY survey_id ORDER BY survey_id DESC;

-- name: ListSurveyTakers :many
SELECT commander_id FROM surveys_done WHERE survey_id = ?;

-- name: DeleteSurveysDone :execrows
DELETE FROM surveys_done WHERE survey_id = ?;
