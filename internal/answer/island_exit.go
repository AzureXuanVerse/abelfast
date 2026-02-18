package answer

import (
	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/protobuf"
	"google.golang.org/protobuf/proto"
)

func IslandExit(buffer *[]byte, client *connection.Client) (int, int, error) {
	var request protobuf.CS_21204
	if err := proto.Unmarshal(*buffer, &request); err != nil {
		return 0, 21205, err
	}

	result := uint32(1)
	if globalIslandRuntimeState.releaseSession(client.Commander.CommanderID, request.GetIslandId()) {
		result = 0
	}

	response := protobuf.SC_21205{Result: proto.Uint32(result)}
	return client.SendMessage(21205, &response)
}
