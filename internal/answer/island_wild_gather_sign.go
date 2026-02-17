package answer

import (
	"encoding/json"
	"strconv"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/db"
	"github.com/ggmolly/belfast/internal/orm"
	"github.com/ggmolly/belfast/internal/protobuf"
	"google.golang.org/protobuf/proto"
)

const (
	islandWildGatherCategory   = "ShareCfg/island_wild_gather.json"
	islandWildGatherCategoryLC = "sharecfgdata/island_wild_gather.json"
)

type islandWildGatherTemplate struct {
	Show uint32 `json:"show"`
}

func HandleIslandWildGatherSign(buffer *[]byte, client *connection.Client) (int, int, error) {
	var request protobuf.CS_21526
	if err := proto.Unmarshal(*buffer, &request); err != nil {
		return 0, 21527, err
	}

	result := uint32(1)
	if request.GetIslandId() != 0 && request.GetGatherId() != 0 && request.GetIslandId() != client.Commander.CommanderID {
		enabled, err := isIslandWildGatherSignEnabled(request.GetGatherId())
		if err == nil && enabled {
			state := &orm.IslandWildGatherSignState{
				IslandID:          request.GetIslandId(),
				GatherID:          request.GetGatherId(),
				SignerCommanderID: client.Commander.CommanderID,
				Mark:              client.Commander.CommanderID,
			}
			if saveErr := orm.UpsertIslandWildGatherSignState(state); saveErr == nil {
				result = 0
			}
		}
	}

	ack := protobuf.SC_21527{Result: proto.Uint32(result)}
	if _, _, err := client.SendMessage(21527, &ack); err != nil {
		return 0, 21527, err
	}

	if result == 0 {
		push := &protobuf.SC_21528{
			IslandId: proto.Uint32(request.GetIslandId()),
			GatherList: []*protobuf.PB_ISLAND_GATHER_PUSH{
				{
					Id:       proto.Uint32(request.GetGatherId()),
					Pos:      proto.Uint32(0),
					State:    proto.Uint32(1),
					Mark:     proto.Uint32(client.Commander.CommanderID),
					PushType: proto.Uint32(1),
				},
			},
		}
		broadcastIslandPacket(client.Server, request.GetIslandId(), 21528, push)
		if !globalIslandRuntimeState.hasMatchingSession(client.Commander.CommanderID, request.GetIslandId()) {
			client.SendMessage(21528, push)
		}
	}

	return 0, 21527, nil
}

func isIslandWildGatherSignEnabled(gatherID uint32) (bool, error) {
	template, err := loadIslandWildGatherTemplate(islandWildGatherCategory, gatherID)
	if err != nil {
		if db.IsNotFound(err) {
			fallback, fallbackErr := loadIslandWildGatherTemplate(islandWildGatherCategoryLC, gatherID)
			if fallbackErr != nil {
				return false, fallbackErr
			}
			return fallback.Show == 3, nil
		}
		return false, err
	}
	return template.Show == 3, nil
}

func loadIslandWildGatherTemplate(category string, gatherID uint32) (*islandWildGatherTemplate, error) {
	entry, err := orm.GetConfigEntry(category, strconv.FormatUint(uint64(gatherID), 10))
	if err != nil {
		return nil, err
	}
	template := &islandWildGatherTemplate{}
	if err := json.Unmarshal(entry.Data, template); err != nil {
		return nil, err
	}
	return template, nil
}
