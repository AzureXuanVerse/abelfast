package answer

import (
	"context"

	"github.com/jackc/pgx/v5"
	"google.golang.org/protobuf/proto"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/orm"
	"github.com/ggmolly/belfast/internal/protobuf"
)

const islandShipAttrLimitUnlockItemID = uint32(100000)

func HandleIslandShipAttrLimitUnlock(buffer *[]byte, client *connection.Client) (int, int, error) {
	var payload protobuf.CS_21603
	if err := proto.Unmarshal(*buffer, &payload); err != nil {
		return 0, 21604, err
	}

	response := &protobuf.SC_21604{Result: proto.Uint32(1)}
	if err := ensureCommanderLoaded(client, "Island/ShipAttrLimitUnlock"); err != nil {
		return client.SendMessage(21604, response)
	}

	shipID := payload.GetShipId()
	if shipID == 0 {
		return client.SendMessage(21604, response)
	}

	err := orm.WithPGXTx(context.Background(), func(tx pgx.Tx) error {
		ship, err := orm.GetIslandShipForUpdateTx(context.Background(), tx, client.Commander.CommanderID, shipID)
		if err != nil {
			return nil
		}
		if ship.UpLimitState != 0 {
			return nil
		}
		if err := orm.ConsumeIslandInventoryTx(context.Background(), tx, client.Commander.CommanderID, islandShipAttrLimitUnlockItemID, 1); err != nil {
			return err
		}
		ship.UpLimitState = 1
		if err := orm.UpsertIslandShipTx(context.Background(), tx, ship); err != nil {
			return err
		}
		response.Result = proto.Uint32(0)
		return nil
	})
	if err != nil {
		_ = client.Commander.Load()
	}

	return client.SendMessage(21604, response)
}
