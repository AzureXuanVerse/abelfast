-- name: ListDailyLevelCounts :many
SELECT * FROM daily_level_counts WHERE commander_id = ? ORDER BY daily_id;

-- name: SaveDailyLevelCount :exec
INSERT INTO daily_level_counts (commander_id, daily_id, count, period) VALUES (?, ?, ?, ?)
ON CONFLICT (commander_id, daily_id) DO UPDATE SET count = excluded.count, period = excluded.period;

-- name: ListDailyQuickStages :many
SELECT stage_id FROM daily_quick_stages WHERE commander_id = ? ORDER BY stage_id;

-- name: AddDailyQuickStage :exec
INSERT INTO daily_quick_stages (commander_id, stage_id) VALUES (?, ?) ON CONFLICT DO NOTHING;
