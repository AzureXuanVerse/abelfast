-- name: GetGuildUser :one
SELECT * FROM guild_users WHERE commander_id = ?;

-- name: CreateGuildUser :exec
INSERT INTO guild_users (commander_id) VALUES (?) ON CONFLICT DO NOTHING;

-- name: SetGuildUserDonations :exec
UPDATE guild_users SET donate_day = ?, donate_count = ?, donate_tasks = ?, extra_donate = ?
WHERE commander_id = ?;

-- name: SetGuildUserBenefitTime :exec
UPDATE guild_users SET benefit_time = ? WHERE commander_id = ?;

-- name: SetGuildUserWeeklyTaskWeek :exec
UPDATE guild_users SET weekly_task_week = ? WHERE commander_id = ?;

-- name: AddGuildUserExtras :exec
UPDATE guild_users SET extra_donate = extra_donate + ?, extra_operation = extra_operation + ?
WHERE commander_id = ?;

-- name: SetGuildUserExtraOperation :exec
UPDATE guild_users SET extra_operation = ? WHERE commander_id = ?;

-- name: GetGuildLogistics :one
SELECT * FROM guild_logistics WHERE guild_id = ?;

-- name: CreateGuildLogistics :exec
INSERT INTO guild_logistics (guild_id) VALUES (?) ON CONFLICT DO NOTHING;

-- name: SetGuildWeeklyTask :exec
UPDATE guild_logistics SET weekly_task_id = ?, weekly_task_progress = ?, weekly_task_week = ?
WHERE guild_id = ?;

-- name: SetGuildBenefit :exec
UPDATE guild_logistics SET benefit_finish_time = ?, last_benefit_finish_time = ?
WHERE guild_id = ?;

-- name: SetGuildDonated :exec
UPDATE guild_logistics SET donate_day = ?, donated_capital = ?, donated_tech = ?
WHERE guild_id = ?;

-- name: AddGuildCapital :exec
UPDATE guilds SET capital = capital + ? WHERE id = ?;

-- name: SpendGuildCapital :execrows
UPDATE guilds SET capital = capital - sqlc.arg(amount) WHERE id = sqlc.arg(id) AND capital >= sqlc.arg(amount);

-- name: SetGuildLevel :exec
UPDATE guilds SET level = ?, exp = ? WHERE id = ?;

-- name: SetGuildMemberLiveness :exec
UPDATE guild_members SET liveness = ?, duty = ? WHERE commander_id = ?;

-- name: AddGuildCapitalLog :exec
INSERT INTO guild_capital_logs (guild_id, member_id, name, event_type, event_target, time)
VALUES (?, ?, ?, ?, ?, ?);

-- name: ListGuildCapitalLogs :many
SELECT * FROM (
	SELECT * FROM guild_capital_logs WHERE guild_id = sqlc.arg(guild_id) ORDER BY id DESC LIMIT sqlc.arg(count)
) ORDER BY id;

-- name: GetGuildRankScore :one
SELECT * FROM guild_rank_scores WHERE guild_id = ? AND commander_id = ? AND kind = ?;

-- name: SetGuildRankScore :exec
INSERT INTO guild_rank_scores (guild_id, commander_id, kind, week_start, week, month_start, month, total)
VALUES (?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT (guild_id, commander_id, kind) DO UPDATE SET
	week_start = excluded.week_start, week = excluded.week,
	month_start = excluded.month_start, month = excluded.month, total = excluded.total;

-- name: ListGuildRankScores :many
SELECT * FROM guild_rank_scores WHERE guild_id = ? AND kind = ? ORDER BY commander_id;

-- name: ListGuildTasks :many
SELECT task_id, accept_time, submit_time FROM commander_tasks WHERE commander_id = ?;

-- name: DeleteCommanderTask :exec
DELETE FROM commander_tasks WHERE commander_id = ? AND task_id = ?;
