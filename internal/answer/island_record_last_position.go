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

func HandleIslandRecordLastPosition(buffer *[]byte, client *connection.Client) (int, int, error) {
	var request protobuf.CS_21229
	if err := proto.Unmarshal(*buffer, &request); err != nil {
		return 0, 0, err
	}

	if request.GetIslandId() != 0 && request.GetIslandId() != client.Commander.CommanderID {
		return 0, 0, nil
	}

	position := request.GetPlayerPosition()
	if position == nil {
		return 0, 0, nil
	}

	err := db.DefaultStore.WithPGXTx(context.Background(), func(tx pgx.Tx) error {
		snapshot, err := orm.GetIslandSnapshotForUpdateTx(context.Background(), tx, client.Commander.CommanderID)
		if err != nil {
			if !db.IsNotFound(err) {
				return err
			}
			snapshot = defaultIslandSnapshot(client.Commander.CommanderID)
			snapshot.DailyTimestamp = uint32(time.Now().UTC().Unix())
		}

		snapshot.MapID = position.GetMapId()
		if pos := position.GetPosition(); pos != nil {
			snapshot.PositionX = pos.GetX()
			snapshot.PositionY = pos.GetY()
			snapshot.PositionZ = pos.GetZ()
		}
		if rot := position.GetRotation(); rot != nil {
			snapshot.RotationX = rot.GetX()
			snapshot.RotationY = rot.GetY()
			snapshot.RotationZ = rot.GetZ()
		}

		return orm.UpsertIslandSnapshotTx(context.Background(), tx, snapshot)
	})
	if err != nil {
		return 0, 0, err
	}

	return 0, 0, nil
}
