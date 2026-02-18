package answer

import (
	"context"

	"github.com/jackc/pgx/v5"
	"google.golang.org/protobuf/proto"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/orm"
	"github.com/ggmolly/belfast/internal/protobuf"
)

func HandleIslandUseShipExpBook(buffer *[]byte, client *connection.Client) (int, int, error) {
	var payload protobuf.CS_21607
	if err := proto.Unmarshal(*buffer, &payload); err != nil {
		return 0, 21608, err
	}

	response := &protobuf.SC_21608{Result: proto.Uint32(1), AddExp: proto.Uint32(0)}
	if err := ensureCommanderLoaded(client, "Island/UseShipExpBook"); err != nil {
		return client.SendMessage(21608, response)
	}

	shipID := payload.GetShipId()
	if shipID == 0 || len(payload.GetItemList()) == 0 {
		return client.SendMessage(21608, response)
	}

	err := orm.WithPGXTx(context.Background(), func(tx pgx.Tx) error {
		ship, err := orm.GetIslandShipForUpdateTx(context.Background(), tx, client.Commander.CommanderID, shipID)
		if err != nil {
			return nil
		}
		_, found, err := loadIslandCharaTemplate(ship.ShipID)
		if err != nil || !found {
			return nil
		}

		totalExp := uint32(0)
		consumes := make(map[uint32]uint32)
		for i := range payload.GetItemList() {
			itemID := payload.GetItemList()[i].GetId()
			num := payload.GetItemList()[i].GetNum()
			if itemID == 0 || num == 0 {
				return nil
			}
			cfg, found, err := loadIslandItemTemplate(itemID)
			if err != nil || !found {
				return nil
			}
			if cfg.Usage != "usage_expbook" && cfg.Usage != "usage_ship_exp" {
				return nil
			}
			var useArgs []uint32
			if err := decodeUsageArg(cfg.UsageArg, &useArgs); err != nil || len(useArgs) == 0 {
				return nil
			}
			totalExp += useArgs[0] * num
			consumes[itemID] += num
		}
		if totalExp == 0 {
			return nil
		}

		maxLevel := ship.BreakLv * 10
		if maxLevel == 0 {
			maxLevel = 10
		}

		appliedExp := uint32(0)
		remaining := totalExp
		for remaining > 0 && ship.Level < maxLevel {
			lvlCfg, found, err := loadIslandCharaLevel(ship.Level)
			if err != nil || !found || lvlCfg.LevelUpExp == 0 {
				break
			}
			need := lvlCfg.LevelUpExp
			if ship.Exp+remaining < need {
				ship.Exp += remaining
				appliedExp += remaining
				remaining = 0
				break
			}
			used := need - ship.Exp
			ship.Level++
			ship.Exp = 0
			remaining -= used
			appliedExp += used
		}
		if ship.Level >= maxLevel {
			ship.Level = maxLevel
			ship.Exp = 0
		}
		if appliedExp == 0 {
			return nil
		}

		for itemID, count := range consumes {
			if err := orm.ConsumeIslandInventoryTx(context.Background(), tx, client.Commander.CommanderID, itemID, count); err != nil {
				return err
			}
		}
		if err := orm.UpsertIslandShipTx(context.Background(), tx, ship); err != nil {
			return err
		}

		response.Result = proto.Uint32(0)
		response.AddExp = proto.Uint32(appliedExp)
		return nil
	})
	if err != nil {
		_ = client.Commander.Load()
	}

	return client.SendMessage(21608, response)
}
