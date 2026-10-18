package game

import "github.com/ggmolly/belfast/internal/gamedata"

// loadGameData reads every config table the handlers use, once at startup:
// as is (gamedata.Load) or into what the handlers index (ships, construction,
// the campaign, tasks...).
func loadGameData() error {
	return gamedata.LoadAll(
		loadShipData,
		loadSkinData,
		loadChapterData,
		loadExperienceData,
		loadSignInData,
		loadTaskData,
		loadJuusData,
		gamedata.Load("constellation_challenge_month", &limitChallengeMonths),
		loadArenaShop,
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
		loadCryptolaliaData,
		loadMonthShops,
		loadResourceFields,
	)
}
