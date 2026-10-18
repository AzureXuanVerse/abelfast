package game

import "github.com/ggmolly/belfast/internal/gamedata"

// loadGameData reads every config table the handlers use, once at startup:
// as is (gamedata.Load) or into what the handlers index (ships, construction,
// the campaign, tasks...).
func loadGameData() error {
	return gamedata.LoadAll(
		loadShipData,
		loadBuildExchange,
		loadSkinData,
		loadChapterData,
		loadChapterAutoData,
		loadExperienceData,
		loadSignInData,
		loadTaskData,
		loadJuusData,
		gamedata.Load("constellation_challenge_month", &limitChallengeMonths),
		loadArenaShop,
		loadArenaRanks,
		loadCommissionData,
		loadMedalShop,
		loadShopData,
		loadItemData,
		loadEducateData,
		loadNewCommander,
		loadStarterFurniture,
		loadDormData,
		loadMailData,
		loadDiscussionData,
		loadCollectionData,
		loadMetaTacticsData,
		loadSkillClassData,
		loadFleetTechData,
		loadCryptolaliaData,
		loadMonthShops,
		loadResourceFields,
		loadMeowfficerData,
		loadMeowfficerManageData,
		loadMeowfficerTalentData,
		loadMeowfficerBattleData,
		loadDailyLevels,
		loadDockyardData,
		loadGuildData,
	)
}
