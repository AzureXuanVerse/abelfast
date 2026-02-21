package answer

import (
	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/orm"
	"github.com/ggmolly/belfast/internal/protobuf"
	"google.golang.org/protobuf/proto"
)

var fleetRenameApply = func(fleet *orm.Fleet, name string) error {
	return fleet.RenameFleet(name)
}

func FleetRename(buffer *[]byte, client *connection.Client) (int, int, error) {
	var payload protobuf.CS_12104
	if err := proto.Unmarshal(*buffer, &payload); err != nil {
		return 0, 12104, err
	}
	response := protobuf.SC_12105{
		Result: proto.Uint32(0),
	}

	// Check if the commander has this fleet, if the fleet exists, rename it
	fleet, ok := client.Commander.FleetsMap[payload.GetId()]
	if !ok {
		response.Result = proto.Uint32(1)
	} else {
		if err := fleetRenameApply(fleet, payload.GetName()); err != nil {
			response.Result = proto.Uint32(2)
		}
	}

	if _, _, err := client.SendMessage(12105, &response); err != nil {
		return 0, 12105, err
	}
	if response.GetResult() != 0 {
		return 0, 12105, nil
	}

	if err := pushFleetSync(client, fleet); err != nil {
		return 0, 12106, err
	}

	return 0, 12105, nil
}
