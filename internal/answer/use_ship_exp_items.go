package answer

import (
	"context"

	"github.com/jackc/pgx/v5"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/orm"
	"github.com/ggmolly/belfast/internal/protobuf"
	"google.golang.org/protobuf/proto"
)

func UseAddShipExpItem(buffer *[]byte, client *connection.Client) (int, int, error) {
	var payload protobuf.CS_22011
	if err := proto.Unmarshal(*buffer, &payload); err != nil {
		return 0, 22012, err
	}

	response := protobuf.SC_22012{Result: proto.Uint32(1)}
	if client.Commander.OwnedShipsMap == nil || client.Commander.CommanderItemsMap == nil || client.Commander.MiscItemsMap == nil {
		if err := client.Commander.Load(); err != nil {
			return 0, 22012, err
		}
	}

	shipID := payload.GetShipId()
	if shipID == 0 {
		return client.SendMessage(22012, &response)
	}
	ownedShip, ok := client.Commander.OwnedShipsMap[shipID]
	if !ok {
		return client.SendMessage(22012, &response)
	}

	bookCounts, ok := normalizeShipExpBooks(payload.GetBooks())
	if !ok {
		return client.SendMessage(22012, &response)
	}
	bookSet, err := loadShipExpBookSet()
	if err != nil {
		return client.SendMessage(22012, &response)
	}

	totalExp := uint32(0)
	for itemID, count := range bookCounts {
		if _, exists := bookSet[itemID]; !exists {
			return client.SendMessage(22012, &response)
		}
		if !client.Commander.HasEnoughItem(itemID, count) {
			return client.SendMessage(22012, &response)
		}
		itemConfig, err := loadItemStatisticsConfig(itemID)
		if err != nil {
			return client.SendMessage(22012, &response)
		}
		expPerItem, err := parseUsageArgExpValue(itemConfig.UsageArg)
		if err != nil {
			return client.SendMessage(22012, &response)
		}
		totalExp += expPerItem * count
	}

	updatedShip := *ownedShip
	if err := applyOwnedShipExpGain(&updatedShip, totalExp); err != nil {
		return 0, 22012, err
	}
	if updatedShip.Level == ownedShip.Level && updatedShip.Exp == ownedShip.Exp && updatedShip.SurplusExp == ownedShip.SurplusExp {
		return client.SendMessage(22012, &response)
	}

	ctx := context.Background()
	err = orm.WithPGXTx(ctx, func(tx pgx.Tx) error {
		for itemID, count := range bookCounts {
			if err := client.Commander.ConsumeItemTx(ctx, tx, itemID, count); err != nil {
				return err
			}
		}
		_, err := tx.Exec(ctx, `
UPDATE owned_ships
SET level = $3, exp = $4, surplus_exp = $5
WHERE owner_id = $1 AND id = $2
`, int64(client.Commander.CommanderID), int64(ownedShip.ID), int64(updatedShip.Level), int64(updatedShip.Exp), int64(updatedShip.SurplusExp))
		return err
	})
	if err != nil {
		if isNotEnoughItemsError(err) {
			return client.SendMessage(22012, &response)
		}
		return 0, 22012, err
	}

	ownedShip.Level = updatedShip.Level
	ownedShip.Exp = updatedShip.Exp
	ownedShip.SurplusExp = updatedShip.SurplusExp
	response.Result = proto.Uint32(0)
	return client.SendMessage(22012, &response)
}

func normalizeShipExpBooks(books []*protobuf.ITEM_INFO) (map[uint32]uint32, bool) {
	if len(books) == 0 {
		return nil, false
	}
	merged := make(map[uint32]uint32, len(books))
	for _, entry := range books {
		if entry == nil {
			return nil, false
		}
		itemID := entry.GetId()
		count := entry.GetNum()
		if itemID == 0 || count == 0 {
			return nil, false
		}
		merged[itemID] += count
	}
	return merged, true
}

func isNotEnoughItemsError(err error) bool {
	if err == nil {
		return false
	}
	return err.Error() == "not enough items"
}
