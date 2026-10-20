-- name: ListMiniGameHubs :many
SELECT * FROM commander_minigame_hubs WHERE commander_id = ? ORDER BY hub_id;

-- name: GetMiniGameHub :one
SELECT * FROM commander_minigame_hubs WHERE commander_id = ? AND hub_id = ?;

-- name: OpenMiniGameHub :exec
INSERT INTO commander_minigame_hubs (commander_id, hub_id, opened_at) VALUES (?, ?, ?)
ON CONFLICT (commander_id, hub_id) DO NOTHING;

-- name: CompleteMiniGameStage :execrows
UPDATE commander_minigame_hubs SET used_cnt = used_cnt + 1
WHERE commander_id = sqlc.arg(commander_id) AND hub_id = sqlc.arg(hub_id) AND used_cnt = sqlc.arg(used_cnt);

-- name: ClaimMiniGameUltimate :execrows
UPDATE commander_minigame_hubs SET ultimate = 1
WHERE commander_id = ? AND hub_id = ? AND ultimate = 0;

-- name: ListMiniGameScores :many
SELECT * FROM commander_minigame_scores WHERE commander_id = ? ORDER BY game_id;

-- name: SaveMiniGameScore :exec
INSERT INTO commander_minigame_scores (commander_id, game_id, score, extra) VALUES (?, ?, ?, ?)
ON CONFLICT (commander_id, game_id) DO UPDATE SET score = excluded.score, extra = excluded.extra;

-- name: ListMiniGameRanking :many
SELECT * FROM commander_minigame_scores
WHERE game_id = sqlc.arg(game_id) AND commander_id IN (sqlc.slice(commander_ids))
ORDER BY score DESC, extra DESC, commander_id;

-- name: GetMiniGameData :one
SELECT * FROM commander_minigame_data WHERE commander_id = ? AND game_id = ?;

-- name: SaveMiniGameData :exec
INSERT INTO commander_minigame_data (commander_id, game_id, datas, count, count_day, gold) VALUES (?, ?, ?, ?, ?, ?)
ON CONFLICT (commander_id, game_id) DO UPDATE SET
	datas = excluded.datas, count = excluded.count, count_day = excluded.count_day, gold = excluded.gold;

-- name: SumMiniGameGold :one
SELECT CAST(COALESCE(SUM(gold), 0) AS INTEGER) FROM commander_minigame_data WHERE game_id = ?;

-- name: ListMiniGameValues :many
SELECT * FROM commander_minigame_values WHERE commander_id = ? AND game_id = ? ORDER BY list_key, key;

-- name: SaveMiniGameValue :exec
INSERT INTO commander_minigame_values (commander_id, game_id, list_key, key, value, value2) VALUES (?, ?, ?, ?, ?, ?)
ON CONFLICT (commander_id, game_id, list_key, key) DO UPDATE SET value = excluded.value, value2 = excluded.value2;
