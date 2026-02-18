package answer

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
	"google.golang.org/protobuf/proto"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/db"
	"github.com/ggmolly/belfast/internal/orm"
	"github.com/ggmolly/belfast/internal/protobuf"
)

func IslandSetAccessTypeLegacy(buffer *[]byte, client *connection.Client) (int, int, error) {
	var payload protobuf.CS_21300
	if err := proto.Unmarshal(*buffer, &payload); err != nil {
		return 0, 21301, err
	}

	response := &protobuf.SC_21301{Result: proto.Uint32(1)}
	openFlag := payload.GetOpenFlag()
	if openFlag != 0 && openFlag != 1 {
		return client.SendMessage(21301, response)
	}

	err := db.DefaultStore.WithPGXTx(context.Background(), func(tx pgx.Tx) error {
		snapshot, err := orm.GetIslandSnapshotForUpdateTx(context.Background(), tx, client.Commander.CommanderID)
		if err != nil {
			if !db.IsNotFound(err) {
				return err
			}
			snapshot = defaultIslandSnapshot(client.Commander.CommanderID)
			snapshot.DailyTimestamp = uint32(time.Now().UTC().Unix())
			if err := orm.UpsertIslandSnapshotTx(context.Background(), tx, snapshot); err != nil {
				return err
			}
			snapshot, err = orm.GetIslandSnapshotForUpdateTx(context.Background(), tx, client.Commander.CommanderID)
			if err != nil {
				return err
			}
		}

		snapshot.OpenFlag = openFlag
		if err := orm.UpsertIslandSnapshotTx(context.Background(), tx, snapshot); err != nil {
			return err
		}

		response.Result = proto.Uint32(0)
		return nil
	})
	if err != nil {
		response.Result = proto.Uint32(1)
	}

	return client.SendMessage(21301, response)
}
