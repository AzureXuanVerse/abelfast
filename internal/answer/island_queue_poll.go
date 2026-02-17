package answer

import (
	"time"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/protobuf"
	"google.golang.org/protobuf/proto"
)

func HandleIslandQueuePoll(buffer *[]byte, client *connection.Client) (int, int, error) {
	var request protobuf.CS_21208
	if err := proto.Unmarshal(*buffer, &request); err != nil {
		return 0, 21203, err
	}

	islandID := request.GetIslandId()
	if islandID == 0 {
		response := buildIslandEnterResponse(1, 0, 0, 0)
		return client.SendMessage(21203, response)
	}

	result, pos, cd := globalIslandRuntimeState.poll(
		client.Commander.CommanderID,
		client.Commander.Name,
		islandID,
		uint32(time.Now().UTC().Unix()),
	)
	response := buildIslandEnterResponse(result, islandID, pos, cd)
	return client.SendMessage(21203, response)
}
