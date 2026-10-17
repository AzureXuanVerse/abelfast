package game

import "github.com/ggmolly/belfast/internal/gamedata"

// loadGameData reads every config table the handlers use, once at startup:
// as is (loadTable) or into what the handlers index (ships, construction,
// the campaign, tasks...).
func loadGameData() error {
	return gamedata.LoadAll(
		loadShipData,
		loadChapterData,
		loadSignInData,
		loadTaskData,
		loadJuusData,
		loadBuildTimes,
		gamedata.Load("constellation_challenge_month", &limitChallengeMonths),
		loadArenaShop,
		loadCommissionTeams,
		gamedata.Load("honormedal_goods_list", &medalGoods),
		loadEducateData,
	)
}
