package answer

import (
	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/orm"
	"google.golang.org/protobuf/proto"

	"github.com/ggmolly/belfast/internal/protobuf"
)

func CommanderFleet(buffer *[]byte, client *connection.Client) (int, int, error) {
	var response protobuf.SC_12101

	for i := range client.Commander.Fleets {
		response.GroupList = append(response.GroupList, commanderFleetGroupInfo(&client.Commander.Fleets[i]))
	}

	return client.SendMessage(12101, &response)
}

func commanderFleetGroupInfo(fleet *orm.Fleet) *protobuf.GROUPINFO_P12 {
	shipList := make([]uint32, len(fleet.ShipList))
	for i, ship := range fleet.ShipList {
		shipList[i] = uint32(ship)
	}

	return &protobuf.GROUPINFO_P12{
		Id:         proto.Uint32(fleet.GameID),
		Name:       proto.String(fleet.Name),
		ShipList:   shipList,
		Commanders: []*protobuf.COMMANDERSINFO{},
	}
}
