package answer

import (
	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/protobuf"
	"google.golang.org/protobuf/proto"
)

func HandleIslandReconnect(buffer *[]byte, client *connection.Client) (int, int, error) {
	var request protobuf.CS_21230
	if err := proto.Unmarshal(*buffer, &request); err != nil {
		return 0, 21231, err
	}

	result := uint32(1)
	if globalIslandRuntimeState.hasMatchingSession(client.Commander.CommanderID, request.GetIslandId()) {
		result = 0
	}

	response := protobuf.SC_21231{
		Result: proto.Uint32(result),
	}
	return client.SendMessage(21231, &response)
}
