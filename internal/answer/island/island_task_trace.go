package island

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"google.golang.org/protobuf/proto"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/db"
	"github.com/ggmolly/belfast/internal/orm"
	"github.com/ggmolly/belfast/internal/protobuf"
)

func IslandSetTraceTask(buffer *[]byte, client *connection.Client) (int, int, error) {
	var payload protobuf.CS_21034
	if err := proto.Unmarshal(*buffer, &payload); err != nil {
		return 0, 21035, err
	}

	response := protobuf.SC_21035{Result: proto.Uint32(taskResultFailed)}
	if payload.TaskId == nil {
		return client.SendMessage(21035, &response)
	}

	err := db.DefaultStore.WithPGXTx(context.Background(), func(tx pgx.Tx) error {
		state, err := orm.LoadIslandTaskProgressForUpdateTx(context.Background(), tx, client.Commander.CommanderID, time.Now().UTC())
		if err != nil {
			return err
		}

		taskID := payload.GetTaskId()
		if taskID != 0 {
			active := false
			for _, entry := range state.ActiveTasks {
				if entry.TaskID == taskID {
					active = true
					break
				}
			}
			if !active {
				return errIslandTaskInvalid
			}
		}

		state.TraceTaskID = taskID
		if err := orm.SaveIslandTaskProgressTx(context.Background(), tx, state); err != nil {
			return err
		}

		response.Result = proto.Uint32(taskResultSuccess)
		return nil
	})
	if err != nil {
		if errors.Is(err, errIslandTaskInvalid) {
			return client.SendMessage(21035, &response)
		}
		return 0, 21035, err
	}

	return client.SendMessage(21035, &response)
}
