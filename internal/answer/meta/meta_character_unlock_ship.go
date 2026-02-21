package meta

import (
	"context"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/db"
	"github.com/ggmolly/belfast/internal/orm"
	"github.com/ggmolly/belfast/internal/protobuf"
	"github.com/jackc/pgx/v5"
	"google.golang.org/protobuf/proto"
)

func MetaCharacterUnlockShip(buffer *[]byte, client *connection.Client) (int, int, error) {
	var payload protobuf.CS_63305
	if err := proto.Unmarshal(*buffer, &payload); err != nil {
		return 0, 63306, err
	}

	response := protobuf.SC_63306{Result: proto.Uint32(1)}
	if err := ensureCommanderMetaLoaded(client.Commander); err != nil {
		return 0, 63306, err
	}

	metaCfg, err := orm.GetShipStrengthenMetaConfig(payload.GetMetaId())
	if err != nil || metaCfg.Type != 1 || metaCfg.ShipID == 0 {
		return client.SendMessage(63306, &response)
	}
	if _, err := orm.GetShipTemplateConfig(metaCfg.ShipID); err != nil {
		return client.SendMessage(63306, &response)
	}

	ctx := context.Background()
	var ownedShipID uint32
	err = orm.WithPGXTx(ctx, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `SELECT commander_id FROM commanders WHERE commander_id = $1 FOR UPDATE`, int64(client.Commander.CommanderID)); err != nil {
			return err
		}
		row := tx.QueryRow(ctx, `
SELECT id
FROM owned_ships
WHERE owner_id = $1 AND ship_id = $2 AND deleted_at IS NULL
ORDER BY id ASC
LIMIT 1
`, int64(client.Commander.CommanderID), int64(metaCfg.ShipID))
		var existingID uint32
		if err := row.Scan(&existingID); err == nil {
			ownedShipID = existingID
			return nil
		} else if !db.IsNotFound(db.MapNotFound(err)) {
			return err
		}
		newShip, err := client.Commander.AddShipTx(ctx, tx, metaCfg.ShipID)
		if err != nil {
			return err
		}
		ownedShipID = newShip.ID
		return nil
	})
	if err != nil {
		return client.SendMessage(63306, &response)
	}

	ship := client.Commander.OwnedShipsMap[ownedShipID]
	if ship == nil {
		if loadErr := client.Commander.Load(); loadErr != nil {
			return 0, 63306, loadErr
		}
		ship = client.Commander.OwnedShipsMap[ownedShipID]
	}
	if ship == nil {
		return client.SendMessage(63306, &response)
	}

	flags, err := orm.ListRandomFlagShipPhantoms(client.Commander.CommanderID, []uint32{ship.ID})
	if err != nil {
		return 0, 63306, err
	}
	shadows, err := orm.ListOwnedShipShadowSkins(client.Commander.CommanderID, []uint32{ship.ID})
	if err != nil {
		return 0, 63306, err
	}

	response.Result = proto.Uint32(0)
	response.Ship = orm.ToProtoOwnedShip(*ship, flags[ship.ID], shadows[ship.ID])
	return client.SendMessage(63306, &response)
}
