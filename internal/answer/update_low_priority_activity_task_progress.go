package answer

import (
	"context"

	"github.com/jackc/pgx/v5"
	"google.golang.org/protobuf/proto"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/orm"
	"github.com/ggmolly/belfast/internal/protobuf"
)

func UpdateLowPriorityActivityTaskProgress(buffer *[]byte, client *connection.Client) (int, int, error) {
	var payload protobuf.CS_20209
	if err := proto.Unmarshal(*buffer, &payload); err != nil {
		return 0, 20210, err
	}

	response := &protobuf.SC_20210{Result: proto.Uint32(activityTaskResultFailure)}
	updates := payload.GetProgressinfo()
	if len(updates) == 0 {
		return client.SendMessage(20210, response)
	}

	activityTaskCache := make(map[uint32]map[uint32]struct{})
	for _, update := range updates {
		actID := update.GetActId()
		taskID := update.GetTaskId()
		mode := update.GetMode()
		if actID == 0 || taskID == 0 {
			return client.SendMessage(20210, response)
		}
		if mode != orm.ActivityTaskProgressModeSet && mode != orm.ActivityTaskProgressModeAppend {
			return client.SendMessage(20210, response)
		}

		taskSet, ok := activityTaskCache[actID]
		if !ok {
			loaded, err := loadActivityTaskIDSet(actID)
			if err != nil {
				return 0, 20210, err
			}
			activityTaskCache[actID] = loaded
			taskSet = loaded
		}
		if _, ok := taskSet[taskID]; !ok {
			return client.SendMessage(20210, response)
		}
	}

	err := orm.WithPGXTx(context.Background(), func(tx pgx.Tx) error {
		for _, update := range updates {
			if err := orm.UpsertCommanderActivityTaskProgressTx(context.Background(), tx, client.Commander.CommanderID, update.GetActId(), update.GetTaskId(), update.GetMode(), update.GetProgress()); err != nil {
				return err
			}
		}
		response.Result = proto.Uint32(activityTaskResultSuccess)
		return nil
	})
	if err != nil {
		return 0, 20210, err
	}

	return client.SendMessage(20210, response)
}
