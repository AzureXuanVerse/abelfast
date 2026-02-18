package answer

import (
	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/protobuf"
	"google.golang.org/protobuf/proto"
)

func IslandEnterMap(buffer *[]byte, client *connection.Client) (int, int, error) {
	var request protobuf.CS_21213
	if err := proto.Unmarshal(*buffer, &request); err != nil {
		return 0, 21214, err
	}

	if !globalIslandRuntimeState.hasMatchingSession(client.Commander.CommanderID, request.GetIslandId()) {
		response := protobuf.SC_21214{
			Result:       proto.Uint32(1),
			ObjectList:   []*protobuf.PB_OBJECT{},
			GatherList:   []*protobuf.PB_ISLAND_WILD_GATHER{},
			FragmentList: []*protobuf.PB_ISLAND_COLLECT_FRAGMENT{},
			NpcList:      []*protobuf.PB_ISLAND_NPC{},
		}
		return client.SendMessage(21214, &response)
	}

	response, err := loadIslandMapSnapshot(request.GetIslandId(), request.GetMapId())
	if err != nil {
		failed := protobuf.SC_21214{
			Result:       proto.Uint32(1),
			ObjectList:   []*protobuf.PB_OBJECT{},
			GatherList:   []*protobuf.PB_ISLAND_WILD_GATHER{},
			FragmentList: []*protobuf.PB_ISLAND_COLLECT_FRAGMENT{},
			NpcList:      []*protobuf.PB_ISLAND_NPC{},
		}
		return client.SendMessage(21214, &failed)
	}

	return client.SendMessage(21214, response)
}
