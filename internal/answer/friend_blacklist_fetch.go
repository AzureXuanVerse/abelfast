package answer

import (
	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/orm"
	"github.com/ggmolly/belfast/internal/protobuf"
	"google.golang.org/protobuf/proto"
)

func GetFriendBlacklist(buffer *[]byte, client *connection.Client) (int, int, error) {
	var payload protobuf.CS_50016
	if err := proto.Unmarshal(*buffer, &payload); err != nil {
		return 0, 50017, err
	}

	blacklist, err := orm.GetFriendBlacklist(client.Commander.CommanderID)
	if err != nil {
		return 0, 50017, err
	}

	commanders, err := orm.GetCommanderCoresByIDs(blacklist)
	if err != nil {
		return 0, 50017, err
	}

	response := protobuf.SC_50017{BlackList: make([]*protobuf.PLAYER_INFO_P50, 0, len(blacklist))}
	for _, blacklistedID := range blacklist {
		commander, ok := commanders[blacklistedID]
		if !ok {
			continue
		}
		response.BlackList = append(response.BlackList, &protobuf.PLAYER_INFO_P50{
			Id:   proto.Uint32(commander.CommanderID),
			Name: proto.String(commander.Name),
			Lv:   proto.Uint32(uint32(commander.Level)),
		})
	}

	return client.SendMessage(50017, &response)
}
