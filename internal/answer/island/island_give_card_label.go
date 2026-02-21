package island

import (
	"context"

	"github.com/jackc/pgx/v5"
	"google.golang.org/protobuf/proto"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/db"
	"github.com/ggmolly/belfast/internal/orm"
	"github.com/ggmolly/belfast/internal/protobuf"
)

func IslandGiveCardLabel(buffer *[]byte, client *connection.Client) (int, int, error) {
	var payload protobuf.CS_21336
	if err := proto.Unmarshal(*buffer, &payload); err != nil {
		return 0, 21337, err
	}

	response := &protobuf.SC_21337{Result: proto.Uint32(1)}
	targetID := payload.GetUserId()
	if targetID == 0 || targetID == client.Commander.CommanderID {
		return client.SendMessage(21337, response)
	}
	if _, err := orm.GetCommanderCoreByID(targetID); err != nil {
		return client.SendMessage(21337, response)
	}

	validLabels, err := loadIslandCardLabelIDs()
	if err != nil {
		return client.SendMessage(21337, response)
	}
	if _, ok := validLabels[payload.GetLabelId()]; !ok {
		return client.SendMessage(21337, response)
	}

	err = db.DefaultStore.WithPGXTx(context.Background(), func(tx pgx.Tx) error {
		inserted, err := orm.AddIslandCardLabelGiftTx(context.Background(), tx, client.Commander.CommanderID, targetID, payload.GetLabelId())
		if err != nil {
			return err
		}
		if !inserted {
			return nil
		}

		state, err := orm.GetIslandCardStateForUpdateTx(context.Background(), tx, targetID)
		if err != nil {
			return err
		}
		orm.AddIslandCardLabelCount(state, payload.GetLabelId())
		if err := orm.SaveIslandCardStateTx(context.Background(), tx, state); err != nil {
			return err
		}

		response.Result = proto.Uint32(0)
		return nil
	})
	if err != nil {
		return client.SendMessage(21337, response)
	}

	return client.SendMessage(21337, response)
}
