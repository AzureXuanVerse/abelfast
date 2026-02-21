package island

import (
	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/protobuf"
	"google.golang.org/protobuf/proto"
)

func IslandHeartbeat(buffer *[]byte, client *connection.Client) (int, int, error) {
	var request protobuf.CS_21215
	if err := proto.Unmarshal(*buffer, &request); err != nil {
		return 0, 21216, err
	}

	visitorList := []*protobuf.PB_VISITOR{}
	if globalIslandRuntimeState.hasMatchingSession(client.Commander.CommanderID, request.GetIslandId()) {
		visitorList = globalIslandRuntimeState.drainVisitorFeed(client.Commander.CommanderID)
	}

	response := protobuf.SC_21216{VisitorList: visitorList}
	return client.SendMessage(21216, &response)
}
