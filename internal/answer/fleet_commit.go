package answer

import (
	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/orm"
	"github.com/ggmolly/belfast/internal/protobuf"
	"google.golang.org/protobuf/proto"
)

var fleetCommitCreateFleet = orm.CreateFleet

var fleetCommitUpdateShipList = func(fleet *orm.Fleet, commander *orm.Commander, shipList []uint32) error {
	return fleet.UpdateShipList(commander, shipList)
}

func FleetCommit(buffer *[]byte, client *connection.Client) (int, int, error) {
	var payload protobuf.CS_12102

	response := protobuf.SC_12103{
		Result: proto.Uint32(0),
	}

	if err := proto.Unmarshal(*buffer, &payload); err != nil {
		return 0, 12103, err
	}
	fleet, ok := client.Commander.FleetsMap[payload.GetId()]
	if !ok {
		// Create the fleet
		if err := fleetCommitCreateFleet(client.Commander, payload.GetId(), "", payload.ShipList); err != nil {
			response.Result = proto.Uint32(1)
		}
	} else {
		// Update the fleet
		if err := fleetCommitUpdateShipList(fleet, client.Commander, payload.ShipList); err != nil {
			response.Result = proto.Uint32(1)
		}
	}

	if _, _, err := client.SendMessage(12103, &response); err != nil {
		return 0, 12103, err
	}
	if response.GetResult() != 0 {
		return 0, 12103, nil
	}

	updatedFleet := client.Commander.FleetsMap[payload.GetId()]
	if err := pushFleetSync(client, updatedFleet); err != nil {
		return 0, 12106, err
	}

	return 0, 12103, nil
}
