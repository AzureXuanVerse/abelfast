package answer

import (
	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/orm"
	"github.com/ggmolly/belfast/internal/protobuf"
	"google.golang.org/protobuf/proto"
)

func RejectFriendRequest(buffer *[]byte, client *connection.Client) (int, int, error) {
	var payload protobuf.CS_50009
	if err := proto.Unmarshal(*buffer, &payload); err != nil {
		return 0, 50010, err
	}

	response := protobuf.SC_50010{Result: proto.Uint32(friendOperationSuccess)}
	targetID := client.Commander.CommanderID
	requesterID := payload.GetId()

	if requesterID == 0 {
		if err := orm.DeleteAllFriendRequestsForTarget(targetID); err != nil {
			response.Result = proto.Uint32(friendOperationFailure)
		}
		return client.SendMessage(50010, &response)
	}

	if _, err := orm.DeleteFriendRequest(targetID, requesterID); err != nil {
		response.Result = proto.Uint32(friendOperationFailure)
	}

	return client.SendMessage(50010, &response)
}
