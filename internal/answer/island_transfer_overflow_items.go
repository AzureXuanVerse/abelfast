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

func IslandTransferOverflowItems(buffer *[]byte, client *connection.Client) (int, int, error) {
	var payload protobuf.CS_21006
	if err := proto.Unmarshal(*buffer, &payload); err != nil {
		return 0, 21007, err
	}

	response := &protobuf.SC_21007{Result: proto.Uint32(0), ItemList: []*protobuf.PB_ISLAND_ITEM{}}
	if payload.GetType() != 0 {
		response.Result = proto.Uint32(1)
		return client.SendMessage(21007, response)
	}
	if err := ensureCommanderLoaded(client, "Island/OverflowTransfer"); err != nil {
		response.Result = proto.Uint32(1)
		return client.SendMessage(21007, response)
	}

	err := db.DefaultStore.WithPGXTx(context.Background(), func(tx pgx.Tx) error {
		overflowRows, err := orm.ListIslandOverflowInventoryForUpdateTx(context.Background(), tx, client.Commander.CommanderID)
		if err != nil {
			response.Result = proto.Uint32(1)
			return err
		}
		for i := range overflowRows {
			if overflowRows[i].Count == 0 {
				continue
			}
			if err := orm.AddIslandInventoryTx(context.Background(), tx, client.Commander.CommanderID, overflowRows[i].ItemID, overflowRows[i].Count); err != nil {
				response.Result = proto.Uint32(1)
				return err
			}
			response.ItemList = append(response.ItemList, &protobuf.PB_ISLAND_ITEM{Id: proto.Uint32(overflowRows[i].ItemID), Num: proto.Uint32(overflowRows[i].Count)})
		}
		if err := orm.ClearIslandOverflowInventoryTx(context.Background(), tx, client.Commander.CommanderID); err != nil {
			response.Result = proto.Uint32(1)
			return err
		}
		return nil
	})
	if err != nil {
		return client.SendMessage(21007, response)
	}
	return client.SendMessage(21007, response)
}
