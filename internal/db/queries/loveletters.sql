-- name: ListLoveLetterMedals :many
SELECT group_id, exp, level FROM commander_love_letter_medals WHERE commander_id = ? ORDER BY group_id;

-- name: SetLoveLetterMedal :exec
INSERT INTO commander_love_letter_medals (commander_id, group_id, exp, level) VALUES (?, ?, ?, ?)
ON CONFLICT (commander_id, group_id) DO UPDATE SET exp = excluded.exp, level = excluded.level;

-- name: ListLoveLetters :many
SELECT letter_id FROM commander_love_letters WHERE commander_id = ? ORDER BY letter_id;

-- name: UnlockLoveLetter :exec
INSERT INTO commander_love_letters (commander_id, letter_id) VALUES (?, ?) ON CONFLICT DO NOTHING;

-- name: ListLoveLetterGifts :many
SELECT item_id, group_id, year FROM commander_love_letter_gifts WHERE commander_id = ? ORDER BY pos;

-- name: ClearLoveLetterGifts :exec
DELETE FROM commander_love_letter_gifts WHERE commander_id = ?;

-- name: AddLoveLetterGift :exec
INSERT INTO commander_love_letter_gifts (commander_id, pos, item_id, group_id, year) VALUES (?, ?, ?, ?, ?);

-- name: ListLoveLetterRewards :many
SELECT reward_id FROM commander_love_letter_rewards WHERE commander_id = ? ORDER BY reward_id;

-- name: AddLoveLetterReward :execrows
INSERT INTO commander_love_letter_rewards (commander_id, reward_id) VALUES (?, ?) ON CONFLICT DO NOTHING;
