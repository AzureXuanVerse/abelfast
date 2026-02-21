package island

import (
	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/protobuf"
	"google.golang.org/protobuf/proto"
)

func IslandAnimationOp(buffer *[]byte, client *connection.Client) (int, int, error) {
	var request protobuf.CS_21700
	if err := proto.Unmarshal(*buffer, &request); err != nil {
		return 0, 0, err
	}

	if !globalIslandRuntimeState.hasMatchingSession(client.Commander.CommanderID, request.GetIslandId()) {
		return 0, 0, nil
	}

	push := &protobuf.SC_21701{
		IslandId: proto.Uint32(request.GetIslandId()),
		PlayerId: proto.Uint32(client.Commander.CommanderID),
		TargetId: proto.Uint32(request.GetTargetId()),
		ActionId: proto.Uint32(request.GetActionId()),
	}
	broadcastIslandPacket(client.Server, request.GetIslandId(), 21701, push)
	return 0, 0, nil
}
