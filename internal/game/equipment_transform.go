package game

import (
	"context"
	"slices"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/db"
	"github.com/ggmolly/belfast/internal/protobuf"
)

// itemRevertEquipment is the item a revert costs (Item.REVERT_EQUIPMENT_ID).
const itemRevertEquipment = 15007

// equipTransform is an equip_upgrade_data row: a +0 piece of gear turned
// into another one (the equipment upgrade path).
type equipTransform struct {
	From  uint32      `json:"upgrade_from"`
	To    uint32      `json:"target_id"`
	Gold  uint32      `json:"coin_consume"`
	Items [][2]uint32 `json:"material_consume"` // [item, count]
}

var equipTransforms map[uint32]equipTransform

// equipRoot is the +0 a piece of gear was upgraded from
// (Equipment.GetEquipRootStatic).
func equipRoot(equipID uint32) uint32 {
	for equipTemplates[equipID].Prev != 0 {
		equipID = equipTemplates[equipID].Prev
	}
	return equipID
}

// equipRevertReturns is what the levels of equipID paid for its upgrades
// (Equipment.GetRevertRewardsStatic, getRevertAwards): the trans_use_gold
// and trans_use_item of every level below it.
func equipRevertReturns(equipID uint32) (gold uint32, items map[uint32]uint32) {
	items = map[uint32]uint32{}
	for prev := equipTemplates[equipID].Prev; prev != 0; prev = equipTemplates[prev].Prev {
		gold += equipTemplates[prev].equipUpgrade.Gold
		for _, item := range equipTemplates[prev].equipUpgrade.Items {
			items[item[0]] += item[1]
		}
	}
	return gold, items
}

// fitsSlot tells whether equipID can be in slot pos of a ship of
// templateID (Ship:isForbiddenAtPos): its type is one the slot takes
// and its ship_type_forbidden doesn't list the ship's type. The type and
// the forbidden list are on the +0's row.
func fitsSlot(templateID, equipID, pos uint32) bool {
	ship, ok := ownableShips[templateID]
	equip, known := equipTemplates[equipID]
	if !ok || !known {
		return false
	}
	if equip.Base != 0 {
		equip = equipTemplates[equip.Base]
	}
	return slices.Contains(ship.slotTypes(pos), equip.Type) && !slices.Contains(equip.ShipTypeForbidden, ship.Type)
}

// transformEquipment turns one piece of equipID into upgradeID's target
// (TransformEquipmentCommand:ExecuteEquipTransform), refused when its +0
// isn't the formula's upgrade_from. As the client counts it, it pays
// coin_consume and material_consume and gets back what the piece's
// upgrades cost in items (not their gold) and its destory_gold.
func (tx *Tx) transformEquipment(equipID, upgradeID uint32) (uint32, error) {
	upgrade, ok := equipTransforms[upgradeID]
	if _, known := equipTemplates[equipID]; !ok || !known || equipRoot(equipID) != upgrade.From {
		return 0, errRefused
	}
	if err := tx.SpendResource(ResourceGold, upgrade.Gold); err != nil {
		return 0, err
	}
	for _, item := range upgrade.Items {
		if err := tx.SpendItem(item[0], item[1]); err != nil {
			return 0, err
		}
	}
	_, items := equipRevertReturns(equipID)
	for item, count := range items {
		if err := tx.AddItem(item, count); err != nil {
			return 0, err
		}
	}
	if err := tx.AddResource(ResourceGold, equipReturns[equipID].Gold); err != nil {
		return 0, err
	}
	return upgrade.To, nil
}

// handleRevertEquipment reverts a piece of depot gear to its +0 for a
// revert item (RevertEquipmentCommand, CS_14010; the info layer offers it
// for gear above +0 when the bag holds one). On result 0 the client swaps
// the piece for its root, takes the item and adds getRevertAwards, the
// upgrades' trans_use_gold and trans_use_item.
func (game *Game) handleRevertEquipment(client *connection.Client, req *protobuf.CS_14010) error {
	ctx := context.Background()
	equipID := req.GetEquipId()
	err := errRefused
	if equip, ok := equipTemplates[equipID]; ok && equip.Prev != 0 {
		err = game.tx(ctx, client, func(tx *Tx) error {
			if err := tx.SpendItem(itemRevertEquipment, 1); err != nil {
				return err
			}
			if err := tx.SpendEquipment(equipID, 1); err != nil {
				return err
			}
			if err := tx.AddEquipment(equipRoot(equipID), 1); err != nil {
				return err
			}
			gold, items := equipRevertReturns(equipID)
			if err := tx.AddResource(ResourceGold, gold); err != nil {
				return err
			}
			for item, count := range items {
				if err := tx.AddItem(item, count); err != nil {
					return err
				}
			}
			return nil
		})
	}
	if err != nil && !refused(err) {
		return err
	}
	return client.Send(&protobuf.SC_14011{Result: result(err)})
}

// handleTransformShipEquipment takes the gear in a ship slot one step down
// its upgrade path (TransformEquipmentCommand, CS_14013, one request per
// formula). On result 0 the client puts the new +0 in the slot, or in the
// depot when the slot can't take it (Ship:isForbiddenAtPos).
func (game *Game) handleTransformShipEquipment(client *connection.Client, req *protobuf.CS_14013) error {
	ctx := context.Background()
	err := game.tx(ctx, client, func(tx *Tx) error {
		equips, err := shipEquipments(ctx, tx.Queries, client.CommanderID, req.GetShipId())
		if err != nil {
			return err
		}
		ship, err := tx.Queries.GetShip(ctx, db.GetShipParams{ID: req.GetShipId(), CommanderID: client.CommanderID})
		if err != nil {
			return err
		}
		pos := req.GetPos()
		if pos < 1 || int(pos) > len(equips) || equips[pos-1].EquipID == 0 {
			return errRefused
		}
		equip := equips[pos-1]
		next, err := tx.transformEquipment(equip.EquipID, req.GetUpgradeId())
		if err != nil {
			return err
		}
		equip.EquipID = next
		if !fitsSlot(ship.TemplateID, next, pos) {
			if err := tx.AddEquipment(next, 1); err != nil {
				return err
			}
			equip.EquipID = 0
		}
		return tx.SetShipEquipment(ctx, db.SetShipEquipmentParams{ShipID: equip.ShipID, Pos: pos, EquipID: equip.EquipID, SkinID: equip.SkinID})
	})
	if err != nil && !refused(err) {
		return err
	}
	return client.Send(&protobuf.SC_14014{Result: result(err)})
}

// handleTransformEquipment takes one piece of depot gear one step down its
// upgrade path (TransformEquipmentCommand, CS_14015, one request per
// formula, after a CS_14006 when it starts from a blueprint). On result 0
// the client removes one equip_id from the depot and adds one of the
// formula's target_id.
func (game *Game) handleTransformEquipment(client *connection.Client, req *protobuf.CS_14015) error {
	ctx := context.Background()
	err := game.tx(ctx, client, func(tx *Tx) error {
		if err := tx.SpendEquipment(req.GetEquipId(), 1); err != nil {
			return err
		}
		next, err := tx.transformEquipment(req.GetEquipId(), req.GetUpgradeId())
		if err != nil {
			return err
		}
		return tx.AddEquipment(next, 1)
	})
	if err != nil && !refused(err) {
		return err
	}
	return client.Send(&protobuf.SC_14016{Result: result(err)})
}
