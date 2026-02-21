package answer

import (
	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/protobuf"
	"google.golang.org/protobuf/proto"
)

func MetaCharacterTacticsRequestCommandResponse(buffer *[]byte, client *connection.Client) (int, int, error) {
	var payload protobuf.CS_63313
	if err := proto.Unmarshal(*buffer, &payload); err != nil {
		return 0, 63314, err
	}

	response := protobuf.SC_63314{
		ShipId:    proto.Uint32(payload.GetShipId()),
		DoubleExp: proto.Uint32(0),
		Exp:       proto.Uint32(0),
		SkillId:   proto.Uint32(0),
		SwitchCnt: proto.Uint32(0),
		Tasks:     []*protobuf.FINISH_TASK{},
		SkillExp:  []*protobuf.SKILL_EXP{},
	}
	if err := ensureCommanderMetaLoaded(client.Commander); err != nil {
		return 0, 63314, err
	}
	ship, ok := client.Commander.OwnedShipsMap[payload.GetShipId()]
	if !ok {
		return client.SendMessage(63314, &response)
	}
	slots, _, err := metaSkillSlots(ship)
	if err != nil || len(slots) == 0 {
		return client.SendMessage(63314, &response)
	}

	state, skillStates, tasks, err := getMetaTacticsSnapshot(client.Commander.CommanderID, ship.ID)
	if err != nil {
		return 0, 63314, err
	}
	response.DoubleExp = proto.Uint32(state.DoubleExp)
	response.Exp = proto.Uint32(state.DailyExp)
	response.SkillId = proto.Uint32(state.CurrentSkillID)
	response.SwitchCnt = proto.Uint32(state.SwitchCnt)

	if len(skillStates) == 0 {
		response.SkillExp = []*protobuf.SKILL_EXP{}
	} else {
		response.SkillExp = buildMetaSkillExpPayload(skillStates)
	}
	if len(tasks) == 0 {
		response.Tasks = []*protobuf.FINISH_TASK{}
	} else {
		response.Tasks = make([]*protobuf.FINISH_TASK, 0, len(tasks))
		for _, task := range tasks {
			response.Tasks = append(response.Tasks, &protobuf.FINISH_TASK{
				SkillId:   proto.Uint32(task.SkillID),
				TaskId:    proto.Uint32(task.TaskID),
				FinishCnt: proto.Uint32(task.FinishCnt),
			})
		}
	}

	return client.SendMessage(63314, &response)
}
