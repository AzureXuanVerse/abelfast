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

func IslandGiveCardLike(buffer *[]byte, client *connection.Client) (int, int, error) {
	var payload protobuf.CS_21334
	if err := proto.Unmarshal(*buffer, &payload); err != nil {
		return 0, 21335, err
	}

	response := &protobuf.SC_21335{Result: proto.Uint32(1)}
	targetID := payload.GetUserId()
	if targetID == 0 || targetID == client.Commander.CommanderID {
		return client.SendMessage(21335, response)
	}
	if _, err := orm.GetCommanderCoreByID(targetID); err != nil {
		return client.SendMessage(21335, response)
	}

	err := db.DefaultStore.WithPGXTx(context.Background(), func(tx pgx.Tx) error {
		state, err := orm.GetIslandCardStateForUpdateTx(context.Background(), tx, targetID)
		if err != nil {
			return err
		}
		if state.SocialFlag == 0 {
			return nil
		}

		inserted, err := orm.AddIslandCardLikeTx(context.Background(), tx, client.Commander.CommanderID, targetID)
		if err != nil {
			return err
		}
		if !inserted {
			return nil
		}
		state.GoodNum++
		if err := orm.SaveIslandCardStateTx(context.Background(), tx, state); err != nil {
			return err
		}

		response.Result = proto.Uint32(0)
		return nil
	})
	if err != nil {
		return client.SendMessage(21335, response)
	}

	return client.SendMessage(21335, response)
}
