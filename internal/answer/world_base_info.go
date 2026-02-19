package answer

import (
	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/orm"

	"github.com/ggmolly/belfast/internal/protobuf"
	"google.golang.org/protobuf/proto"
)

func WorldBaseInfo(buffer *[]byte, client *connection.Client) (int, int, error) {
	runtime, err := orm.LoadOrCreateWorldRuntime(client.Commander.CommanderID)
	if err != nil {
		return 0, 33114, err
	}
	isWorldOpen := uint32(0)
	if runtime.MapID != 0 {
		isWorldOpen = 1
	}
	var response protobuf.SC_33114
	response.IsWorldOpen = proto.Uint32(isWorldOpen)
	response.Progress = proto.Uint32(runtime.Progress)
	response.ShipIdList = append([]uint32{}, runtime.FleetShipIDs...)
	response.CmdIdList = append([]uint32{}, runtime.CommanderIDs...)
	return client.SendMessage(33114, &response)
}
