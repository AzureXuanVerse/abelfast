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

func IslandRemoveExpiredTicket(buffer *[]byte, client *connection.Client) (int, int, error) {
	var payload protobuf.CS_21425
	if err := proto.Unmarshal(*buffer, &payload); err != nil {
		return 0, 21426, err
	}

	response := &protobuf.SC_21426{Result: proto.Uint32(0)}
	keys := payload.GetTicketKeys()
	if len(keys) == 0 {
		return client.SendMessage(21426, response)
	}
	if err := ensureCommanderLoaded(client, "Island/RemoveExpiredTicket"); err != nil {
		response.Result = proto.Uint32(1)
		return client.SendMessage(21426, response)
	}

	deleteKeys := make([]orm.IslandSpeedupTicketKey, 0, len(keys))
	for i := range keys {
		if keys[i] == nil || keys[i].GetSpeedId() == 0 {
			response.Result = proto.Uint32(1)
			return client.SendMessage(21426, response)
		}
		deleteKeys = append(deleteKeys, orm.IslandSpeedupTicketKey{SpeedID: keys[i].GetSpeedId(), EndTime: keys[i].GetEndTime()})
	}

	err := db.DefaultStore.WithPGXTx(context.Background(), func(tx pgx.Tx) error {
		return orm.DeleteIslandSpeedupTicketKeysTx(context.Background(), tx, client.Commander.CommanderID, deleteKeys)
	})
	if err != nil {
		response.Result = proto.Uint32(1)
	}
	return client.SendMessage(21426, response)
}
