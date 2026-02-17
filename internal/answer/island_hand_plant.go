package answer

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"google.golang.org/protobuf/proto"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/consts"
	"github.com/ggmolly/belfast/internal/db"
	"github.com/ggmolly/belfast/internal/orm"
	"github.com/ggmolly/belfast/internal/protobuf"
)

const (
	islandProductionSlotCategory   = "ShareCfg/island_production_slot.json"
	islandProductionSlotCategoryLC = "sharecfgdata/island_production_slot.json"

	islandHandPlantSlotType = uint32(1)

	islandHandPlantResultSuccess = uint32(0)
	islandHandPlantResultFailure = uint32(1)
)

var errIslandHandPlantAbort = errors.New("island hand-plant business rule failure")

type islandProductionSlotConfig struct {
	ID      uint32   `json:"id"`
	Place   uint32   `json:"place"`
	Type    uint32   `json:"type"`
	Formula []uint32 `json:"formula"`
}

type islandFormulaConfigV2 struct {
	ID          uint32            `json:"id"`
	Cost        [][]uint32        `json:"cost"`
	Workload    uint32            `json:"workload"`
	ItemID      uint32            `json:"item_id"`
	DropDisplay []json.RawMessage `json:"drop_display"`
}

type islandSetEntry struct {
	ID          uint32            `json:"id"`
	Key         string            `json:"key"`
	Description []json.RawMessage `json:"description"`
	KeyValue    uint32            `json:"key_value"`
	Value       uint32            `json:"value"`

	BaseEfficiency uint32 `json:"base_efficiency"`
}

func StartIslandHandPlant(buffer *[]byte, client *connection.Client) (int, int, error) {
	var payload protobuf.CS_21509
	if err := proto.Unmarshal(*buffer, &payload); err != nil {
		return 0, 21510, err
	}

	response := &protobuf.SC_21510{
		Result:   proto.Uint32(islandHandPlantResultSuccess),
		HandList: []*protobuf.PB_ISLAND_HAND_AREA{},
	}

	buildID := payload.GetBuildId()
	slotIDs, ok := normalizeUniqueIDs(payload.GetSlotList())
	if buildID == 0 || payload.GetFormulaId() == 0 || !ok {
		response.Result = proto.Uint32(islandHandPlantResultFailure)
		return client.SendMessage(21510, response)
	}

	if err := ensureCommanderLoaded(client, "Island/HandPlantStart"); err != nil {
		response.Result = proto.Uint32(islandHandPlantResultFailure)
		return client.SendMessage(21510, response)
	}

	now := uint32(time.Now().UTC().Unix())
	err := db.DefaultStore.WithPGXTx(context.Background(), func(tx pgx.Tx) error {
		slotsCfg, err := loadIslandProductionSlots()
		if err != nil {
			response.Result = proto.Uint32(islandHandPlantResultFailure)
			return err
		}

		formula, exists, err := loadIslandHandPlantFormula(payload.GetFormulaId())
		if err != nil {
			response.Result = proto.Uint32(islandHandPlantResultFailure)
			return err
		}
		if !exists {
			response.Result = proto.Uint32(islandHandPlantResultFailure)
			return errIslandHandPlantAbort
		}

		for _, slotID := range slotIDs {
			slotCfg, found := slotsCfg[slotID]
			if !found || slotCfg.Place != buildID || slotCfg.Type != islandHandPlantSlotType {
				response.Result = proto.Uint32(islandHandPlantResultFailure)
				return errIslandHandPlantAbort
			}
			if !containsFormulaID(slotCfg.Formula, formula.ID) {
				response.Result = proto.Uint32(islandHandPlantResultFailure)
				return errIslandHandPlantAbort
			}
		}

		if err := orm.EnsureIslandHandPlantRowsTx(context.Background(), tx, client.Commander.CommanderID, slotIDs); err != nil {
			response.Result = proto.Uint32(islandHandPlantResultFailure)
			return err
		}

		stateBySlot, err := loadIslandHandPlantStateForSlotsTx(context.Background(), tx, client.Commander.CommanderID, slotIDs, true)
		if err != nil {
			response.Result = proto.Uint32(islandHandPlantResultFailure)
			return err
		}
		for _, slotID := range slotIDs {
			if stateBySlot[slotID].State != 0 {
				response.Result = proto.Uint32(islandHandPlantResultFailure)
				return errIslandHandPlantAbort
			}
		}

		baseEfficiency, ok, err := loadIslandBaseEfficiency()
		if err != nil {
			response.Result = proto.Uint32(islandHandPlantResultFailure)
			return err
		}
		if !ok || baseEfficiency == 0 {
			response.Result = proto.Uint32(islandHandPlantResultFailure)
			return errIslandHandPlantAbort
		}

		for _, cost := range formula.Cost {
			if len(cost) < 2 {
				continue
			}
			itemID := cost[0]
			required := cost[1] * uint32(len(slotIDs))
			if required == 0 {
				continue
			}
			consumed, err := consumeCommanderItemTx(context.Background(), tx, client.Commander.CommanderID, itemID, required)
			if err != nil {
				response.Result = proto.Uint32(islandHandPlantResultFailure)
				return err
			}
			if !consumed {
				response.Result = proto.Uint32(islandHandPlantResultFailure)
				return errIslandHandPlantAbort
			}
		}

		duration := (formula.Workload + baseEfficiency - 1) / baseEfficiency
		if duration == 0 {
			duration = 1
		}

		updated := make([]*protobuf.PB_ISLAND_HAND_AREA, 0, len(slotIDs))
		for _, slotID := range slotIDs {
			entry := &orm.IslandHandPlant{
				CommanderID: client.Commander.CommanderID,
				BuildID:     buildID,
				SlotID:      slotID,
				State:       1,
				FormulaID:   formula.ID,
				StartTime:   now,
				EndTime:     now + duration,
			}
			if err := orm.UpsertIslandHandPlantTx(context.Background(), tx, entry); err != nil {
				response.Result = proto.Uint32(islandHandPlantResultFailure)
				return err
			}
			updated = append(updated, islandHandPlantToProto(entry))
		}

		response.Result = proto.Uint32(islandHandPlantResultSuccess)
		response.HandList = updated
		return nil
	})
	if err != nil && !errors.Is(err, errIslandHandPlantAbort) {
		return client.SendMessage(21510, response)
	}
	if err == nil {
		_ = client.Commander.Load()
	}

	return client.SendMessage(21510, response)
}

func IslandClaimHandPlantAward(buffer *[]byte, client *connection.Client) (int, int, error) {
	var payload protobuf.CS_21511
	if err := proto.Unmarshal(*buffer, &payload); err != nil {
		return 0, 21512, err
	}

	response := &protobuf.SC_21512{
		Result:   proto.Uint32(islandHandPlantResultSuccess),
		DropList: []*protobuf.DROPINFO{},
	}

	buildID := payload.GetBuildId()
	areaIDs, ok := normalizeUniqueIDs(payload.GetAreaIds())
	if buildID == 0 || !ok {
		response.Result = proto.Uint32(islandHandPlantResultFailure)
		return client.SendMessage(21512, response)
	}

	err := db.DefaultStore.WithPGXTx(context.Background(), func(tx pgx.Tx) error {
		slotsCfg, err := loadIslandProductionSlots()
		if err != nil {
			response.Result = proto.Uint32(islandHandPlantResultFailure)
			return err
		}
		for _, areaID := range areaIDs {
			slotCfg, found := slotsCfg[areaID]
			if !found || slotCfg.Place != buildID || slotCfg.Type != islandHandPlantSlotType {
				response.Result = proto.Uint32(islandHandPlantResultFailure)
				return errIslandHandPlantAbort
			}
		}

		stateBySlot, err := loadIslandHandPlantStateForSlotsTx(context.Background(), tx, client.Commander.CommanderID, areaIDs, true)
		if err != nil {
			response.Result = proto.Uint32(islandHandPlantResultFailure)
			return err
		}

		now := uint32(time.Now().UTC().Unix())
		dropMap := make(map[string]*protobuf.DROPINFO)
		for _, areaID := range areaIDs {
			state := stateBySlot[areaID]
			if state.State == 0 || state.FormulaID == 0 || state.EndTime == 0 || state.EndTime > now {
				response.Result = proto.Uint32(islandHandPlantResultFailure)
				return errIslandHandPlantAbort
			}

			formula, exists, err := loadIslandHandPlantFormula(state.FormulaID)
			if err != nil {
				response.Result = proto.Uint32(islandHandPlantResultFailure)
				return err
			}
			if !exists {
				response.Result = proto.Uint32(islandHandPlantResultFailure)
				return errIslandHandPlantAbort
			}
			for _, drop := range islandFormulaDrops(formula) {
				mergeDrop(dropMap, drop.GetType(), drop.GetId(), drop.GetNumber())
			}
		}

		drops := make([]*protobuf.DROPINFO, 0, len(dropMap))
		for _, drop := range dropMap {
			if err := orm.AddIslandInventoryTx(context.Background(), tx, client.Commander.CommanderID, drop.GetId(), drop.GetNumber()); err != nil {
				response.Result = proto.Uint32(islandHandPlantResultFailure)
				return err
			}
			drops = append(drops, drop)
		}
		sort.Slice(drops, func(i, j int) bool {
			if drops[i].GetType() == drops[j].GetType() {
				return drops[i].GetId() < drops[j].GetId()
			}
			return drops[i].GetType() < drops[j].GetType()
		})

		if err := orm.ResetIslandHandPlantsTx(context.Background(), tx, client.Commander.CommanderID, areaIDs); err != nil {
			response.Result = proto.Uint32(islandHandPlantResultFailure)
			return err
		}

		response.Result = proto.Uint32(islandHandPlantResultSuccess)
		response.DropList = drops
		return nil
	})
	if err != nil && !errors.Is(err, errIslandHandPlantAbort) {
		return client.SendMessage(21512, response)
	}

	return client.SendMessage(21512, response)
}

func HandleIslandStopHandPlantHalfway(buffer *[]byte, client *connection.Client) (int, int, error) {
	var payload protobuf.CS_21516
	if err := proto.Unmarshal(*buffer, &payload); err != nil {
		return 0, 21517, err
	}

	response := &protobuf.SC_21517{Result: proto.Uint32(islandHandPlantResultSuccess)}

	buildID := payload.GetBuildId()
	slotIDs, ok := normalizeUniqueIDs(payload.GetSlotList())
	if buildID == 0 || !ok {
		response.Result = proto.Uint32(islandHandPlantResultFailure)
		return client.SendMessage(21517, response)
	}

	err := db.DefaultStore.WithPGXTx(context.Background(), func(tx pgx.Tx) error {
		slotsCfg, err := loadIslandProductionSlots()
		if err != nil {
			response.Result = proto.Uint32(islandHandPlantResultFailure)
			return err
		}
		for _, slotID := range slotIDs {
			slotCfg, found := slotsCfg[slotID]
			if !found || slotCfg.Place != buildID || slotCfg.Type != islandHandPlantSlotType {
				response.Result = proto.Uint32(islandHandPlantResultFailure)
				return errIslandHandPlantAbort
			}
		}

		stateBySlot, err := loadIslandHandPlantStateForSlotsTx(context.Background(), tx, client.Commander.CommanderID, slotIDs, true)
		if err != nil {
			response.Result = proto.Uint32(islandHandPlantResultFailure)
			return err
		}
		for _, slotID := range slotIDs {
			if stateBySlot[slotID].State == 0 {
				response.Result = proto.Uint32(islandHandPlantResultFailure)
				return errIslandHandPlantAbort
			}
		}

		if err := orm.ResetIslandHandPlantsTx(context.Background(), tx, client.Commander.CommanderID, slotIDs); err != nil {
			response.Result = proto.Uint32(islandHandPlantResultFailure)
			return err
		}

		response.Result = proto.Uint32(islandHandPlantResultSuccess)
		return nil
	})
	if err != nil && !errors.Is(err, errIslandHandPlantAbort) {
		return client.SendMessage(21517, response)
	}

	return client.SendMessage(21517, response)
}

func normalizeUniqueIDs(values []uint32) ([]uint32, bool) {
	if len(values) == 0 {
		return nil, false
	}
	seen := make(map[uint32]struct{}, len(values))
	out := make([]uint32, 0, len(values))
	for _, value := range values {
		if value == 0 {
			return nil, false
		}
		if _, exists := seen[value]; exists {
			return nil, false
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out, true
}

func loadIslandProductionSlots() (map[uint32]islandProductionSlotConfig, error) {
	entries, err := listConfigEntriesWithFallback(islandProductionSlotCategory, islandProductionSlotCategoryLC, orm.ListConfigEntries)
	if err != nil {
		return nil, err
	}
	slots := make(map[uint32]islandProductionSlotConfig, len(entries))
	for _, entry := range entries {
		var cfg islandProductionSlotConfig
		if err := json.Unmarshal(entry.Data, &cfg); err != nil {
			return nil, err
		}
		if cfg.ID == 0 {
			id, parseErr := strconv.ParseUint(entry.Key, 10, 32)
			if parseErr == nil {
				cfg.ID = uint32(id)
			}
		}
		slots[cfg.ID] = cfg
	}
	return slots, nil
}

func loadIslandHandPlantFormula(formulaID uint32) (*islandFormulaConfigV2, bool, error) {
	key := fmt.Sprintf("%d", formulaID)
	if entry, err := orm.GetConfigEntry(islandFormulaCategory, key); err == nil {
		var direct islandFormulaConfigV2
		if err := json.Unmarshal(entry.Data, &direct); err == nil {
			if direct.ID == 0 {
				direct.ID = formulaID
			}
			return &direct, true, nil
		}
	}
	if entry, err := orm.GetConfigEntry(islandFormulaCategoryLC, key); err == nil {
		var direct islandFormulaConfigV2
		if err := json.Unmarshal(entry.Data, &direct); err == nil {
			if direct.ID == 0 {
				direct.ID = formulaID
			}
			return &direct, true, nil
		}
	}

	entries, err := listConfigEntriesWithFallback(islandFormulaCategory, islandFormulaCategoryLC, orm.ListConfigEntries)
	if err != nil {
		return nil, false, err
	}
	for i := range entries {
		var single islandFormulaConfigV2
		if err := json.Unmarshal(entries[i].Data, &single); err == nil {
			if single.ID == formulaID {
				return &single, true, nil
			}
		}
		var list []islandFormulaConfigV2
		if err := json.Unmarshal(entries[i].Data, &list); err == nil {
			for j := range list {
				if list[j].ID == formulaID {
					return &list[j], true, nil
				}
			}
		}
	}

	return nil, false, nil
}

func loadIslandBaseEfficiency() (uint32, bool, error) {
	if value, ok, err := loadIslandBaseEfficiencyFromCategory(islandSetCategory); ok || err != nil {
		return value, ok, err
	}
	return loadIslandBaseEfficiencyFromCategory(islandSetCategoryLC)
}

func loadIslandBaseEfficiencyFromCategory(category string) (uint32, bool, error) {
	if entry, err := orm.GetConfigEntry(category, "base_efficiency"); err == nil {
		if value, ok := parseIslandSetBaseEfficiency(entry.Data); ok {
			return value, true, nil
		}
	}
	entries, err := orm.ListConfigEntries(category)
	if err != nil {
		return 0, false, err
	}
	for _, entry := range entries {
		if value, ok := parseIslandSetBaseEfficiency(entry.Data); ok {
			return value, true, nil
		}
	}
	return 0, false, nil
}

func parseIslandSetBaseEfficiency(payload []byte) (uint32, bool) {
	var raw islandSetEntry
	if err := json.Unmarshal(payload, &raw); err != nil {
		return 0, false
	}
	if raw.BaseEfficiency > 0 {
		return raw.BaseEfficiency, true
	}
	if strings.EqualFold(raw.Key, "base_efficiency") {
		if len(raw.Description) > 0 {
			if value := parseUint32FromJSON(raw.Description[0]); value > 0 {
				return value, true
			}
		}
		if raw.KeyValue > 0 {
			return raw.KeyValue, true
		}
		if raw.Value > 0 {
			return raw.Value, true
		}
	}
	return 0, false
}

func parseUint32FromJSON(raw json.RawMessage) uint32 {
	var asUint uint32
	if err := json.Unmarshal(raw, &asUint); err == nil {
		return asUint
	}
	var asInt int64
	if err := json.Unmarshal(raw, &asInt); err == nil && asInt > 0 {
		return uint32(asInt)
	}
	var asString string
	if err := json.Unmarshal(raw, &asString); err == nil {
		value, parseErr := strconv.ParseUint(asString, 10, 32)
		if parseErr == nil {
			return uint32(value)
		}
	}
	return 0
}

func islandFormulaDrops(formula *islandFormulaConfigV2) []*protobuf.DROPINFO {
	drops := make([]*protobuf.DROPINFO, 0)
	for _, raw := range formula.DropDisplay {
		var numbers []uint32
		if err := json.Unmarshal(raw, &numbers); err == nil && len(numbers) >= 3 {
			drops = append(drops, newDropInfo(numbers[0], numbers[1], numbers[2]))
			continue
		}
		var mixed []json.RawMessage
		if err := json.Unmarshal(raw, &mixed); err == nil && len(mixed) >= 3 {
			typ := parseUint32FromJSON(mixed[0])
			id := parseUint32FromJSON(mixed[1])
			count := parseUint32FromJSON(mixed[2])
			if typ > 0 && id > 0 && count > 0 {
				drops = append(drops, newDropInfo(typ, id, count))
			}
		}
	}
	if len(drops) == 0 && formula.ItemID > 0 {
		drops = append(drops, newDropInfo(consts.DROP_TYPE_ISLAND_ITEM, formula.ItemID, 1))
	}
	return drops
}

func mergeDrop(dropMap map[string]*protobuf.DROPINFO, typ uint32, id uint32, count uint32) {
	if typ == 0 || id == 0 || count == 0 {
		return
	}
	key := fmt.Sprintf("%d:%d", typ, id)
	if current, ok := dropMap[key]; ok {
		current.Number = proto.Uint32(current.GetNumber() + count)
		return
	}
	dropMap[key] = newDropInfo(typ, id, count)
}

func loadIslandHandPlantStateForSlotsTx(ctx context.Context, tx pgx.Tx, commanderID uint32, slotIDs []uint32, forUpdate bool) (map[uint32]orm.IslandHandPlant, error) {
	var (
		rows []orm.IslandHandPlant
		err  error
	)
	if forUpdate {
		rows, err = orm.ListIslandHandPlantsBySlotIDsForUpdateTx(ctx, tx, commanderID, slotIDs)
	} else {
		rows, err = orm.ListIslandHandPlantsBySlotIDsTx(ctx, tx, commanderID, slotIDs)
	}
	if err != nil {
		return nil, err
	}
	bySlot := make(map[uint32]orm.IslandHandPlant, len(slotIDs))
	for _, row := range rows {
		bySlot[row.SlotID] = row
	}
	for _, slotID := range slotIDs {
		if _, ok := bySlot[slotID]; !ok {
			bySlot[slotID] = orm.IslandHandPlant{CommanderID: commanderID, SlotID: slotID}
		}
	}
	return bySlot, nil
}

func islandHandPlantToProto(value *orm.IslandHandPlant) *protobuf.PB_ISLAND_HAND_AREA {
	return &protobuf.PB_ISLAND_HAND_AREA{
		Id:        proto.Uint32(value.SlotID),
		State:     proto.Uint32(value.State),
		FormulaId: proto.Uint32(value.FormulaID),
		StartTime: proto.Uint32(value.StartTime),
		EndTime:   proto.Uint32(value.EndTime),
	}
}

func containsFormulaID(values []uint32, target uint32) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
