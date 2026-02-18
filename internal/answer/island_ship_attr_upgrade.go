package answer

import (
	"context"

	"github.com/jackc/pgx/v5"
	"google.golang.org/protobuf/proto"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/orm"
	"github.com/ggmolly/belfast/internal/protobuf"
)

func HandleIslandShipAttrUpgrade(buffer *[]byte, client *connection.Client) (int, int, error) {
	var payload protobuf.CS_21605
	if err := proto.Unmarshal(*buffer, &payload); err != nil {
		return 0, 21606, err
	}

	response := &protobuf.SC_21606{Result: proto.Uint32(1)}
	if err := ensureCommanderLoaded(client, "Island/ShipAttrUpgrade"); err != nil {
		return client.SendMessage(21606, response)
	}

	shipID := payload.GetShipId()
	attrType := payload.GetType()
	if shipID == 0 || attrType == 0 || len(payload.GetItemList()) == 0 {
		return client.SendMessage(21606, response)
	}

	err := orm.WithPGXTx(context.Background(), func(tx pgx.Tx) error {
		ship, err := orm.GetIslandShipForUpdateTx(context.Background(), tx, client.Commander.CommanderID, shipID)
		if err != nil {
			return nil
		}
		template, found, err := loadIslandCharaTemplate(ship.ShipID)
		if err != nil || !found {
			return nil
		}
		allowedItems := extractAttrItemMap(template.AttItem)
		allowedForType, ok := allowedItems[attrType]
		if !ok || len(allowedForType) == 0 {
			return nil
		}

		extraCaps := extractExtraMax(template.ExtraMax)
		capPair, ok := extraCaps[attrType]
		if !ok {
			return nil
		}
		cap := capPair[0]
		if ship.UpLimitState != 0 {
			cap = capPair[1]
		}

		addValue := uint32(0)
		consumes := make(map[uint32]uint32)
		for i := range payload.GetItemList() {
			itemID := payload.GetItemList()[i].GetId()
			num := payload.GetItemList()[i].GetNum()
			if itemID == 0 || num == 0 {
				return nil
			}
			if _, allowed := allowedForType[itemID]; !allowed {
				return nil
			}
			cfg, found, err := loadIslandItemTemplate(itemID)
			if err != nil || !found {
				return nil
			}
			value := uint32(0)
			var one uint32
			if err := decodeUsageArg(cfg.UsageArg, &one); err == nil {
				value = one
			} else {
				var list []uint32
				if err := decodeUsageArg(cfg.UsageArg, &list); err == nil && len(list) > 0 {
					value = list[0]
				}
			}
			if value == 0 {
				return nil
			}
			addValue += value * num
			consumes[itemID] += num
		}

		currentValue := uint32(0)
		idx := -1
		for i := range ship.ExtraAttrs {
			if ship.ExtraAttrs[i].ID != attrType {
				continue
			}
			idx = i
			currentValue = ship.ExtraAttrs[i].Value
			break
		}
		if currentValue >= cap || addValue == 0 {
			return nil
		}
		if currentValue+addValue > cap {
			addValue = cap - currentValue
		}
		if addValue == 0 {
			return nil
		}

		for itemID, count := range consumes {
			if err := orm.ConsumeIslandInventoryTx(context.Background(), tx, client.Commander.CommanderID, itemID, count); err != nil {
				return err
			}
		}

		if idx >= 0 {
			ship.ExtraAttrs[idx].Value = currentValue + addValue
		} else {
			ship.ExtraAttrs = append(ship.ExtraAttrs, orm.IslandShipAttr{ID: attrType, Value: addValue})
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

	return client.SendMessage(21606, response)
}
