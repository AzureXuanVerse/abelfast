package answer

import (
	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/orm"

	"github.com/ggmolly/belfast/internal/protobuf"
	"google.golang.org/protobuf/proto"
)

func GuildGetUserInfoCommand(buffer *[]byte, client *connection.Client) (int, int, error) {
	info, err := orm.GetGuildUserInfo(client.Commander.CommanderID)
	if err != nil {
		return 0, 60103, err
	}
	response := protobuf.SC_60103{
		UserInfo: &protobuf.USER_GUILD_INFO{
			DonateCount:    proto.Uint32(info.DonateCount),
			DonateTasks:    info.DonateTasks,
			BenefitTime:    proto.Uint32(info.BenefitTime),
			TechId:         nil,
			WeeklyTaskFlag: proto.Uint32(info.WeeklyTaskFlag),
			ExtraDonate:    proto.Uint32(info.ExtraDonate),
			ExtraOperation: proto.Uint32(info.ExtraOperation),
		},
	}
	techIDs, err := orm.ListGuildUserTechnologyState(client.Commander.CommanderID)
	if err != nil {
		return 0, 60103, err
	}
	response.UserInfo.TechId = techIDs
	return client.SendMessage(60103, &response)
}
