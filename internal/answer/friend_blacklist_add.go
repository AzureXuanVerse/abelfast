package answer

import (
	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/orm"
	"github.com/ggmolly/belfast/internal/protobuf"
	"google.golang.org/protobuf/proto"
)

func AddFriendBlacklist(buffer *[]byte, client *connection.Client) (int, int, error) {
	var payload protobuf.CS_50109
	if err := proto.Unmarshal(*buffer, &payload); err != nil {
		return 0, 50110, err
	}

	targetID := payload.GetId()
	if targetID == 0 || targetID == client.Commander.CommanderID {
		response := protobuf.SC_50110{Result: proto.Uint32(friendBlacklistResultInvalidTarget)}
		return client.SendMessage(50110, &response)
	}

	exists, err := orm.CommanderIDExists(targetID)
	if err != nil {
		return 0, 50110, err
	}
	if !exists {
		response := protobuf.SC_50110{Result: proto.Uint32(friendBlacklistResultInvalidTarget)}
		return client.SendMessage(50110, &response)
	}

	if _, err := orm.AddFriendBlacklist(client.Commander.CommanderID, targetID); err != nil {
		return 0, 50110, err
	}

	response := protobuf.SC_50110{Result: proto.Uint32(friendBlacklistResultSuccess)}
	return client.SendMessage(50110, &response)
}
