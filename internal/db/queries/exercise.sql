-- name: GetExercise :one
SELECT * FROM commander_exercise WHERE commander_id = ?;

-- name: SaveExercise :exec
INSERT INTO commander_exercise (commander_id, season, score, best_rank, fight_count, refilled_at,
	flash_count, flash_day, vanguard, main, rivals, shop_refreshes, shop_day)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT (commander_id) DO UPDATE SET season = excluded.season, score = excluded.score,
	best_rank = excluded.best_rank, fight_count = excluded.fight_count, refilled_at = excluded.refilled_at,
	flash_count = excluded.flash_count, flash_day = excluded.flash_day, vanguard = excluded.vanguard,
	main = excluded.main, rivals = excluded.rivals, shop_refreshes = excluded.shop_refreshes,
	shop_day = excluded.shop_day;

-- name: ListExerciseStandings :many
-- every commander with their score in the season starting at season (0
-- without one), best first, then by id: the newest commander ranks last
-- among equals
SELECT c.id, c.name, c.level, CAST(coalesce(e.score, 0) AS INTEGER) AS score
FROM commanders c
LEFT JOIN commander_exercise e ON e.commander_id = c.id AND e.season = sqlc.arg(season)
ORDER BY score DESC, c.id;

-- name: DeleteShopPurchases :exec
-- goods bought from a stock that was restocked
DELETE FROM commander_shop_purchases
WHERE commander_id = sqlc.arg(commander_id) AND expires_at = sqlc.arg(expires_at) AND shop_id IN (sqlc.slice(shop_ids));
