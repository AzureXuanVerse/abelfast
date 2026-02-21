package answer

import (
	"context"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/orm"
	"github.com/ggmolly/belfast/internal/protobuf"
	"github.com/jackc/pgx/v5"
	"google.golang.org/protobuf/proto"
)

func MetaCharacterRepair(buffer *[]byte, client *connection.Client) (int, int, error) {
	var payload protobuf.CS_63301
	if err := proto.Unmarshal(*buffer, &payload); err != nil {
		return 0, 63302, err
	}

	response := protobuf.SC_63302{Result: proto.Uint32(1)}
	if err := ensureCommanderMetaLoaded(client.Commander); err != nil {
		return 0, 63302, err
	}

	shipID := payload.GetShipId()
	repairID := payload.GetRepairId()
	if shipID == 0 || repairID == 0 {
		return client.SendMessage(63302, &response)
	}
	ship, ok := client.Commander.OwnedShipsMap[shipID]
	if !ok {
		return client.SendMessage(63302, &response)
	}

	metaID := ship.ShipID / 10
	metaCfg, err := orm.GetShipStrengthenMetaConfig(metaID)
	if err != nil {
		return client.SendMessage(63302, &response)
	}
	allowed := map[uint32][]uint32{
		1: metaCfg.RepairCannon,
		2: metaCfg.RepairTorpedo,
		3: metaCfg.RepairAir,
		4: metaCfg.RepairReload,
	}
	repairs, err := orm.ListOwnedShipMetaRepairIDs(client.Commander.CommanderID, ship.ID)
	if err != nil {
		return 0, 63302, err
	}
	consumed := make(map[uint32]struct{}, len(repairs))
	for _, id := range repairs {
		consumed[id] = struct{}{}
	}

	validStep := false
	for _, chain := range allowed {
		if len(chain) == 0 {
			continue
		}
		next := uint32(0)
		for _, id := range chain {
			if _, ok := consumed[id]; !ok {
				next = id
				break
			}
		}
		if next == repairID {
			validStep = true
			break
		}
	}
	if !validStep {
		return client.SendMessage(63302, &response)
	}

	repairCfg, err := orm.GetShipMetaRepairConfig(repairID)
	if err != nil || repairCfg.ItemID == 0 || repairCfg.ItemNum == 0 {
		return client.SendMessage(63302, &response)
	}
	if !client.Commander.HasEnoughItem(repairCfg.ItemID, repairCfg.ItemNum) {
		return client.SendMessage(63302, &response)
	}

	ctx := context.Background()
	err = orm.WithPGXTx(ctx, func(tx pgx.Tx) error {
		if err := client.Commander.ConsumeItemTx(ctx, tx, repairCfg.ItemID, repairCfg.ItemNum); err != nil {
			return err
		}
		return orm.AddOwnedShipMetaRepairTx(ctx, tx, client.Commander.CommanderID, ship.ID, repairID)
	})
	if err != nil {
		return client.SendMessage(63302, &response)
	}

	response.Result = proto.Uint32(0)
	return client.SendMessage(63302, &response)
}
