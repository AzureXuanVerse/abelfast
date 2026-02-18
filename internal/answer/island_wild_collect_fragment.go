package answer

import (
	"context"

	"github.com/jackc/pgx/v5"
	"google.golang.org/protobuf/proto"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/db"
	"github.com/ggmolly/belfast/internal/orm"
	"github.com/ggmolly/belfast/internal/protobuf"
)

func HandleIslandWildCollectFragment(buffer *[]byte, client *connection.Client) (int, int, error) {
	var request protobuf.CS_21529
	if err := proto.Unmarshal(*buffer, &request); err != nil {
		return 0, 21530, err
	}

	response := &protobuf.SC_21530{Result: proto.Uint32(1)}
	if request.GetIslandId() == 0 || request.GetFragmentId() == 0 || !globalIslandRuntimeState.hasMatchingSession(client.Commander.CommanderID, request.GetIslandId()) {
		return client.SendMessage(21530, response)
	}

	if _, err := loadIslandCollectFragmentTemplate(request.GetFragmentId()); err != nil {
		return client.SendMessage(21530, response)
	}

	err := db.DefaultStore.WithPGXTx(context.Background(), func(tx pgx.Tx) error {
		created, err := orm.CreateIslandCollectFragmentStateTx(context.Background(), tx, request.GetIslandId(), request.GetFragmentId(), client.Commander.CommanderID, client.Commander.CommanderID)
		if err != nil {
			return err
		}
		if created {
			response.Result = proto.Uint32(0)
		}
		return nil
	})
	if err != nil {
		return client.SendMessage(21530, response)
	}

	if _, _, err := client.SendMessage(21530, response); err != nil {
		return 0, 21530, err
	}

	if response.GetResult() == 0 {
		push := &protobuf.SC_21535{
			IslandId: proto.Uint32(request.GetIslandId()),
			FragmentData: []*protobuf.PB_ISLAND_COLLECT_FRAGMENT_PUSH{{
				Id:       proto.Uint32(request.GetFragmentId()),
				Pos:      proto.Uint32(0),
				Mark:     proto.Uint32(0),
				PushType: proto.Uint32(2),
			}},
		}
		broadcastIslandPacket(client.Server, request.GetIslandId(), 21535, push)
		client.SendMessage(21535, push)
	}

	return 0, 21530, nil
}
