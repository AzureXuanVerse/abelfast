-- The back-office's timers, progress and research screens (internal/admin
-- area_progress.go, internal/game admin_progress.go).

-- name: ProgressListCommissionShips :many
SELECT cs.collection_id, cs.pos, s.id, s.template_id, s.skin_id, s.name
FROM commander_commission_ships cs JOIN ships s ON s.id = cs.ship_id
WHERE cs.commander_id = ? ORDER BY cs.collection_id, cs.pos;

-- name: ProgressListLessons :many
SELECT c.room_id, c.ship_id, s.template_id, s.skin_id, s.name, c.skill_id, c.item_id, c.start_time, c.finish_time
FROM commander_skill_classes c JOIN ships s ON s.id = c.ship_id
WHERE c.commander_id = ? ORDER BY c.room_id;

-- name: ProgressSetLessonFinish :exec
UPDATE commander_skill_classes SET finish_time = ? WHERE commander_id = ? AND room_id = ?;

-- name: ProgressResetQuickFinishes :exec
UPDATE commanders SET quick_finishes = 0 WHERE id = ?;

-- name: ProgressResetCommissionDraws :exec
UPDATE commander_commission_refresh SET daily_drawn = 0 WHERE commander_id = ?;

-- name: ProgressSetAutoBattleFinish :exec
UPDATE chapter_auto_battles SET finish_at = ? WHERE commander_id = ? AND seq = ?;

-- name: ProgressSetResearchQueueFinish :exec
UPDATE technology_queue SET finish_at = ? WHERE id = ? AND commander_id = ?;

-- name: ProgressDeleteFleetTechAttrsOfType :exec
DELETE FROM fleet_tech_attrs WHERE commander_id = ? AND ship_type = ?;

-- name: ProgressCollection :one
SELECT
	(SELECT COUNT(*) FROM commander_ship_groups g WHERE g.commander_id = sqlc.arg(commander_id)) AS ship_groups,
	(SELECT COUNT(*) FROM commander_stories s WHERE s.commander_id = sqlc.arg(commander_id)) AS stories,
	(SELECT COUNT(*) FROM commander_trophy_claims t WHERE t.commander_id = sqlc.arg(commander_id)) AS trophies;
