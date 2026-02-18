package answer

import (
	"context"

	"github.com/jackc/pgx/v5"
	"google.golang.org/protobuf/proto"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/orm"
	"github.com/ggmolly/belfast/internal/protobuf"
)

func IslandBuyRoleSkinColor(buffer *[]byte, client *connection.Client) (int, int, error) {
	var payload protobuf.CS_21619
	if err := proto.Unmarshal(*buffer, &payload); err != nil {
		return 0, 21620, err
	}

	response := &protobuf.SC_21620{Result: proto.Uint32(1)}
	if err := ensureCommanderLoaded(client, "Island/BuyRoleSkinColor"); err != nil {
		return client.SendMessage(21620, response)
	}

	shipID := payload.GetShipId()
	colorID := payload.GetColorId()
	if shipID == 0 || colorID == 0 {
		return client.SendMessage(21620, response)
	}

	err := orm.WithPGXTx(context.Background(), func(tx pgx.Tx) error {
		ship, err := orm.GetIslandShipForUpdateTx(context.Background(), tx, client.Commander.CommanderID, shipID)
		if err != nil {
			return nil
		}
		colorCfg, found, err := loadIslandSkinColorConfig(colorID)
		if err != nil || !found {
			return nil
		}

		skinID := ship.CurSkinID
		if skinID == 0 {
			skinID = ship.ShipID
		}
		skinCfg, found, err := loadIslandSkinConfig(skinID)
		if err != nil || !found {
			return nil
		}
		if skinCfg.ShipGroup != colorCfg.SkinGroup {
			return nil
		}

		skinState, err := orm.GetIslandShipSkinState(client.Commander.CommanderID, shipID, skinID)
		if err != nil {
			skinState = &orm.IslandShipSkinState{CommanderID: client.Commander.CommanderID, ShipID: shipID, SkinID: skinID, ColorList: []uint32{}}
		}
		for i := range skinState.ColorList {
			if skinState.ColorList[i] == colorID {
				return nil
			}
		}
		for i := range colorCfg.Cost {
			if len(colorCfg.Cost[i]) < 3 {
				continue
			}
			if colorCfg.Cost[i][0] == 41 {
				if err := orm.ConsumeIslandInventoryTx(context.Background(), tx, client.Commander.CommanderID, colorCfg.Cost[i][1], colorCfg.Cost[i][2]); err != nil {
					return err
				}
			}
		}
		skinState.ColorList = append(skinState.ColorList, colorID)
		skinState.ColorID = colorID
		if err := orm.UpsertIslandShipSkinState(skinState); err != nil {
			return err
		}

		response.Result = proto.Uint32(0)
		return nil
	})
	if err != nil {
		_ = client.Commander.Load()
	}

	return client.SendMessage(21620, response)
}
