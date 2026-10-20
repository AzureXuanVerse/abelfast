-- name: ListActivityData1 :many
SELECT data1 FROM commander_activities WHERE activity_id = ?;

-- name: ListActivityData1Lists :many
SELECT data1_list FROM commander_activities WHERE activity_id = ?;

-- name: ListActivityBossMarks :many
SELECT * FROM activity_boss_marks WHERE activity_id = ? ORDER BY boss_id, mark DESC;

-- name: AddActivityBossMark :exec
INSERT INTO activity_boss_marks (activity_id, boss_id, mark, reached_at) VALUES (?, ?, ?, ?)
ON CONFLICT (activity_id, boss_id, mark) DO NOTHING;
