package game

import (
	"context"
	"encoding/json"
	"maps"
	"slices"
	"time"

	"google.golang.org/protobuf/proto"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/db"
	"github.com/ggmolly/belfast/internal/gamedata"
	"github.com/ggmolly/belfast/internal/protobuf"
)

// activityTypeAtelier is ActivityConst.ACTIVITY_TYPE_ATELIER_LINK: the
// Atelier Ryza / Atelier Yumia synthesis (AtelierActivity,
// model/vo/activityvos/atelieractivity.lua), its materials, recipe counts and
// buff slots kept by the server and requested with CS_26051 after SC_11200.
const activityTypeAtelier = 88

// DROP_TYPE_RYZA_DROP: an activity_ryza_item material, stored in the
// Atelier (activity_drop_type 1001's activity_id; Drop's addDrop adds it to
// that activity, model/vo/drop.lua).
const dropTypeRyza = 1001

// DROP_TYPE_SPWEAPON: an augment module (spweapon_data_statistics), some
// recipes' product.
const dropTypeSpWeapon = 21

// TaskSubAtelierComposite is task_data_template sub_type 9001: syntheses
// done, target the recipe id (0 for any): "Successfully carry out Synthesis a
// total of 10 times". The client doesn't count it (Task.getProgress).
const TaskSubAtelierComposite = 9001

// atelierSlots is the number of buff slots (AtelierActivity.slots).
const atelierSlots = 5

// AtelierMaterial.TYPE / AtelierFormulaCircle.TYPE values the rules use
const (
	atelierItemNormal      = 1
	atelierItemNeutralizer = 2
	atelierItemSiren       = 4
	atelierItemStrengthen  = 5

	atelierCircleNormal          = 2
	atelierCircleSiren           = 3
	atelierCircleAny             = 4
	atelierCircleNone            = 5
	atelierCircleElement         = 6
	atelierCircleCategory        = 7
	atelierCircleElementCategory = 8

	atelierElementAny   = 0
	atelierElementSiren = 5
)

// activity_ryza_recipe row (AtelierFormula): item_id the [type, id] it
// makes, item_num how many times it can be synthesized (-1 no limit),
// recipe_circle the slots to fill, version the Atelier it belongs to
// (config_client's index: 1 Ryza, 2 Yumia).
type atelierRecipe struct {
	ItemID       []uint32 `json:"item_id"`
	ItemNum      int      `json:"item_num"`
	RecipeCircle []uint32 `json:"recipe_circle"`
	Version      uint32   `json:"version"`
}

// activity_ryza_item row (AtelierMaterial): prop its elements, prop_level
// its level, prop_type its category, benefit_buff the buff of each level a
// buff slot can hold ("" when it has none).
type atelierItem struct {
	Type        uint32          `json:"type"`
	Prop        []uint32        `json:"prop"`
	PropLevel   uint32          `json:"prop_level"`
	PropType    uint32          `json:"prop_type"`
	Version     uint32          `json:"version"`
	BenefitBuff json.RawMessage `json:"benefit_buff"`
}

// buffs is how many levels of benefit_buff a buff slot can hold of it.
func (item atelierItem) buffs() int {
	var buffs []uint32
	if json.Unmarshal(item.BenefitBuff, &buffs) != nil {
		return 0
	}
	return len(buffs)
}

// activity_ryza_recipe_circle row (AtelierFormulaCircle): prop is the
// element, prop_level the level, prop_type the category, ryza_item_id the
// one material it takes (0 when the type decides).
type atelierCircle struct {
	Type       uint32 `json:"type"`
	Prop       uint32 `json:"prop"`
	PropLevel  uint32 `json:"prop_level"`
	PropType   uint32 `json:"prop_type"`
	RyzaItemID uint32 `json:"ryza_item_id"`
}

// element is AtelierFormulaCircle:GetElement.
func (circle atelierCircle) element() uint32 {
	switch circle.Type {
	case atelierCircleSiren:
		return atelierElementSiren
	case atelierCircleAny:
		return atelierElementAny
	}
	return circle.Prop
}

var (
	atelierRecipes map[uint32]atelierRecipe
	atelierItems   map[uint32]atelierItem
	atelierCircles map[uint32]atelierCircle
)

func loadAtelierData() error {
	return gamedata.LoadAll(
		gamedata.Load("activity_ryza_recipe", &atelierRecipes),
		gamedata.Load("activity_ryza_item", &atelierItems),
		gamedata.Load("activity_ryza_recipe_circle", &atelierCircles),
	)
}

// atelierAccepts is AtelierFormulaCircle:CanUseMaterial: whether circle takes
// the material itemID in recipe.
func atelierAccepts(recipe atelierRecipe, circle atelierCircle, itemID uint32) bool {
	item, ok := atelierItems[itemID]
	if !ok || item.Version != max(recipe.Version, 1) {
		return false
	}
	// the recipe's own product, or a neutralizer of the level of the
	// neutralizer it makes, can't go in a normal or any circle
	ownProduct := func() bool {
		if len(recipe.ItemID) < 2 || recipe.ItemID[0] != dropTypeRyza {
			return false
		}
		if recipe.ItemID[1] == itemID {
			return true
		}
		product := atelierItems[recipe.ItemID[1]]
		return product.Type == atelierItemNeutralizer && item.Type == atelierItemNeutralizer && product.PropLevel == item.PropLevel
	}
	switch {
	case circle.RyzaItemID != 0:
		return itemID == circle.RyzaItemID
	case circle.Type == atelierCircleNormal:
		return (item.Type == atelierItemNormal || item.Type == atelierItemNeutralizer) &&
			slices.Contains(item.Prop, circle.element()) && !ownProduct() && item.PropLevel == circle.PropLevel
	case circle.Type == atelierCircleAny:
		return (item.Type == atelierItemNormal || item.Type == atelierItemNeutralizer || item.Type == atelierItemSiren) &&
			!ownProduct() && item.PropLevel == circle.PropLevel
	case circle.Type == atelierCircleNone:
		return item.PropType != 0
	case circle.Type == atelierCircleElement:
		return slices.Contains(item.Prop, circle.element()) && item.PropType != 0
	case circle.Type == atelierCircleCategory:
		return item.PropType == circle.PropType
	case circle.Type == atelierCircleElementCategory:
		return slices.Contains(item.Prop, circle.element()) && item.PropType == circle.PropType
	}
	return false
}

// addAtelierItem stores count of the material itemID (DROP_TYPE_RYZA_DROP).
func (tx *Tx) addAtelierItem(itemID, count uint32) error {
	return tx.AddAtelierItem(tx.ctx, db.AddAtelierItemParams{CommanderID: tx.commanderID, ItemID: itemID, Count: count})
}

// addSpWeapons stores count new augments templateID (DROP_TYPE_SPWEAPON), as
// crafted ones (attributes rolled from 0), pushed in SC_14200
// (EquipmentProxy adds them).
func (tx *Tx) addSpWeapons(templateID, count uint32) error {
	config, ok := spweapons[templateID]
	if !ok {
		return gamedata.Missing("spweapon_data_statistics", "%d", templateID)
	}
	push := &protobuf.SC_14200{}
	for range count {
		attr1, attr2 := rollSpWeapon(config, 0)
		row, err := tx.CreateSpWeapon(tx.ctx, db.CreateSpWeaponParams{CommanderID: tx.commanderID, TemplateID: templateID, Attr1: attr1, Attr2: attr2})
		if err != nil {
			return err
		}
		push.SpweaponList = append(push.SpweaponList, spweaponInfo(row))
	}
	tx.push(push)
	return nil
}

// atelierActivity is whether id is an Atelier activity running for the
// commander.
func (game *Game) atelierActivity(ctx context.Context, commanderID, id uint32) (bool, error) {
	config, open, err := game.commanderActivity(ctx, commanderID, id, time.Now())
	return open && config.Type == activityTypeAtelier, err
}

// handleAtelierRequest answers AtelierRequestCommand
// (controller/command/activity/atelierrequestcommand.lua), sent after every
// SC_11200 holding an ACTIVITY_TYPE_ATELIER_LINK activity: the materials
// held (key the activity_ryza_item id, value the count; InitItems adds them
// to what the activity holds), the times each recipe was synthesized
// (InitFormulaUseCounts) and the buff slots set (UpdateBuffSlots). No
// official capture has it: built from the Lua alone.
func (game *Game) handleAtelierRequest(client *connection.Client, req *protobuf.CS_26051) error {
	ctx := context.Background()
	open, err := game.atelierActivity(ctx, client.CommanderID, req.GetActId())
	if err != nil {
		return err
	}
	if !open {
		return client.Send(&protobuf.SC_26052{Result: result(errRefused)})
	}
	items, err := game.DB.ListAtelierItems(ctx, client.CommanderID)
	if err != nil {
		return err
	}
	recipes, err := game.DB.ListAtelierRecipes(ctx, client.CommanderID)
	if err != nil {
		return err
	}
	slots, err := game.DB.ListAtelierSlots(ctx, client.CommanderID)
	if err != nil {
		return err
	}
	reply := &protobuf.SC_26052{Result: result(nil)}
	for _, item := range items {
		reply.Items = append(reply.Items, &protobuf.KVDATA{Key: proto.Uint32(item.ItemID), Value: proto.Uint32(item.Count)})
	}
	for _, recipe := range recipes {
		// InitFormulaUseCounts indexes the recipes by id: an unknown one
		// would break the client
		if _, ok := atelierRecipes[recipe.RecipeID]; ok {
			reply.Recipes = append(reply.Recipes, &protobuf.KVDATA{Key: proto.Uint32(recipe.RecipeID), Value: proto.Uint32(recipe.Used)})
		}
	}
	for _, slot := range slots {
		reply.Slots = append(reply.Slots, &protobuf.BUFF_SLOT{Pos: proto.Uint32(slot.Pos), Itemid: proto.Uint32(slot.ItemID), Itemnum: proto.Uint32(slot.ItemNum)})
	}
	return client.Send(reply)
}

// handleAtelierComposite answers AtelierCompositeCommand
// (controller/command/activity/ateliercompositecommand.lua): synthesizes
// recipe_id times times from items, one material per recipe circle (key the
// activity_ryza_recipe_circle id, value the activity_ryza_item id,
// AtelierCompositeConfirmView). On result 0 the client takes times of each
// material off, counts the uses and adds award_list (addTranDrop). The rules
// are the client's (AtelierFormulaDetailView.OnClickComposite,
// AtelierFormula:IsAvaliable, AtelierFormulaCircle:CanUseMaterial): every
// circle filled with a material it takes, the recipe's item_num not
// exceeded, the materials held. The product is item_id, one per synthesis.
// The atelier's own unlock (config_client unlockStageID / unlockStoryID) is
// only checked by the client. No official capture has it: built from the
// Lua alone.
func (game *Game) handleAtelierComposite(client *connection.Client, req *protobuf.CS_26053) error {
	ctx := context.Background()
	open, err := game.atelierActivity(ctx, client.CommanderID, req.GetActId())
	if err != nil {
		return err
	}
	reply := &protobuf.SC_26054{}
	err = game.tx(ctx, client, func(tx *Tx) error {
		recipe, ok := atelierRecipes[req.GetRecipeId()]
		times := req.GetTimes()
		if !open || !ok || times == 0 || len(recipe.ItemID) < 2 || len(req.GetItems()) != len(recipe.RecipeCircle) {
			return errRefused
		}
		used := uint32(0)
		recipes, err := tx.ListAtelierRecipes(tx.ctx, tx.commanderID)
		if err != nil {
			return err
		}
		for _, row := range recipes {
			if row.RecipeID == req.GetRecipeId() {
				used = row.Used
			}
		}
		if recipe.ItemNum >= 0 && used+times > uint32(recipe.ItemNum) {
			return errRefused
		}
		filled := map[uint32]bool{}
		spent := map[uint32]uint32{}
		for _, entry := range req.GetItems() {
			circleID, itemID := entry.GetKey(), entry.GetValue()
			circle, ok := atelierCircles[circleID]
			if !ok || filled[circleID] || !slices.Contains(recipe.RecipeCircle, circleID) || !atelierAccepts(recipe, circle, itemID) {
				return errRefused
			}
			filled[circleID] = true
			spent[itemID] += times
		}
		for _, itemID := range slices.Sorted(maps.Keys(spent)) {
			n, err := tx.SpendAtelierItem(tx.ctx, db.SpendAtelierItemParams{CommanderID: tx.commanderID, ItemID: itemID, Count: spent[itemID]})
			if err != nil {
				return err
			}
			if n == 0 {
				return ErrNotEnough
			}
		}
		if err := tx.AddAtelierRecipeUse(tx.ctx, db.AddAtelierRecipeUseParams{CommanderID: tx.commanderID, RecipeID: req.GetRecipeId(), Used: times}); err != nil {
			return err
		}
		reply.AwardList = []*protobuf.DROPINFO{dropInfo(recipe.ItemID[0], recipe.ItemID[1], times)}
		tx.event(TaskEvent{SubType: TaskSubAtelierComposite, Target: req.GetRecipeId(), Count: times})
		return tx.grantDrops(reply.AwardList)
	})
	if err != nil && !refused(err) {
		return err
	}
	reply.Result = result(err)
	if err != nil {
		reply.AwardList = nil
	}
	return client.Send(reply)
}

// handleAtelierRefreshBuff answers AtelierRefreshBuffCommand
// (controller/command/activity/atelierrefreshbuffcommand.lua, AtelierBuffLayer):
// the five buff slots as they now are, only the filled ones sent (pos
// 1-5, itemid a STRENGTHEN material, itemnum how many of its benefit_buff
// levels); on result 0 the client takes all five as it asked. The rules are
// the buff layer's: a material in one slot at most, no more levels than it
// has benefit_buff nor than the commander holds of it. The materials stay
// held. No official capture has it: built from the Lua alone.
func (game *Game) handleAtelierRefreshBuff(client *connection.Client, req *protobuf.CS_26055) error {
	ctx := context.Background()
	open, err := game.atelierActivity(ctx, client.CommanderID, req.GetActId())
	if err != nil {
		return err
	}
	err = game.tx(ctx, client, func(tx *Tx) error {
		if !open || len(req.GetSlots()) > atelierSlots {
			return errRefused
		}
		items, err := tx.ListAtelierItems(tx.ctx, tx.commanderID)
		if err != nil {
			return err
		}
		held := make(map[uint32]uint32, len(items))
		for _, item := range items {
			held[item.ItemID] = item.Count
		}
		positions := map[uint32]bool{}
		used := map[uint32]bool{}
		for _, slot := range req.GetSlots() {
			pos, itemID, num := slot.GetPos(), slot.GetItemid(), slot.GetItemnum()
			item, ok := atelierItems[itemID]
			if pos < 1 || pos > atelierSlots || positions[pos] || used[itemID] || !ok || item.Type != atelierItemStrengthen ||
				num == 0 || int(num) > item.buffs() || num > held[itemID] {
				return errRefused
			}
			positions[pos], used[itemID] = true, true
		}
		if err := tx.ClearAtelierSlots(tx.ctx, tx.commanderID); err != nil {
			return err
		}
		for _, slot := range req.GetSlots() {
			err := tx.SetAtelierSlot(tx.ctx, db.SetAtelierSlotParams{CommanderID: tx.commanderID, Pos: slot.GetPos(), ItemID: slot.GetItemid(), ItemNum: slot.GetItemnum()})
			if err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil && !refused(err) {
		return err
	}
	return client.Send(&protobuf.SC_26056{Result: result(err)})
}
