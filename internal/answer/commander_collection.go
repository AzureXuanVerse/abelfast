package answer

import (
	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/orm"
	"github.com/ggmolly/belfast/internal/protobuf"
	"google.golang.org/protobuf/proto"
)

func CommanderCollection(buffer *[]byte, client *connection.Client) (int, int, error) {
	// Out of all commander's OwnedShips, return the max star, max intimacy, and max level
	// of each ship group (= TemplateID divided by 10)

	rows, err := listCollectionShipStats(client.Commander.CommanderID)
	if err != nil {
		return 0, 17001, err
	}

	stats := make([]*protobuf.SHIP_STATISTICS_INFO, len(rows))
	for i := range rows {
		stats[i] = buildShipStatisticsInfo(rows[i])
	}

	trophyProgress, err := orm.ListCommanderTrophyProgress(client.Commander.CommanderID)
	if err != nil {
		return 0, 17001, err
	}
	progressList := make([]*protobuf.ACHIEVEMENT_INFO, len(trophyProgress))
	for i := range trophyProgress {
		progressList[i] = buildAchievementInfo(trophyProgress[i])
	}

	progress, err := orm.ListCommanderStoreupAwardProgress(client.Commander.CommanderID)
	if err != nil {
		return 0, 17001, err
	}
	awards := make([]*protobuf.SHIP_STATISTICS_AWARD, 0, len(progress))
	for i := range progress {
		if progress[i].LastAwardIndex == 0 {
			continue
		}
		awards = append(awards, &protobuf.SHIP_STATISTICS_AWARD{
			Id:         proto.Uint32(progress[i].StoreupID),
			AwardIndex: []uint32{progress[i].LastAwardIndex},
		})
	}

	response := protobuf.SC_17001{
		DailyDiscuss:  proto.Uint32(0),
		ProgressList:  progressList,
		ShipInfoList:  stats,
		ShipAwardList: awards,
	}
	return client.SendMessage(17001, &response)
}
