-- The back-office's commander activities, shops and archives (internal/admin
-- area_life.go, internal/game/admin_life.go).

-- Activities: every per-activity table, listed and cleared by activity.
-- name: LifeListCommanderActivities :many
SELECT * FROM commander_activities WHERE commander_id = ? ORDER BY activity_id;

-- name: LifeDeleteCommanderActivities :exec
DELETE FROM commander_activities WHERE commander_id = sqlc.arg(commander_id) AND activity_id = sqlc.arg(activity_id);

-- name: LifeListCommanderActivityKeyValues :many
SELECT * FROM commander_activity_key_values WHERE commander_id = ? ORDER BY activity_id, list_key, item_key;

-- name: LifeDeleteCommanderActivityKeyValues :exec
DELETE FROM commander_activity_key_values WHERE commander_id = sqlc.arg(commander_id) AND activity_id = sqlc.arg(activity_id);

-- name: LifeListCommanderActivityTasks :many
SELECT * FROM commander_activity_tasks WHERE commander_id = ? ORDER BY act_id, task_id;

-- name: LifeDeleteCommanderActivityTasks :exec
DELETE FROM commander_activity_tasks WHERE commander_id = sqlc.arg(commander_id) AND act_id = sqlc.arg(activity_id);

-- name: LifeListActivityFleetShips :many
SELECT * FROM activity_fleet_ships WHERE commander_id = ? ORDER BY activity_id, fleet_id, pos;

-- name: LifeDeleteActivityFleetShips :exec
DELETE FROM activity_fleet_ships WHERE commander_id = sqlc.arg(commander_id) AND activity_id = sqlc.arg(activity_id);

-- name: LifeListActivityFleetMeowfficers :many
SELECT * FROM activity_fleet_meowfficers WHERE commander_id = ? ORDER BY activity_id, fleet_id, pos;

-- name: LifeDeleteActivityFleetMeowfficers :exec
DELETE FROM activity_fleet_meowfficers WHERE commander_id = sqlc.arg(commander_id) AND activity_id = sqlc.arg(activity_id);

-- name: LifeListChallengeRuns :many
SELECT * FROM challenge_runs WHERE commander_id = ? ORDER BY activity_id, mode;

-- name: LifeDeleteChallengeRuns :exec
DELETE FROM challenge_runs WHERE commander_id = sqlc.arg(commander_id) AND activity_id = sqlc.arg(activity_id);

-- name: LifeListChallengeShips :many
SELECT * FROM challenge_ships WHERE commander_id = ? ORDER BY activity_id, mode, fleet_id, pos;

-- name: LifeDeleteChallengeShips :exec
DELETE FROM challenge_ships WHERE commander_id = sqlc.arg(commander_id) AND activity_id = sqlc.arg(activity_id);

-- name: LifeListChallengeMeowfficers :many
SELECT * FROM challenge_meowfficers WHERE commander_id = ? ORDER BY activity_id, mode, fleet_id, pos;

-- name: LifeDeleteChallengeMeowfficers :exec
DELETE FROM challenge_meowfficers WHERE commander_id = sqlc.arg(commander_id) AND activity_id = sqlc.arg(activity_id);

-- name: LifeListChallengeRecords :many
SELECT * FROM challenge_records WHERE commander_id = ? ORDER BY activity_id, season;

-- name: LifeDeleteChallengeRecords :exec
DELETE FROM challenge_records WHERE commander_id = sqlc.arg(commander_id) AND activity_id = sqlc.arg(activity_id);

-- name: LifeListCommanderAuction :many
SELECT * FROM commander_auction WHERE commander_id = ? ORDER BY activity_id;

-- name: LifeDeleteCommanderAuction :exec
DELETE FROM commander_auction WHERE commander_id = sqlc.arg(commander_id) AND activity_id = sqlc.arg(activity_id);

-- name: LifeListCommanderCityRebuild :many
SELECT * FROM commander_city_rebuild WHERE commander_id = ? ORDER BY activity_id;

-- name: LifeDeleteCommanderCityRebuild :exec
DELETE FROM commander_city_rebuild WHERE commander_id = sqlc.arg(commander_id) AND activity_id = sqlc.arg(activity_id);

-- name: LifeListCommanderCityRebuildUnits :many
SELECT * FROM commander_city_rebuild_units WHERE commander_id = ? ORDER BY activity_id, unit_id;

-- name: LifeDeleteCommanderCityRebuildUnits :exec
DELETE FROM commander_city_rebuild_units WHERE commander_id = sqlc.arg(commander_id) AND activity_id = sqlc.arg(activity_id);

-- name: LifeListCommanderCityRebuildBuffs :many
SELECT * FROM commander_city_rebuild_buffs WHERE commander_id = ? ORDER BY activity_id, buff_group;

-- name: LifeDeleteCommanderCityRebuildBuffs :exec
DELETE FROM commander_city_rebuild_buffs WHERE commander_id = sqlc.arg(commander_id) AND activity_id = sqlc.arg(activity_id);

-- name: LifeListCommanderColoringCells :many
SELECT * FROM commander_coloring_cells WHERE commander_id = ? ORDER BY activity_id, group_id, row, col;

-- name: LifeDeleteCommanderColoringCells :exec
DELETE FROM commander_coloring_cells WHERE commander_id = sqlc.arg(commander_id) AND activity_id = sqlc.arg(activity_id);

-- name: LifeListCommanderColoringAwards :many
SELECT * FROM commander_coloring_awards WHERE commander_id = ? ORDER BY activity_id, group_id;

-- name: LifeDeleteCommanderColoringAwards :exec
DELETE FROM commander_coloring_awards WHERE commander_id = sqlc.arg(commander_id) AND activity_id = sqlc.arg(activity_id);

-- name: LifeListCommanderFeasts :many
SELECT * FROM commander_feasts WHERE commander_id = ? ORDER BY activity_id;

-- name: LifeDeleteCommanderFeasts :exec
DELETE FROM commander_feasts WHERE commander_id = sqlc.arg(commander_id) AND activity_id = sqlc.arg(activity_id);

-- name: LifeListCommanderFeastShips :many
SELECT * FROM commander_feast_ships WHERE commander_id = ? ORDER BY activity_id, group_id;

-- name: LifeDeleteCommanderFeastShips :exec
DELETE FROM commander_feast_ships WHERE commander_id = sqlc.arg(commander_id) AND activity_id = sqlc.arg(activity_id);

-- name: LifeListCommanderFeastInvitations :many
SELECT * FROM commander_feast_invitations WHERE commander_id = ? ORDER BY activity_id, group_id;

-- name: LifeDeleteCommanderFeastInvitations :exec
DELETE FROM commander_feast_invitations WHERE commander_id = sqlc.arg(commander_id) AND activity_id = sqlc.arg(activity_id);

-- name: LifeListCommanderIslandNodes :many
SELECT * FROM commander_island_nodes WHERE commander_id = ? ORDER BY activity_id, node_id;

-- name: LifeDeleteCommanderIslandNodes :exec
DELETE FROM commander_island_nodes WHERE commander_id = sqlc.arg(commander_id) AND activity_id = sqlc.arg(activity_id);

-- name: LifeListCommanderPermanentActivities :many
SELECT * FROM commander_permanent_activities WHERE commander_id = ? ORDER BY activity_id;

-- name: LifeDeleteCommanderPermanentActivities :exec
DELETE FROM commander_permanent_activities WHERE commander_id = sqlc.arg(commander_id) AND activity_id = sqlc.arg(activity_id);

-- name: LifeListRemasterAwards :many
SELECT * FROM remaster_awards WHERE commander_id = ? ORDER BY act_id, chapter_id, pos;

-- name: LifeDeleteRemasterAwards :exec
DELETE FROM remaster_awards WHERE commander_id = sqlc.arg(commander_id) AND act_id = sqlc.arg(activity_id);

-- name: LifeListNewServerShopGoods :many
SELECT * FROM new_server_shop_goods WHERE commander_id = ? ORDER BY activity_id, goods_id;

-- name: LifeDeleteNewServerShopGoods :exec
DELETE FROM new_server_shop_goods WHERE commander_id = sqlc.arg(commander_id) AND activity_id = sqlc.arg(activity_id);

-- name: LifeListNewServerShopSubGoods :many
SELECT * FROM new_server_shop_sub_goods WHERE commander_id = ? ORDER BY activity_id, goods_id, item_id;

-- name: LifeDeleteNewServerShopSubGoods :exec
DELETE FROM new_server_shop_sub_goods WHERE commander_id = sqlc.arg(commander_id) AND activity_id = sqlc.arg(activity_id);

-- Activities: the systems kept apart from any activity id, listed and
-- cleared whole.
-- name: LifeListCommanderAtelierItems :many
SELECT * FROM commander_atelier_items WHERE commander_id = ?;

-- name: LifeDeleteCommanderAtelierItems :exec
DELETE FROM commander_atelier_items WHERE commander_id = ?;

-- name: LifeListCommanderAtelierRecipes :many
SELECT * FROM commander_atelier_recipes WHERE commander_id = ?;

-- name: LifeDeleteCommanderAtelierRecipes :exec
DELETE FROM commander_atelier_recipes WHERE commander_id = ?;

-- name: LifeListCommanderAtelierSlots :many
SELECT * FROM commander_atelier_slots WHERE commander_id = ?;

-- name: LifeDeleteCommanderAtelierSlots :exec
DELETE FROM commander_atelier_slots WHERE commander_id = ?;

-- name: LifeListCommanderJuusPosts :many
SELECT * FROM commander_juus_posts WHERE commander_id = ?;

-- name: LifeDeleteCommanderJuusPosts :exec
DELETE FROM commander_juus_posts WHERE commander_id = ?;

-- name: LifeListCommanderJuusTopics :many
SELECT * FROM commander_juus_topics WHERE commander_id = ?;

-- name: LifeDeleteCommanderJuusTopics :exec
DELETE FROM commander_juus_topics WHERE commander_id = ?;

-- name: LifeListCommanderJuusReplies :many
SELECT * FROM commander_juus_replies WHERE commander_id = ?;

-- name: LifeDeleteCommanderJuusReplies :exec
DELETE FROM commander_juus_replies WHERE commander_id = ?;

-- name: LifeListCommanderJuusGroups :many
SELECT * FROM commander_juus_groups WHERE commander_id = ?;

-- name: LifeDeleteCommanderJuusGroups :exec
DELETE FROM commander_juus_groups WHERE commander_id = ?;

-- name: LifeListCommanderApartment :many
SELECT * FROM commander_apartment WHERE commander_id = ?;

-- name: LifeDeleteCommanderApartment :exec
DELETE FROM commander_apartment WHERE commander_id = ?;

-- name: LifeListApartmentShips :many
SELECT * FROM apartment_ships WHERE commander_id = ?;

-- name: LifeDeleteApartmentShips :exec
DELETE FROM apartment_ships WHERE commander_id = ?;

-- name: LifeListApartmentHiddenParts :many
SELECT * FROM apartment_hidden_parts WHERE commander_id = ?;

-- name: LifeDeleteApartmentHiddenParts :exec
DELETE FROM apartment_hidden_parts WHERE commander_id = ?;

-- name: LifeListApartmentRooms :many
SELECT * FROM apartment_rooms WHERE commander_id = ?;

-- name: LifeDeleteApartmentRooms :exec
DELETE FROM apartment_rooms WHERE commander_id = ?;

-- name: LifeListApartmentFurniture :many
SELECT * FROM apartment_furniture WHERE commander_id = ?;

-- name: LifeDeleteApartmentFurniture :exec
DELETE FROM apartment_furniture WHERE commander_id = ?;

-- name: LifeListApartmentGifts :many
SELECT * FROM apartment_gifts WHERE commander_id = ?;

-- name: LifeDeleteApartmentGifts :exec
DELETE FROM apartment_gifts WHERE commander_id = ?;

-- name: LifeListCommanderApartmentChats :many
SELECT * FROM commander_apartment_chats WHERE commander_id = ?;

-- name: LifeDeleteCommanderApartmentChats :exec
DELETE FROM commander_apartment_chats WHERE commander_id = ?;

-- name: LifeListCommanderApartmentEvents :many
SELECT * FROM commander_apartment_events WHERE commander_id = ?;

-- name: LifeDeleteCommanderApartmentEvents :exec
DELETE FROM commander_apartment_events WHERE commander_id = ?;

-- name: LifeListCommanderApartmentInsContents :many
SELECT * FROM commander_apartment_ins_contents WHERE commander_id = ?;

-- name: LifeDeleteCommanderApartmentInsContents :exec
DELETE FROM commander_apartment_ins_contents WHERE commander_id = ?;

-- name: LifeListCommanderApartmentInsReplies :many
SELECT * FROM commander_apartment_ins_replies WHERE commander_id = ?;

-- name: LifeDeleteCommanderApartmentInsReplies :exec
DELETE FROM commander_apartment_ins_replies WHERE commander_id = ?;

-- name: LifeListCommanderLoveLetterMedals :many
SELECT * FROM commander_love_letter_medals WHERE commander_id = ?;

-- name: LifeDeleteCommanderLoveLetterMedals :exec
DELETE FROM commander_love_letter_medals WHERE commander_id = ?;

-- name: LifeListCommanderLoveLetters :many
SELECT * FROM commander_love_letters WHERE commander_id = ?;

-- name: LifeDeleteCommanderLoveLetters :exec
DELETE FROM commander_love_letters WHERE commander_id = ?;

-- name: LifeListCommanderLoveLetterGifts :many
SELECT * FROM commander_love_letter_gifts WHERE commander_id = ?;

-- name: LifeDeleteCommanderLoveLetterGifts :exec
DELETE FROM commander_love_letter_gifts WHERE commander_id = ?;

-- name: LifeListCommanderLoveLetterRewards :many
SELECT * FROM commander_love_letter_rewards WHERE commander_id = ?;

-- name: LifeDeleteCommanderLoveLetterRewards :exec
DELETE FROM commander_love_letter_rewards WHERE commander_id = ?;

-- name: LifeListCommanderEducate :many
SELECT * FROM commander_educate WHERE commander_id = ?;

-- name: LifeDeleteCommanderEducate :exec
DELETE FROM commander_educate WHERE commander_id = ?;

-- name: LifeListCommanderReflux :many
SELECT * FROM commander_reflux WHERE commander_id = ?;

-- name: LifeDeleteCommanderReflux :exec
DELETE FROM commander_reflux WHERE commander_id = ?;

-- name: LifeListCommanderRemasters :many
SELECT * FROM commander_remasters WHERE commander_id = ?;

-- name: LifeDeleteCommanderRemasters :exec
DELETE FROM commander_remasters WHERE commander_id = ?;

-- name: LifeListRemasterStates :many
SELECT * FROM remaster_states WHERE commander_id = ?;

-- name: LifeDeleteRemasterStates :exec
DELETE FROM remaster_states WHERE commander_id = ?;

-- name: LifeListCommanderMinigameHubs :many
SELECT * FROM commander_minigame_hubs WHERE commander_id = ?;

-- name: LifeDeleteCommanderMinigameHubs :exec
DELETE FROM commander_minigame_hubs WHERE commander_id = ?;

-- name: LifeListCommanderMinigameData :many
SELECT * FROM commander_minigame_data WHERE commander_id = ?;

-- name: LifeDeleteCommanderMinigameData :exec
DELETE FROM commander_minigame_data WHERE commander_id = ?;

-- name: LifeListCommanderMinigameScores :many
SELECT * FROM commander_minigame_scores WHERE commander_id = ?;

-- name: LifeDeleteCommanderMinigameScores :exec
DELETE FROM commander_minigame_scores WHERE commander_id = ?;

-- name: LifeListCommanderMinigameValues :many
SELECT * FROM commander_minigame_values WHERE commander_id = ?;

-- name: LifeDeleteCommanderMinigameValues :exec
DELETE FROM commander_minigame_values WHERE commander_id = ?;

-- name: LifeListCommanderGameRoom :many
SELECT * FROM commander_game_room WHERE commander_id = ?;

-- name: LifeDeleteCommanderGameRoom :exec
DELETE FROM commander_game_room WHERE commander_id = ?;

-- name: LifeListCommanderGameRoomScores :many
SELECT * FROM commander_game_room_scores WHERE commander_id = ?;

-- name: LifeDeleteCommanderGameRoomScores :exec
DELETE FROM commander_game_room_scores WHERE commander_id = ?;

-- name: LifeListCommanderPlayRoomViews :many
SELECT * FROM commander_play_room_views WHERE commander_id = ?;

-- name: LifeDeleteCommanderPlayRoomViews :exec
DELETE FROM commander_play_room_views WHERE commander_id = ?;

-- name: LifeListCommanderPlayRoomScores :many
SELECT * FROM commander_play_room_scores WHERE commander_id = ?;

-- name: LifeDeleteCommanderPlayRoomScores :exec
DELETE FROM commander_play_room_scores WHERE commander_id = ?;

-- name: LifeListCommanderIslands :many
SELECT * FROM commander_islands WHERE commander_id = ?;

-- name: LifeDeleteCommanderIslands :exec
DELETE FROM commander_islands WHERE commander_id = ?;

-- name: LifeListCommanderNewEducate :many
SELECT * FROM commander_new_educate WHERE commander_id = ? ORDER BY character_id;

-- name: LifeDeleteCommanderNewEducate :exec
DELETE FROM commander_new_educate WHERE commander_id = ? AND character_id = ?;

-- Shops: what was bought, and resetting it.

-- name: LifeListShopPurchases :many
SELECT * FROM commander_shop_purchases WHERE commander_id = ? ORDER BY shop_id, expires_at;

-- name: LifeDeleteShopPurchases :exec
DELETE FROM commander_shop_purchases WHERE commander_id = ? AND shop_id = ?;

-- name: LifeListShopGroupPurchases :many
SELECT * FROM commander_shop_group_purchases WHERE commander_id = ? ORDER BY group_id, expires_at;

-- name: LifeDeleteShopGroupPurchases :exec
DELETE FROM commander_shop_group_purchases WHERE commander_id = ? AND group_id = ?;

-- name: LifeDeleteMedalShopPurchase :exec
DELETE FROM commander_medal_shop_purchases WHERE commander_id = ? AND goods_id = ? AND flash_time = ?;

-- name: LifeClearMedalShopPurchases :exec
DELETE FROM commander_medal_shop_purchases WHERE commander_id = ? AND flash_time = ?;

-- name: LifeDeleteMonthShopPurchase :exec
DELETE FROM commander_month_shop_purchases WHERE commander_id = ? AND goods_id = ? AND month = ?;

-- name: LifeDeleteGuildShopPurchase :exec
DELETE FROM commander_guild_shop_purchases WHERE commander_id = ? AND slot = ?;

-- name: LifeDeleteMiniGameShopPurchases :exec
DELETE FROM commander_mini_game_shop_purchases WHERE commander_id = ? AND goods_id = ?;

-- name: LifeDeleteNewServerShopGoodsItem :exec
DELETE FROM new_server_shop_goods WHERE commander_id = ? AND activity_id = ? AND goods_id = ?;

-- name: LifeDeleteNewServerShopSubGoodsItem :exec
DELETE FROM new_server_shop_sub_goods WHERE commander_id = ? AND activity_id = ? AND goods_id = ?;

-- name: LifeDeleteRequisitions :exec
DELETE FROM commander_requisitions WHERE commander_id = ? AND month = ?;

-- name: LifeDeleteChargeExp :exec
DELETE FROM charge_exp WHERE commander_id = ?;

-- Archives

-- name: LifeDeleteTrophyClaim :exec
DELETE FROM commander_trophy_claims WHERE commander_id = ? AND trophy_id = ?;

-- name: LifeDeleteDisplayMedal :exec
DELETE FROM commander_display_medals WHERE commander_id = ? AND medal_id = ?;

-- name: LifeCountStories :one
SELECT COUNT(*) FROM commander_stories WHERE commander_id = ?;

-- name: LifeDeleteStories :exec
DELETE FROM commander_stories WHERE commander_id = ?;

-- name: LifeDeleteCommonFlags :exec
DELETE FROM commander_common_flags WHERE commander_id = ?;

-- name: LifeDeleteAppreciation :exec
DELETE FROM commander_appreciation WHERE commander_id = ? AND kind = ?;

-- name: LifeDeleteMusicPlayer :exec
DELETE FROM commander_music_player WHERE commander_id = ?;

-- name: LifeGetRandomFlagship :one
SELECT * FROM commander_random_flagship WHERE commander_id = ?;
