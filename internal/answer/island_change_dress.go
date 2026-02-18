package answer

import (
	"context"

	"github.com/jackc/pgx/v5"
	"google.golang.org/protobuf/proto"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/orm"
	"github.com/ggmolly/belfast/internal/protobuf"
)

func HandleIslandChangeDress(buffer *[]byte, client *connection.Client) (int, int, error) {
	var payload protobuf.CS_21617
	if err := proto.Unmarshal(*buffer, &payload); err != nil {
		return 0, 21618, err
	}

	response := &protobuf.SC_21618{Result: proto.Uint32(1)}
	if err := ensureCommanderLoaded(client, "Island/ChangeDress"); err != nil {
		return client.SendMessage(21618, response)
	}

	targetShipID := payload.GetShipId()
	if targetShipID == 0 {
		return client.SendMessage(21618, response)
	}

	err := orm.WithPGXTx(context.Background(), func(tx pgx.Tx) error {
		ship, err := orm.GetIslandShipForUpdateTx(context.Background(), tx, client.Commander.CommanderID, targetShipID)
		if err != nil {
			return nil
		}

		for _, dressID := range payload.GetUnloadDress() {
			if dressID == 0 {
				continue
			}
			removed, err := orm.RemoveIslandShipDressTx(context.Background(), tx, client.Commander.CommanderID, targetShipID, dressID)
			if err != nil {
				return err
			}
			if removed {
				if err := orm.AddIslandRoleDressNum(client.Commander.CommanderID, dressID, 1); err != nil {
					return err
				}
			}
		}

		for _, wear := range payload.GetDress_List() {
			dressID := wear.GetDressId()
			if dressID == 0 {
				return nil
			}
			sourceShipID := wear.GetShipId()
			if sourceShipID == 0 {
				state, err := orm.GetIslandRoleDressState(client.Commander.CommanderID, dressID)
				if err != nil {
					return nil
				}
				if state.Num == 0 {
					return nil
				}
				if err := orm.AddIslandRoleDressNum(client.Commander.CommanderID, dressID, -1); err != nil {
					return err
				}
			} else {
				removed, err := orm.RemoveIslandShipDressTx(context.Background(), tx, client.Commander.CommanderID, sourceShipID, dressID)
				if err != nil || !removed {
					return nil
				}
			}
			if err := orm.UpsertIslandShipDressTx(context.Background(), tx, client.Commander.CommanderID, targetShipID, dressID); err != nil {
				return err
			}
		}

		skinID := payload.GetSkinId()
		colorID := payload.GetColorId()
		if skinID != 0 {
			skinState, err := orm.GetIslandShipSkinState(client.Commander.CommanderID, targetShipID, skinID)
			if err != nil {
				return nil
			}
			if colorID != 0 {
				owned := false
				for i := range skinState.ColorList {
					if skinState.ColorList[i] == colorID {
						owned = true
						break
					}
				}
				if !owned {
					return nil
				}
				skinState.ColorID = colorID
				if err := orm.UpsertIslandShipSkinState(skinState); err != nil {
					return err
				}
			}
			ship.CurSkinID = skinID
		}

		if err := orm.UpsertIslandShipTx(context.Background(), tx, ship); err != nil {
			return err
		}
		response.Result = proto.Uint32(0)
		return nil
	})
	if err != nil {
		_ = client.Commander.Load()
	}

	return client.SendMessage(21618, response)
}
