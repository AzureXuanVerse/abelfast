package island

import (
	"context"

	"github.com/jackc/pgx/v5"
	"google.golang.org/protobuf/proto"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/consts"
	"github.com/ggmolly/belfast/internal/db"
	"github.com/ggmolly/belfast/internal/orm"
	"github.com/ggmolly/belfast/internal/protobuf"
)

func HandleIslandWildGatherCollect(buffer *[]byte, client *connection.Client) (int, int, error) {
	var request protobuf.CS_21524
	if err := proto.Unmarshal(*buffer, &request); err != nil {
		return 0, 21525, err
	}

	response := &protobuf.SC_21525{Result: proto.Uint32(1), DropList: []*protobuf.DROPINFO{}}
	if request.GetIslandId() == 0 || request.GetGatherId() == 0 || !globalIslandRuntimeState.hasMatchingSession(client.Commander.CommanderID, request.GetIslandId()) {
		return client.SendMessage(21525, response)
	}

	template, err := loadIslandWildGatherCollectTemplate(request.GetGatherId())
	if err != nil {
		return client.SendMessage(21525, response)
	}

	drops := buildIslandWildGatherDrops(template)
	if len(drops) == 0 {
		return client.SendMessage(21525, response)
	}

	err = db.DefaultStore.WithPGXTx(context.Background(), func(tx pgx.Tx) error {
		created, err := orm.CreateIslandWildGatherCollectStateTx(context.Background(), tx, request.GetIslandId(), request.GetGatherId(), client.Commander.CommanderID)
		if err != nil {
			return err
		}
		if !created {
			return nil
		}
		if err := applyIslandDropsTx(context.Background(), tx, client, drops); err != nil {
			return err
		}
		response.Result = proto.Uint32(0)
		response.DropList = mergeDropList(drops)
		return nil
	})
	if err != nil {
		return client.SendMessage(21525, response)
	}

	if _, _, err := client.SendMessage(21525, response); err != nil {
		return 0, 21525, err
	}

	if response.GetResult() == 0 {
		push := &protobuf.SC_21528{
			IslandId: proto.Uint32(request.GetIslandId()),
			GatherList: []*protobuf.PB_ISLAND_GATHER_PUSH{{
				Id:       proto.Uint32(request.GetGatherId()),
				Pos:      proto.Uint32(0),
				State:    proto.Uint32(0),
				Mark:     proto.Uint32(0),
				PushType: proto.Uint32(2),
			}},
		}
		broadcastIslandPacket(client.Server, request.GetIslandId(), 21528, push)
		if client.Server == nil {
			client.SendMessage(21528, push)
		} else if existing, ok := client.Server.FindClient(client.Hash); !ok || existing != client {
			client.SendMessage(21528, push)
		}
	}

	return 0, 21525, nil
}

func buildIslandWildGatherDrops(template *islandWildGatherCollectTemplate) []*protobuf.DROPINFO {
	drops := make([]*protobuf.DROPINFO, 0)
	for _, entry := range template.DropDisplay {
		if len(entry) < 3 {
			continue
		}
		drops = append(drops, newDropInfo(entry[0], entry[1], entry[2]))
	}
	if len(drops) == 0 {
		for _, entry := range template.Award {
			if len(entry) < 3 {
				continue
			}
			drops = append(drops, newDropInfo(entry[0], entry[1], entry[2]))
		}
	}
	if len(drops) == 0 {
		for _, entry := range template.DropList {
			if len(entry) < 2 {
				continue
			}
			drops = append(drops, newDropInfo(consts.DROP_TYPE_ISLAND_ITEM, entry[0], entry[1]))
		}
	}
	return mergeDropList(drops)
}
