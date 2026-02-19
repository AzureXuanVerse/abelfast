package answer

import (
	"errors"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/orm"
	"github.com/ggmolly/belfast/internal/protobuf"
	"google.golang.org/protobuf/proto"
)

func AcceptFriendRequest(buffer *[]byte, client *connection.Client) (int, int, error) {
	var payload protobuf.CS_50006
	if err := proto.Unmarshal(*buffer, &payload); err != nil {
		return 0, 50007, err
	}

	response := protobuf.SC_50007{Result: proto.Uint32(friendOperationFailure)}
	targetID := client.Commander.CommanderID
	requesterID := payload.GetId()
	if requesterID == 0 || requesterID == targetID {
		return client.SendMessage(50007, &response)
	}

	if err := orm.CommanderExists(requesterID); err != nil {
		return client.SendMessage(50007, &response)
	}

	targetFriendCount, err := orm.CountFriends(targetID)
	if err != nil {
		return client.SendMessage(50007, &response)
	}
	if targetFriendCount >= maxFriendCount {
		response.Result = proto.Uint32(friendOperationMaxed)
		return client.SendMessage(50007, &response)
	}

	requesterFriendCount, err := orm.CountFriends(requesterID)
	if err != nil {
		return client.SendMessage(50007, &response)
	}
	if requesterFriendCount >= maxFriendCount {
		response.Result = proto.Uint32(friendOperationMaxed)
		return client.SendMessage(50007, &response)
	}

	err = orm.AcceptFriendRequest(targetID, requesterID)
	if err != nil {
		if errors.Is(err, orm.ErrFriendRequestNotFound) {
			return client.SendMessage(50007, &response)
		}
		return client.SendMessage(50007, &response)
	}

	response.Result = proto.Uint32(friendOperationSuccess)
	n, packetID, sendErr := client.SendMessage(50007, &response)
	if sendErr != nil {
		return n, packetID, sendErr
	}

	if client.Server == nil {
		return n, packetID, nil
	}

	targetProfile, err := orm.GetCommanderSocialProfile(targetID)
	if err != nil {
		return n, packetID, nil
	}
	requesterProfile, err := orm.GetCommanderSocialProfile(requesterID)
	if err != nil {
		return n, packetID, nil
	}

	acceptorPush := protobuf.SC_50008{Player: buildFriendInfo(requesterProfile, client)}
	_, _, _ = client.SendMessage(50008, &acceptorPush)

	if requesterClient, ok := client.Server.FindClientByCommander(requesterID); ok {
		requesterPush := protobuf.SC_50008{Player: buildFriendInfo(targetProfile, requesterClient)}
		_, _, _ = requesterClient.SendMessage(50008, &requesterPush)
	}

	return n, packetID, nil
}
