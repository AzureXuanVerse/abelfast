package game

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"sync"

	"github.com/ggmolly/belfast/internal/db"
	"github.com/ggmolly/belfast/internal/gamedata"
	"github.com/ggmolly/belfast/internal/protobuf"
)

// EditCommanderRows changes a commander's rows with fn, as AdminWrite does.
func (game *Game) EditCommanderRows(ctx context.Context, id uint32, fn func(*db.Queries) error) (bool, error) {
	return game.AdminWrite(ctx, id, func(tx *Tx) error { return fn(tx.Queries) })
}

// RenameCommander sets a commander's name as CS_11007 would, without its
// cost and cooldown: a name the client would send (validName) that no other
// commander has.
func (game *Game) RenameCommander(ctx context.Context, id uint32, name string) (bool, error) {
	if !validName(name) {
		return false, adminRefused("a name is 1 to %d characters without spaces", maxNameLength)
	}
	return game.AdminWrite(ctx, id, func(tx *Tx) error {
		taken, err := tx.CommanderNameTaken(ctx, db.CommanderNameTakenParams{Name: name, ID: id})
		if err != nil {
			return err
		}
		if taken {
			return AdminError{Msg: "another commander has that name"}
		}
		return tx.SetCommanderName(ctx, db.SetCommanderNameParams{Name: name, ID: id})
	})
}

// CommanderLevels is the last commander level (user_level) and the exp each
// level takes to the next.
func CommanderLevels() (uint32, map[uint32]uint32) {
	exp := make(map[uint32]uint32, len(commanderLevels))
	for level, config := range commanderLevels {
		exp[level] = config.Exp
	}
	return commanderMaxLevel, exp
}

// MaxStoredMails is how many mails a commander of level can keep in the
// storage (user_level max_markmail, MailScene's collection count).
func MaxStoredMails(level uint32) uint32 {
	rows, err := mailStorage()
	if err != nil {
		return 0
	}
	return rows[level].MaxMarkmail
}

var mailStorage = sync.OnceValues(func() (map[uint32]struct {
	MaxMarkmail uint32 `json:"max_markmail"`
}, error) {
	return gamedata.Table[struct {
		MaxMarkmail uint32 `json:"max_markmail"`
	}]("user_level")
})

// SetCommanderLevel puts a commander at level (1 to the last user_level
// row) with no exp into it. A level up counts for the level tasks and opens
// the research academy as addCommanderExp does.
func (game *Game) SetCommanderLevel(ctx context.Context, id, level uint32) (bool, error) {
	if level < 1 || level > commanderMaxLevel {
		return false, adminRefused("a level is 1 to %d", commanderMaxLevel)
	}
	return game.AdminWrite(ctx, id, func(tx *Tx) error {
		commander, err := tx.GetCommander(ctx, id)
		if err != nil {
			return err
		}
		if err := tx.SetCommanderExp(ctx, db.SetCommanderExpParams{Level: level, Exp: 0, ID: id}); err != nil {
			return err
		}
		if level <= commander.Level {
			return nil
		}
		tx.event(TaskEvent{SubType: TaskSubLevel, Count: level})
		return tx.researchUnlocked(commander.Level, level)
	})
}

// ResetSecondaryPassword removes a commander's secondary password (no row
// is no password, SC_11604 state 0 at its next login): it logs in without
// one, as before it set one.
func (game *Game) ResetSecondaryPassword(ctx context.Context, id uint32) (bool, error) {
	kicked, err := game.AdminWrite(ctx, id, func(tx *Tx) error {
		return tx.DeleteSecondaryPassword(ctx, id)
	})
	if err == nil {
		game.setSecondaryUnlocked(id, false)
	}
	return kicked, err
}

// GrantCosmetics stores icon frames, chat frames and augment modules for a
// commander, which Grant refuses (mailStorable), with the other drops Grant
// takes.
func (game *Game) GrantCosmetics(ctx context.Context, id uint32, drops []*protobuf.DROPINFO) (bool, error) {
	for _, drop := range drops {
		kind, item, number := drop.GetType(), drop.GetId(), drop.GetNumber()
		var ok bool
		switch kind {
		case dropTypeIconFrame:
			_, ok = iconFrames[item]
		case dropTypeChatFrame:
			_, ok = chatFrames[item]
		case dropTypeSpWeapon:
			_, ok = spweapons[item]
		default:
			ok = mailStorable(kind, item)
		}
		if number == 0 || !ok {
			return false, adminRefused("drop %d:%d:%d can't be stored", kind, item, number)
		}
	}
	return game.AdminWrite(ctx, id, func(tx *Tx) error {
		if err := tx.grantDrops(drops); err != nil {
			return err
		}
		return tx.recordTechPoints()
	})
}

// SpWeaponLevel is one level of an augment module, as the back-office edits
// it: its spweapon_data_statistics id and the most a roll adds to each of
// its two values.
type SpWeaponLevel struct {
	TemplateID uint32 `json:"template_id"`
	Level      int    `json:"level"`
	Max1       uint32 `json:"max_1"`
	Max2       uint32 `json:"max_2"`
}

// SpWeaponLevels lists the levels of the augment module template is one of
// (its prev / next chain), first to last.
func SpWeaponLevels(template uint32) []SpWeaponLevel {
	first, seen := template, map[uint32]bool{}
	for spweapons[first].Prev != 0 && !seen[first] {
		seen[first] = true
		first = spweapons[first].Prev
	}
	var levels []SpWeaponLevel
	for id := first; id != 0 && len(levels) < 100; id = spweapons[id].Next {
		config, ok := spweapons[id]
		if !ok {
			break
		}
		levels = append(levels, SpWeaponLevel{TemplateID: id, Level: len(levels), Max1: config.Random1, Max2: config.Random2})
	}
	return levels
}

// SetSpWeapon sets an augment module's level (a template of its chain) and
// rolled values (each at most what its level's roll adds,
// GetAttributesRange). A new level starts with no upgrade points; a reforge
// waiting that no longer fits is dropped.
func (game *Game) SetSpWeapon(ctx context.Context, id, spweaponID, template, attr1, attr2 uint32) (bool, error) {
	return game.AdminWrite(ctx, id, func(tx *Tx) error {
		row, err := tx.GetSpWeapon(ctx, db.GetSpWeaponParams{CommanderID: id, ID: spweaponID})
		if errors.Is(err, sql.ErrNoRows) {
			return adminRefused("no augment module %d", spweaponID)
		}
		if err != nil {
			return err
		}
		var level *SpWeaponLevel
		for _, candidate := range SpWeaponLevels(row.TemplateID) {
			if candidate.TemplateID == template {
				level = &candidate
			}
		}
		if level == nil {
			return adminRefused("%d isn't a level of augment module %d", template, row.TemplateID)
		}
		if attr1 > level.Max1 || attr2 > level.Max2 {
			return adminRefused("rolls are at most %d and %d at this level", level.Max1, level.Max2)
		}
		pt := row.Pt
		if template != row.TemplateID {
			pt = 0
		}
		temp1, temp2 := row.AttrTemp1, row.AttrTemp2
		if temp1 > level.Max1 || temp2 > level.Max2 {
			temp1, temp2 = 0, 0
		}
		return tx.UpdateSpWeapon(ctx, db.UpdateSpWeaponParams{
			TemplateID: template, Attr1: attr1, Attr2: attr2, AttrTemp1: temp1, AttrTemp2: temp2, Pt: pt,
			CommanderID: id, ID: spweaponID,
		})
	})
}

// commanderTables are the tables holding a commander's rows, generated from
// the schema with their queries (queries/admin_players.sql): its own rows,
// the rows of what it owns (ships, augments, meowfficers, gear codes,
// discussions) and other commanders' rows naming it (friends, blacklists,
// requests, guild boss fleets lent its ships). The guild logs keep the names
// they logged. The rows pointing at others come first, the commander last.
var commanderTables = []struct {
	name   string
	export func(*db.Queries, context.Context, uint32) (string, error)
	delete func(*db.Queries, context.Context, uint32) error
}{
	{"activity_fleet_meowfficers", (*db.Queries).AdminExportActivityFleetMeowfficers, (*db.Queries).AdminDeleteActivityFleetMeowfficers},
	{"activity_fleet_ships", (*db.Queries).AdminExportActivityFleetShips, (*db.Queries).AdminDeleteActivityFleetShips},
	{"apartment_furniture", (*db.Queries).AdminExportApartmentFurniture, (*db.Queries).AdminDeleteApartmentFurniture},
	{"apartment_gifts", (*db.Queries).AdminExportApartmentGifts, (*db.Queries).AdminDeleteApartmentGifts},
	{"apartment_hidden_parts", (*db.Queries).AdminExportApartmentHiddenParts, (*db.Queries).AdminDeleteApartmentHiddenParts},
	{"apartment_rooms", (*db.Queries).AdminExportApartmentRooms, (*db.Queries).AdminDeleteApartmentRooms},
	{"apartment_ships", (*db.Queries).AdminExportApartmentShips, (*db.Queries).AdminDeleteApartmentShips},
	{"attire_frames", (*db.Queries).AdminExportAttireFrames, (*db.Queries).AdminDeleteAttireFrames},
	{"blacklist", (*db.Queries).AdminExportBlacklist, (*db.Queries).AdminDeleteBlacklist},
	{"builds", (*db.Queries).AdminExportBuilds, (*db.Queries).AdminDeleteBuilds},
	{"challenge_meowfficers", (*db.Queries).AdminExportChallengeMeowfficers, (*db.Queries).AdminDeleteChallengeMeowfficers},
	{"challenge_records", (*db.Queries).AdminExportChallengeRecords, (*db.Queries).AdminDeleteChallengeRecords},
	{"challenge_ships", (*db.Queries).AdminExportChallengeShips, (*db.Queries).AdminDeleteChallengeShips},
	{"challenge_runs", (*db.Queries).AdminExportChallengeRuns, (*db.Queries).AdminDeleteChallengeRuns},
	{"chapter_auto_battles", (*db.Queries).AdminExportChapterAutoBattles, (*db.Queries).AdminDeleteChapterAutoBattles},
	{"chapter_auto_records", (*db.Queries).AdminExportChapterAutoRecords, (*db.Queries).AdminDeleteChapterAutoRecords},
	{"chapter_auto_states", (*db.Queries).AdminExportChapterAutoStates, (*db.Queries).AdminDeleteChapterAutoStates},
	{"chapter_auto_tickets", (*db.Queries).AdminExportChapterAutoTickets, (*db.Queries).AdminDeleteChapterAutoTickets},
	{"chapter_drop_ships", (*db.Queries).AdminExportChapterDropShips, (*db.Queries).AdminDeleteChapterDropShips},
	{"chapter_elite_fleets", (*db.Queries).AdminExportChapterEliteFleets, (*db.Queries).AdminDeleteChapterEliteFleets},
	{"chapter_progress", (*db.Queries).AdminExportChapterProgress, (*db.Queries).AdminDeleteChapterProgress},
	{"chapter_states", (*db.Queries).AdminExportChapterStates, (*db.Queries).AdminDeleteChapterStates},
	{"charge_exp", (*db.Queries).AdminExportChargeExp, (*db.Queries).AdminDeleteChargeExp},
	{"cheater_marks", (*db.Queries).AdminExportCheaterMarks, (*db.Queries).AdminDeleteCheaterMarks},
	{"commander_activities", (*db.Queries).AdminExportCommanderActivities, (*db.Queries).AdminDeleteCommanderActivities},
	{"commander_activity_key_values", (*db.Queries).AdminExportCommanderActivityKeyValues, (*db.Queries).AdminDeleteCommanderActivityKeyValues},
	{"commander_activity_tasks", (*db.Queries).AdminExportCommanderActivityTasks, (*db.Queries).AdminDeleteCommanderActivityTasks},
	{"commander_apartment", (*db.Queries).AdminExportCommanderApartment, (*db.Queries).AdminDeleteCommanderApartment},
	{"commander_apartment_chats", (*db.Queries).AdminExportCommanderApartmentChats, (*db.Queries).AdminDeleteCommanderApartmentChats},
	{"commander_apartment_events", (*db.Queries).AdminExportCommanderApartmentEvents, (*db.Queries).AdminDeleteCommanderApartmentEvents},
	{"commander_apartment_ins_contents", (*db.Queries).AdminExportCommanderApartmentInsContents, (*db.Queries).AdminDeleteCommanderApartmentInsContents},
	{"commander_apartment_ins_replies", (*db.Queries).AdminExportCommanderApartmentInsReplies, (*db.Queries).AdminDeleteCommanderApartmentInsReplies},
	{"commander_appearance", (*db.Queries).AdminExportCommanderAppearance, (*db.Queries).AdminDeleteCommanderAppearance},
	{"commander_appreciation", (*db.Queries).AdminExportCommanderAppreciation, (*db.Queries).AdminDeleteCommanderAppreciation},
	{"commander_atelier_items", (*db.Queries).AdminExportCommanderAtelierItems, (*db.Queries).AdminDeleteCommanderAtelierItems},
	{"commander_atelier_recipes", (*db.Queries).AdminExportCommanderAtelierRecipes, (*db.Queries).AdminDeleteCommanderAtelierRecipes},
	{"commander_atelier_slots", (*db.Queries).AdminExportCommanderAtelierSlots, (*db.Queries).AdminDeleteCommanderAtelierSlots},
	{"commander_auction", (*db.Queries).AdminExportCommanderAuction, (*db.Queries).AdminDeleteCommanderAuction},
	{"commander_blueprint_state", (*db.Queries).AdminExportCommanderBlueprintState, (*db.Queries).AdminDeleteCommanderBlueprintState},
	{"commander_blueprints", (*db.Queries).AdminExportCommanderBlueprints, (*db.Queries).AdminDeleteCommanderBlueprints},
	{"commander_build_states", (*db.Queries).AdminExportCommanderBuildStates, (*db.Queries).AdminDeleteCommanderBuildStates},
	{"commander_city_rebuild", (*db.Queries).AdminExportCommanderCityRebuild, (*db.Queries).AdminDeleteCommanderCityRebuild},
	{"commander_city_rebuild_buffs", (*db.Queries).AdminExportCommanderCityRebuildBuffs, (*db.Queries).AdminDeleteCommanderCityRebuildBuffs},
	{"commander_city_rebuild_units", (*db.Queries).AdminExportCommanderCityRebuildUnits, (*db.Queries).AdminDeleteCommanderCityRebuildUnits},
	{"commander_collection_awards", (*db.Queries).AdminExportCommanderCollectionAwards, (*db.Queries).AdminDeleteCommanderCollectionAwards},
	{"commander_coloring_awards", (*db.Queries).AdminExportCommanderColoringAwards, (*db.Queries).AdminDeleteCommanderColoringAwards},
	{"commander_coloring_cells", (*db.Queries).AdminExportCommanderColoringCells, (*db.Queries).AdminDeleteCommanderColoringCells},
	{"commander_commission_refresh", (*db.Queries).AdminExportCommanderCommissionRefresh, (*db.Queries).AdminDeleteCommanderCommissionRefresh},
	{"commander_commission_ships", (*db.Queries).AdminExportCommanderCommissionShips, (*db.Queries).AdminDeleteCommanderCommissionShips},
	{"commander_commissions", (*db.Queries).AdminExportCommanderCommissions, (*db.Queries).AdminDeleteCommanderCommissions},
	{"commander_common_flags", (*db.Queries).AdminExportCommanderCommonFlags, (*db.Queries).AdminDeleteCommanderCommonFlags},
	{"commander_cooldowns", (*db.Queries).AdminExportCommanderCooldowns, (*db.Queries).AdminDeleteCommanderCooldowns},
	{"commander_cryptolalia", (*db.Queries).AdminExportCommanderCryptolalia, (*db.Queries).AdminDeleteCommanderCryptolalia},
	{"commander_display_medals", (*db.Queries).AdminExportCommanderDisplayMedals, (*db.Queries).AdminDeleteCommanderDisplayMedals},
	{"commander_dorms", (*db.Queries).AdminExportCommanderDorms, (*db.Queries).AdminDeleteCommanderDorms},
	{"commander_educate", (*db.Queries).AdminExportCommanderEducate, (*db.Queries).AdminDeleteCommanderEducate},
	{"commander_equip_skins", (*db.Queries).AdminExportCommanderEquipSkins, (*db.Queries).AdminDeleteCommanderEquipSkins},
	{"commander_equipments", (*db.Queries).AdminExportCommanderEquipments, (*db.Queries).AdminDeleteCommanderEquipments},
	{"commander_exercise", (*db.Queries).AdminExportCommanderExercise, (*db.Queries).AdminDeleteCommanderExercise},
	{"commander_feast_invitations", (*db.Queries).AdminExportCommanderFeastInvitations, (*db.Queries).AdminDeleteCommanderFeastInvitations},
	{"commander_feast_ships", (*db.Queries).AdminExportCommanderFeastShips, (*db.Queries).AdminDeleteCommanderFeastShips},
	{"commander_feasts", (*db.Queries).AdminExportCommanderFeasts, (*db.Queries).AdminDeleteCommanderFeasts},
	{"commander_furniture", (*db.Queries).AdminExportCommanderFurniture, (*db.Queries).AdminDeleteCommanderFurniture},
	{"commander_game_room", (*db.Queries).AdminExportCommanderGameRoom, (*db.Queries).AdminDeleteCommanderGameRoom},
	{"commander_game_room_scores", (*db.Queries).AdminExportCommanderGameRoomScores, (*db.Queries).AdminDeleteCommanderGameRoomScores},
	{"commander_guild_shop_purchases", (*db.Queries).AdminExportCommanderGuildShopPurchases, (*db.Queries).AdminDeleteCommanderGuildShopPurchases},
	{"commander_guild_shops", (*db.Queries).AdminExportCommanderGuildShops, (*db.Queries).AdminDeleteCommanderGuildShops},
	{"commander_guild_technologies", (*db.Queries).AdminExportCommanderGuildTechnologies, (*db.Queries).AdminDeleteCommanderGuildTechnologies},
	{"commander_handbook_finished", (*db.Queries).AdminExportCommanderHandbookFinished, (*db.Queries).AdminDeleteCommanderHandbookFinished},
	{"commander_handbook_pages", (*db.Queries).AdminExportCommanderHandbookPages, (*db.Queries).AdminDeleteCommanderHandbookPages},
	{"commander_island_nodes", (*db.Queries).AdminExportCommanderIslandNodes, (*db.Queries).AdminDeleteCommanderIslandNodes},
	{"commander_islands", (*db.Queries).AdminExportCommanderIslands, (*db.Queries).AdminDeleteCommanderIslands},
	{"commander_items", (*db.Queries).AdminExportCommanderItems, (*db.Queries).AdminDeleteCommanderItems},
	{"commander_juus_groups", (*db.Queries).AdminExportCommanderJuusGroups, (*db.Queries).AdminDeleteCommanderJuusGroups},
	{"commander_juus_posts", (*db.Queries).AdminExportCommanderJuusPosts, (*db.Queries).AdminDeleteCommanderJuusPosts},
	{"commander_juus_replies", (*db.Queries).AdminExportCommanderJuusReplies, (*db.Queries).AdminDeleteCommanderJuusReplies},
	{"commander_juus_topics", (*db.Queries).AdminExportCommanderJuusTopics, (*db.Queries).AdminDeleteCommanderJuusTopics},
	{"commander_love_letter_gifts", (*db.Queries).AdminExportCommanderLoveLetterGifts, (*db.Queries).AdminDeleteCommanderLoveLetterGifts},
	{"commander_love_letter_medals", (*db.Queries).AdminExportCommanderLoveLetterMedals, (*db.Queries).AdminDeleteCommanderLoveLetterMedals},
	{"commander_love_letter_rewards", (*db.Queries).AdminExportCommanderLoveLetterRewards, (*db.Queries).AdminDeleteCommanderLoveLetterRewards},
	{"commander_love_letters", (*db.Queries).AdminExportCommanderLoveLetters, (*db.Queries).AdminDeleteCommanderLoveLetters},
	{"commander_medal_shop_purchases", (*db.Queries).AdminExportCommanderMedalShopPurchases, (*db.Queries).AdminDeleteCommanderMedalShopPurchases},
	{"commander_mini_game_shop_purchases", (*db.Queries).AdminExportCommanderMiniGameShopPurchases, (*db.Queries).AdminDeleteCommanderMiniGameShopPurchases},
	{"commander_minigame_data", (*db.Queries).AdminExportCommanderMinigameData, (*db.Queries).AdminDeleteCommanderMinigameData},
	{"commander_minigame_hubs", (*db.Queries).AdminExportCommanderMinigameHubs, (*db.Queries).AdminDeleteCommanderMinigameHubs},
	{"commander_minigame_scores", (*db.Queries).AdminExportCommanderMinigameScores, (*db.Queries).AdminDeleteCommanderMinigameScores},
	{"commander_minigame_values", (*db.Queries).AdminExportCommanderMinigameValues, (*db.Queries).AdminDeleteCommanderMinigameValues},
	{"commander_month_shop_purchases", (*db.Queries).AdminExportCommanderMonthShopPurchases, (*db.Queries).AdminDeleteCommanderMonthShopPurchases},
	{"commander_music_player", (*db.Queries).AdminExportCommanderMusicPlayer, (*db.Queries).AdminDeleteCommanderMusicPlayer},
	{"commander_new_educate", (*db.Queries).AdminExportCommanderNewEducate, (*db.Queries).AdminDeleteCommanderNewEducate},
	{"commander_permanent_activities", (*db.Queries).AdminExportCommanderPermanentActivities, (*db.Queries).AdminDeleteCommanderPermanentActivities},
	{"commander_play_room_scores", (*db.Queries).AdminExportCommanderPlayRoomScores, (*db.Queries).AdminDeleteCommanderPlayRoomScores},
	{"commander_play_room_views", (*db.Queries).AdminExportCommanderPlayRoomViews, (*db.Queries).AdminDeleteCommanderPlayRoomViews},
	{"commander_random_flagship", (*db.Queries).AdminExportCommanderRandomFlagship, (*db.Queries).AdminDeleteCommanderRandomFlagship},
	{"commander_reflux", (*db.Queries).AdminExportCommanderReflux, (*db.Queries).AdminDeleteCommanderReflux},
	{"commander_remasters", (*db.Queries).AdminExportCommanderRemasters, (*db.Queries).AdminDeleteCommanderRemasters},
	{"commander_requisitions", (*db.Queries).AdminExportCommanderRequisitions, (*db.Queries).AdminDeleteCommanderRequisitions},
	{"commander_resources", (*db.Queries).AdminExportCommanderResources, (*db.Queries).AdminDeleteCommanderResources},
	{"commander_secondary_passwords", (*db.Queries).AdminExportCommanderSecondaryPasswords, (*db.Queries).AdminDeleteCommanderSecondaryPasswords},
	{"commander_secretaries", (*db.Queries).AdminExportCommanderSecretaries, (*db.Queries).AdminDeleteCommanderSecretaries},
	{"commander_ship_groups", (*db.Queries).AdminExportCommanderShipGroups, (*db.Queries).AdminDeleteCommanderShipGroups},
	{"commander_shop_group_purchases", (*db.Queries).AdminExportCommanderShopGroupPurchases, (*db.Queries).AdminDeleteCommanderShopGroupPurchases},
	{"commander_shop_purchases", (*db.Queries).AdminExportCommanderShopPurchases, (*db.Queries).AdminDeleteCommanderShopPurchases},
	{"commander_skill_classes", (*db.Queries).AdminExportCommanderSkillClasses, (*db.Queries).AdminDeleteCommanderSkillClasses},
	{"commander_skins", (*db.Queries).AdminExportCommanderSkins, (*db.Queries).AdminDeleteCommanderSkins},
	{"commander_stories", (*db.Queries).AdminExportCommanderStories, (*db.Queries).AdminDeleteCommanderStories},
	{"commander_task_counters", (*db.Queries).AdminExportCommanderTaskCounters, (*db.Queries).AdminDeleteCommanderTaskCounters},
	{"commander_tasks", (*db.Queries).AdminExportCommanderTasks, (*db.Queries).AdminDeleteCommanderTasks},
	{"commander_trophies", (*db.Queries).AdminExportCommanderTrophies, (*db.Queries).AdminDeleteCommanderTrophies},
	{"commander_trophy_claims", (*db.Queries).AdminExportCommanderTrophyClaims, (*db.Queries).AdminDeleteCommanderTrophyClaims},
	{"commander_virtual_items", (*db.Queries).AdminExportCommanderVirtualItems, (*db.Queries).AdminDeleteCommanderVirtualItems},
	{"commander_weekly", (*db.Queries).AdminExportCommanderWeekly, (*db.Queries).AdminDeleteCommanderWeekly},
	{"commander_weekly_tasks", (*db.Queries).AdminExportCommanderWeeklyTasks, (*db.Queries).AdminDeleteCommanderWeeklyTasks},
	{"compensations", (*db.Queries).AdminExportCompensations, (*db.Queries).AdminDeleteCompensations},
	{"daily_level_counts", (*db.Queries).AdminExportDailyLevelCounts, (*db.Queries).AdminDeleteDailyLevelCounts},
	{"daily_quick_stages", (*db.Queries).AdminExportDailyQuickStages, (*db.Queries).AdminDeleteDailyQuickStages},
	{"dorm_floors", (*db.Queries).AdminExportDormFloors, (*db.Queries).AdminDeleteDormFloors},
	{"dorm_ships", (*db.Queries).AdminExportDormShips, (*db.Queries).AdminDeleteDormShips},
	{"dorm_theme_favs", (*db.Queries).AdminExportDormThemeFavs, (*db.Queries).AdminDeleteDormThemeFavs},
	{"dorm_theme_likes", (*db.Queries).AdminExportDormThemeLikes, (*db.Queries).AdminDeleteDormThemeLikes},
	{"dorm_themes", (*db.Queries).AdminExportDormThemes, (*db.Queries).AdminDeleteDormThemes},
	{"equip_code_likes", (*db.Queries).AdminExportEquipCodeLikes, (*db.Queries).AdminDeleteEquipCodeLikes},
	{"equip_code_reports", (*db.Queries).AdminExportEquipCodeReports, (*db.Queries).AdminDeleteEquipCodeReports},
	{"equip_codes", (*db.Queries).AdminExportEquipCodes, (*db.Queries).AdminDeleteEquipCodes},
	{"exchange_code_redemptions", (*db.Queries).AdminExportExchangeCodeRedemptions, (*db.Queries).AdminDeleteExchangeCodeRedemptions},
	{"fleet_meowfficers", (*db.Queries).AdminExportFleetMeowfficers, (*db.Queries).AdminDeleteFleetMeowfficers},
	{"fleet_ships", (*db.Queries).AdminExportFleetShips, (*db.Queries).AdminDeleteFleetShips},
	{"fleet_tech_attrs", (*db.Queries).AdminExportFleetTechAttrs, (*db.Queries).AdminDeleteFleetTechAttrs},
	{"fleet_techs", (*db.Queries).AdminExportFleetTechs, (*db.Queries).AdminDeleteFleetTechs},
	{"fleets", (*db.Queries).AdminExportFleets, (*db.Queries).AdminDeleteFleets},
	{"friend_requests", (*db.Queries).AdminExportFriendRequests, (*db.Queries).AdminDeleteFriendRequests},
	{"friends", (*db.Queries).AdminExportFriends, (*db.Queries).AdminDeleteFriends},
	{"guild_assault_ships", (*db.Queries).AdminExportGuildAssaultShips, (*db.Queries).AdminDeleteGuildAssaultShips},
	{"guild_boss_fleet_meowfficers", (*db.Queries).AdminExportGuildBossFleetMeowfficers, (*db.Queries).AdminDeleteGuildBossFleetMeowfficers},
	{"guild_boss_fleet_ships", (*db.Queries).AdminExportGuildBossFleetShips, (*db.Queries).AdminDeleteGuildBossFleetShips},
	{"guild_chat", (*db.Queries).AdminExportGuildChat, (*db.Queries).AdminDeleteGuildChat},
	{"guild_members", (*db.Queries).AdminExportGuildMembers, (*db.Queries).AdminDeleteGuildMembers},
	{"guild_operation_members", (*db.Queries).AdminExportGuildOperationMembers, (*db.Queries).AdminDeleteGuildOperationMembers},
	{"guild_operation_perfs", (*db.Queries).AdminExportGuildOperationPerfs, (*db.Queries).AdminDeleteGuildOperationPerfs},
	{"guild_operation_ships", (*db.Queries).AdminExportGuildOperationShips, (*db.Queries).AdminDeleteGuildOperationShips},
	{"guild_rank_scores", (*db.Queries).AdminExportGuildRankScores, (*db.Queries).AdminDeleteGuildRankScores},
	{"guild_report_claims", (*db.Queries).AdminExportGuildReportClaims, (*db.Queries).AdminDeleteGuildReportClaims},
	{"guild_requests", (*db.Queries).AdminExportGuildRequests, (*db.Queries).AdminDeleteGuildRequests},
	{"guild_users", (*db.Queries).AdminExportGuildUsers, (*db.Queries).AdminDeleteGuildUsers},
	{"guild_wait_times", (*db.Queries).AdminExportGuildWaitTimes, (*db.Queries).AdminDeleteGuildWaitTimes},
	{"limit_challenges", (*db.Queries).AdminExportLimitChallenges, (*db.Queries).AdminDeleteLimitChallenges},
	{"mails", (*db.Queries).AdminExportMails, (*db.Queries).AdminDeleteMails},
	{"meowfficer_boxes", (*db.Queries).AdminExportMeowfficerBoxes, (*db.Queries).AdminDeleteMeowfficerBoxes},
	{"meowfficer_catteries", (*db.Queries).AdminExportMeowfficerCatteries, (*db.Queries).AdminDeleteMeowfficerCatteries},
	{"meowfficer_preset_slots", (*db.Queries).AdminExportMeowfficerPresetSlots, (*db.Queries).AdminDeleteMeowfficerPresetSlots},
	{"meowfficer_presets", (*db.Queries).AdminExportMeowfficerPresets, (*db.Queries).AdminDeleteMeowfficerPresets},
	{"meowfficer_states", (*db.Queries).AdminExportMeowfficerStates, (*db.Queries).AdminDeleteMeowfficerStates},
	{"meowfficers", (*db.Queries).AdminExportMeowfficers, (*db.Queries).AdminDeleteMeowfficers},
	{"meta_pt_awards", (*db.Queries).AdminExportMetaPtAwards, (*db.Queries).AdminDeleteMetaPtAwards},
	{"meta_pts", (*db.Queries).AdminExportMetaPts, (*db.Queries).AdminDeleteMetaPts},
	{"new_server_shop_goods", (*db.Queries).AdminExportNewServerShopGoods, (*db.Queries).AdminDeleteNewServerShopGoods},
	{"new_server_shop_sub_goods", (*db.Queries).AdminExportNewServerShopSubGoods, (*db.Queries).AdminDeleteNewServerShopSubGoods},
	{"remaster_awards", (*db.Queries).AdminExportRemasterAwards, (*db.Queries).AdminDeleteRemasterAwards},
	{"remaster_states", (*db.Queries).AdminExportRemasterStates, (*db.Queries).AdminDeleteRemasterStates},
	{"ship_discussion_reports", (*db.Queries).AdminExportShipDiscussionReports, (*db.Queries).AdminDeleteShipDiscussionReports},
	{"ship_discussion_votes", (*db.Queries).AdminExportShipDiscussionVotes, (*db.Queries).AdminDeleteShipDiscussionVotes},
	{"ship_discussions", (*db.Queries).AdminExportShipDiscussions, (*db.Queries).AdminDeleteShipDiscussions},
	{"ship_equipments", (*db.Queries).AdminExportShipEquipments, (*db.Queries).AdminDeleteShipEquipments},
	{"ship_meta_repairs", (*db.Queries).AdminExportShipMetaRepairs, (*db.Queries).AdminDeleteShipMetaRepairs},
	{"ship_meta_tactics", (*db.Queries).AdminExportShipMetaTactics, (*db.Queries).AdminDeleteShipMetaTactics},
	{"ship_skills", (*db.Queries).AdminExportShipSkills, (*db.Queries).AdminDeleteShipSkills},
	{"ship_skin_shadows", (*db.Queries).AdminExportShipSkinShadows, (*db.Queries).AdminDeleteShipSkinShadows},
	{"ship_spweapons", (*db.Queries).AdminExportShipSpweapons, (*db.Queries).AdminDeleteShipSpweapons},
	{"ship_strengths", (*db.Queries).AdminExportShipStrengths, (*db.Queries).AdminDeleteShipStrengths},
	{"ship_transforms", (*db.Queries).AdminExportShipTransforms, (*db.Queries).AdminDeleteShipTransforms},
	{"ships", (*db.Queries).AdminExportShips, (*db.Queries).AdminDeleteShips},
	{"spweapons", (*db.Queries).AdminExportSpweapons, (*db.Queries).AdminDeleteSpweapons},
	{"surveys_done", (*db.Queries).AdminExportSurveysDone, (*db.Queries).AdminDeleteSurveysDone},
	{"technology_catchups", (*db.Queries).AdminExportTechnologyCatchups, (*db.Queries).AdminDeleteTechnologyCatchups},
	{"technology_offers", (*db.Queries).AdminExportTechnologyOffers, (*db.Queries).AdminDeleteTechnologyOffers},
	{"technology_queue", (*db.Queries).AdminExportTechnologyQueue, (*db.Queries).AdminDeleteTechnologyQueue},
	{"technology_states", (*db.Queries).AdminExportTechnologyStates, (*db.Queries).AdminDeleteTechnologyStates},
	{"world_boss_ships", (*db.Queries).AdminExportWorldBossShips, (*db.Queries).AdminDeleteWorldBossShips},
	{"world_bosses", (*db.Queries).AdminExportWorldBosses, (*db.Queries).AdminDeleteWorldBosses},
	{"world_maps", (*db.Queries).AdminExportWorldMaps, (*db.Queries).AdminDeleteWorldMaps},
	{"world_states", (*db.Queries).AdminExportWorldStates, (*db.Queries).AdminDeleteWorldStates},
	{"commanders", (*db.Queries).AdminExportCommanders, (*db.Queries).AdminDeleteCommanders},
}

// ExportCommander is every row of a commander, by table, as JSON arrays of
// objects (commanderTables).
func (game *Game) ExportCommander(ctx context.Context, id uint32) (map[string]json.RawMessage, error) {
	save := make(map[string]json.RawMessage, len(commanderTables))
	err := game.DB.InTx(ctx, func(queries *db.Queries) error {
		for _, table := range commanderTables {
			rows, err := table.export(queries, ctx, id)
			if err != nil {
				return fmt.Errorf("%s: %w", table.name, err)
			}
			if rows != "[]" {
				save[table.name] = json.RawMessage(rows)
			}
		}
		return nil
	})
	return save, err
}

// DeleteCommander deletes every row of a commander (commanderTables) in one
// transaction, kicking it first if it plays. A guild member leaves its guild
// first as if it quit (the members online are told); a guild's commander
// alone in it dissolves it, one with members is refused: the duty has to be
// handed over in the game first. The account stays: its next login creates
// a new commander.
func (game *Game) DeleteCommander(ctx context.Context, id uint32) (bool, error) {
	var members []uint32
	var update *protobuf.SC_60031
	kicked, err := game.AdminWrite(ctx, id, func(tx *Tx) error {
		queries := tx.Queries
		member, ok, err := guildMembership(ctx, queries, id)
		if err != nil {
			return err
		}
		if ok {
			all, err := queries.ListGuildMemberIDs(ctx, member.GuildID)
			if err != nil {
				return err
			}
			switch {
			case member.Duty == guildDutyCommander && len(all) > 1:
				return adminRefused("commands guild %d, %d other member(s) in it: hand the duty over first", member.GuildID, len(all)-1)
			case member.Duty == guildDutyCommander:
				if err := queries.DeleteGuild(ctx, member.GuildID); err != nil {
					return err
				}
			default:
				if update, _, members, err = game.removeGuildMember(tx, member, guildLogQuit); err != nil {
					return err
				}
			}
		}
		for _, table := range commanderTables {
			if err := table.delete(queries, ctx, id); err != nil {
				return fmt.Errorf("%s: %w", table.name, err)
			}
		}
		return nil
	})
	if err != nil {
		return kicked, err
	}
	game.setSecondaryUnlocked(id, false)
	if update != nil {
		game.tellGuild(nil, members, id, update)
	}
	return kicked, nil
}
