package world

import (
	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/orm"

	"github.com/ggmolly/belfast/internal/protobuf"
	"google.golang.org/protobuf/proto"
)

func WorldCheckInfo(buffer *[]byte, client *connection.Client) (int, int, error) {
	runtime, err := orm.LoadOrCreateWorldRuntime(client.Commander.CommanderID)
	if err != nil {
		return 0, 33001, err
	}
	isWorldOpen := uint32(0)
	if runtime.MapID != 0 {
		isWorldOpen = 1
	}
	response := protobuf.SC_33001{
		IsWorldOpen: proto.Uint32(isWorldOpen),
		Camp:        proto.Uint32(runtime.Camp),
		CountInfo:   buildWorldCountInfo(runtime),
	}
	return client.SendMessage(33001, &response)
}
