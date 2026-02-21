package answer

import (
	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/protobuf"
	"google.golang.org/protobuf/proto"
)

func sendCommonFlagPush(client *connection.Client, flagID uint32, enabled bool) (int, int, error) {
	value := uint32(0)
	if enabled {
		value = 1
	}
	response := protobuf.SC_11802{
		Id:    proto.Uint32(flagID),
		Value: proto.Uint32(value),
	}
	return client.SendMessage(11802, &response)
}
