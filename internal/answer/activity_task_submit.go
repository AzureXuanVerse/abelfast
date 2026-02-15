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

func SubmitActivityTask(buffer *[]byte, client *connection.Client) (int, int, error) {
	var payload protobuf.CS_20205
	if err := proto.Unmarshal(*buffer, &payload); err != nil {
		return 0, 20206, err
	}

	response := &protobuf.SC_20206{Result: proto.Uint32(activityTaskResultFailure), AwardList: []*protobuf.DROPINFO{}}
	actID := payload.GetActId()
	taskIDs := payload.GetTaskIds()
	if actID == 0 || len(taskIDs) == 0 {
		return client.SendMessage(20206, response)
	}

	activityTaskIDs, err := loadActivityTaskIDSet(actID)
	if err != nil {
		return 0, 20206, err
	}

	errActivityTaskRejected := errors.New("activity task submit rejected")
	templates := make(map[uint32]activityTaskTemplate)
	orderedTaskIDs := make([]uint32, 0, len(taskIDs))
	uniqueTaskIDs := make(map[uint32]struct{}, len(taskIDs))
	for _, taskID := range taskIDs {
		if taskID == 0 {
			return client.SendMessage(20206, response)
		}
		if _, seen := uniqueTaskIDs[taskID]; seen {
			continue
		}
		if _, ok := activityTaskIDs[taskID]; !ok {
			return client.SendMessage(20206, response)
		}
		template, err := loadActivityTaskTemplate(taskID)
		if err != nil {
			return 0, 20206, err
		}
		uniqueTaskIDs[taskID] = struct{}{}
		templates[taskID] = template
		orderedTaskIDs = append(orderedTaskIDs, taskID)
	}

	drops := make(map[string]*protobuf.DROPINFO)

	err = orm.WithPGXTx(context.Background(), func(tx pgx.Tx) error {
		for _, taskID := range orderedTaskIDs {
			submitted, err := orm.TrySubmitCommanderActivityTaskTx(context.Background(), tx, client.Commander.CommanderID, actID, taskID)
			if err != nil {
				return err
			}
			if !submitted {
				return errActivityTaskRejected
			}

			template := templates[taskID]
			taskDrops, err := buildAwardDropMap(template.AwardDisplay)
			if err != nil {
				return err
			}
			for key, drop := range taskDrops {
				if existing := drops[key]; existing != nil {
					existing.Number = proto.Uint32(existing.GetNumber() + drop.GetNumber())
					continue
				}
				drops[key] = drop
			}
		}

		if err := applyActivityTaskDropsTx(context.Background(), tx, client.Commander.CommanderID, drops); err != nil {
			return err
		}
		response.Result = proto.Uint32(activityTaskResultSuccess)
		return nil
	})
	if err != nil {
		if errors.Is(err, errActivityTaskRejected) {
			return client.SendMessage(20206, response)
		}
		return 0, 20206, err
	}

	if err := client.Commander.Load(); err != nil {
		return 0, 20206, err
	}

	response.AwardList = activityDropMapToSortedList(drops)
	return client.SendMessage(20206, response)
}
