package answer

import (
	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/protobuf"
	"google.golang.org/protobuf/proto"
)

func ShipyardData(buffer *[]byte, client *connection.Client) (int, int, error) {
	state, err := getShipyardStateOrDefault(client.Commander.CommanderID)
	if err != nil {
		return 0, 63100, err
	}
	blueprints, err := listShipyardBlueprintProto(client.Commander.CommanderID)
	if err != nil {
		return 0, 63100, err
	}
	if client.Commander.OwnedShipsMap == nil {
		if err := client.Commander.Load(); err != nil {
			return 0, 63100, err
		}
	}
	response := protobuf.SC_63100{
		ColdTime:                 proto.Uint32(state.ColdTime),
		DailyCatchupStrengthen:   proto.Uint32(state.DailyCatchupStrengthen),
		DailyCatchupStrengthenUr: proto.Uint32(state.DailyCatchupStrengthenUR),
		BlueprintList:            blueprints,
	}
	return client.SendMessage(63100, &response)
}
