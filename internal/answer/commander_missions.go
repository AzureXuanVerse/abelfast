package answer

import (
	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/orm"
	"github.com/ggmolly/belfast/internal/protobuf"
	"google.golang.org/protobuf/proto"
)

func CommanderMissions(buffer *[]byte, client *connection.Client) (int, int, error) {
	tasks, err := orm.ListCommanderTasks(client.Commander.CommanderID)
	if err != nil {
		return 0, 20001, err
	}
	response := protobuf.SC_20001{Info: make([]*protobuf.TASKINFO, 0, len(tasks))}
	for _, task := range tasks {
		response.Info = append(response.Info, &protobuf.TASKINFO{
			Id:         proto.Uint32(task.TaskID),
			Progress:   proto.Uint32(task.Progress),
			AcceptTime: proto.Uint32(task.AcceptTime),
			SubmitTime: proto.Uint32(task.SubmitTime),
		})
	}
	return client.SendMessage(20001, &response)
}
