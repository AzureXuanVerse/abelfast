package meta

import (
	"context"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/db"
	"github.com/ggmolly/belfast/internal/orm"
	"github.com/ggmolly/belfast/internal/protobuf"
	"github.com/jackc/pgx/v5"
	"google.golang.org/protobuf/encoding/protowire"
	"google.golang.org/protobuf/proto"
)

func MetaCharacterRepairLegacy(buffer *[]byte, client *connection.Client) (int, int, error) {
	shipID, repairIDs, err := parseMetaCharacterRepairLegacyRequest(*buffer)
	if err != nil {
		return 0, 70002, err
	}

	response := protobuf.SC_70002{Result: proto.Uint32(1)}
	if err := ensureCommanderMetaLoaded(client.Commander); err != nil {
		return 0, 70002, err
	}

	if shipID == 0 || len(repairIDs) == 0 {
		return client.SendMessage(70002, &response)
	}
	ship, ok := client.Commander.OwnedShipsMap[shipID]
	if !ok {
		return client.SendMessage(70002, &response)
	}

	metaID := ship.ShipID / 10
	metaCfg, err := orm.GetShipStrengthenMetaConfig(metaID)
	if err != nil {
		return client.SendMessage(70002, &response)
	}
	allowed := map[uint32][]uint32{
		1: metaCfg.RepairCannon,
		2: metaCfg.RepairTorpedo,
		3: metaCfg.RepairAir,
		4: metaCfg.RepairReload,
	}
	repairs, err := orm.ListOwnedShipMetaRepairIDs(client.Commander.CommanderID, ship.ID)
	if err != nil {
		return 0, 70002, err
	}
	consumed := make(map[uint32]struct{}, len(repairs)+len(repairIDs))
	for _, id := range repairs {
		consumed[id] = struct{}{}
	}

	requiredItems := make(map[uint32]uint32)
	for _, repairID := range repairIDs {
		if repairID == 0 {
			return client.SendMessage(70002, &response)
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
			return client.SendMessage(70002, &response)
		}

		repairCfg, err := orm.GetShipMetaRepairConfig(repairID)
		if err != nil || repairCfg.ItemID == 0 || repairCfg.ItemNum == 0 {
			return client.SendMessage(70002, &response)
		}
		requiredItems[repairCfg.ItemID] += repairCfg.ItemNum
		consumed[repairID] = struct{}{}
	}

	for itemID, itemNum := range requiredItems {
		if !client.Commander.HasEnoughItem(itemID, itemNum) {
			return client.SendMessage(70002, &response)
		}
	}

	ctx := context.Background()
	err = orm.WithPGXTx(ctx, func(tx pgx.Tx) error {
		for itemID, itemNum := range requiredItems {
			if err := client.Commander.ConsumeItemTx(ctx, tx, itemID, itemNum); err != nil {
				return err
			}
		}
		for _, repairID := range repairIDs {
			if err := orm.AddOwnedShipMetaRepairTx(ctx, tx, client.Commander.CommanderID, ship.ID, repairID); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return client.SendMessage(70002, &response)
	}

	response.Result = proto.Uint32(0)
	return client.SendMessage(70002, &response)
}

func parseMetaCharacterRepairLegacyRequest(payload []byte) (uint32, []uint32, error) {
	var shipID uint32
	repairIDs := make([]uint32, 0, 4)

	for len(payload) > 0 {
		num, wireType, n := protowire.ConsumeTag(payload)
		if n < 0 {
			return 0, nil, protowire.ParseError(n)
		}
		payload = payload[n:]

		switch num {
		case 1:
			if wireType != protowire.VarintType {
				return 0, nil, protowire.ParseError(-1)
			}
			v, m := protowire.ConsumeVarint(payload)
			if m < 0 {
				return 0, nil, protowire.ParseError(m)
			}
			shipID = uint32(v)
			payload = payload[m:]
		case 2:
			switch wireType {
			case protowire.VarintType:
				v, m := protowire.ConsumeVarint(payload)
				if m < 0 {
					return 0, nil, protowire.ParseError(m)
				}
				repairIDs = append(repairIDs, uint32(v))
				payload = payload[m:]
			case protowire.BytesType:
				packed, m := protowire.ConsumeBytes(payload)
				if m < 0 {
					return 0, nil, protowire.ParseError(m)
				}
				payload = payload[m:]
				for len(packed) > 0 {
					v, k := protowire.ConsumeVarint(packed)
					if k < 0 {
						return 0, nil, protowire.ParseError(k)
					}
					repairIDs = append(repairIDs, uint32(v))
					packed = packed[k:]
				}
			default:
				return 0, nil, protowire.ParseError(-1)
			}
		default:
			m := protowire.ConsumeFieldValue(num, wireType, payload)
			if m < 0 {
				return 0, nil, protowire.ParseError(m)
			}
			payload = payload[m:]
		}
	}

	return shipID, repairIDs, nil
}

func MetaCharActiveEnergyLegacy(buffer *[]byte, client *connection.Client) (int, int, error) {
	var payload protobuf.CS_70003
	if err := proto.Unmarshal(*buffer, &payload); err != nil {
		return 0, 70004, err
	}

	response := protobuf.SC_70004{Result: proto.Uint32(1)}
	if err := ensureCommanderMetaLoaded(client.Commander); err != nil {
		return 0, 70004, err
	}

	shipID := payload.GetId()
	ship, ok := client.Commander.OwnedShipsMap[shipID]
	if !ok {
		return client.SendMessage(70004, &response)
	}

	breakoutCfg, err := orm.GetShipMetaBreakoutConfig(ship.ShipID)
	if err != nil || breakoutCfg.BreakoutID == 0 {
		return client.SendMessage(70004, &response)
	}
	if ship.Level < breakoutCfg.Level {
		return client.SendMessage(70004, &response)
	}

	metaCfg, err := orm.GetShipStrengthenMetaConfig(ship.ShipID / 10)
	if err != nil {
		return client.SendMessage(70004, &response)
	}
	if breakoutCfg.Repair > 0 && metaCfg.RepairTotalExp > 0 {
		repairIDs, err := orm.ListOwnedShipMetaRepairIDs(client.Commander.CommanderID, ship.ID)
		if err != nil {
			return 0, 70004, err
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
			return client.SendMessage(70004, &response)
		}
	}

	if breakoutCfg.Gold > 0 && !client.Commander.HasEnoughGold(breakoutCfg.Gold) {
		return client.SendMessage(70004, &response)
	}
	if breakoutCfg.Item1 != 0 && breakoutCfg.Item1Num > 0 && !client.Commander.HasEnoughItem(breakoutCfg.Item1, breakoutCfg.Item1Num) {
		return client.SendMessage(70004, &response)
	}
	if breakoutCfg.Item2 != 0 && breakoutCfg.Item2Num > 0 && !client.Commander.HasEnoughItem(breakoutCfg.Item2, breakoutCfg.Item2Num) {
		return client.SendMessage(70004, &response)
	}

	nextTemplate, err := orm.GetShipTemplateConfig(breakoutCfg.BreakoutID)
	if err != nil {
		return client.SendMessage(70004, &response)
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
		return client.SendMessage(70004, &response)
	}

	ship.ShipID = breakoutCfg.BreakoutID
	ship.MaxLevel = nextTemplate.MaxLevel
	response.Result = proto.Uint32(0)
	return client.SendMessage(70004, &response)
}

func MetaCharacterUnlockShipLegacy(buffer *[]byte, client *connection.Client) (int, int, error) {
	var payload protobuf.CS_70005
	if err := proto.Unmarshal(*buffer, &payload); err != nil {
		return 0, 70006, err
	}

	response := protobuf.SC_70006{Result: proto.Uint32(1)}
	if err := ensureCommanderMetaLoaded(client.Commander); err != nil {
		return 0, 70006, err
	}

	metaCfg, err := orm.GetShipStrengthenMetaConfig(payload.GetId())
	if err != nil || metaCfg.Type != 1 || metaCfg.ShipID == 0 {
		return client.SendMessage(70006, &response)
	}
	if _, err := orm.GetShipTemplateConfig(metaCfg.ShipID); err != nil {
		return client.SendMessage(70006, &response)
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
		return client.SendMessage(70006, &response)
	}

	ship := client.Commander.OwnedShipsMap[ownedShipID]
	if ship == nil {
		if loadErr := client.Commander.Load(); loadErr != nil {
			return 0, 70006, loadErr
		}
		ship = client.Commander.OwnedShipsMap[ownedShipID]
	}
	if ship == nil {
		return client.SendMessage(70006, &response)
	}

	flags, err := orm.ListRandomFlagShipPhantoms(client.Commander.CommanderID, []uint32{ship.ID})
	if err != nil {
		return 0, 70006, err
	}
	shadows, err := orm.ListOwnedShipShadowSkins(client.Commander.CommanderID, []uint32{ship.ID})
	if err != nil {
		return 0, 70006, err
	}

	response.Result = proto.Uint32(0)
	response.Ship = orm.ToProtoOwnedShip(*ship, flags[ship.ID], shadows[ship.ID])
	return client.SendMessage(70006, &response)
}
