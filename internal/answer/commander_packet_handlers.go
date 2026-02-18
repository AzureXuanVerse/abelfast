package answer

import (
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/ggmolly/belfast/internal/config"
	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/db"
	"github.com/ggmolly/belfast/internal/orm"
	"github.com/ggmolly/belfast/internal/protobuf"
	"google.golang.org/protobuf/proto"
)

const (
	commanderPacketResultOK   = uint32(0)
	commanderPacketResultFail = uint32(1)

	commanderNameMaxLength  = 12
	commanderPrefabMaxID    = 5
	commanderPrefabMaxSlots = 2
)

func FetchCommanderCandidateTalents(buffer *[]byte, client *connection.Client) (int, int, error) {
	var payload protobuf.CS_25010
	if err := proto.Unmarshal(*buffer, &payload); err != nil {
		return 0, 25011, err
	}
	response := &protobuf.SC_25011{Result: proto.Uint32(commanderPacketResultFail)}

	state, err := orm.GetOrCreateCommanderPacketState(client.Commander.CommanderID, payload.GetCommanderid())
	if err != nil {
		return 0, 25011, err
	}

	candidates, err := buildCommanderTalentCandidates(state)
	if err != nil {
		return client.SendMessage(25011, response)
	}
	if len(candidates) == 0 {
		return client.SendMessage(25011, response)
	}

	state.PendingAbilityIDs = candidates
	if err := orm.SaveCommanderPacketState(state); err != nil {
		return 0, 25011, err
	}

	response.Result = proto.Uint32(commanderPacketResultOK)
	response.Abilityid = candidates
	return client.SendMessage(25011, response)
}

func LearnCommanderTalent(buffer *[]byte, client *connection.Client) (int, int, error) {
	var payload protobuf.CS_25012
	if err := proto.Unmarshal(*buffer, &payload); err != nil {
		return 0, 25013, err
	}
	response := &protobuf.SC_25013{Result: proto.Uint32(commanderPacketResultFail)}

	state, err := orm.GetOrCreateCommanderPacketState(client.Commander.CommanderID, payload.GetCommanderid())
	if err != nil {
		return 0, 25013, err
	}

	targetID := payload.GetTargetid()
	if !containsUint32Value(state.PendingAbilityIDs, targetID) {
		return client.SendMessage(25013, response)
	}

	targetTemplate, err := orm.GetCommanderAbilityTemplate(targetID)
	if err != nil {
		if errors.Is(err, db.ErrNotFound) {
			return client.SendMessage(25013, response)
		}
		return 0, 25013, err
	}

	replaceID := payload.GetReplaceid()
	if replaceID != 0 {
		if !containsUint32Value(state.AbilityIDs, replaceID) {
			return client.SendMessage(25013, response)
		}
		replaceTemplate, err := orm.GetCommanderAbilityTemplate(replaceID)
		if err != nil {
			if errors.Is(err, db.ErrNotFound) {
				return client.SendMessage(25013, response)
			}
			return 0, 25013, err
		}
		if replaceTemplate.GroupID != targetTemplate.GroupID {
			return client.SendMessage(25013, response)
		}
	}

	if replaceID == 0 {
		for _, abilityID := range state.AbilityIDs {
			template, err := orm.GetCommanderAbilityTemplate(abilityID)
			if err != nil {
				if errors.Is(err, db.ErrNotFound) {
					continue
				}
				return 0, 25013, err
			}
			if template.GroupID == targetTemplate.GroupID {
				return client.SendMessage(25013, response)
			}
		}
	}

	if targetTemplate.Cost > 0 {
		if !client.Commander.HasEnoughGold(targetTemplate.Cost) {
			return client.SendMessage(25013, response)
		}
		if err := client.Commander.ConsumeResource(1, targetTemplate.Cost); err != nil {
			return client.SendMessage(25013, response)
		}
	}

	if replaceID != 0 {
		for idx, abilityID := range state.AbilityIDs {
			if abilityID == replaceID {
				state.AbilityIDs[idx] = targetID
				break
			}
		}
	} else {
		state.AbilityIDs = append(state.AbilityIDs, targetID)
	}

	if targetTemplate.Worth > 0 {
		state.UsedPt += targetTemplate.Worth
	} else {
		state.UsedPt++
	}
	state.PendingAbilityIDs = []uint32{}

	if err := orm.SaveCommanderPacketState(state); err != nil {
		return 0, 25013, err
	}

	response.Result = proto.Uint32(commanderPacketResultOK)
	return client.SendMessage(25013, response)
}

func ResetCommanderTalents(buffer *[]byte, client *connection.Client) (int, int, error) {
	var payload protobuf.CS_25014
	if err := proto.Unmarshal(*buffer, &payload); err != nil {
		return 0, 25015, err
	}
	response := &protobuf.SC_25015{Result: proto.Uint32(commanderPacketResultFail)}

	state, err := orm.GetOrCreateCommanderPacketState(client.Commander.CommanderID, payload.GetCommanderid())
	if err != nil {
		return 0, 25015, err
	}
	if state.UsedPt == 0 && sameUint32Set(state.AbilityIDs, state.AbilityOriginIDs) {
		return client.SendMessage(25015, response)
	}

	gameset, err := orm.LoadCommanderGameSet()
	if err != nil {
		return 0, 25015, err
	}

	now := time.Now().UTC()
	if state.AbilityResetAt.Add(time.Duration(gameset.AbilityResetCooldownSeconds) * time.Second).After(now) {
		return client.SendMessage(25015, response)
	}

	cost := resolveCommanderResetCost(gameset.SkillResetCosts, state.UsedPt)
	if cost > 0 {
		if !client.Commander.HasEnoughGold(cost) {
			return client.SendMessage(25015, response)
		}
		if err := client.Commander.ConsumeResource(1, cost); err != nil {
			return client.SendMessage(25015, response)
		}
	}

	state.AbilityIDs = append([]uint32(nil), state.AbilityOriginIDs...)
	state.UsedPt = 0
	state.PendingAbilityIDs = []uint32{}
	state.AbilityResetAt = now
	if err := orm.SaveCommanderPacketState(state); err != nil {
		return 0, 25015, err
	}

	response.Result = proto.Uint32(commanderPacketResultOK)
	return client.SendMessage(25015, response)
}

func SetCommanderLockState(buffer *[]byte, client *connection.Client) (int, int, error) {
	var payload protobuf.CS_25016
	if err := proto.Unmarshal(*buffer, &payload); err != nil {
		return 0, 25017, err
	}
	response := &protobuf.SC_25017{Result: proto.Uint32(commanderPacketResultFail)}

	flag := payload.GetFlag()
	if flag != 0 && flag != 1 {
		return client.SendMessage(25017, response)
	}

	state, err := orm.GetOrCreateCommanderPacketState(client.Commander.CommanderID, payload.GetCommanderid())
	if err != nil {
		return 0, 25017, err
	}

	state.IsLocked = flag == 1
	if err := orm.SaveCommanderPacketState(state); err != nil {
		return 0, 25017, err
	}

	response.Result = proto.Uint32(commanderPacketResultOK)
	return client.SendMessage(25017, response)
}

func RenameCommander(buffer *[]byte, client *connection.Client) (int, int, error) {
	var payload protobuf.CS_25020
	if err := proto.Unmarshal(*buffer, &payload); err != nil {
		return 0, 25021, err
	}
	response := &protobuf.SC_25021{Result: proto.Uint32(commanderPacketResultFail)}

	state, err := orm.GetOrCreateCommanderPacketState(client.Commander.CommanderID, payload.GetCommanderid())
	if err != nil {
		return 0, 25021, err
	}

	name := strings.TrimSpace(payload.GetName())
	if !isValidCommanderName(name, state.Name, commanderNameMaxLength) {
		return client.SendMessage(25021, response)
	}

	gameset, err := orm.LoadCommanderGameSet()
	if err != nil {
		return 0, 25021, err
	}

	now := time.Now().UTC()
	if state.RenameCooldownAt.After(now) {
		return client.SendMessage(25021, response)
	}

	state.Name = name
	state.RenameCooldownAt = now.Add(time.Duration(gameset.RenameCooldownSeconds) * time.Second)
	if err := orm.SaveCommanderPacketState(state); err != nil {
		return 0, 25021, err
	}

	response.Result = proto.Uint32(commanderPacketResultOK)
	return client.SendMessage(25021, response)
}

func SetCommanderPrefabFleet(buffer *[]byte, client *connection.Client) (int, int, error) {
	var payload protobuf.CS_25022
	if err := proto.Unmarshal(*buffer, &payload); err != nil {
		return 0, 25023, err
	}
	response := &protobuf.SC_25023{Result: proto.Uint32(commanderPacketResultFail)}

	prefabID := payload.GetId()
	if !isValidPrefabID(prefabID) {
		return client.SendMessage(25023, response)
	}

	inputSlots := payload.GetCommandersid()
	if len(inputSlots) == 0 {
		return client.SendMessage(25023, response)
	}

	slotsByPos := make(map[uint32]uint32, len(inputSlots))
	nonZeroCount := 0
	for _, slot := range inputSlots {
		pos := slot.GetPos()
		if pos == 0 || pos > commanderPrefabMaxSlots {
			return client.SendMessage(25023, response)
		}
		if _, exists := slotsByPos[pos]; exists {
			return client.SendMessage(25023, response)
		}
		commanderID := slot.GetId()
		if commanderID != 0 {
			if _, err := orm.GetCommanderPacketState(client.Commander.CommanderID, commanderID); err != nil {
				if errors.Is(err, db.ErrNotFound) {
					return client.SendMessage(25023, response)
				}
				return 0, 25023, err
			}
			nonZeroCount++
		}
		slotsByPos[pos] = commanderID
	}
	if nonZeroCount == 0 {
		return client.SendMessage(25023, response)
	}

	prefab, err := orm.GetCommanderPrefabFleet(client.Commander.CommanderID, prefabID)
	if err != nil {
		if !errors.Is(err, db.ErrNotFound) {
			return 0, 25023, err
		}
		prefab = &orm.CommanderPrefabFleet{
			OwnerCommanderID: client.Commander.CommanderID,
			PrefabID:         prefabID,
			Name:             fmt.Sprintf("Preset Fleet %d", prefabID),
		}
	}

	positions := make([]uint32, 0, len(slotsByPos))
	for pos := range slotsByPos {
		positions = append(positions, pos)
	}
	sort.Slice(positions, func(i, j int) bool { return positions[i] < positions[j] })
	prefab.CommanderSlots = make([]orm.CommanderPrefabSlot, 0, len(positions))
	for _, pos := range positions {
		prefab.CommanderSlots = append(prefab.CommanderSlots, orm.CommanderPrefabSlot{Pos: pos, CommanderID: slotsByPos[pos]})
	}

	if err := orm.SaveCommanderPrefabFleet(prefab); err != nil {
		return 0, 25023, err
	}

	response.Result = proto.Uint32(commanderPacketResultOK)
	return client.SendMessage(25023, response)
}

func RenameCommanderPrefabFleet(buffer *[]byte, client *connection.Client) (int, int, error) {
	var payload protobuf.CS_25024
	if err := proto.Unmarshal(*buffer, &payload); err != nil {
		return 0, 25025, err
	}
	response := &protobuf.SC_25025{Result: proto.Uint32(commanderPacketResultFail)}

	prefabID := payload.GetId()
	if !isValidPrefabID(prefabID) {
		return client.SendMessage(25025, response)
	}

	prefab, err := orm.GetCommanderPrefabFleet(client.Commander.CommanderID, prefabID)
	if err != nil {
		if errors.Is(err, db.ErrNotFound) {
			return client.SendMessage(25025, response)
		}
		return 0, 25025, err
	}

	name := strings.TrimSpace(payload.GetName())
	if !isValidCommanderName(name, prefab.Name, commanderNameMaxLength) {
		return client.SendMessage(25025, response)
	}

	gameset, err := orm.LoadCommanderGameSet()
	if err != nil {
		return 0, 25025, err
	}
	now := time.Now().UTC()
	if prefab.RenameCooldownAt.After(now) {
		return client.SendMessage(25025, response)
	}

	prefab.Name = name
	prefab.RenameCooldownAt = now.Add(time.Duration(gameset.RenameCooldownSeconds) * time.Second)
	if err := orm.SaveCommanderPrefabFleet(prefab); err != nil {
		return 0, 25025, err
	}

	response.Result = proto.Uint32(commanderPacketResultOK)
	return client.SendMessage(25025, response)
}

func buildCommanderTalentCandidates(state *orm.CommanderPacketState) ([]uint32, error) {
	if len(state.PendingAbilityIDs) > 0 {
		return nil, fmt.Errorf("pending candidate list already exists")
	}

	learnedSet := make(map[uint32]struct{}, len(state.AbilityIDs))
	for _, abilityID := range state.AbilityIDs {
		learnedSet[abilityID] = struct{}{}
	}

	candidateSet := map[uint32]struct{}{}
	for _, abilityID := range state.AbilityIDs {
		template, err := orm.GetCommanderAbilityTemplate(abilityID)
		if err != nil {
			if errors.Is(err, db.ErrNotFound) {
				continue
			}
			return nil, err
		}
		if template.Next != 0 {
			if _, learned := learnedSet[template.Next]; !learned {
				candidateSet[template.Next] = struct{}{}
			}
		}
	}

	if len(candidateSet) == 0 {
		groups, err := orm.ListCommanderAbilityGroups()
		if err != nil {
			return nil, err
		}
		learnedGroups := map[uint32]struct{}{}
		for _, abilityID := range state.AbilityIDs {
			template, err := orm.GetCommanderAbilityTemplate(abilityID)
			if err != nil {
				if errors.Is(err, db.ErrNotFound) {
					continue
				}
				return nil, err
			}
			learnedGroups[template.GroupID] = struct{}{}
		}
		for _, group := range groups {
			if _, learned := learnedGroups[group.ID]; learned {
				continue
			}
			for _, abilityID := range group.AbilityList {
				if _, exists := learnedSet[abilityID]; exists {
					continue
				}
				candidateSet[abilityID] = struct{}{}
				break
			}
		}
	}

	candidates := make([]uint32, 0, len(candidateSet))
	for abilityID := range candidateSet {
		candidates = append(candidates, abilityID)
	}
	sort.Slice(candidates, func(i, j int) bool { return candidates[i] < candidates[j] })
	if len(candidates) > 3 {
		candidates = candidates[:3]
	}
	return candidates, nil
}

func resolveCommanderResetCost(costs []uint32, usedPt uint32) uint32 {
	if len(costs) == 0 {
		return 0
	}
	if usedPt == 0 {
		return 0
	}
	idx := int(usedPt - 1)
	if idx < 0 {
		idx = 0
	}
	if idx >= len(costs) {
		idx = len(costs) - 1
	}
	return costs[idx]
}

func isValidCommanderName(name string, currentName string, maxLength int) bool {
	if name == "" || name == currentName {
		return false
	}
	runeCount := utf8.RuneCountInString(name)
	if runeCount < 1 || runeCount > maxLength {
		return false
	}

	createCfg := config.Current().CreatePlayer
	lowerName := strings.ToLower(name)
	for _, blocked := range createCfg.NameBlacklist {
		blocked = strings.TrimSpace(blocked)
		if blocked == "" {
			continue
		}
		if strings.Contains(lowerName, strings.ToLower(blocked)) {
			return false
		}
	}
	if createCfg.NameIllegalPattern != "" {
		matcher, err := regexp.Compile(createCfg.NameIllegalPattern)
		if err != nil {
			return false
		}
		if matcher.MatchString(name) {
			return false
		}
	}
	return true
}

func isValidPrefabID(prefabID uint32) bool {
	return prefabID >= 1 && prefabID <= commanderPrefabMaxID
}

func containsUint32Value(values []uint32, target uint32) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func sameUint32Set(left []uint32, right []uint32) bool {
	if len(left) != len(right) {
		return false
	}
	if len(left) == 0 {
		return true
	}
	counts := map[uint32]int{}
	for _, value := range left {
		counts[value]++
	}
	for _, value := range right {
		counts[value]--
		if counts[value] < 0 {
			return false
		}
	}
	for _, count := range counts {
		if count != 0 {
			return false
		}
	}
	return true
}
