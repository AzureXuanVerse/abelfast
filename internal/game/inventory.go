package game

import (
	"context"

	"google.golang.org/protobuf/proto"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/db"
	"github.com/ggmolly/belfast/internal/protobuf"
)

// player_resource ids
const (
	ResourceGold    = 1
	ResourceOil     = 2
	ResourceGem     = 4  // bought gems
	ResourceFreeGem = 14 // gems earned in game
)

// item_data_statistics ids
const (
	ItemWisdomCube    = 20001
	ItemQuickFinisher = 15003 // ITEM_ID_EQUIP_QUICK_FINISH
	ItemOxyCola       = 50001
	ItemSecretCoolant = 50002
)

// spweaponBagSize is SC_14001.spweapon_bag_size as captured for a new
// commander.
const spweaponBagSize = 150

// AddResource gives the commander amount of a player_resource.
func (tx *Tx) AddResource(resourceID, amount uint32) error {
	return tx.Queries.AddResource(tx.ctx, db.AddResourceParams{CommanderID: tx.commanderID, ResourceID: resourceID, Amount: amount})
}

// SpendResource takes amount of a player_resource, ErrNotEnough if the
// commander has less. Oil spent counts for TaskSubSpendOil.
func (tx *Tx) SpendResource(resourceID, amount uint32) error {
	if amount == 0 {
		return nil
	}
	n, err := tx.Queries.SpendResource(tx.ctx, db.SpendResourceParams{CommanderID: tx.commanderID, ResourceID: resourceID, Amount: amount})
	if err == nil && n == 0 {
		err = ErrNotEnough
	}
	if err == nil && resourceID == ResourceOil {
		tx.event(TaskEvent{SubType: TaskSubSpendOil, Count: amount})
	}
	return err
}

// SpendShopResource pays amount of a price's player_resource. Gems, whichever
// of the two the price names, come out of the free ones first, then the
// bought ones, as Player:consume does (model/vo/player.lua).
func (tx *Tx) SpendShopResource(resourceID, amount uint32) error {
	if resourceID != ResourceGem && resourceID != ResourceFreeGem {
		return tx.SpendResource(resourceID, amount)
	}
	rows, err := tx.ListResources(tx.ctx, tx.commanderID)
	if err != nil {
		return err
	}
	free := uint32(0)
	for _, row := range rows {
		if row.ResourceID == ResourceFreeGem {
			free = row.Amount
		}
	}
	fromFree := min(amount, free)
	if err := tx.SpendResource(ResourceFreeGem, fromFree); err != nil {
		return err
	}
	return tx.SpendResource(ResourceGem, amount-fromFree)
}

// AddItem puts count of an item in the commander's bag, which counts for
// TaskSubObtainItem.
func (tx *Tx) AddItem(itemID, count uint32) error {
	if err := tx.Queries.AddItem(tx.ctx, db.AddItemParams{CommanderID: tx.commanderID, ItemID: itemID, Count: count}); err != nil {
		return err
	}
	tx.event(TaskEvent{SubType: TaskSubObtainItem, Target: itemID, Count: count})
	return nil
}

// SpendItem takes count of an item from the bag, ErrNotEnough if there are
// fewer. It counts for TaskSubUseItem.
func (tx *Tx) SpendItem(itemID, count uint32) error {
	if count == 0 {
		return nil
	}
	n, err := tx.Queries.SpendItem(tx.ctx, db.SpendItemParams{CommanderID: tx.commanderID, ItemID: itemID, Count: count})
	if err == nil && n == 0 {
		err = ErrNotEnough
	}
	if err == nil {
		tx.event(TaskEvent{SubType: TaskSubUseItem, Target: itemID, Count: count})
	}
	return err
}

// AddVirtualItem adds amount to an item_virtual_data_statistics counter.
// Weekly mission points count for TaskSubWeeklyPt.
func (tx *Tx) AddVirtualItem(itemID, amount uint32) error {
	if err := tx.Queries.AddVirtualItem(tx.ctx, db.AddVirtualItemParams{CommanderID: tx.commanderID, ItemID: itemID, Amount: amount}); err != nil {
		return err
	}
	if itemID == VirtualItemWeeklyPt {
		tx.event(TaskEvent{SubType: TaskSubWeeklyPt, Count: amount})
	}
	return nil
}

// AddEquipment puts count of an equip_data_template id in the depot.
func (tx *Tx) AddEquipment(equipID, count uint32) error {
	return tx.Queries.AddEquipment(tx.ctx, db.AddEquipmentParams{CommanderID: tx.commanderID, EquipID: equipID, Count: count})
}

// SpendEquipment takes count of an equipment from the depot, ErrNotEnough if
// there are fewer.
func (tx *Tx) SpendEquipment(equipID, count uint32) error {
	n, err := tx.Queries.SpendEquipment(tx.ctx, db.SpendEquipmentParams{CommanderID: tx.commanderID, EquipID: equipID, Count: count})
	if err == nil && n == 0 {
		err = ErrNotEnough
	}
	return err
}

// grantStarterItems fills the bag of a new commander (new_commander.json).
func grantStarterItems(tx *Tx) error {
	for _, item := range newCommander.Items {
		if err := tx.AddItem(item.ID, item.Count); err != nil {
			return err
		}
	}
	return nil
}

func resourceList(rows []db.ListResourcesRow) []*protobuf.RESOURCE {
	list := make([]*protobuf.RESOURCE, len(rows))
	for i, row := range rows {
		list[i] = &protobuf.RESOURCE{Type: proto.Uint32(row.ResourceID), Num: proto.Uint32(row.Amount)}
	}
	return list
}

// bag is SC_15001, BagProxy's items. limit_list is as captured: the
// Specialized Core with nothing counted against its limit.
func bag(ctx context.Context, queries *db.Queries, commanderID uint32) (*protobuf.SC_15001, error) {
	rows, err := queries.ListItems(ctx, commanderID)
	if err != nil {
		return nil, err
	}
	msg := &protobuf.SC_15001{ItemList: make([]*protobuf.ITEMINFO, len(rows)), LimitList: []*protobuf.ITEMINFO{{Id: proto.Uint32(itemSpecializedCore), Count: proto.Uint32(0)}}}
	for i, row := range rows {
		msg.ItemList[i] = &protobuf.ITEMINFO{Id: proto.Uint32(row.ItemID), Count: proto.Uint32(row.Count)}
	}
	return msg, nil
}

// equipmentDepot is SC_14001, EquipmentProxy's equipment not on a ship
// (Equipment.New keyed by template id), no augments.
func equipmentDepot(ctx context.Context, queries *db.Queries, commanderID uint32) (*protobuf.SC_14001, error) {
	rows, err := queries.ListEquipments(ctx, commanderID)
	if err != nil {
		return nil, err
	}
	msg := &protobuf.SC_14001{
		EquipList:       make([]*protobuf.EQUIPINFO, len(rows)),
		SpweaponBagSize: proto.Uint32(spweaponBagSize),
	}
	for i, row := range rows {
		msg.EquipList[i] = &protobuf.EQUIPINFO{Id: proto.Uint32(row.EquipID), Count: proto.Uint32(row.Count)}
	}
	return msg, nil
}

// handleUpgradeShipEquipment upgrades the equipment in a ship slot by lv
// levels (UpGradeEquipmentCommands, CS_14002). Each level costs the current
// equip_data_template's trans_use_gold and trans_use_item and moves to its
// next; the client takes the same costs off its own state on result 0.
func (game *Game) handleUpgradeShipEquipment(client *connection.Client, req *protobuf.CS_14002) error {
	ctx := context.Background()
	err := game.tx(ctx, client, func(tx *Tx) error {
		equips, err := shipEquipments(ctx, tx.Queries, client.CommanderID, req.GetShipId())
		if err != nil {
			return err
		}
		pos := req.GetPos()
		if pos < 1 || int(pos) > len(equips) || equips[pos-1].EquipID == 0 || req.GetLv() == 0 {
			return errRefused
		}
		equip := equips[pos-1]
		for range req.GetLv() {
			upgrade, ok := equipUpgrades[equip.EquipID]
			if !ok {
				return errRefused
			}
			if err := tx.SpendResource(ResourceGold, upgrade.Gold); err != nil {
				return err
			}
			for _, item := range upgrade.Items {
				if err := tx.SpendItem(item[0], item[1]); err != nil {
					return err
				}
			}
			equip.EquipID = upgrade.Next
		}
		tx.event(TaskEvent{SubType: TaskSubEnhanceGear, Count: 1})
		return tx.SetShipEquipment(ctx, db.SetShipEquipmentParams{ShipID: equip.ShipID, Pos: pos, EquipID: equip.EquipID, SkinID: equip.SkinID})
	})
	if err != nil && !refused(err) {
		return err
	}
	return client.Send(&protobuf.SC_14003{Result: result(err)})
}

// handleComposeEquipment crafts equipment from blueprints
// (CompositeEquipmentCommand, CS_14006): num times compose_data_template id,
// each costing gold_num gold and material_num of material_id, into the depot.
func (game *Game) handleComposeEquipment(client *connection.Client, req *protobuf.CS_14006) error {
	ctx := context.Background()
	recipe, ok := composes[req.GetId()]
	num := req.GetNum()
	err := errRefused
	if ok && num > 0 {
		err = game.tx(ctx, client, func(tx *Tx) error {
			if err := tx.SpendResource(ResourceGold, recipe.Gold*num); err != nil {
				return err
			}
			if err := tx.SpendItem(recipe.Material, recipe.MaterialCount*num); err != nil {
				return err
			}
			tx.event(TaskEvent{SubType: TaskSubCraftGear, Count: num})
			return tx.AddEquipment(recipe.Equip, num)
		})
	}
	if err != nil && !refused(err) {
		return err
	}
	return client.Send(&protobuf.SC_14007{Result: result(err)})
}

// handleSellItems sells bag items for their item_data_statistics price
// (SellItemCommand, CS_15008). The home screen sends it on every visit with
// the expired discount / skin trial items, usually none; the client adds the
// price (BagProxy:GetSellingPrice) itself on result 0.
func (game *Game) handleSellItems(client *connection.Client, req *protobuf.CS_15008) error {
	ctx := context.Background()
	err := game.tx(ctx, client, func(tx *Tx) error {
		for _, item := range req.GetItemList() {
			price, ok := itemPrices[item.GetId()]
			if !ok {
				return errRefused
			}
			if err := tx.SpendItem(item.GetId(), item.GetCount()); err != nil {
				return err
			}
			if err := tx.AddResource(price[0], price[1]*item.GetCount()); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil && !refused(err) {
		return err
	}
	return client.Send(&protobuf.SC_15009{Result: result(err)})
}

// handleTracking takes the client's analytics events (CS_10991), no reply.
//
//belfast:static
func (game *Game) handleTracking(client *connection.Client, req *protobuf.CS_10991) error {
	return nil
}

// handleTrack takes the client's UI action events (TrackCommand, CS_10993),
// no reply.
//
//belfast:static
func (game *Game) handleTrack(client *connection.Client, req *protobuf.CS_10993) error {
	return nil
}
