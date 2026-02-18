package answer

import (
	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/protobuf"
	"google.golang.org/protobuf/proto"
)

const islandSyncObjectBatchLimit = 128

func IslandSyncData(buffer *[]byte, client *connection.Client) (int, int, error) {
	var request protobuf.CS_21211
	if err := proto.Unmarshal(*buffer, &request); err != nil {
		return 0, 0, err
	}

	if !globalIslandRuntimeState.hasMatchingSession(client.Commander.CommanderID, request.GetIslandId()) {
		return 0, 0, nil
	}
	if len(request.GetSyncObList()) == 0 || len(request.GetSyncObList()) > islandSyncObjectBatchLimit {
		return 0, 0, nil
	}

	push := &protobuf.SC_21212{SyncObList: request.GetSyncObList()}
	broadcastIslandPacketExcludingCommander(client.Server, request.GetIslandId(), 21212, push, client.Commander.CommanderID)
	return 0, 0, nil
}
