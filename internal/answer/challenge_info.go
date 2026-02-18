package answer

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/orm"
	"github.com/ggmolly/belfast/internal/protobuf"
	"google.golang.org/protobuf/proto"
)

type activityEventChallenge struct {
	ID            uint32       `json:"id"`
	Buff          []uint32     `json:"buff"`
	InfiniteStage [][][]uint32 `json:"infinite_stage"`
}

func ChallengeInfo(buffer *[]byte, client *connection.Client) (int, int, error) {
	var payload protobuf.CS_24004
	if err := proto.Unmarshal(*buffer, &payload); err != nil {
		return 0, 24005, err
	}

	activity, err := loadActivityTemplate(payload.GetActivityId())
	if err != nil {
		return 0, 24005, err
	}
	if activity.Type != activityTypeChallenge {
		return 0, 24005, fmt.Errorf("unexpected challenge activity type: %d", activity.Type)
	}

	config, err := loadActivityEventChallenge(activity)
	if err != nil {
		return 0, 24005, err
	}

	seasonID := uint32(1)
	if config.ID > 0 {
		seasonID = config.ID
	}
	currentChallenge := &protobuf.CHALLENGEINFO{
		SeasonMaxScore:   proto.Uint32(0),
		ActivityMaxScore: proto.Uint32(0),
		SeasonMaxLevel:   proto.Uint32(0),
		ActivityMaxLevel: proto.Uint32(0),
		SeasonId:         proto.Uint32(seasonID),
		DungeonIdList:    challengeDungeonList(config, seasonID),
		BuffList:         config.Buff,
	}
	userChallenge, err := buildChallengeUserInfo(client, payload.GetActivityId(), currentChallenge)
	if err != nil {
		return 0, 24005, err
	}

	response := protobuf.SC_24005{
		Result:           proto.Uint32(0),
		CurrentChallenge: currentChallenge,
		UserChallenge:    userChallenge,
	}
	return client.SendMessage(24005, &response)
}

func loadActivityEventChallenge(activity activityTemplate) (activityEventChallenge, error) {
	entry, err := orm.GetConfigEntry("ShareCfg/activity_event_challenge.json", strconv.FormatUint(uint64(activity.ConfigID), 10))
	if err != nil {
		return activityEventChallenge{}, err
	}
	var config activityEventChallenge
	if err := json.Unmarshal(entry.Data, &config); err != nil {
		return activityEventChallenge{}, err
	}
	return config, nil
}

func challengeDungeonList(config activityEventChallenge, seasonID uint32) []uint32 {
	if len(config.InfiniteStage) == 0 {
		return []uint32{}
	}
	seasonIndex := int(seasonID - 1)
	if seasonIndex < 0 || seasonIndex >= len(config.InfiniteStage) {
		seasonIndex = 0
	}
	if len(config.InfiniteStage[seasonIndex]) == 0 {
		return []uint32{}
	}
	return cloneUint32Slice(config.InfiniteStage[seasonIndex][0])
}

func buildChallengeUserInfo(client *connection.Client, activityID uint32, currentChallenge *protobuf.CHALLENGEINFO) ([]*protobuf.USERCHALLENGEINFO, error) {
	states, err := orm.ListChallengeModeStates(client.Commander.CommanderID, activityID)
	if err != nil {
		return nil, err
	}
	if len(states) == 0 {
		return []*protobuf.USERCHALLENGEINFO{}, nil
	}
	if client.Commander.OwnedShipsMap == nil {
		if err := client.Commander.Load(); err != nil {
			return nil, err
		}
	}

	result := make([]*protobuf.USERCHALLENGEINFO, 0, len(states))
	for _, state := range states {
		seasonID := state.SeasonID
		if seasonID == 0 {
			seasonID = currentChallenge.GetSeasonId()
		}
		groups := []*protobuf.GROUPINFOINCHALLENGE{}
		if state.RegularGroupID > 0 {
			groups = append(groups, buildChallengeGroupInChallenge(client, state.RegularGroupID, state.RegularShipIDs, state.RegularCommanders))
		}
		if state.SubmarineGroupID > 0 {
			groups = append(groups, buildChallengeGroupInChallenge(client, state.SubmarineGroupID, state.SubmarineShipIDs, state.SubmarineCommanders))
		}
		result = append(result, &protobuf.USERCHALLENGEINFO{
			CurrentScore:  proto.Uint32(state.CurrentScore),
			Level:         proto.Uint32(state.Level),
			GroupincList:  groups,
			Mode:          proto.Uint32(state.Mode),
			Issl:          proto.Uint32(state.Issl),
			SeasonId:      proto.Uint32(seasonID),
			DungeonIdList: cloneUint32Slice(currentChallenge.GetDungeonIdList()),
			BuffList:      cloneUint32Slice(currentChallenge.GetBuffList()),
		})
	}
	sort.Slice(result, func(i int, j int) bool {
		return result[i].GetMode() < result[j].GetMode()
	})
	return result, nil
}

func buildChallengeGroupInChallenge(client *connection.Client, groupID uint32, shipIDs []uint32, commanders []orm.ChallengeCommanderSlot) *protobuf.GROUPINFOINCHALLENGE {
	ships := make([]*protobuf.SHIPINCHALLENGE, 0, len(shipIDs))
	for _, shipID := range shipIDs {
		owned, ok := client.Commander.OwnedShipsMap[shipID]
		if !ok {
			continue
		}
		ships = append(ships, &protobuf.SHIPINCHALLENGE{
			Id:       proto.Uint32(shipID),
			HpRant:   proto.Uint32(challengeShipFullHPRatio),
			ShipInfo: orm.ToProtoOwnedShip(*owned, nil, nil),
		})
	}
	protoCommanders := make([]*protobuf.COMMANDERINCHALLENGE, 0, len(commanders))
	for _, slot := range commanders {
		if slot.CommanderID == 0 {
			continue
		}
		meow, err := orm.GetCommanderMeow(client.Commander.CommanderID, slot.CommanderID)
		if err != nil {
			continue
		}
		protoCommanders = append(protoCommanders, &protobuf.COMMANDERINCHALLENGE{
			Pos:           proto.Uint32(slot.Pos),
			Commanderinfo: orm.ToProtoCommanderInfo(*meow),
		})
	}
	return &protobuf.GROUPINFOINCHALLENGE{
		Id:         proto.Uint32(groupID),
		Ships:      ships,
		Commanders: protoCommanders,
	}
}
