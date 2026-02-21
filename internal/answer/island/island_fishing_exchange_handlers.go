package island

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"google.golang.org/protobuf/proto"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/consts"
	"github.com/ggmolly/belfast/internal/db"
	"github.com/ggmolly/belfast/internal/orm"
	"github.com/ggmolly/belfast/internal/protobuf"
)

const (
	islandFishingBaitCategory   = "ShareCfg/island_fish_bait.json"
	islandFishingBaitCategoryLC = "sharecfgdata/island_fish_bait.json"
	islandExchangeCategory      = "ShareCfg/island_exchange_template.json"
	islandExchangeCategoryLC    = "sharecfgdata/island_exchange_template.json"

	islandFishingEndResultCancel  = uint32(1)
	islandFishingEndResultFail    = uint32(2)
	islandFishingEndResultSuccess = uint32(3)

	islandFishingResultOK      = uint32(0)
	islandFishingResultInvalid = uint32(1)
	islandFishingResultLack    = uint32(2)
	islandFishingResultPersist = uint32(5)
)

type islandFishingCost struct {
	DropType uint32
	ID       uint32
	Count    uint32
}

type islandFishingBaitConfig struct {
	ID      uint32
	FishRod uint32
	Costs   []islandFishingCost
}

type islandExchangeTemplate struct {
	ID          uint32
	OriginType  uint32
	OriginID    uint32
	OriginCount uint32
	TargetType  uint32
	TargetID    uint32
	TargetCount uint32
}

var errIslandFishingInsufficientCostRollback = errors.New("island fishing insufficient cost rollback")

func IslandFishingResult(buffer *[]byte, client *connection.Client) (int, int, error) {
	var payload protobuf.CS_21062
	if err := proto.Unmarshal(*buffer, &payload); err != nil {
		return 0, 21063, err
	}

	response := &protobuf.SC_21063{Result: proto.Uint32(islandFishingResultInvalid)}
	if err := ensureCommanderLoaded(client, "Island/FishingResult"); err != nil {
		response.Result = proto.Uint32(islandFishingResultPersist)
		return client.SendMessage(21063, response)
	}
	if payload.GetIslandId() == 0 || payload.GetPointId() == 0 || !isValidIslandFishingEndResult(payload.GetEndResult()) {
		return client.SendMessage(21063, response)
	}

	roll, ok := consumeIslandFishingRoll(client.Commander.CommanderID, payload.GetIslandId(), payload.GetPointId(), islandFishingNow())
	if !ok {
		return client.SendMessage(21063, response)
	}

	if payload.GetEndResult() == islandFishingEndResultSuccess {
		err := db.DefaultStore.WithPGXTx(context.Background(), func(tx pgx.Tx) error {
			state, err := orm.GetIslandFishingStateForUpdateTx(context.Background(), tx, client.Commander.CommanderID)
			if err != nil && !db.IsNotFound(err) {
				response.Result = proto.Uint32(islandFishingResultPersist)
				return err
			}
			if state == nil {
				state = &orm.IslandFishingState{CommanderID: client.Commander.CommanderID, FishWeights: []orm.IslandFishWeightState{}}
			}
			state.FishWeights = upsertIslandFishWeight(state.FishWeights, roll.FishID, roll.Weight, roll.GoldState)
			if err := orm.UpsertIslandFishingStateTx(context.Background(), tx, state); err != nil {
				response.Result = proto.Uint32(islandFishingResultPersist)
				return err
			}
			return nil
		})
		if err != nil {
			return client.SendMessage(21063, response)
		}
	}

	response.Result = proto.Uint32(islandFishingResultOK)
	return client.SendMessage(21063, response)
}

func IslandExchangeLure(buffer *[]byte, client *connection.Client) (int, int, error) {
	var payload protobuf.CS_21064
	if err := proto.Unmarshal(*buffer, &payload); err != nil {
		return 0, 21065, err
	}

	response := &protobuf.SC_21065{Result: proto.Uint32(islandFishingResultInvalid)}
	if payload.GetBaitId() == 0 {
		return client.SendMessage(21065, response)
	}
	if err := ensureCommanderLoaded(client, "Island/ExchangeLure"); err != nil {
		response.Result = proto.Uint32(islandFishingResultPersist)
		return client.SendMessage(21065, response)
	}

	baitCfg, found, err := loadIslandFishingBaitConfig(payload.GetBaitId())
	if err != nil {
		response.Result = proto.Uint32(islandFishingResultPersist)
		return client.SendMessage(21065, response)
	}
	if !found {
		return client.SendMessage(21065, response)
	}

	err = db.DefaultStore.WithPGXTx(context.Background(), func(tx pgx.Tx) error {
		for i := range baitCfg.Costs {
			if err := consumeIslandFishingCostTx(context.Background(), tx, client, baitCfg.Costs[i]); err != nil {
				if isInsufficientIslandFishingCost(err) {
					response.Result = proto.Uint32(islandFishingResultLack)
					return nil
				}
				response.Result = proto.Uint32(islandFishingResultPersist)
				return err
			}
		}

		state, err := orm.GetIslandFishingStateForUpdateTx(context.Background(), tx, client.Commander.CommanderID)
		if err != nil && !db.IsNotFound(err) {
			response.Result = proto.Uint32(islandFishingResultPersist)
			return err
		}
		if state == nil {
			state = &orm.IslandFishingState{CommanderID: client.Commander.CommanderID, FishWeights: []orm.IslandFishWeightState{}}
		}
		state.BaitID = baitCfg.ID
		if baitCfg.FishRod != 0 {
			state.FishRod = baitCfg.FishRod
		} else {
			state.FishRod = baitCfg.ID
		}
		if err := orm.UpsertIslandFishingStateTx(context.Background(), tx, state); err != nil {
			response.Result = proto.Uint32(islandFishingResultPersist)
			return err
		}

		response.Result = proto.Uint32(islandFishingResultOK)
		return nil
	})
	if err != nil {
		return client.SendMessage(21065, response)
	}
	return client.SendMessage(21065, response)
}

func IslandExchangeItem(buffer *[]byte, client *connection.Client) (int, int, error) {
	var payload protobuf.CS_21066
	if err := proto.Unmarshal(*buffer, &payload); err != nil {
		return 0, 21067, err
	}

	response := &protobuf.SC_21067{Result: proto.Uint32(islandFishingResultInvalid), DropList: []*protobuf.DROPINFO{}}
	if err := ensureCommanderLoaded(client, "Island/ExchangeItem"); err != nil {
		response.Result = proto.Uint32(islandFishingResultPersist)
		return client.SendMessage(21067, response)
	}

	makes := payload.GetMakes()
	if len(makes) == 0 {
		return client.SendMessage(21067, response)
	}

	costs := make([]islandFishingCost, 0, len(makes))
	drops := make([]*protobuf.DROPINFO, 0, len(makes))
	for i := range makes {
		if makes[i] == nil || makes[i].GetMakeId() == 0 || makes[i].GetNum() == 0 {
			return client.SendMessage(21067, response)
		}
		template, found, err := loadIslandExchangeTemplateConfig(makes[i].GetMakeId())
		if err != nil {
			response.Result = proto.Uint32(islandFishingResultPersist)
			return client.SendMessage(21067, response)
		}
		if !found {
			return client.SendMessage(21067, response)
		}
		totalCost, ok := mulUint32Checked(template.OriginCount, makes[i].GetNum())
		if !ok {
			return client.SendMessage(21067, response)
		}
		totalDrop, ok := mulUint32Checked(template.TargetCount, makes[i].GetNum())
		if !ok {
			return client.SendMessage(21067, response)
		}
		costs = append(costs, islandFishingCost{DropType: template.OriginType, ID: template.OriginID, Count: totalCost})
		drops = append(drops, newDropInfo(template.TargetType, template.TargetID, totalDrop))
	}

	err := db.DefaultStore.WithPGXTx(context.Background(), func(tx pgx.Tx) error {
		for i := range costs {
			if err := consumeIslandFishingCostTx(context.Background(), tx, client, costs[i]); err != nil {
				if isInsufficientIslandFishingCost(err) {
					response.Result = proto.Uint32(islandFishingResultLack)
					return errIslandFishingInsufficientCostRollback
				}
				response.Result = proto.Uint32(islandFishingResultPersist)
				return err
			}
		}
		if err := applyIslandDropsTx(context.Background(), tx, client, drops); err != nil {
			response.Result = proto.Uint32(islandFishingResultPersist)
			return err
		}
		response.Result = proto.Uint32(islandFishingResultOK)
		response.DropList = mergeDropList(drops)
		return nil
	})
	if err != nil {
		if errors.Is(err, errIslandFishingInsufficientCostRollback) {
			return client.SendMessage(21067, response)
		}
		return client.SendMessage(21067, response)
	}

	return client.SendMessage(21067, response)
}

func isValidIslandFishingEndResult(endResult uint32) bool {
	switch endResult {
	case islandFishingEndResultCancel, islandFishingEndResultFail, islandFishingEndResultSuccess:
		return true
	default:
		return false
	}
}

func upsertIslandFishWeight(weights []orm.IslandFishWeightState, fishID uint32, weight uint32, goldState uint32) []orm.IslandFishWeightState {
	for i := range weights {
		if weights[i].FishID != fishID {
			continue
		}
		if weights[i].MinWeight == 0 || weight < weights[i].MinWeight {
			weights[i].MinWeight = weight
		}
		if weight > weights[i].MaxWeight {
			weights[i].MaxWeight = weight
		}
		if goldState > weights[i].GoldState {
			weights[i].GoldState = goldState
		}
		return weights
	}
	return append(weights, orm.IslandFishWeightState{FishID: fishID, MinWeight: weight, MaxWeight: weight, GoldState: goldState})
}

func consumeIslandFishingCostTx(ctx context.Context, tx pgx.Tx, client *connection.Client, cost islandFishingCost) error {
	switch cost.DropType {
	case consts.DROP_TYPE_RESOURCE:
		return client.Commander.ConsumeResourceTx(ctx, tx, cost.ID, cost.Count)
	case consts.DROP_TYPE_ITEM:
		return client.Commander.ConsumeItemTx(ctx, tx, cost.ID, cost.Count)
	case consts.DROP_TYPE_ISLAND_ITEM:
		return orm.ConsumeIslandInventoryCheckedTx(ctx, tx, client.Commander.CommanderID, cost.ID, cost.Count)
	default:
		return fmt.Errorf("unsupported consume drop type %d", cost.DropType)
	}
}

func isInsufficientIslandFishingCost(err error) bool {
	if err == nil {
		return false
	}
	if db.IsNotFound(err) || errors.Is(err, orm.ErrInsufficientIslandInventory) {
		return true
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "not enough")
}

func loadIslandFishingBaitConfig(baitID uint32) (*islandFishingBaitConfig, bool, error) {
	raw, found, err := loadIslandConfigRawByID(islandFishingBaitCategory, islandFishingBaitCategoryLC, baitID)
	if err != nil || !found {
		return nil, found, err
	}

	var mapped map[string]json.RawMessage
	if err := json.Unmarshal(raw, &mapped); err != nil {
		return nil, false, err
	}

	cfg := &islandFishingBaitConfig{ID: baitID, FishRod: baitID, Costs: []islandFishingCost{}}
	if v, ok := rawUint32(mapped["id"]); ok {
		cfg.ID = v
	}
	if v, ok := rawUint32(mapped["fish_rod"]); ok {
		cfg.FishRod = v
	} else if v, ok := rawUint32(mapped["rod"]); ok {
		cfg.FishRod = v
	}

	costKeys := []string{"cost", "consume", "consume_list", "exchange_cost", "use_item", "cost_item"}
	for i := range costKeys {
		rows, ok, err := decodeUint32Rows(mapped[costKeys[i]])
		if err != nil {
			return nil, false, err
		}
		if !ok {
			continue
		}
		for _, row := range rows {
			if len(row) < 2 {
				continue
			}
			cost := islandFishingCost{DropType: consts.DROP_TYPE_ISLAND_ITEM, ID: row[0], Count: row[1]}
			if len(row) >= 3 {
				cost.DropType = row[0]
				cost.ID = row[1]
				cost.Count = row[2]
			}
			if cost.ID == 0 || cost.Count == 0 {
				continue
			}
			cfg.Costs = append(cfg.Costs, cost)
		}
		break
	}

	if resourceType, ok := rawUint32(mapped["resource_type"]); ok {
		if resourceNum, ok := rawUint32(mapped["resource_num"]); ok && resourceNum > 0 {
			cfg.Costs = append(cfg.Costs, islandFishingCost{DropType: consts.DROP_TYPE_RESOURCE, ID: resourceType, Count: resourceNum})
		}
	}

	return cfg, true, nil
}

func loadIslandExchangeTemplateConfig(makeID uint32) (*islandExchangeTemplate, bool, error) {
	raw, found, err := loadIslandConfigRawByID(islandExchangeCategory, islandExchangeCategoryLC, makeID)
	if err != nil || !found {
		return nil, found, err
	}
	var mapped map[string]json.RawMessage
	if err := json.Unmarshal(raw, &mapped); err != nil {
		return nil, false, err
	}

	origin, ok, err := decodeUint32Row(mapped["origin_item"])
	if err != nil {
		return nil, false, err
	}
	if !ok {
		return nil, false, nil
	}
	target, ok, err := decodeUint32Row(mapped["target_item"])
	if err != nil {
		return nil, false, err
	}
	if !ok {
		return nil, false, nil
	}

	template := &islandExchangeTemplate{ID: makeID, OriginCount: 1, TargetCount: 1}
	if v, ok := rawUint32(mapped["id"]); ok {
		template.ID = v
	}

	if len(origin) < 2 || len(target) < 2 {
		return nil, false, nil
	}
	template.OriginType = origin[0]
	template.OriginID = origin[1]
	if len(origin) >= 3 && origin[2] > 0 {
		template.OriginCount = origin[2]
	}
	template.TargetType = target[0]
	template.TargetID = target[1]
	if len(target) >= 3 && target[2] > 0 {
		template.TargetCount = target[2]
	}
	if v, ok := rawUint32(mapped["target_num"]); ok && v > 0 {
		template.TargetCount = v
	}

	if template.OriginType == 0 || template.OriginID == 0 || template.TargetType == 0 || template.TargetID == 0 || template.OriginCount == 0 || template.TargetCount == 0 {
		return nil, false, nil
	}
	return template, true, nil
}

func loadIslandConfigRawByID(category string, fallbackCategory string, id uint32) (json.RawMessage, bool, error) {
	key := fmt.Sprintf("%d", id)
	if entry, err := orm.GetConfigEntry(category, key); err == nil {
		return entry.Data, true, nil
	}
	if entry, err := orm.GetConfigEntry(fallbackCategory, key); err == nil {
		return entry.Data, true, nil
	}

	entries, err := orm.ListConfigEntries(category)
	if err != nil || len(entries) == 0 {
		entries, err = orm.ListConfigEntries(fallbackCategory)
		if err != nil {
			return nil, false, err
		}
	}
	for i := range entries {
		var mapped map[string]json.RawMessage
		if err := json.Unmarshal(entries[i].Data, &mapped); err != nil {
			continue
		}
		if v, ok := rawUint32(mapped["id"]); ok && v == id {
			return entries[i].Data, true, nil
		}
	}
	return nil, false, nil
}

func decodeUint32Rows(raw json.RawMessage) ([][]uint32, bool, error) {
	if len(raw) == 0 {
		return nil, false, nil
	}
	var matrix [][]uint32
	if err := json.Unmarshal(raw, &matrix); err == nil {
		return matrix, true, nil
	}
	var row []uint32
	if err := json.Unmarshal(raw, &row); err == nil {
		return [][]uint32{row}, true, nil
	}
	var generic any
	if err := json.Unmarshal(raw, &generic); err != nil {
		return nil, false, err
	}
	rows := [][]uint32{}
	switch typed := generic.(type) {
	case []any:
		if len(typed) == 0 {
			return [][]uint32{}, true, nil
		}
		if _, ok := typed[0].([]any); ok {
			for i := range typed {
				items, ok := typed[i].([]any)
				if !ok {
					continue
				}
				decoded := make([]uint32, 0, len(items))
				for j := range items {
					v, ok := anyToUint32(items[j])
					if !ok {
						decoded = nil
						break
					}
					decoded = append(decoded, v)
				}
				if len(decoded) > 0 {
					rows = append(rows, decoded)
				}
			}
			return rows, true, nil
		}
		decoded := make([]uint32, 0, len(typed))
		for i := range typed {
			v, ok := anyToUint32(typed[i])
			if !ok {
				continue
			}
			decoded = append(decoded, v)
		}
		return [][]uint32{decoded}, true, nil
	default:
		return nil, false, nil
	}
}

func decodeUint32Row(raw json.RawMessage) ([]uint32, bool, error) {
	rows, ok, err := decodeUint32Rows(raw)
	if err != nil || !ok || len(rows) == 0 {
		return nil, ok, err
	}
	return rows[0], true, nil
}

func anyToUint32(value any) (uint32, bool) {
	switch typed := value.(type) {
	case float64:
		if typed < 0 || typed > float64(^uint32(0)) {
			return 0, false
		}
		return uint32(typed), true
	case int:
		if typed < 0 {
			return 0, false
		}
		return uint32(typed), true
	case int64:
		if typed < 0 || typed > int64(^uint32(0)) {
			return 0, false
		}
		return uint32(typed), true
	case uint32:
		return typed, true
	default:
		return 0, false
	}
}

func mulUint32Checked(left uint32, right uint32) (uint32, bool) {
	if right != 0 && left > ^uint32(0)/right {
		return 0, false
	}
	return left * right, true
}
