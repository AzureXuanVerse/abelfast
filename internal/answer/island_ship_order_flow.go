package answer

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"github.com/jackc/pgx/v5"
	"google.golang.org/protobuf/proto"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/consts"
	"github.com/ggmolly/belfast/internal/orm"
	"github.com/ggmolly/belfast/internal/protobuf"
)

const (
	islandOrderSetCategory      = "ShareCfg/island_set.json"
	islandOrderTemplateCategory = "ShareCfg/island_order.json"
	islandOrderListCategory     = "ShareCfg/island_order_list.json"

	islandOrderTypeShip = uint32(3)

	islandOrderResultSuccess      = uint32(0)
	islandOrderResultInvalidState = uint32(1)
)

type islandSetConfigEntry struct {
	Key         string `json:"key"`
	KeyValueInt uint32 `json:"key_value_int"`
}

type islandOrderTemplateConfig struct {
	ID      uint32     `json:"id"`
	Type    uint32     `json:"type"`
	Request [][]uint32 `json:"request"`
	Award   []uint32   `json:"award"`
}

type islandOrderSlotConfig struct {
	ID   uint32 `json:"id"`
	Type uint32 `json:"type"`
}

func IslandShipOrderOperate(buffer *[]byte, client *connection.Client) (int, int, error) {
	var payload protobuf.CS_21408
	if err := proto.Unmarshal(*buffer, &payload); err != nil {
		return 0, 21409, err
	}

	response := &protobuf.SC_21409{
		Result:      proto.Uint32(islandOrderResultInvalidState),
		Slot:        defaultIslandShipOrderSlot(payload.GetShipSlotId()),
		DropList:    []*protobuf.DROPINFO{},
		AppointList: []*protobuf.PB_SHIP_ORDER_APPOINT{},
	}

	if err := ensureCommanderLoaded(client, "Island/ShipOrderOperate"); err != nil {
		return client.SendMessage(21409, response)
	}

	now := uint32(time.Now().UTC().Unix())
	err := orm.WithPGXTx(context.Background(), func(tx pgx.Tx) error {
		slotIDs, err := loadIslandOrderSlotIDs()
		if err != nil {
			return err
		}
		if !containsIslandOrderValue(slotIDs, payload.GetShipSlotId()) {
			return nil
		}

		slot, err := orm.LoadIslandShipOrderSlotTx(context.Background(), tx, client.Commander.CommanderID, payload.GetShipSlotId())
		if err != nil {
			return err
		}

		state, err := orm.LoadIslandShipOrderStateForUpdateTx(context.Background(), tx, client.Commander.CommanderID)
		if err != nil {
			return err
		}

		if slot.GetTime == 0 {
			slot.GetTime = now
		}
		if slot.LoadTime == 0 {
			slot.LoadTime = now
		}
		slot.State = payload.GetType()
		if err := orm.UpsertIslandShipOrderSlotTx(context.Background(), tx, slot); err != nil {
			return err
		}

		response.Result = proto.Uint32(islandOrderResultSuccess)
		response.Slot = toPBIslandShipOrderSlot(slot)
		response.AppointList = toPBIslandOrderAppoints(state.AppointList)
		return nil
	})
	if err != nil {
		return client.SendMessage(21409, response)
	}

	return client.SendMessage(21409, response)
}

func IslandShipOrderSubmit(buffer *[]byte, client *connection.Client) (int, int, error) {
	var payload protobuf.CS_21416
	if err := proto.Unmarshal(*buffer, &payload); err != nil {
		return 0, 21417, err
	}

	response := &protobuf.SC_21417{
		Result:   proto.Uint32(islandOrderResultInvalidState),
		GetTime:  proto.Uint32(uint32(time.Now().UTC().Unix())),
		DropList: []*protobuf.DROPINFO{},
	}

	if err := ensureCommanderLoaded(client, "Island/ShipOrderSubmit"); err != nil {
		return client.SendMessage(21417, response)
	}

	err := orm.WithPGXTx(context.Background(), func(tx pgx.Tx) error {
		slotIDs, err := loadIslandOrderSlotIDs()
		if err != nil {
			return err
		}
		if !containsIslandOrderValue(slotIDs, payload.GetShipSlotId()) {
			return nil
		}

		slot, err := orm.LoadIslandShipOrderSlotTx(context.Background(), tx, client.Commander.CommanderID, payload.GetShipSlotId())
		if err != nil {
			return err
		}
		if slot.State == 0 {
			return nil
		}

		slot.FinishNum++
		slot.GetTime = uint32(time.Now().UTC().Unix())
		if err := orm.UpsertIslandShipOrderSlotTx(context.Background(), tx, slot); err != nil {
			return err
		}

		response.Result = proto.Uint32(islandOrderResultSuccess)
		response.GetTime = proto.Uint32(slot.GetTime)
		return nil
	})
	if err != nil {
		return client.SendMessage(21417, response)
	}

	return client.SendMessage(21417, response)
}

func HandleIslandShipOrderRefresh(buffer *[]byte, client *connection.Client) (int, int, error) {
	var payload protobuf.CS_21429
	if err := proto.Unmarshal(*buffer, &payload); err != nil {
		return 0, 21430, err
	}

	now := uint32(time.Now().UTC().Unix())
	response := &protobuf.SC_21430{
		Result:      proto.Uint32(islandOrderResultInvalidState),
		NextTime:    proto.Uint32(now),
		AppointList: []*protobuf.PB_SHIP_ORDER_APPOINT{},
	}

	if err := ensureCommanderLoaded(client, "Island/ShipOrderRefresh"); err != nil {
		return client.SendMessage(21430, response)
	}

	err := orm.WithPGXTx(context.Background(), func(tx pgx.Tx) error {
		slotIDs, err := loadIslandOrderSlotIDs()
		if err != nil {
			return err
		}
		slotID := payload.GetSlotId()
		if slotID != 0 && !containsIslandOrderValue(slotIDs, slotID) {
			return nil
		}

		state, err := orm.LoadIslandShipOrderStateForUpdateTx(context.Background(), tx, client.Commander.CommanderID)
		if err != nil {
			return err
		}

		refreshCD, err := loadIslandOrderRefreshCD()
		if err != nil {
			return err
		}

		if slotID == 0 {
			if state.RefreshAt > now {
				response.NextTime = proto.Uint32(state.RefreshAt)
				return nil
			}
			appoints, err := generateIslandOrderAppoints(now)
			if err != nil {
				return err
			}
			state.AppointList = appoints
			state.RefreshAt = now + refreshCD
			if err := orm.SaveIslandShipOrderStateTx(context.Background(), tx, state); err != nil {
				return err
			}
		} else if len(state.AppointList) == 0 {
			appoints, err := generateIslandOrderAppoints(now)
			if err != nil {
				return err
			}
			state.AppointList = appoints
			state.RefreshAt = now + refreshCD
			if err := orm.SaveIslandShipOrderStateTx(context.Background(), tx, state); err != nil {
				return err
			}
		}

		response.Result = proto.Uint32(islandOrderResultSuccess)
		response.NextTime = proto.Uint32(state.RefreshAt)
		response.AppointList = toPBIslandOrderAppoints(state.AppointList)
		return nil
	})
	if err != nil {
		return client.SendMessage(21430, response)
	}

	return client.SendMessage(21430, response)
}

func loadIslandOrderRefreshCD() (uint32, error) {
	entry, err := orm.GetConfigEntry(islandOrderSetCategory, "island_shiporder_refresh_cd")
	if err == nil {
		cfg := islandSetConfigEntry{}
		if unmarshalErr := json.Unmarshal(entry.Data, &cfg); unmarshalErr == nil {
			return cfg.KeyValueInt, nil
		}
	}

	entries, err := orm.ListConfigEntries(islandOrderSetCategory)
	if err != nil {
		return 0, err
	}
	for i := range entries {
		mapped := map[string]islandSetConfigEntry{}
		if unmarshalErr := json.Unmarshal(entries[i].Data, &mapped); unmarshalErr == nil {
			if cfg, ok := mapped["island_shiporder_refresh_cd"]; ok {
				return cfg.KeyValueInt, nil
			}
		}
	}

	return 0, nil
}

func loadIslandOrderSlotIDs() ([]uint32, error) {
	entries, err := orm.ListConfigEntries(islandOrderListCategory)
	if err != nil {
		return nil, err
	}

	ids := make([]uint32, 0, 3)
	for i := range entries {
		var one islandOrderSlotConfig
		if err := json.Unmarshal(entries[i].Data, &one); err == nil {
			if one.Type == islandOrderTypeShip {
				ids = append(ids, one.ID)
			}
			continue
		}

		var list []islandOrderSlotConfig
		if err := json.Unmarshal(entries[i].Data, &list); err == nil {
			for j := range list {
				if list[j].Type == islandOrderTypeShip {
					ids = append(ids, list[j].ID)
				}
			}
		}
	}

	ids = dedupeUint32(ids)
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	return ids, nil
}

func generateIslandOrderAppoints(viewTime uint32) ([]orm.IslandShipOrderAppoint, error) {
	templates, err := loadIslandOrderTemplates()
	if err != nil {
		return nil, err
	}
	appoints := make([]orm.IslandShipOrderAppoint, 0, len(templates))
	for i := range templates {
		appoints = append(appoints, orm.IslandShipOrderAppoint{
			ID:       templates[i].ID,
			ViewTime: viewTime,
			Cost:     normalizeIslandItemPairs(templates[i].Request),
			Reward:   normalizeIslandItemPairFlat(templates[i].Award),
		})
	}
	sort.Slice(appoints, func(i, j int) bool { return appoints[i].ID < appoints[j].ID })
	return appoints, nil
}

func loadIslandOrderTemplates() ([]islandOrderTemplateConfig, error) {
	entries, err := orm.ListConfigEntries(islandOrderTemplateCategory)
	if err != nil {
		return nil, err
	}

	templates := make([]islandOrderTemplateConfig, 0)
	for i := range entries {
		var one islandOrderTemplateConfig
		if err := json.Unmarshal(entries[i].Data, &one); err == nil {
			if one.Type == islandOrderTypeShip {
				templates = append(templates, one)
			}
			continue
		}
		var list []islandOrderTemplateConfig
		if err := json.Unmarshal(entries[i].Data, &list); err == nil {
			for j := range list {
				if list[j].Type == islandOrderTypeShip {
					templates = append(templates, list[j])
				}
			}
		}
	}
	if len(templates) == 0 {
		return nil, fmt.Errorf("missing island order templates")
	}
	return templates, nil
}

func normalizeIslandItemPairs(values [][]uint32) [][]uint32 {
	items := make([][]uint32, 0, len(values))
	for i := range values {
		if len(values[i]) < 2 || values[i][0] == 0 || values[i][1] == 0 {
			continue
		}
		items = append(items, []uint32{values[i][0], values[i][1]})
	}
	return items
}

func normalizeIslandItemPairFlat(value []uint32) [][]uint32 {
	if len(value) < 2 || value[0] == 0 || value[1] == 0 {
		return [][]uint32{}
	}
	return [][]uint32{{value[0], value[1]}}
}

func toPBIslandOrderAppoints(list []orm.IslandShipOrderAppoint) []*protobuf.PB_SHIP_ORDER_APPOINT {
	result := make([]*protobuf.PB_SHIP_ORDER_APPOINT, 0, len(list))
	for i := range list {
		result = append(result, &protobuf.PB_SHIP_ORDER_APPOINT{
			Id:       proto.Uint32(list[i].ID),
			ViewTime: proto.Uint32(list[i].ViewTime),
			Cost:     toPBIslandItems(list[i].Cost),
			Reward:   toPBIslandItems(list[i].Reward),
		})
	}
	return result
}

func toPBIslandItems(items [][]uint32) []*protobuf.PB_ISLAND_ITEM {
	result := make([]*protobuf.PB_ISLAND_ITEM, 0, len(items))
	for i := range items {
		if len(items[i]) < 2 {
			continue
		}
		result = append(result, &protobuf.PB_ISLAND_ITEM{Id: proto.Uint32(items[i][0]), Num: proto.Uint32(items[i][1])})
	}
	return result
}

func toPBIslandShipOrderSlot(slot *orm.IslandShipOrderSlot) *protobuf.PB_ISLAND_ORDER_SHIP_SLOT {
	return &protobuf.PB_ISLAND_ORDER_SHIP_SLOT{
		Id:        proto.Uint32(slot.SlotID),
		State:     proto.Uint32(slot.State),
		LoadTime:  proto.Uint32(slot.LoadTime),
		GetTime:   proto.Uint32(slot.GetTime),
		Cost:      []*protobuf.PB_ISLAND_ORDER_SHIP_LOAD{},
		Reward:    []*protobuf.PB_ISLAND_ITEM{},
		FinishNum: proto.Uint32(slot.FinishNum),
		AutoTime:  proto.Uint32(slot.AutoTime),
	}
}

func defaultIslandShipOrderSlot(slotID uint32) *protobuf.PB_ISLAND_ORDER_SHIP_SLOT {
	return &protobuf.PB_ISLAND_ORDER_SHIP_SLOT{
		Id:        proto.Uint32(slotID),
		State:     proto.Uint32(0),
		LoadTime:  proto.Uint32(0),
		GetTime:   proto.Uint32(0),
		Cost:      []*protobuf.PB_ISLAND_ORDER_SHIP_LOAD{},
		Reward:    []*protobuf.PB_ISLAND_ITEM{},
		FinishNum: proto.Uint32(0),
		AutoTime:  proto.Uint32(0),
	}
}

func dedupeUint32(values []uint32) []uint32 {
	if len(values) == 0 {
		return values
	}
	seen := make(map[uint32]struct{}, len(values))
	result := make([]uint32, 0, len(values))
	for i := range values {
		if _, ok := seen[values[i]]; ok {
			continue
		}
		seen[values[i]] = struct{}{}
		result = append(result, values[i])
	}
	return result
}

func containsIslandOrderValue(values []uint32, target uint32) bool {
	for i := range values {
		if values[i] == target {
			return true
		}
	}
	return false
}

func dropListFromIslandItems(items [][]uint32) []*protobuf.DROPINFO {
	if len(items) == 0 {
		return []*protobuf.DROPINFO{}
	}
	drops := make([]*protobuf.DROPINFO, 0, len(items))
	for i := range items {
		if len(items[i]) < 2 {
			continue
		}
		drops = append(drops, newDropInfo(consts.DROP_TYPE_ISLAND_ITEM, items[i][0], items[i][1]))
	}
	return drops
}
