package answer

import (
	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/protobuf"
	"google.golang.org/protobuf/proto"
)

func IslandSyncControl(buffer *[]byte, client *connection.Client) (int, int, error) {
	var request protobuf.CS_21209
	if err := proto.Unmarshal(*buffer, &request); err != nil {
		return 0, 21210, err
	}

	result := uint32(1)
	if globalIslandRuntimeState.hasMatchingSession(client.Commander.CommanderID, request.GetIslandId()) {
		updated, ok := globalIslandRuntimeState.applyIslandControl(
			client.Commander.CommanderID,
			request.GetIslandId(),
			request.GetType(),
			request.GetObjId(),
			request.GetSlotId(),
			request.GetOp(),
			request.GetStatus(),
		)
		if ok {
			result = 0
			push := &protobuf.SC_21207{
				IslandId:   proto.Uint32(request.GetIslandId()),
				ObjectList: []*protobuf.PB_OBJECT{updated},
			}
			broadcastIslandPacket(client.Server, request.GetIslandId(), 21207, push)
		}
	}

	response := protobuf.SC_21210{Result: proto.Uint32(result)}
	return client.SendMessage(21210, &response)
}
