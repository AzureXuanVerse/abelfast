-- name: CountCommandersRegisteredSince :one
SELECT COUNT(*) FROM commanders WHERE registered_at >= ?;

-- name: CountMailsSince :one
SELECT COUNT(*) FROM mails WHERE date >= ?;

-- name: ListMailBatchesSince :many
-- the mails sent from a time on, one row per title, sender and second (a
-- mail to several commanders at once)
SELECT title, sender, date, COUNT(*) AS recipients FROM mails
WHERE date >= ? GROUP BY title, sender, date ORDER BY date DESC LIMIT ?;

-- name: ListFlagshipDisplays :many
-- every commander's first secretary, the ship its avatar shows
SELECT cs.commander_id, s.template_id, s.skin_id FROM commander_secretaries cs
JOIN ships s ON s.id = cs.ship_id AND s.commander_id = cs.commander_id
WHERE cs.position = (SELECT MIN(position) FROM commander_secretaries WHERE commander_id = cs.commander_id);

