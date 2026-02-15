package answer

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/orm"
	"github.com/ggmolly/belfast/internal/protobuf"
	"google.golang.org/protobuf/proto"
)

func QuickFinishActivityTask(buffer *[]byte, client *connection.Client) (int, int, error) {
	var payload protobuf.CS_20207
	if err := proto.Unmarshal(*buffer, &payload); err != nil {
		return 0, 20208, err
	}

	response := &protobuf.SC_20208{Result: proto.Uint32(activityTaskResultFailure), AwardList: []*protobuf.DROPINFO{}}
	actID := payload.GetActId()
	taskID := payload.GetTaskId()
	itemCost := payload.GetItemCost()
	if actID == 0 || taskID == 0 || itemCost == 0 {
		return client.SendMessage(20208, response)
	}

	activityTaskIDs, err := loadActivityTaskIDSet(actID)
	if err != nil {
		return 0, 20208, err
	}
	if _, ok := activityTaskIDs[taskID]; !ok {
		return client.SendMessage(20208, response)
	}

	template, err := loadActivityTaskTemplate(taskID)
	if err != nil {
		return 0, 20208, err
	}
	if template.QuickFinish == 0 || template.QuickFinish != itemCost {
		return client.SendMessage(20208, response)
	}

	drops, err := buildAwardDropMap(template.AwardDisplay)
	if err != nil {
		return 0, 20208, err
	}

	errQuickFinishRejected := errors.New("quick finish rejected")
	err = orm.WithPGXTx(context.Background(), func(tx pgx.Tx) error {
		submitted, err := orm.TrySubmitCommanderActivityTaskTx(context.Background(), tx, client.Commander.CommanderID, actID, taskID)
		if err != nil {
			return err
		}
		if !submitted {
			return errQuickFinishRejected
		}

		consumed, err := consumeCommanderItemTx(context.Background(), tx, client.Commander.CommanderID, quickTaskTicketItemID, itemCost)
		if err != nil {
			return err
		}
		if !consumed {
			return errQuickFinishRejected
		}

		if err := applyActivityTaskDropsTx(context.Background(), tx, client.Commander.CommanderID, drops); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		if errors.Is(err, errQuickFinishRejected) {
			return client.SendMessage(20208, response)
		}
		return 0, 20208, err
	}

	if err := client.Commander.Load(); err != nil {
		return 0, 20208, err
	}

	response.Result = proto.Uint32(activityTaskResultSuccess)
	response.AwardList = activityDropMapToSortedList(drops)
	return client.SendMessage(20208, response)
}
