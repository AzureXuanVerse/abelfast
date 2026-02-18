package answer

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"google.golang.org/protobuf/proto"

	"github.com/ggmolly/belfast/internal/orm"
	"github.com/ggmolly/belfast/internal/protobuf"
)

const (
	islandWorldObjectsCategory   = "ShareCfg/island_world_objects.json"
	islandWorldObjectsCategoryLC = "sharecfgdata/island_world_objects.json"
	islandMapCategory            = "ShareCfg/island_map.json"
	islandMapCategoryLC          = "sharecfgdata/island_map.json"
	islandFragmentCategory       = "ShareCfg/island_collect_fragment.json"
	islandFragmentCategoryLC     = "sharecfgdata/island_collect_fragment.json"
	islandNpcCategory            = "ShareCfg/island_npc.json"
	islandNpcCategoryLC          = "sharecfgdata/island_npc.json"
)

func loadIslandMapSnapshot(islandID uint32, mapID uint32) (*protobuf.SC_21214, error) {
	if mapID == 0 {
		return nil, fmt.Errorf("invalid map id")
	}

	mapExists, err := islandMapExists(mapID)
	if err != nil {
		return nil, err
	}
	if !mapExists {
		return nil, fmt.Errorf("map not found")
	}

	objectTemplates, err := loadIslandObjectTemplates(mapID)
	if err != nil {
		return nil, err
	}
	objectList := globalIslandRuntimeState.seedIslandObjects(islandID, objectTemplates)

	gatherList, err := loadIslandGatherSnapshot(mapID)
	if err != nil {
		return nil, err
	}

	fragmentList, err := loadIslandFragmentSnapshot(mapID)
	if err != nil {
		return nil, err
	}

	npcList, err := loadIslandNPCSnapshot(mapID)
	if err != nil {
		return nil, err
	}

	return &protobuf.SC_21214{
		Result:       proto.Uint32(0),
		ObjectList:   objectList,
		GatherList:   gatherList,
		FragmentList: fragmentList,
		NpcList:      npcList,
	}, nil
}

func islandMapExists(mapID uint32) (bool, error) {
	rows, err := listIslandConfigEntriesWithFallback(islandMapCategory, islandMapCategoryLC)
	if err != nil {
		return false, err
	}
	if len(rows) == 0 {
		return false, nil
	}

	key := strconv.FormatUint(uint64(mapID), 10)
	for _, row := range rows {
		if row.Key == key {
			return true, nil
		}
		payload, err := decodeJSONMap(row.Data)
		if err != nil {
			continue
		}
		if parseUint32Any(payload["id"]) == mapID || parseUint32Any(payload["map_id"]) == mapID || parseUint32Any(payload["mapId"]) == mapID {
			return true, nil
		}
	}

	return false, nil
}

func loadIslandObjectTemplates(mapID uint32) ([]islandObjectTemplate, error) {
	rows, err := listIslandConfigEntriesWithFallback(islandWorldObjectsCategory, islandWorldObjectsCategoryLC)
	if err != nil {
		return nil, err
	}

	templates := make([]islandObjectTemplate, 0)
	for _, row := range rows {
		payload, err := decodeJSONMap(row.Data)
		if err != nil {
			continue
		}

		entryMapID := parseUint32Any(payload["map_id"])
		if entryMapID == 0 {
			entryMapID = parseUint32Any(payload["mapId"])
		}
		if entryMapID == 0 {
			entryMapID = parseUint32Any(payload["map"])
		}
		if entryMapID != mapID {
			continue
		}

		objectID := parseUint32Any(payload["id"])
		if objectID == 0 {
			objectID = parseConfigKeyUint32(row.Key)
		}
		if objectID == 0 {
			continue
		}

		objectType := parseUint32Any(payload["type"])
		if objectType == 0 {
			objectType = parseUint32Any(payload["object_type"])
		}
		if objectType == 0 {
			objectType = 1
		}

		template := islandObjectTemplate{
			ID:      objectID,
			Type:    objectType,
			Status:  parseUint32Any(payload["status"]),
			MapID:   mapID,
			SlotIDs: parseSlotIDs(payload),
		}
		if len(template.SlotIDs) == 0 {
			template.SlotIDs = []uint32{1}
		}
		templates = append(templates, template)
	}

	return templates, nil
}

func loadIslandGatherSnapshot(mapID uint32) ([]*protobuf.PB_ISLAND_WILD_GATHER, error) {
	rows, err := listIslandConfigEntriesWithFallback(islandWildGatherCategory, islandWildGatherCategoryLC)
	if err != nil {
		return nil, err
	}

	gathers := make([]*protobuf.PB_ISLAND_WILD_GATHER, 0)
	for _, row := range rows {
		payload, err := decodeJSONMap(row.Data)
		if err != nil {
			continue
		}
		if !matchesMap(payload, mapID) {
			continue
		}

		id := parseUint32Any(payload["id"])
		if id == 0 {
			id = parseConfigKeyUint32(row.Key)
		}
		if id == 0 {
			continue
		}

		gathers = append(gathers, &protobuf.PB_ISLAND_WILD_GATHER{
			Id:    proto.Uint32(id),
			Pos:   proto.Uint32(firstNonZero(parseUint32Any(payload["pos"]), parseUint32Any(payload["object_id"]))),
			State: proto.Uint32(parseUint32Any(payload["state"])),
			Mark:  proto.Uint32(parseUint32Any(payload["mark"])),
		})
	}

	return gathers, nil
}

func loadIslandFragmentSnapshot(mapID uint32) ([]*protobuf.PB_ISLAND_COLLECT_FRAGMENT, error) {
	rows, err := listIslandConfigEntriesWithFallback(islandFragmentCategory, islandFragmentCategoryLC)
	if err != nil {
		return nil, err
	}

	fragments := make([]*protobuf.PB_ISLAND_COLLECT_FRAGMENT, 0)
	for _, row := range rows {
		payload, err := decodeJSONMap(row.Data)
		if err != nil {
			continue
		}
		if !matchesMap(payload, mapID) {
			continue
		}

		id := parseUint32Any(payload["id"])
		if id == 0 {
			id = parseConfigKeyUint32(row.Key)
		}
		if id == 0 {
			continue
		}

		fragments = append(fragments, &protobuf.PB_ISLAND_COLLECT_FRAGMENT{
			Id:   proto.Uint32(id),
			Pos:  proto.Uint32(firstNonZero(parseUint32Any(payload["pos"]), parseUint32Any(payload["object_id"]))),
			Mark: proto.Uint32(parseUint32Any(payload["mark"])),
		})
	}

	return fragments, nil
}

func loadIslandNPCSnapshot(mapID uint32) ([]*protobuf.PB_ISLAND_NPC, error) {
	rows, err := listIslandConfigEntriesWithFallback(islandNpcCategory, islandNpcCategoryLC)
	if err != nil {
		return nil, err
	}

	npcs := make([]*protobuf.PB_ISLAND_NPC, 0)
	for _, row := range rows {
		payload, err := decodeJSONMap(row.Data)
		if err != nil {
			continue
		}
		if !matchesMap(payload, mapID) {
			continue
		}

		npcID := parseUint32Any(payload["id"])
		if npcID == 0 {
			npcID = parseConfigKeyUint32(row.Key)
		}
		if npcID == 0 {
			continue
		}

		npcs = append(npcs, &protobuf.PB_ISLAND_NPC{
			Id:       proto.Uint32(npcID),
			ObjectId: proto.Uint32(parseUint32Any(payload["object_id"])),
		})
	}

	return npcs, nil
}

func listIslandConfigEntriesWithFallback(primaryCategory string, fallbackCategory string) ([]orm.ConfigEntry, error) {
	primaryRows, err := orm.ListConfigEntries(primaryCategory)
	if err != nil {
		return nil, err
	}
	if len(primaryRows) > 0 {
		return primaryRows, nil
	}
	return orm.ListConfigEntries(fallbackCategory)
}

func decodeJSONMap(raw json.RawMessage) (map[string]any, error) {
	payload := make(map[string]any)
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, err
	}
	return payload, nil
}

func parseSlotIDs(payload map[string]any) []uint32 {
	if value, ok := payload["slot_ids"]; ok {
		if parsed := parseUint32List(value); len(parsed) > 0 {
			return parsed
		}
	}
	if value, ok := payload["slotIds"]; ok {
		if parsed := parseUint32List(value); len(parsed) > 0 {
			return parsed
		}
	}
	slotCount := firstNonZero(parseUint32Any(payload["slot_count"]), parseUint32Any(payload["slotCount"]), parseUint32Any(payload["slot_num"]), parseUint32Any(payload["slotNum"]))
	if slotCount == 0 {
		return nil
	}
	slots := make([]uint32, 0, slotCount)
	for i := uint32(1); i <= slotCount; i++ {
		slots = append(slots, i)
	}
	return slots
}

func parseUint32List(value any) []uint32 {
	list, ok := value.([]any)
	if !ok {
		return nil
	}
	parsed := make([]uint32, 0, len(list))
	for _, item := range list {
		v := parseUint32Any(item)
		if v == 0 {
			continue
		}
		parsed = append(parsed, v)
	}
	return parsed
}

func parseUint32Any(value any) uint32 {
	switch v := value.(type) {
	case float64:
		if v <= 0 {
			return 0
		}
		return uint32(v)
	case float32:
		if v <= 0 {
			return 0
		}
		return uint32(v)
	case int:
		if v <= 0 {
			return 0
		}
		return uint32(v)
	case int32:
		if v <= 0 {
			return 0
		}
		return uint32(v)
	case int64:
		if v <= 0 {
			return 0
		}
		return uint32(v)
	case uint32:
		return v
	case uint64:
		return uint32(v)
	case json.Number:
		parsed, err := v.Int64()
		if err != nil || parsed <= 0 {
			return 0
		}
		return uint32(parsed)
	case string:
		trimmed := strings.TrimSpace(v)
		if trimmed == "" {
			return 0
		}
		parsed, err := strconv.ParseUint(trimmed, 10, 32)
		if err != nil {
			return 0
		}
		return uint32(parsed)
	default:
		return 0
	}
}

func parseConfigKeyUint32(value string) uint32 {
	parsed, err := strconv.ParseUint(value, 10, 32)
	if err != nil {
		return 0
	}
	return uint32(parsed)
}

func matchesMap(payload map[string]any, mapID uint32) bool {
	entryMapID := firstNonZero(parseUint32Any(payload["map_id"]), parseUint32Any(payload["mapId"]), parseUint32Any(payload["map"]))
	if entryMapID == 0 {
		return true
	}
	return entryMapID == mapID
}

func firstNonZero(values ...uint32) uint32 {
	for _, value := range values {
		if value != 0 {
			return value
		}
	}
	return 0
}
