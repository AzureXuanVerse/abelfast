package answer

import (
	"context"
	"encoding/json"
	"errors"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/consts"
	"github.com/ggmolly/belfast/internal/orm"
	"github.com/ggmolly/belfast/internal/protobuf"
	"github.com/jackc/pgx/v5"
	"google.golang.org/protobuf/proto"
)

const (
	atelierResultSuccess             = uint32(0)
	atelierResultMalformedRequest    = uint32(1)
	atelierResultInvalidActivity     = uint32(2)
	atelierResultInvalidRecipeOrItem = uint32(3)
	atelierResultInsufficientItems   = uint32(4)
	atelierResultRecipeLimitReached  = uint32(5)
	atelierResultStorageFailure      = uint32(6)

	atelierRecipeCategory = "ShareCfg/activity_ryza_recipe.json"
	atelierCircleCategory = "ShareCfg/activity_ryza_recipe_circle.json"
	atelierItemCategory   = "ShareCfg/activity_ryza_item.json"
)

type atelierRecipeConfig struct {
	ID           uint32   `json:"id"`
	ItemID       []uint32 `json:"item_id"`
	ItemNum      uint32   `json:"item_num"`
	RecipeCircle []uint32 `json:"recipe_circle"`
}

type atelierRecipeCircleConfig struct {
	ID         uint32 `json:"id"`
	RecipeID   uint32 `json:"recipe_id"`
	RyzaItemID uint32 `json:"ryza_item_id"`
}

type atelierItemConfig struct {
	ID          uint32          `json:"id"`
	BenefitBuff json.RawMessage `json:"benefit_buff"`
}

type atelierReward struct {
	DropType uint32
	DropID   uint32
	Count    uint32
}

func parseAtelierRecipeConfig(recipeID uint32) (*atelierRecipeConfig, error) {
	entry, err := orm.GetConfigEntry(atelierRecipeCategory, strconv.FormatUint(uint64(recipeID), 10))
	if err != nil {
		return nil, err
	}
	recipe := &atelierRecipeConfig{}
	if err := json.Unmarshal(entry.Data, recipe); err != nil {
		return nil, err
	}
	if recipe.ID == 0 {
		recipe.ID = recipeID
	}
	return recipe, nil
}

func parseAtelierRecipeAllowedItems(recipe *atelierRecipeConfig) (map[uint32]struct{}, error) {
	allowed := make(map[uint32]struct{})
	for _, circleID := range recipe.RecipeCircle {
		entry, err := orm.GetConfigEntry(atelierCircleCategory, strconv.FormatUint(uint64(circleID), 10))
		if err != nil {
			return nil, err
		}
		circle := &atelierRecipeCircleConfig{}
		if err := json.Unmarshal(entry.Data, circle); err != nil {
			return nil, err
		}
		if circle.RecipeID != 0 && circle.RecipeID != recipe.ID {
			continue
		}
		if circle.RyzaItemID != 0 {
			allowed[circle.RyzaItemID] = struct{}{}
		}
	}
	return allowed, nil
}

func parseAtelierItemConfig(itemID uint32) (*atelierItemConfig, error) {
	entry, err := orm.GetConfigEntry(atelierItemCategory, strconv.FormatUint(uint64(itemID), 10))
	if err != nil {
		return nil, err
	}
	item := &atelierItemConfig{}
	if err := json.Unmarshal(entry.Data, item); err != nil {
		return nil, err
	}
	if item.ID == 0 {
		item.ID = itemID
	}
	return item, nil
}

func (c *atelierItemConfig) buffTierCount() int {
	if len(c.BenefitBuff) == 0 {
		return 0
	}
	trimmed := strings.TrimSpace(string(c.BenefitBuff))
	if trimmed == "" || trimmed == "null" || trimmed == `""` {
		return 0
	}
	list := []uint32{}
	if err := json.Unmarshal(c.BenefitBuff, &list); err != nil {
		return 0
	}
	return len(list)
}

func ensureAtelierActivity(actID uint32) error {
	if actID == 0 {
		return errors.New("missing activity")
	}
	activity, err := loadActivityTemplate(actID)
	if err != nil {
		return err
	}
	if activity.Type != activityTypeAtelierLink {
		return errors.New("unexpected activity type")
	}
	return nil
}

func sortedAtelierKVDATA(entries map[uint32]uint32) []*protobuf.KVDATA {
	if len(entries) == 0 {
		return []*protobuf.KVDATA{}
	}
	keys := make([]uint32, 0, len(entries))
	for key, value := range entries {
		if key == 0 || value == 0 {
			continue
		}
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i int, j int) bool { return keys[i] < keys[j] })
	out := make([]*protobuf.KVDATA, 0, len(keys))
	for _, key := range keys {
		out = append(out, &protobuf.KVDATA{Key: proto.Uint32(key), Value: proto.Uint32(entries[key])})
	}
	return out
}

func sortedAtelierSlots(slots map[uint32]orm.AtelierBuffSlotState) []*protobuf.BUFF_SLOT {
	out := make([]*protobuf.BUFF_SLOT, 0, 5)
	for pos := uint32(1); pos <= 5; pos++ {
		slot := slots[pos]
		out = append(out, &protobuf.BUFF_SLOT{
			Pos:     proto.Uint32(pos),
			Itemid:  proto.Uint32(slot.ItemID),
			Itemnum: proto.Uint32(slot.ItemNum),
		})
	}
	return out
}

func applyAtelierRewardsTx(ctx context.Context, tx pgx.Tx, client *connection.Client, state *orm.AtelierState, rewards []atelierReward) ([]*protobuf.DROPINFO, error) {
	awardMap := make(map[string]*protobuf.DROPINFO)
	for _, reward := range rewards {
		if reward.DropType == 0 || reward.DropID == 0 || reward.Count == 0 {
			continue
		}
		if err := applyAtelierRewardTx(ctx, tx, client, state, reward); err != nil {
			return nil, err
		}
		key := strconv.FormatUint(uint64(reward.DropType), 10) + ":" + strconv.FormatUint(uint64(reward.DropID), 10)
		existing := awardMap[key]
		if existing == nil {
			awardMap[key] = &protobuf.DROPINFO{
				Type:   proto.Uint32(reward.DropType),
				Id:     proto.Uint32(reward.DropID),
				Number: proto.Uint32(reward.Count),
			}
			continue
		}
		existing.Number = proto.Uint32(existing.GetNumber() + reward.Count)
	}
	list := make([]*protobuf.DROPINFO, 0, len(awardMap))
	for _, award := range awardMap {
		list = append(list, award)
	}
	sort.Slice(list, func(i int, j int) bool {
		if list[i].GetType() == list[j].GetType() {
			return list[i].GetId() < list[j].GetId()
		}
		return list[i].GetType() < list[j].GetType()
	})
	return list, nil
}

func applyAtelierRewardTx(ctx context.Context, tx pgx.Tx, client *connection.Client, state *orm.AtelierState, reward atelierReward) error {
	switch reward.DropType {
	case consts.DROP_TYPE_RESOURCE:
		return client.Commander.AddResourceTx(ctx, tx, reward.DropID, reward.Count)
	case consts.DROP_TYPE_ITEM, consts.DROP_TYPE_LOVE_LETTER:
		return client.Commander.AddItemTx(ctx, tx, reward.DropID, reward.Count)
	case consts.DROP_TYPE_EQUIP:
		return addOwnedEquipmentPGXTx(ctx, tx, client.Commander, reward.DropID, reward.Count)
	case consts.DROP_TYPE_SHIP:
		for i := uint32(0); i < reward.Count; i++ {
			if _, err := client.Commander.AddShipTx(ctx, tx, reward.DropID); err != nil {
				return err
			}
		}
		return nil
	case consts.DROP_TYPE_FURNITURE:
		now := uint32(time.Now().Unix())
		return orm.AddCommanderFurnitureTx(ctx, tx, client.Commander.CommanderID, reward.DropID, reward.Count, now)
	case consts.DROP_TYPE_SKIN:
		for i := uint32(0); i < reward.Count; i++ {
			if err := client.Commander.GiveSkinTx(ctx, tx, reward.DropID); err != nil {
				return err
			}
		}
		return nil
	case consts.DROP_TYPE_SPWEAPON:
		for i := uint32(0); i < reward.Count; i++ {
			var created orm.OwnedSpWeapon
			err := tx.QueryRow(ctx, `
INSERT INTO owned_spweapons (
  owner_id,
  template_id,
  attr_1,
  attr_2,
  attr_temp_1,
  attr_temp_2,
  effect,
  pt,
  equipped_ship_id
) VALUES (
  $1, $2, 0, 0, 0, 0, 0, 0, 0
)
RETURNING owner_id, id, template_id, attr_1, attr_2, attr_temp_1, attr_temp_2, effect, pt, equipped_ship_id
`, int64(client.Commander.CommanderID), int64(reward.DropID)).Scan(
				&created.OwnerID,
				&created.ID,
				&created.TemplateID,
				&created.Attr1,
				&created.Attr2,
				&created.AttrTemp1,
				&created.AttrTemp2,
				&created.Effect,
				&created.Pt,
				&created.EquippedShipID,
			)
			if err != nil {
				return err
			}
			client.Commander.OwnedSpWeapons = append(client.Commander.OwnedSpWeapons, created)
			if client.Commander.OwnedSpWeaponsMap == nil {
				client.Commander.OwnedSpWeaponsMap = make(map[uint32]*orm.OwnedSpWeapon)
			}
			client.Commander.OwnedSpWeaponsMap[created.ID] = &client.Commander.OwnedSpWeapons[len(client.Commander.OwnedSpWeapons)-1]
		}
		return nil
	case consts.DROP_TYPE_RYZA_DROP:
		state.Items[reward.DropID] += reward.Count
		return nil
	case consts.DROP_TYPE_VITEM:
		return nil
	default:
		return errors.New("unsupported atelier drop type")
	}
}
