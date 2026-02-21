package answer

import (
	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/orm"
	"github.com/ggmolly/belfast/internal/protobuf"
)

func pushNewShips(client *connection.Client, ships []*orm.OwnedShip) (bool, error) {
	if len(ships) == 0 {
		return false, nil
	}

	ownedShips := make([]orm.OwnedShip, 0, len(ships))
	for _, ship := range ships {
		if ship == nil {
			continue
		}
		ownedShips = append(ownedShips, *ship)
	}
	if len(ownedShips) == 0 {
		return false, nil
	}

	push := protobuf.SC_12042{ShipList: orm.ToProtoOwnedShipList(ownedShips, nil, nil)}
	if _, _, err := client.SendMessage(12042, &push); err != nil {
		return false, err
	}

	return true, nil
}
