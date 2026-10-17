-- name: CreateFleet :exec
INSERT INTO fleets (commander_id, fleet_id, name) VALUES (?, ?, ?) ON CONFLICT DO NOTHING;

-- name: ListFleets :many
SELECT fleet_id, name FROM fleets WHERE commander_id = ? ORDER BY fleet_id DESC;

-- name: ListFleetShips :many
SELECT fleet_id, ship_id FROM fleet_ships WHERE commander_id = ? ORDER BY fleet_id, position;

-- name: ClearFleetShips :exec
DELETE FROM fleet_ships WHERE commander_id = ? AND fleet_id = ?;

-- name: AddFleetShip :exec
INSERT INTO fleet_ships (commander_id, fleet_id, position, ship_id) VALUES (?, ?, ?, ?);

-- name: ListChapterProgress :many
SELECT * FROM chapter_progress WHERE commander_id = ? ORDER BY chapter_id;

-- name: GetChapterProgress :one
SELECT * FROM chapter_progress WHERE commander_id = ? AND chapter_id = ?;

-- name: SaveChapterProgress :exec
INSERT INTO chapter_progress (commander_id, chapter_id, progress, kill_boss_count, kill_enemy_count, take_box_count, defeat_count, today_defeat_count, pass_count)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT (commander_id, chapter_id) DO UPDATE SET
	progress = excluded.progress,
	kill_boss_count = excluded.kill_boss_count,
	kill_enemy_count = excluded.kill_enemy_count,
	take_box_count = excluded.take_box_count,
	defeat_count = excluded.defeat_count,
	today_defeat_count = excluded.today_defeat_count,
	pass_count = excluded.pass_count;

-- name: GetChapterState :one
SELECT * FROM chapter_states WHERE commander_id = ?;

-- name: SaveChapterState :exec
INSERT INTO chapter_states (commander_id, chapter_id, state, ambushed) VALUES (?, ?, ?, ?)
ON CONFLICT (commander_id) DO UPDATE SET chapter_id = excluded.chapter_id, state = excluded.state, ambushed = excluded.ambushed;

-- name: DeleteChapterState :exec
DELETE FROM chapter_states WHERE commander_id = ?;

-- name: AddCommission :exec
INSERT INTO commander_commissions (commander_id, collection_id, finish_time, over_time) VALUES (?, ?, 0, 0) ON CONFLICT DO NOTHING;

-- name: ListCommissions :many
SELECT * FROM commander_commissions WHERE commander_id = ? ORDER BY rowid;

-- Spending is the inventory area's; this is the minimum battles need.
-- name: SpendResource :exec
UPDATE commander_resources SET amount = MAX(amount - ?, 0) WHERE commander_id = ? AND resource_id = ?;

-- Battle results on a ship, the ships area owns the table.
-- name: UpdateShipBattleState :exec
UPDATE ships SET level = ?, exp = ?, intimacy = ?, energy = ? WHERE id = ? AND commander_id = ?;
