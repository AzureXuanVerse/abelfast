-- name: ListTrophyClaims :many
SELECT trophy_id, claimed_at FROM commander_trophy_claims WHERE commander_id = ?;

-- name: ClaimTrophy :execrows
INSERT INTO commander_trophy_claims (commander_id, trophy_id, claimed_at) VALUES (?, ?, ?)
ON CONFLICT DO NOTHING;

-- name: ListDisplayMedals :many
SELECT medal_id FROM commander_display_medals WHERE commander_id = ? ORDER BY position;

-- name: ClearDisplayMedals :exec
DELETE FROM commander_display_medals WHERE commander_id = ?;

-- name: AddDisplayMedal :exec
INSERT INTO commander_display_medals (commander_id, position, medal_id) VALUES (?, ?, ?);
