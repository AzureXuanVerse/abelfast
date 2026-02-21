package island

import (
	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/orm"
	"github.com/ggmolly/belfast/internal/protobuf"
	"google.golang.org/protobuf/proto"
)

func IslandTradeInvitation(buffer *[]byte, client *connection.Client) (int, int, error) {
	var request protobuf.CS_21245
	if err := proto.Unmarshal(*buffer, &request); err != nil {
		return 0, 21246, err
	}

	response := protobuf.SC_21246{Result: proto.Uint32(1)}
	candidateTargets := normalizeCommanderIDs(request.GetFriendList(), client.Commander.CommanderID)
	if len(candidateTargets) == 0 {
		return client.SendMessage(21246, &response)
	}

	validTargets := make([]uint32, 0, len(candidateTargets))
	for _, targetCommanderID := range candidateTargets {
		if err := orm.CommanderExists(targetCommanderID); err != nil {
			continue
		}
		validTargets = append(validTargets, targetCommanderID)
	}
	if len(validTargets) == 0 {
		return client.SendMessage(21246, &response)
	}

	inviteState, err := orm.GetOrCreateCommanderIslandTradeInviteState(client.Commander.CommanderID)
	if err != nil {
		return client.SendMessage(21246, &response)
	}
	inviteState.InvitedCommanderIDs = mergeUniqueCommanderIDs(inviteState.InvitedCommanderIDs, validTargets)
	if err := orm.SaveCommanderIslandTradeInviteState(inviteState); err != nil {
		return client.SendMessage(21246, &response)
	}

	if client.Server != nil {
		for _, targetCommanderID := range validTargets {
			peer, ok := client.Server.FindClientByCommander(targetCommanderID)
			if !ok {
				continue
			}
			peer.SendMessage(21247, &protobuf.SC_21247{
				IslandId: proto.Uint32(client.Commander.CommanderID),
				MapId:    proto.Uint32(request.GetMapId()),
				Price:    proto.Uint32(request.GetPrice()),
			})
		}
	}

	response.Result = proto.Uint32(0)
	return client.SendMessage(21246, &response)
}
