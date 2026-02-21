package friend

import (
	"time"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/orm"
	"github.com/ggmolly/belfast/internal/protobuf"
	"google.golang.org/protobuf/proto"
)

func SendFriendRequest(buffer *[]byte, client *connection.Client) (int, int, error) {
	var payload protobuf.CS_50003
	if err := proto.Unmarshal(*buffer, &payload); err != nil {
		return 0, 50004, err
	}

	response := protobuf.SC_50004{Result: proto.Uint32(friendOperationFailure)}
	requesterID := client.Commander.CommanderID
	targetID := payload.GetId()
	if targetID == 0 || targetID == requesterID {
		return client.SendMessage(50004, &response)
	}

	if err := orm.CommanderExists(targetID); err != nil {
		return client.SendMessage(50004, &response)
	}

	alreadyFriends, err := orm.AreFriends(requesterID, targetID)
	if err != nil {
		return client.SendMessage(50004, &response)
	}
	if alreadyFriends {
		return client.SendMessage(50004, &response)
	}

	created, err := orm.CreateFriendRequest(requesterID, targetID, payload.GetContent())
	if err != nil {
		return client.SendMessage(50004, &response)
	}

	if created {
		response.Result = proto.Uint32(friendOperationSuccess)
	} else {
		response.Result = proto.Uint32(friendOperationFailure)
	}

	n, packetID, sendErr := client.SendMessage(50004, &response)
	if sendErr != nil {
		return n, packetID, sendErr
	}

	if !created || client.Server == nil {
		return n, packetID, nil
	}

	targetClient, ok := client.Server.FindClientByCommander(targetID)
	if !ok {
		return n, packetID, nil
	}

	requesterProfile, err := orm.GetCommanderSocialProfile(requesterID)
	if err != nil {
		return n, packetID, nil
	}
	push := protobuf.SC_50005{
		Msg: &protobuf.MSG_INFO_P50{
			Timestamp: proto.Uint32(uint32(time.Now().UTC().Unix())),
			Player:    buildPlayerInfoP50(requesterProfile),
			Content:   proto.String(payload.GetContent()),
		},
	}
	_, _, _ = targetClient.SendMessage(50005, &push)
	return n, packetID, nil
}
