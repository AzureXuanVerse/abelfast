package island

import (
	"encoding/json"
	"math/rand"
	"strconv"
	"sync"
	"time"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/orm"
	"github.com/ggmolly/belfast/internal/protobuf"
	"google.golang.org/protobuf/proto"
)

const (
	islandFishPointCategory   = "ShareCfg/island_fish_point.json"
	islandFishPointCategoryLC = "sharecfgdata/island_fish_point.json"
	islandFishCategory        = "ShareCfg/island_fish.json"
	islandFishCategoryLC      = "sharecfgdata/island_fish.json"
)

type islandFishTemplate struct {
	ID        uint32 `json:"id"`
	MinWeight uint32 `json:"min_weight"`
	MaxWeight uint32 `json:"max_weight"`
	GoldState uint32 `json:"gold_state"`
}

type islandFishingRoll struct {
	FishID    uint32
	Weight    uint32
	GoldState uint32
	ExpiresAt time.Time
}

var (
	islandFishingStateMu sync.Mutex
	islandFishingState   = map[string]islandFishingRoll{}
	islandFishingRNGMu   sync.Mutex

	islandFishingNow = func() time.Time { return time.Now().UTC() }
	islandFishingRNG = rand.New(rand.NewSource(time.Now().UnixNano()))
)

func IslandGoFishing(buffer *[]byte, client *connection.Client) (int, int, error) {
	var payload protobuf.CS_21060
	if err := proto.Unmarshal(*buffer, &payload); err != nil {
		return 0, 21061, err
	}

	response := &protobuf.SC_21061{
		Result:    proto.Uint32(1),
		FishId:    proto.Uint32(0),
		Weight:    proto.Uint32(0),
		GoldState: proto.Uint32(0),
	}
	if client.Commander == nil {
		return client.SendMessage(21061, response)
	}

	islandID := payload.GetIslandId()
	pointID := payload.GetPointId()
	if islandID == 0 || pointID == 0 {
		return client.SendMessage(21061, response)
	}

	if !isIslandFishingPointKnown(pointID) {
		return client.SendMessage(21061, response)
	}

	fishes := listIslandFishTemplates()
	if len(fishes) == 0 {
		return client.SendMessage(21061, response)
	}
	selected := fishes[islandFishingIntn(len(fishes))]
	minWeight := maxUint32(selected.MinWeight, 1)
	maxWeight := maxUint32(selected.MaxWeight, minWeight)
	weight := minWeight
	if maxWeight > minWeight {
		weight = minWeight + uint32(islandFishingIntn(int(maxWeight-minWeight+1)))
	}

	now := islandFishingNow()
	roll := islandFishingRoll{FishID: selected.ID, Weight: weight, GoldState: selected.GoldState, ExpiresAt: now.Add(5 * time.Minute)}
	storeIslandFishingRoll(client.Commander.CommanderID, islandID, pointID, roll)

	response.Result = proto.Uint32(0)
	response.FishId = proto.Uint32(roll.FishID)
	response.Weight = proto.Uint32(roll.Weight)
	response.GoldState = proto.Uint32(roll.GoldState)
	return client.SendMessage(21061, response)
}

func islandFishingIntn(n int) int {
	islandFishingRNGMu.Lock()
	value := islandFishingRNG.Intn(n)
	islandFishingRNGMu.Unlock()
	return value
}

func islandFishingKey(commanderID uint32, islandID uint32, pointID uint32) string {
	return strconv.FormatUint(uint64(commanderID), 10) + ":" + strconv.FormatUint(uint64(islandID), 10) + ":" + strconv.FormatUint(uint64(pointID), 10)
}

func storeIslandFishingRoll(commanderID uint32, islandID uint32, pointID uint32, roll islandFishingRoll) {
	islandFishingStateMu.Lock()
	islandFishingState[islandFishingKey(commanderID, islandID, pointID)] = roll
	islandFishingStateMu.Unlock()
}

func consumeIslandFishingRoll(commanderID uint32, islandID uint32, pointID uint32, now time.Time) (islandFishingRoll, bool) {
	key := islandFishingKey(commanderID, islandID, pointID)
	islandFishingStateMu.Lock()
	roll, ok := islandFishingState[key]
	if !ok {
		islandFishingStateMu.Unlock()
		return islandFishingRoll{}, false
	}
	delete(islandFishingState, key)
	islandFishingStateMu.Unlock()
	if now.After(roll.ExpiresAt) {
		return islandFishingRoll{}, false
	}
	return roll, true
}

func isIslandFishingPointKnown(pointID uint32) bool {
	if _, err := orm.GetConfigEntry(islandFishPointCategory, strconv.FormatUint(uint64(pointID), 10)); err == nil {
		return true
	}
	if _, err := orm.GetConfigEntry(islandFishPointCategoryLC, strconv.FormatUint(uint64(pointID), 10)); err == nil {
		return true
	}
	return false
}

func listIslandFishTemplates() []islandFishTemplate {
	entries, err := orm.ListConfigEntries(islandFishCategory)
	if err != nil || len(entries) == 0 {
		entries, err = orm.ListConfigEntries(islandFishCategoryLC)
		if err != nil {
			return []islandFishTemplate{}
		}
	}
	result := make([]islandFishTemplate, 0, len(entries))
	for _, entry := range entries {
		var parsed islandFishTemplate
		if err := json.Unmarshal(entry.Data, &parsed); err != nil {
			continue
		}
		if parsed.ID == 0 {
			parsed.ID = parseUint32Key(entry.Key)
		}
		if parsed.ID == 0 {
			continue
		}
		if parsed.MinWeight == 0 {
			parsed.MinWeight = 1
		}
		if parsed.MaxWeight < parsed.MinWeight {
			parsed.MaxWeight = parsed.MinWeight
		}
		result = append(result, parsed)
	}
	return result
}
