package answer

import (
	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/orm"
	"github.com/ggmolly/belfast/internal/protobuf"
	"google.golang.org/protobuf/proto"
)

func RelieveFriendBlacklist(buffer *[]byte, client *connection.Client) (int, int, error) {
	var payload protobuf.CS_50107
	if err := proto.Unmarshal(*buffer, &payload); err != nil {
		return 0, 50108, err
	}

	targetID := payload.GetId()
	if targetID == 0 || targetID == client.Commander.CommanderID {
		response := protobuf.SC_50108{Result: proto.Uint32(friendBlacklistResultInvalidTarget)}
		return client.SendMessage(50108, &response)
	}

	removed, err := orm.RemoveFriendBlacklist(client.Commander.CommanderID, targetID)
	if err != nil {
		return 0, 50108, err
	}

	if !removed {
		response := protobuf.SC_50108{Result: proto.Uint32(friendBlacklistResultNotFound)}
		return client.SendMessage(50108, &response)
	}

	response := protobuf.SC_50108{Result: proto.Uint32(friendBlacklistResultSuccess)}
	return client.SendMessage(50108, &response)
}
