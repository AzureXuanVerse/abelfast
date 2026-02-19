package answer

import (
	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/orm"
	"github.com/ggmolly/belfast/internal/protobuf"
	"google.golang.org/protobuf/proto"
)

func DeleteFriend(buffer *[]byte, client *connection.Client) (int, int, error) {
	request := &protobuf.CS_50011{}
	if err := proto.Unmarshal(*buffer, request); err != nil {
		return 0, 50012, err
	}

	targetCommanderID := request.GetId()
	response := &protobuf.SC_50012{Result: proto.Uint32(1)}
	if targetCommanderID == 0 || targetCommanderID == client.Commander.CommanderID {
		return client.SendMessage(50012, response)
	}

	deleted, err := orm.DeleteCommanderFriendRelationPair(client.Commander.CommanderID, targetCommanderID)
	if err != nil {
		return client.SendMessage(50012, response)
	}
	if !deleted {
		return client.SendMessage(50012, response)
	}

	response.Result = proto.Uint32(0)
	if _, _, err := client.SendMessage(50012, response); err != nil {
		return 0, 50012, err
	}

	push := &protobuf.SC_50013{Id: proto.Uint32(targetCommanderID)}
	if _, _, err := client.SendMessage(50013, push); err != nil {
		return 0, 50013, err
	}

	if client.Server != nil {
		if targetClient, ok := client.Server.FindClientByCommander(targetCommanderID); ok {
			peerPush := &protobuf.SC_50013{Id: proto.Uint32(client.Commander.CommanderID)}
			_, _, _ = targetClient.SendMessage(50013, peerPush)
		}
	}

	return 0, 50013, nil
}
