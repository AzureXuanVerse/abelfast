-- name: ListCollectionAwards :many
SELECT storeup_id, award_index FROM commander_collection_awards
WHERE commander_id = ? ORDER BY storeup_id, award_index;

-- name: LastCollectionAward :one
SELECT CAST(COALESCE(MAX(award_index), 0) AS INTEGER) FROM commander_collection_awards
WHERE commander_id = ? AND storeup_id = ?;

-- name: AddCollectionAward :execrows
INSERT INTO commander_collection_awards (commander_id, storeup_id, award_index) VALUES (?, ?, ?)
ON CONFLICT DO NOTHING;
