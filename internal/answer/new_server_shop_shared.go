package answer

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/consts"
	"github.com/ggmolly/belfast/internal/db"
	"github.com/ggmolly/belfast/internal/orm"
	"github.com/ggmolly/belfast/internal/protobuf"
	"google.golang.org/protobuf/proto"
)

const (
	activityTypeNewServerShop   = 83
	activityTypeBlackFridayShop = 38

	newServerShopResultOK           = 0
	newServerShopResultFailed       = 1
	newServerShopResultInsufficient = 2
	newServerShopResultLimit        = 3
	newServerShopResultUnsupported  = 4

	newServerShopGoodsTypeFixed      = 1
	newServerShopGoodsTypeSelectable = 4
)

type newServerShopTemplateEntry struct {
	ID                 uint32   `json:"id"`
	Goods              []uint32 `json:"goods"`
	GoodsPurchaseLimit uint32   `json:"goods_purchase_limit"`
	GoodsType          uint32   `json:"goods_type"`
	Num                uint32   `json:"num"`
	Type               uint32   `json:"type"`
	ResourceCategory   uint32   `json:"resource_category"`
	ResourceType       uint32   `json:"resource_type"`
	ResourceNum        uint32   `json:"resource_num"`
}

type newServerShopActivity struct {
	ActivityID uint32
	StartTime  uint32
	StopTime   uint32
	Goods      []newServerShopTemplateEntry
	GoodsByID  map[uint32]newServerShopTemplateEntry
}

func loadNewServerShopActivity(actID uint32, now time.Time) (*newServerShopActivity, bool, error) {
	template, err := loadActivityTemplate(actID)
	if err != nil {
		if db.IsNotFound(err) {
			return nil, false, nil
		}
		return nil, false, err
	}
	if template.Type != activityTypeNewServerShop && template.Type != activityTypeBlackFridayShop {
		return nil, false, nil
	}

	startTime, stopTime, active, err := parseActivityTimeWindow(template.Time, now)
	if err != nil {
		return nil, false, err
	}
	if !active {
		return nil, false, nil
	}

	goodsIDs, err := parseActivityConfigIDs(template.ConfigData)
	if err != nil {
		return nil, false, err
	}
	if len(goodsIDs) == 0 {
		return nil, false, nil
	}

	goods := make([]newServerShopTemplateEntry, 0, len(goodsIDs))
	goodsByID := make(map[uint32]newServerShopTemplateEntry, len(goodsIDs))
	for _, goodsID := range goodsIDs {
		entry, found, err := loadNewServerShopTemplateEntry(goodsID, template.Type)
		if err != nil {
			return nil, false, err
		}
		if !found {
			return nil, false, nil
		}
		goods = append(goods, *entry)
		goodsByID[entry.ID] = *entry
	}

	return &newServerShopActivity{
		ActivityID: actID,
		StartTime:  startTime,
		StopTime:   stopTime,
		Goods:      goods,
		GoodsByID:  goodsByID,
	}, true, nil
}

func loadNewServerShopTemplateEntry(id uint32, activityType uint32) (*newServerShopTemplateEntry, bool, error) {
	key := strconv.FormatUint(uint64(id), 10)
	categories := []string{"ShareCfg/newserver_shop_template.json"}
	if activityType == activityTypeBlackFridayShop {
		categories = []string{"ShareCfg/blackfriday_shop_template.json", "ShareCfg/newserver_shop_template.json"}
	}

	for _, category := range categories {
		if entry, err := orm.GetConfigEntry(category, key); err == nil {
			var out newServerShopTemplateEntry
			if err := json.Unmarshal(entry.Data, &out); err != nil {
				return nil, false, err
			}
			if out.ID == 0 {
				out.ID = id
			}
			return &out, true, nil
		} else if !db.IsNotFound(err) {
			return nil, false, err
		}

		entries, err := orm.ListConfigEntries(category)
		if err != nil {
			return nil, false, err
		}
		for i := range entries {
			var single newServerShopTemplateEntry
			if err := json.Unmarshal(entries[i].Data, &single); err == nil {
				if single.ID == id {
					return &single, true, nil
				}
			}
			var list []newServerShopTemplateEntry
			if err := json.Unmarshal(entries[i].Data, &list); err != nil {
				continue
			}
			for j := range list {
				if list[j].ID == id {
					return &list[j], true, nil
				}
			}
		}
	}
	return nil, false, nil
}

func parseActivityTimeWindow(raw json.RawMessage, now time.Time) (uint32, uint32, bool, error) {
	var label string
	if err := json.Unmarshal(raw, &label); err == nil {
		switch label {
		case "always":
			unix := uint32(now.Unix())
			return unix, unix + 31536000, true, nil
		default:
			return 0, 0, false, nil
		}
	}

	var timer []any
	if err := json.Unmarshal(raw, &timer); err != nil {
		return 0, 0, false, err
	}
	if len(timer) < 3 {
		return 0, 0, false, nil
	}
	tag, ok := timer[0].(string)
	if !ok || tag != "timer" {
		return 0, 0, false, nil
	}
	start, ok := parseNewServerShopTimerPoint(timer[1])
	if !ok {
		return 0, 0, false, nil
	}
	stop, ok := parseNewServerShopTimerPoint(timer[2])
	if !ok {
		return 0, 0, false, nil
	}
	return uint32(start.Unix()), uint32(stop.Unix()), !now.Before(start) && !now.After(stop), nil
}

func parseNewServerShopTimerPoint(raw any) (time.Time, bool) {
	point, ok := raw.([]any)
	if !ok || len(point) != 2 {
		return time.Time{}, false
	}
	date, ok := point[0].([]any)
	if !ok || len(date) != 3 {
		return time.Time{}, false
	}
	clock, ok := point[1].([]any)
	if !ok || len(clock) != 3 {
		return time.Time{}, false
	}
	year, ok := parseJSONInt(date[0])
	if !ok {
		return time.Time{}, false
	}
	month, ok := parseJSONInt(date[1])
	if !ok {
		return time.Time{}, false
	}
	day, ok := parseJSONInt(date[2])
	if !ok {
		return time.Time{}, false
	}
	hour, ok := parseJSONInt(clock[0])
	if !ok {
		return time.Time{}, false
	}
	minute, ok := parseJSONInt(clock[1])
	if !ok {
		return time.Time{}, false
	}
	second, ok := parseJSONInt(clock[2])
	if !ok {
		return time.Time{}, false
	}
	return time.Date(year, time.Month(month), day, hour, minute, second, 0, time.UTC), true
}

func defaultNewServerShopState(commanderID uint32, activityID uint32, goods []newServerShopTemplateEntry) *orm.NewServerShopState {
	state := &orm.NewServerShopState{
		CommanderID: commanderID,
		ActivityID:  activityID,
		Goods:       make([]orm.NewServerShopGoodsState, 0, len(goods)),
	}
	for _, entry := range goods {
		state.Goods = append(state.Goods, orm.NewServerShopGoodsState{ID: entry.ID, Count: entry.GoodsPurchaseLimit, BoughtRecord: []uint32{}})
	}
	return state
}

func normalizeNewServerShopState(state *orm.NewServerShopState, goods []newServerShopTemplateEntry) bool {
	if state == nil {
		return false
	}
	changed := false
	index := make(map[uint32]int, len(state.Goods))
	for i := range state.Goods {
		index[state.Goods[i].ID] = i
		if state.Goods[i].BoughtRecord == nil {
			state.Goods[i].BoughtRecord = []uint32{}
			changed = true
		}
	}
	ordered := make([]orm.NewServerShopGoodsState, 0, len(goods))
	for _, entry := range goods {
		i, ok := index[entry.ID]
		if !ok {
			ordered = append(ordered, orm.NewServerShopGoodsState{ID: entry.ID, Count: entry.GoodsPurchaseLimit, BoughtRecord: []uint32{}})
			changed = true
			continue
		}
		current := state.Goods[i]
		if current.Count > entry.GoodsPurchaseLimit {
			current.Count = entry.GoodsPurchaseLimit
			changed = true
		}
		ordered = append(ordered, current)
	}
	if len(ordered) != len(state.Goods) {
		changed = true
	}
	state.Goods = ordered
	return changed
}

func newServerShopResponseGoods(activity *newServerShopActivity, state *orm.NewServerShopState) []*protobuf.ACT_GOODS_INFO {
	stateByID := make(map[uint32]orm.NewServerShopGoodsState, len(state.Goods))
	for i := range state.Goods {
		stateByID[state.Goods[i].ID] = state.Goods[i]
	}
	result := make([]*protobuf.ACT_GOODS_INFO, 0, len(activity.Goods))
	for _, entry := range activity.Goods {
		gs := stateByID[entry.ID]
		info := &protobuf.ACT_GOODS_INFO{Id: proto.Uint32(entry.ID), Count: proto.Uint32(gs.Count), BoughtRecord: gs.BoughtRecord}
		result = append(result, info)
	}
	return result
}

func consumeNewServerShopCostTx(ctx context.Context, tx pgx.Tx, client *connection.Client, entry newServerShopTemplateEntry, purchaseCount uint32) error {
	totalCost := entry.ResourceNum * purchaseCount
	switch entry.ResourceCategory {
	case consts.DROP_TYPE_RESOURCE:
		if !client.Commander.HasEnoughResource(entry.ResourceType, totalCost) {
			return fmt.Errorf("insufficient resource")
		}
		return client.Commander.ConsumeResourceTx(ctx, tx, entry.ResourceType, totalCost)
	case consts.DROP_TYPE_ITEM:
		if !client.Commander.HasEnoughItem(entry.ResourceType, totalCost) {
			return fmt.Errorf("insufficient item")
		}
		return client.Commander.ConsumeItemTx(ctx, tx, entry.ResourceType, totalCost)
	default:
		return fmt.Errorf("unsupported cost category %d", entry.ResourceCategory)
	}
}

func applyNewServerShopDropsTx(ctx context.Context, tx pgx.Tx, client *connection.Client, drops []*protobuf.DROPINFO) error {
	for _, drop := range drops {
		switch drop.GetType() {
		case consts.DROP_TYPE_RESOURCE:
			if err := client.Commander.AddResourceTx(ctx, tx, drop.GetId(), drop.GetNumber()); err != nil {
				return err
			}
		case consts.DROP_TYPE_ITEM, consts.DROP_TYPE_LOVE_LETTER:
			if err := client.Commander.AddItemTx(ctx, tx, drop.GetId(), drop.GetNumber()); err != nil {
				return err
			}
		case consts.DROP_TYPE_EQUIP:
			if err := addOwnedEquipmentPGXTx(ctx, tx, client.Commander, drop.GetId(), drop.GetNumber()); err != nil {
				return err
			}
		case consts.DROP_TYPE_SHIP:
			for i := uint32(0); i < drop.GetNumber(); i++ {
				if _, err := client.Commander.AddShipTx(ctx, tx, drop.GetId()); err != nil {
					return err
				}
			}
		case consts.DROP_TYPE_FURNITURE:
			now := uint32(time.Now().Unix())
			if err := orm.AddCommanderFurnitureTx(ctx, tx, client.Commander.CommanderID, drop.GetId(), drop.GetNumber(), now); err != nil {
				return err
			}
		case consts.DROP_TYPE_SKIN:
			for i := uint32(0); i < drop.GetNumber(); i++ {
				if err := client.Commander.GiveSkinTx(ctx, tx, drop.GetId()); err != nil {
					return err
				}
			}
		case consts.DROP_TYPE_VITEM:
			continue
		default:
			return fmt.Errorf("unsupported reward type %d", drop.GetType())
		}
	}
	return nil
}

func sortedUniqueUint32(values []uint32) []uint32 {
	if len(values) == 0 {
		return []uint32{}
	}
	set := make(map[uint32]struct{}, len(values))
	for _, value := range values {
		set[value] = struct{}{}
	}
	out := make([]uint32, 0, len(set))
	for value := range set {
		out = append(out, value)
	}
	sort.Slice(out, func(i int, j int) bool {
		return out[i] < out[j]
	})
	return out
}
