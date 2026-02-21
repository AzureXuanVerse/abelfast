package island

import (
	"google.golang.org/protobuf/proto"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/db"
	"github.com/ggmolly/belfast/internal/orm"
	"github.com/ggmolly/belfast/internal/protobuf"
)

const islandCardAchievementMaxDisplay = 4

func IslandSetCardAchievements(buffer *[]byte, client *connection.Client) (int, int, error) {
	var payload protobuf.CS_21338
	if err := proto.Unmarshal(*buffer, &payload); err != nil {
		return 0, 21339, err
	}

	response := &protobuf.SC_21339{Result: proto.Uint32(1)}
	requestedGroups := dedupeTaskIDs(payload.GetGroupList())
	if len(requestedGroups) > islandCardAchievementMaxDisplay {
		return client.SendMessage(21339, response)
	}

	byID, byGroup, err := loadIslandAchievementCardConfig()
	if err != nil {
		return client.SendMessage(21339, response)
	}

	achievementState, err := orm.GetIslandAchievementState(client.Commander.CommanderID)
	if err != nil {
		if !db.IsNotFound(err) {
			return client.SendMessage(21339, response)
		}
		achievementState = orm.NewIslandAchievementState(client.Commander.CommanderID)
	}

	finishedSet := make(map[uint32]struct{}, len(achievementState.FinishList))
	for _, achievementID := range achievementState.FinishList {
		finishedSet[achievementID] = struct{}{}
	}

	selected := make([]uint32, 0, len(requestedGroups))
	for _, groupID := range requestedGroups {
		groupAchievements, ok := byGroup[groupID]
		if !ok || len(groupAchievements) == 0 {
			return client.SendMessage(21339, response)
		}
		selectedID := uint32(0)
		for _, achievementID := range groupAchievements {
			if _, done := finishedSet[achievementID]; !done {
				continue
			}
			cfg := byID[achievementID]
			if selectedID == 0 || cfg.Stage > byID[selectedID].Stage {
				selectedID = achievementID
			}
		}
		if selectedID == 0 {
			return client.SendMessage(21339, response)
		}
		selected = append(selected, selectedID)
	}

	cardState, err := orm.GetIslandCardState(client.Commander.CommanderID)
	if err != nil {
		if !db.IsNotFound(err) {
			return client.SendMessage(21339, response)
		}
		cardState = orm.NewIslandCardState(client.Commander.CommanderID)
	}
	cardState.AchieveDisplayIDs = selected
	if err := orm.UpsertIslandCardState(cardState); err != nil {
		return client.SendMessage(21339, response)
	}

	response.Result = proto.Uint32(0)
	return client.SendMessage(21339, response)
}
