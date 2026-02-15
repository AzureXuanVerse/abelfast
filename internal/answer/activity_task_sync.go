package answer

import (
	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/orm"
	"github.com/ggmolly/belfast/internal/protobuf"
	"google.golang.org/protobuf/proto"
)

func ActivityTaskStateSync(buffer *[]byte, client *connection.Client) (int, int, error) {
	initInfo, info, err := buildActivityTaskSyncInfo(client.Commander.CommanderID)
	if err != nil {
		return 0, 20204, err
	}

	if _, _, err := client.SendMessage(20201, &protobuf.SC_20201{Info: initInfo}); err != nil {
		return 0, 20201, err
	}
	if _, _, err := client.SendMessage(20202, &protobuf.SC_20202{Info: info}); err != nil {
		return 0, 20202, err
	}
	if _, _, err := client.SendMessage(20203, &protobuf.SC_20203{Info: info}); err != nil {
		return 0, 20203, err
	}
	return client.SendMessage(20204, &protobuf.SC_20204{Info: info})
}

func buildActivityTaskSyncInfo(commanderID uint32) ([]*protobuf.ACT_TASK_INIT_LIST, []*protobuf.ACT_TASK_LIST, error) {
	rows, err := orm.ListCommanderActivityTasks(commanderID)
	if err != nil {
		return nil, nil, err
	}

	type taskGroup struct {
		tasks   []*protobuf.ACT_TASK
		finishs []uint32
	}
	grouped := make(map[uint32]*taskGroup)
	order := make([]uint32, 0)

	for _, row := range rows {
		group := grouped[row.ActID]
		if group == nil {
			group = &taskGroup{tasks: []*protobuf.ACT_TASK{}, finishs: []uint32{}}
			grouped[row.ActID] = group
			order = append(order, row.ActID)
		}
		group.tasks = append(group.tasks, &protobuf.ACT_TASK{Id: proto.Uint32(row.TaskID), Progress: proto.Uint32(row.Progress)})
		if row.Submitted {
			group.finishs = append(group.finishs, row.TaskID)
		}
	}

	initInfo := make([]*protobuf.ACT_TASK_INIT_LIST, 0, len(order))
	info := make([]*protobuf.ACT_TASK_LIST, 0, len(order))
	for _, actID := range order {
		group := grouped[actID]
		initInfo = append(initInfo, &protobuf.ACT_TASK_INIT_LIST{
			ActId:     proto.Uint32(actID),
			Tasks:     group.tasks,
			FinishIds: group.finishs,
		})
		info = append(info, &protobuf.ACT_TASK_LIST{ActId: proto.Uint32(actID), Tasks: group.tasks})
	}

	return initInfo, info, nil
}
