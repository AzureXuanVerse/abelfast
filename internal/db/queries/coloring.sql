-- name: ListColoringCells :many
SELECT row, col, color FROM commander_coloring_cells
WHERE commander_id = ? AND activity_id = ? AND group_id = ?
ORDER BY row, col;

-- name: CountColoringCells :one
SELECT COUNT(*) FROM commander_coloring_cells
WHERE commander_id = ? AND activity_id = ? AND group_id = ?;

-- name: SetColoringCell :exec
INSERT INTO commander_coloring_cells (commander_id, activity_id, group_id, row, col, color) VALUES (?, ?, ?, ?, ?, ?)
ON CONFLICT (commander_id, activity_id, group_id, row, col) DO UPDATE SET color = excluded.color;

-- name: DeleteColoringCell :exec
DELETE FROM commander_coloring_cells
WHERE commander_id = ? AND activity_id = ? AND group_id = ? AND row = ? AND col = ?;

-- name: ClearColoringCells :exec
DELETE FROM commander_coloring_cells
WHERE commander_id = ? AND activity_id = ? AND group_id = ?;

-- name: ListColoringAwards :many
SELECT group_id, drops FROM commander_coloring_awards
WHERE commander_id = ? AND activity_id = ?
ORDER BY group_id;

-- name: AddColoringAward :execrows
INSERT INTO commander_coloring_awards (commander_id, activity_id, group_id, drops) VALUES (?, ?, ?, ?)
ON CONFLICT DO NOTHING;
