package answer

import (
	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/orm"
	"github.com/ggmolly/belfast/internal/protobuf"
	"google.golang.org/protobuf/proto"
)

var setFavoriteShipPreference = func(ship *orm.OwnedShip, flag uint32) error {
	return ship.SetFavorite(flag)
}

func SetFavoriteShip(buffer *[]byte, client *connection.Client) (int, int, error) {
	var data protobuf.CS_12040
	if err := proto.Unmarshal(*buffer, &data); err != nil {
		return 0, 12041, err
	}
	response := protobuf.SC_12041{
		Result: proto.Uint32(0),
	}

	// Check if the ship is in the dock
	if ship, ok := client.Commander.OwnedShipsMap[data.GetShipId()]; ok {
		if err := setFavoriteShipPreference(ship, data.GetFlag()); err != nil {
			response.Result = proto.Uint32(1)
		}
	} else {
		response.Result = proto.Uint32(1)
	}

	return client.SendMessage(12041, &response)
}
