package profile

import (
	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/orm"
	"github.com/ggmolly/belfast/internal/protobuf"
	"google.golang.org/protobuf/proto"
)

func GetPlayerSummaryInfo(buffer *[]byte, client *connection.Client) (int, int, error) {
	var payload protobuf.CS_26021
	if err := proto.Unmarshal(*buffer, &payload); err != nil {
		return 0, 26022, err
	}

	stats, err := orm.GetPlayerSummaryStats(client.Commander.CommanderID)
	if err != nil {
		return 0, 26022, err
	}

	response := protobuf.SC_26022{
		Result:          proto.Uint32(0),
		RegisterDate:    proto.Uint32(stats.RegisterDate),
		GuildName:       proto.String(""),
		ChapterId:       proto.Uint32(stats.ChapterID),
		MarryNumber:     proto.Uint32(stats.MarryNumber),
		MedalNumber:     proto.Uint32(stats.MedalNumber),
		FurnitureNumber: proto.Uint32(stats.FurnitureNumber),
		FurnitureWorth:  proto.Uint32(stats.FurnitureWorth),
		CharacterId:     proto.Uint32(stats.CharacterID),
		FirstLadyId:     proto.Uint32(stats.FirstLadyID),
		FirstLadyName:   proto.String(stats.FirstLadyName),
		FirstLadyTime:   proto.Uint32(stats.FirstLadyTime),
		FirstOnline:     proto.Uint32(stats.FirstOnline),
		WorldMaxTask:    proto.Uint32(stats.WorldMaxTask),
		CollectNum:      proto.Uint32(stats.CollectNum),
		Combat:          proto.Uint32(stats.Combat),
		ShipNumTotal:    proto.Uint32(stats.ShipNumTotal),
		ShipNum_120:     proto.Uint32(stats.ShipNum120),
		ShipNum_125:     proto.Uint32(stats.ShipNum125),
		Love200Num:      proto.Uint32(stats.Love200Num),
		SkinNum:         proto.Uint32(stats.SkinNum),
		SkinShipNum:     proto.Uint32(stats.SkinShipNum),
	}

	return client.SendMessage(26022, &response)
}
