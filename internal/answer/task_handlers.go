package answer

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"google.golang.org/protobuf/proto"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/db"
	"github.com/ggmolly/belfast/internal/orm"
	"github.com/ggmolly/belfast/internal/protobuf"
)

const (
	taskResultSuccess      = uint32(0)
	taskResultFailed       = uint32(1)
	quickTaskPassTicketID  = uint32(15013)
	taskTemplateCategory   = "ShareCfg/task_data_template.json"
	taskTemplateCategoryLC = "sharecfgdata/task_data_template.json"
)

type taskTemplate struct {
	ID           uint32     `json:"id"`
	TargetNum    uint32     `json:"target_num"`
	QuickFinish  uint32     `json:"quick_finish"`
	AwardDisplay [][]uint32 `json:"award_display"`
}

func UpdateTaskProgress(buffer *[]byte, client *connection.Client) (int, int, error) {
	var payload protobuf.CS_20009
	if err := proto.Unmarshal(*buffer, &payload); err != nil {
		return 0, 20010, err
	}

	response := protobuf.SC_20010{Result: proto.Uint32(taskResultFailed)}
	updates := payload.GetProgressinfo()
	if len(updates) == 0 {
		return client.SendMessage(20010, &response)
	}

	now := uint32(time.Now().Unix())
	err := orm.WithPGXTx(context.Background(), func(tx pgx.Tx) error {
		for _, update := range updates {
			if update == nil || update.Id == nil || update.Mode == nil || update.Progress == nil {
				return errors.New("invalid task update")
			}
			template, ok, err := loadTaskTemplate(update.GetId())
			if err != nil {
				return err
			}
			if !ok {
				return errors.New("unknown task id")
			}
			if err := orm.UpsertCommanderTaskProgressTx(context.Background(), tx, client.Commander.CommanderID, update.GetId(), update.GetMode(), update.GetProgress(), template.TargetNum, now); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return client.SendMessage(20010, &response)
	}

	response.Result = proto.Uint32(taskResultSuccess)
	return client.SendMessage(20010, &response)
}

func TaskProgressEvent(buffer *[]byte, client *connection.Client) (int, int, error) {
	var payload protobuf.CS_20016
	if err := proto.Unmarshal(*buffer, &payload); err != nil {
		return 0, 20017, err
	}

	response := protobuf.SC_20017{Result: proto.Uint32(taskResultFailed)}
	if payload.EventType == nil || payload.EventTarget == nil || payload.EventCount == nil {
		return client.SendMessage(20017, &response)
	}
	if payload.GetEventType() == 0 || payload.GetEventTarget() == 0 || payload.GetEventCount() == 0 {
		return client.SendMessage(20017, &response)
	}

	response.Result = proto.Uint32(taskResultSuccess)
	return client.SendMessage(20017, &response)
}

func SubmitTask(buffer *[]byte, client *connection.Client) (int, int, error) {
	var payload protobuf.CS_20005
	if err := proto.Unmarshal(*buffer, &payload); err != nil {
		return 0, 20006, err
	}

	response := protobuf.SC_20006{Result: proto.Uint32(taskResultFailed), AwardList: []*protobuf.DROPINFO{}}
	template, ok, err := loadTaskTemplate(payload.GetId())
	if err != nil || !ok {
		return client.SendMessage(20006, &response)
	}

	drops, ok, err := submitTaskWithOptionalTicket(client, payload.GetId(), template, 0)
	if err != nil {
		return 0, 20006, err
	}
	if !ok {
		return client.SendMessage(20006, &response)
	}

	response.Result = proto.Uint32(taskResultSuccess)
	response.AwardList = dropMapToSortedList(drops)
	return client.SendMessage(20006, &response)
}

func SubmitTaskBatch(buffer *[]byte, client *connection.Client) (int, int, error) {
	var payload protobuf.CS_20011
	if err := proto.Unmarshal(*buffer, &payload); err != nil {
		return 0, 20012, err
	}

	response := protobuf.SC_20012{IdList: []uint32{}, AwardList: []*protobuf.DROPINFO{}}
	seen := make(map[uint32]struct{})
	merged := make(map[string]*protobuf.DROPINFO)

	for _, taskID := range payload.GetIdList() {
		if _, ok := seen[taskID]; ok {
			continue
		}
		seen[taskID] = struct{}{}

		template, ok, err := loadTaskTemplate(taskID)
		if err != nil {
			return 0, 20012, err
		}
		if !ok {
			continue
		}

		drops, submitted, err := submitTaskWithOptionalTicket(client, taskID, template, 0)
		if err != nil {
			return 0, 20012, err
		}
		if !submitted {
			continue
		}

		response.IdList = append(response.IdList, taskID)
		for _, drop := range drops {
			accumulateDrop(merged, drop.GetType(), drop.GetId(), drop.GetNumber())
		}
	}

	response.AwardList = dropMapToSortedList(merged)
	return client.SendMessage(20012, &response)
}

func SubmitQuickTask(buffer *[]byte, client *connection.Client) (int, int, error) {
	var payload protobuf.CS_20013
	if err := proto.Unmarshal(*buffer, &payload); err != nil {
		return 0, 20014, err
	}

	response := protobuf.SC_20014{Result: proto.Uint32(taskResultFailed), AwardList: []*protobuf.DROPINFO{}}
	template, ok, err := loadTaskTemplate(payload.GetId())
	if err != nil || !ok {
		return client.SendMessage(20014, &response)
	}
	if template.QuickFinish == 0 || payload.GetItemCost() != template.QuickFinish {
		return client.SendMessage(20014, &response)
	}

	drops, ok, err := submitTaskWithOptionalTicket(client, payload.GetId(), template, template.QuickFinish)
	if err != nil {
		return 0, 20014, err
	}
	if !ok {
		return client.SendMessage(20014, &response)
	}

	response.Result = proto.Uint32(taskResultSuccess)
	response.AwardList = dropMapToSortedList(drops)
	return client.SendMessage(20014, &response)
}

func submitTaskWithOptionalTicket(client *connection.Client, taskID uint32, template *taskTemplate, ticketCost uint32) (map[string]*protobuf.DROPINFO, bool, error) {
	if template == nil {
		return nil, false, nil
	}

	drops := buildTaskDrops(template)
	err := orm.WithPGXTx(context.Background(), func(tx pgx.Tx) error {
		ctx := context.Background()
		now := uint32(time.Now().Unix())

		task, err := orm.GetCommanderTaskTx(ctx, tx, client.Commander.CommanderID, taskID)
		if err != nil && !(ticketCost > 0 && db.IsNotFound(err)) {
			return err
		}
		if task != nil && task.SubmitTime != 0 {
			return db.ErrNotFound
		}

		if ticketCost > 0 {
			if !client.Commander.HasEnoughItem(quickTaskPassTicketID, ticketCost) {
				return db.ErrNotFound
			}
			if err := client.Commander.ConsumeItemTx(ctx, tx, quickTaskPassTicketID, ticketCost); err != nil {
				return err
			}
			if err := orm.UpsertCommanderTaskProgressTx(ctx, tx, client.Commander.CommanderID, taskID, orm.TaskProgressUpdate, template.TargetNum, template.TargetNum, now); err != nil {
				return err
			}
		}
		if task == nil {
			task, err = orm.GetCommanderTaskTx(ctx, tx, client.Commander.CommanderID, taskID)
			if err != nil {
				return err
			}
		}
		if template.TargetNum > 0 && task.Progress < template.TargetNum {
			return db.ErrNotFound
		}

		submitted, err := orm.MarkCommanderTaskSubmittedTx(ctx, tx, client.Commander.CommanderID, taskID, now)
		if err != nil {
			return err
		}
		if !submitted {
			return db.ErrNotFound
		}

		if err := applyLoveLetterDropsTx(ctx, tx, client, drops); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		if db.IsNotFound(err) {
			return nil, false, nil
		}
		return nil, false, err
	}
	return drops, true, nil
}

func loadTaskTemplate(taskID uint32) (*taskTemplate, bool, error) {
	entry, err := orm.GetConfigEntry(taskTemplateCategory, fmt.Sprintf("%d", taskID))
	if err != nil {
		if db.IsNotFound(err) {
			entry, err = orm.GetConfigEntry(taskTemplateCategoryLC, fmt.Sprintf("%d", taskID))
			if err != nil {
				if db.IsNotFound(err) {
					return nil, false, nil
				}
				return nil, false, err
			}
		} else {
			return nil, false, err
		}
	}

	var template taskTemplate
	if err := json.Unmarshal(entry.Data, &template); err != nil {
		return nil, false, err
	}
	return &template, true, nil
}

func buildTaskDrops(template *taskTemplate) map[string]*protobuf.DROPINFO {
	drops := make(map[string]*protobuf.DROPINFO)
	for _, entry := range template.AwardDisplay {
		if len(entry) < 3 {
			continue
		}
		accumulateDrop(drops, entry[0], entry[1], entry[2])
	}
	return drops
}
