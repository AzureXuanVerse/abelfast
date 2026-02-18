package answer

import (
	"google.golang.org/protobuf/proto"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/orm"
	"github.com/ggmolly/belfast/internal/protobuf"
)

func HandleIslandWildCollectFragmentSign(buffer *[]byte, client *connection.Client) (int, int, error) {
	var request protobuf.CS_21531
	if err := proto.Unmarshal(*buffer, &request); err != nil {
		return 0, 21532, err
	}

	response := &protobuf.SC_21532{Result: proto.Uint32(1)}
	if request.GetIslandId() == 0 || request.GetFragmentId() == 0 || request.GetIslandId() == client.Commander.CommanderID {
		return client.SendMessage(21532, response)
	}

	template, err := loadIslandCollectFragmentTemplate(request.GetFragmentId())
	if err != nil || template.Show != 3 {
		return client.SendMessage(21532, response)
	}

	state := &orm.IslandCollectFragmentSignState{
		IslandID:          request.GetIslandId(),
		FragmentID:        request.GetFragmentId(),
		SignerCommanderID: client.Commander.CommanderID,
		Mark:              client.Commander.CommanderID,
	}
	if err := orm.UpsertIslandCollectFragmentSignState(state); err != nil {
		return client.SendMessage(21532, response)
	}

	response.Result = proto.Uint32(0)
	if _, _, err := client.SendMessage(21532, response); err != nil {
		return 0, 21532, err
	}

	push := &protobuf.SC_21535{
		IslandId: proto.Uint32(request.GetIslandId()),
		FragmentData: []*protobuf.PB_ISLAND_COLLECT_FRAGMENT_PUSH{{
			Id:       proto.Uint32(request.GetFragmentId()),
			Pos:      proto.Uint32(0),
			Mark:     proto.Uint32(client.Commander.CommanderID),
			PushType: proto.Uint32(1),
		}},
	}
	broadcastIslandPacket(client.Server, request.GetIslandId(), 21535, push)
	client.SendMessage(21535, push)

	return 0, 21532, nil
}
