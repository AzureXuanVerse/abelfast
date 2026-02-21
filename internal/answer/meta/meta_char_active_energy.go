package meta

import (
	"context"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/orm"
	"github.com/ggmolly/belfast/internal/protobuf"
	"github.com/jackc/pgx/v5"
	"google.golang.org/protobuf/proto"
)

func MetaCharActiveEnergy(buffer *[]byte, client *connection.Client) (int, int, error) {
	var payload protobuf.CS_63303
	if err := proto.Unmarshal(*buffer, &payload); err != nil {
		return 0, 63304, err
	}

	response := protobuf.SC_63304{Result: proto.Uint32(1)}
	if err := ensureCommanderMetaLoaded(client.Commander); err != nil {
		return 0, 63304, err
	}

	shipID := payload.GetShipId()
	ship, ok := client.Commander.OwnedShipsMap[shipID]
	if !ok {
		return client.SendMessage(63304, &response)
	}

	breakoutCfg, err := orm.GetShipMetaBreakoutConfig(ship.ShipID)
	if err != nil || breakoutCfg.BreakoutID == 0 {
		return client.SendMessage(63304, &response)
	}
	if ship.Level < breakoutCfg.Level {
		return client.SendMessage(63304, &response)
	}

	metaCfg, err := orm.GetShipStrengthenMetaConfig(ship.ShipID / 10)
	if err != nil {
		return client.SendMessage(63304, &response)
	}
	if breakoutCfg.Repair > 0 && metaCfg.RepairTotalExp > 0 {
		repairIDs, err := orm.ListOwnedShipMetaRepairIDs(client.Commander.CommanderID, ship.ID)
		if err != nil {
			return 0, 63304, err
		}
		totalRepairExp := uint32(0)
		for _, repairID := range repairIDs {
			repairCfg, cfgErr := orm.GetShipMetaRepairConfig(repairID)
			if cfgErr != nil {
				continue
			}
			totalRepairExp += repairCfg.RepairExp
		}
		repairPercent := uint32(0)
		if metaCfg.RepairTotalExp > 0 {
			repairPercent = totalRepairExp * 100 / metaCfg.RepairTotalExp
		}
		if repairPercent < breakoutCfg.Repair {
			return client.SendMessage(63304, &response)
		}
	}

	if breakoutCfg.Gold > 0 && !client.Commander.HasEnoughGold(breakoutCfg.Gold) {
		return client.SendMessage(63304, &response)
	}
	if breakoutCfg.Item1 != 0 && breakoutCfg.Item1Num > 0 && !client.Commander.HasEnoughItem(breakoutCfg.Item1, breakoutCfg.Item1Num) {
		return client.SendMessage(63304, &response)
	}
	if breakoutCfg.Item2 != 0 && breakoutCfg.Item2Num > 0 && !client.Commander.HasEnoughItem(breakoutCfg.Item2, breakoutCfg.Item2Num) {
		return client.SendMessage(63304, &response)
	}

	nextTemplate, err := orm.GetShipTemplateConfig(breakoutCfg.BreakoutID)
	if err != nil {
		return client.SendMessage(63304, &response)
	}

	ctx := context.Background()
	err = orm.WithPGXTx(ctx, func(tx pgx.Tx) error {
		if breakoutCfg.Gold > 0 {
			if err := client.Commander.ConsumeResourceTx(ctx, tx, 1, breakoutCfg.Gold); err != nil {
				return err
			}
		}
		if breakoutCfg.Item1 != 0 && breakoutCfg.Item1Num > 0 {
			if err := client.Commander.ConsumeItemTx(ctx, tx, breakoutCfg.Item1, breakoutCfg.Item1Num); err != nil {
				return err
			}
		}
		if breakoutCfg.Item2 != 0 && breakoutCfg.Item2Num > 0 {
			if err := client.Commander.ConsumeItemTx(ctx, tx, breakoutCfg.Item2, breakoutCfg.Item2Num); err != nil {
				return err
			}
		}
		_, err := tx.Exec(ctx, `
UPDATE owned_ships
SET ship_id = $3, max_level = $4
WHERE owner_id = $1 AND id = $2
`, int64(client.Commander.CommanderID), int64(ship.ID), int64(breakoutCfg.BreakoutID), int64(nextTemplate.MaxLevel))
		return err
	})
	if err != nil {
		return client.SendMessage(63304, &response)
	}

	ship.ShipID = breakoutCfg.BreakoutID
	ship.MaxLevel = nextTemplate.MaxLevel
	response.Result = proto.Uint32(0)
	return client.SendMessage(63304, &response)
}
