package guild

import (
	"errors"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/db"
	"github.com/ggmolly/belfast/internal/orm"
	"github.com/ggmolly/belfast/internal/protobuf"
	"google.golang.org/protobuf/proto"
)

func GuildGetReportRankCommandResponse(buffer *[]byte, client *connection.Client) (int, int, error) {
	var payload protobuf.CS_61037
	if err := proto.Unmarshal(*buffer, &payload); err != nil {
		return 0, 61038, err
	}
	response := &protobuf.SC_61038{List: []*protobuf.RANK_INFO_P61{}}
	if client.Commander == nil || payload.GetId() == 0 {
		return client.SendMessage(61038, response)
	}
	guild, _, err := orm.GetGuildForCommander(client.Commander.CommanderID)
	if err != nil {
		if errors.Is(err, db.ErrNotFound) {
			return client.SendMessage(61038, response)
		}
		return 0, 61038, err
	}
	ranks, err := orm.ListGuildReportRanks(guild.ID, payload.GetId())
	if err != nil {
		return 0, 61038, err
	}
	response.List = make([]*protobuf.RANK_INFO_P61, 0, len(ranks))
	for _, entry := range ranks {
		response.List = append(response.List, &protobuf.RANK_INFO_P61{UserId: proto.Uint32(entry.UserID), Damage: proto.Uint32(entry.Damage)})
	}
	return client.SendMessage(61038, response)
}
