package island

import (
	"context"
	"encoding/json"
	"sort"
	"strconv"

	"github.com/jackc/pgx/v5"
	"google.golang.org/protobuf/proto"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/db"
	"github.com/ggmolly/belfast/internal/orm"
	"github.com/ggmolly/belfast/internal/protobuf"
)

const (
	islandAchievementCategory   = "ShareCfg/island_achievement.json"
	islandAchievementCategoryLC = "sharecfgdata/island_achievement.json"

	islandAchievementClaimOK      = uint32(0)
	islandAchievementClaimInvalid = uint32(1)
	islandAchievementClaimState   = uint32(2)
	islandAchievementClaimPersist = uint32(3)

	islandAchievementMaxSyncEvents = 200
)

type islandAchievementConfig struct {
	ID           uint32          `json:"id"`
	TargetType   uint32          `json:"target_type"`
	TargetValue  uint32          `json:"target_value1"`
	TargetNum    uint32          `json:"target_num"`
	AwardDisplay [][]uint32      `json:"award_display"`
	AwardRaw     json.RawMessage `json:"award"`
}

type islandAchievementEventKey struct {
	EventType uint32
	EventArg  uint32
}

type islandAchievementSyncRule struct {
	MaxValue uint32
}

func IslandClaimAchievementAward(buffer *[]byte, client *connection.Client) (int, int, error) {
	var payload protobuf.CS_21050
	if err := proto.Unmarshal(*buffer, &payload); err != nil {
		return 0, 21051, err
	}

	response := &protobuf.SC_21051{Result: proto.Uint32(islandAchievementClaimInvalid), DropList: []*protobuf.DROPINFO{}}
	if err := ensureCommanderLoaded(client, "Island/AchievementClaim"); err != nil {
		response.Result = proto.Uint32(islandAchievementClaimPersist)
		return client.SendMessage(21051, response)
	}

	achievementIDs := dedupeTaskIDs(payload.GetIdList())
	if len(achievementIDs) == 0 {
		return client.SendMessage(21051, response)
	}

	configsByID, err := loadIslandAchievementConfigByID()
	if err != nil {
		response.Result = proto.Uint32(islandAchievementClaimPersist)
		return client.SendMessage(21051, response)
	}

	err = db.DefaultStore.WithPGXTx(context.Background(), func(tx pgx.Tx) error {
		state, err := orm.GetIslandAchievementStateForUpdateTx(context.Background(), tx, client.Commander.CommanderID)
		if err != nil {
			response.Result = proto.Uint32(islandAchievementClaimPersist)
			return err
		}

		pendingDrops := make([]*protobuf.DROPINFO, 0)
		for _, achievementID := range achievementIDs {
			cfg, ok := configsByID[achievementID]
			if !ok {
				response.Result = proto.Uint32(islandAchievementClaimInvalid)
				return nil
			}
			if state.HasFinished(achievementID) {
				response.Result = proto.Uint32(islandAchievementClaimState)
				return nil
			}
			if !isIslandAchievementClaimEligible(state, cfg) {
				response.Result = proto.Uint32(islandAchievementClaimState)
				return nil
			}

			awardRows, err := resolveIslandAchievementAwardRows(cfg)
			if err != nil {
				response.Result = proto.Uint32(islandAchievementClaimInvalid)
				return nil
			}
			drops, err := buildAwardDrops(awardRows)
			if err != nil {
				response.Result = proto.Uint32(islandAchievementClaimInvalid)
				return nil
			}
			pendingDrops = append(pendingDrops, drops...)
			state.FinishList = append(state.FinishList, achievementID)
		}

		if err := applyIslandDropsTx(context.Background(), tx, client, pendingDrops); err != nil {
			response.Result = proto.Uint32(islandAchievementClaimPersist)
			return err
		}
		if err := orm.SaveIslandAchievementStateTx(context.Background(), tx, state); err != nil {
			response.Result = proto.Uint32(islandAchievementClaimPersist)
			return err
		}

		response.Result = proto.Uint32(islandAchievementClaimOK)
		response.DropList = mergeDropList(pendingDrops)
		return nil
	})
	if err != nil {
		return client.SendMessage(21051, response)
	}

	return client.SendMessage(21051, response)
}

func IslandSyncAchievementProgress(buffer *[]byte, client *connection.Client) (int, int, error) {
	var payload protobuf.CS_21052
	if err := proto.Unmarshal(*buffer, &payload); err != nil {
		return 0, 21053, err
	}

	response := &protobuf.SC_21053{EventList: []*protobuf.PB_ISLAND_ACHIEVENT{}}
	if err := ensureCommanderLoaded(client, "Island/AchievementSync"); err != nil {
		return 0, 21053, err
	}

	rawEvents := payload.GetEventList()
	if len(rawEvents) > islandAchievementMaxSyncEvents {
		rawEvents = rawEvents[:islandAchievementMaxSyncEvents]
	}
	normalizedEvents := normalizeIslandAchievementEvents(rawEvents)
	configsByID, err := loadIslandAchievementConfigByID()
	if err != nil {
		return 0, 21053, err
	}
	syncRules := buildIslandAchievementSyncRules(configsByID)
	acceptedEvents := make([]orm.IslandAchievementProgressEntry, 0, len(normalizedEvents))

	err = db.DefaultStore.WithPGXTx(context.Background(), func(tx pgx.Tx) error {
		state, err := orm.GetIslandAchievementStateForUpdateTx(context.Background(), tx, client.Commander.CommanderID)
		if err != nil {
			return err
		}
		for _, entry := range normalizedEvents {
			if !isIslandAchievementSyncEventAllowed(entry, syncRules) {
				continue
			}
			if current, exists := state.ProgressValue(entry.EventType, entry.EventArg); exists && entry.Value < current {
				continue
			}
			state.SetProgress(entry.EventType, entry.EventArg, entry.Value)
			acceptedEvents = append(acceptedEvents, entry)
		}
		if err := orm.SaveIslandAchievementStateTx(context.Background(), tx, state); err != nil {
			return err
		}
		response.EventList = buildIslandAchievementEventList(acceptedEvents)
		return nil
	})
	if err != nil {
		return 0, 21053, err
	}

	return client.SendMessage(21053, response)
}

func loadIslandAchievementConfigByID() (map[uint32]islandAchievementConfig, error) {
	entries, err := listConfigEntriesWithFallback(islandAchievementCategory, islandAchievementCategoryLC, orm.ListConfigEntries)
	if err != nil {
		return nil, err
	}

	configs := make(map[uint32]islandAchievementConfig, len(entries))
	for _, entry := range entries {
		cfg := islandAchievementConfig{}
		if err := json.Unmarshal(entry.Data, &cfg); err != nil {
			return nil, err
		}
		if cfg.ID == 0 {
			parsedID, parseErr := strconv.ParseUint(entry.Key, 10, 32)
			if parseErr != nil {
				continue
			}
			cfg.ID = uint32(parsedID)
		}
		if cfg.ID == 0 {
			continue
		}
		configs[cfg.ID] = cfg
	}

	return configs, nil
}

func isIslandAchievementClaimEligible(state *orm.IslandAchievementState, cfg islandAchievementConfig) bool {
	if cfg.TargetType == 0 {
		return false
	}
	if cfg.TargetNum == 0 {
		return true
	}
	progress, ok := state.ProgressValue(cfg.TargetType, cfg.TargetValue)
	if !ok {
		return false
	}
	return progress >= cfg.TargetNum
}

func resolveIslandAchievementAwardRows(cfg islandAchievementConfig) ([][]uint32, error) {
	if len(cfg.AwardDisplay) > 0 {
		return cfg.AwardDisplay, nil
	}
	if len(cfg.AwardRaw) == 0 {
		return nil, nil
	}

	var matrix [][]uint32
	if err := json.Unmarshal(cfg.AwardRaw, &matrix); err == nil {
		return matrix, nil
	}

	var single []uint32
	if err := json.Unmarshal(cfg.AwardRaw, &single); err == nil {
		if len(single) == 0 {
			return nil, nil
		}
		return [][]uint32{single}, nil
	}

	return nil, json.Unmarshal(cfg.AwardRaw, &matrix)
}

func normalizeIslandAchievementEvents(events []*protobuf.PB_ISLAND_ACHIEVENT) []orm.IslandAchievementProgressEntry {
	if len(events) == 0 {
		return []orm.IslandAchievementProgressEntry{}
	}

	type eventKey struct {
		eventType uint32
		eventArg  uint32
	}
	collapsed := make(map[eventKey]orm.IslandAchievementProgressEntry, len(events))
	for _, event := range events {
		if event == nil {
			continue
		}
		eventType := event.GetEventType()
		if eventType == 0 {
			continue
		}
		eventArg := event.GetEventArg()
		collapsed[eventKey{eventType: eventType, eventArg: eventArg}] = orm.IslandAchievementProgressEntry{
			EventType: eventType,
			EventArg:  eventArg,
			Value:     event.GetValue(),
		}
	}

	normalized := make([]orm.IslandAchievementProgressEntry, 0, len(collapsed))
	for _, entry := range collapsed {
		normalized = append(normalized, entry)
	}
	sort.Slice(normalized, func(i, j int) bool {
		if normalized[i].EventType == normalized[j].EventType {
			return normalized[i].EventArg < normalized[j].EventArg
		}
		return normalized[i].EventType < normalized[j].EventType
	})
	return normalized
}

func buildIslandAchievementSyncRules(configsByID map[uint32]islandAchievementConfig) map[islandAchievementEventKey]islandAchievementSyncRule {
	rules := make(map[islandAchievementEventKey]islandAchievementSyncRule, len(configsByID))
	for _, cfg := range configsByID {
		if cfg.TargetType == 0 || cfg.TargetNum == 0 {
			continue
		}
		key := islandAchievementEventKey{EventType: cfg.TargetType, EventArg: cfg.TargetValue}
		rule := rules[key]
		if cfg.TargetNum > rule.MaxValue {
			rule.MaxValue = cfg.TargetNum
		}
		rules[key] = rule
	}
	return rules
}

func isIslandAchievementSyncEventAllowed(entry orm.IslandAchievementProgressEntry, rules map[islandAchievementEventKey]islandAchievementSyncRule) bool {
	rule, ok := rules[islandAchievementEventKey{EventType: entry.EventType, EventArg: entry.EventArg}]
	if !ok {
		return false
	}
	if rule.MaxValue == 0 {
		return false
	}
	return entry.Value <= rule.MaxValue
}
