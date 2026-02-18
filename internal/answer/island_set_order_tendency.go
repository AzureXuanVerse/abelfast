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

const (
	islandOrderTendencyOK      = uint32(0)
	islandOrderTendencyInvalid = uint32(1)
	islandOrderTendencyPersist = uint32(2)
)

func IslandSetOrderTendency(buffer *[]byte, client *connection.Client) (int, int, error) {
	var payload protobuf.CS_21410
	if err := proto.Unmarshal(*buffer, &payload); err != nil {
		return 0, 21411, err
	}

	response := &protobuf.SC_21411{Result: proto.Uint32(islandOrderTendencyInvalid)}
	tendency := payload.GetType()
	if tendency > 2 {
		return client.SendMessage(21411, response)
	}

	err := db.DefaultStore.WithPGXTx(context.Background(), func(tx pgx.Tx) error {
		state, err := orm.GetIslandOrderStateForUpdateTx(context.Background(), tx, client.Commander.CommanderID)
		if err != nil {
			response.Result = proto.Uint32(islandOrderTendencyPersist)
			return err
		}
		state.DailySelect = tendency
		if err := orm.SaveIslandOrderStateTx(context.Background(), tx, state); err != nil {
			response.Result = proto.Uint32(islandOrderTendencyPersist)
			return err
		}

		response.Result = proto.Uint32(islandOrderTendencyOK)
		return nil
	})
	if err != nil {
		return client.SendMessage(21411, response)
	}

	return client.SendMessage(21411, response)
}
