package answer

import (
	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/orm"

	"github.com/ggmolly/belfast/internal/protobuf"
	"google.golang.org/protobuf/proto"
)

func MetaCharacterTacticsInfoRequestCommandResponse(buffer *[]byte, client *connection.Client) (int, int, error) {
	var payload protobuf.CS_63317
	if err := proto.Unmarshal(*buffer, &payload); err != nil {
		return 0, 63318, err
	}
	if err := ensureCommanderMetaLoaded(client.Commander); err != nil {
		return 0, 63318, err
	}

	response := protobuf.SC_63318{InfoList: []*protobuf.META_SKILL_SIMPLE_INFO{}}
	for _, requestedShipID := range payload.GetShipIdList() {
		ship := client.Commander.OwnedShipsMap[requestedShipID]
		if ship == nil {
			continue
		}
		slots, _, err := metaSkillSlots(ship)
		if err != nil || len(slots) == 0 {
			continue
		}
		state, skillStates, _, err := getMetaTacticsSnapshot(client.Commander.CommanderID, ship.ID)
		if err != nil {
			return 0, 63318, err
		}
		if state.SwitchCnt == 0 {
			state.SwitchCnt = orm.DefaultMetaTacticsSwitchCount
		}
		response.InfoList = append(response.InfoList, &protobuf.META_SKILL_SIMPLE_INFO{
			ShipId:   proto.Uint32(ship.ID),
			Exp:      proto.Uint32(state.DailyExp),
			SkillId:  proto.Uint32(state.CurrentSkillID),
			SkillExp: buildMetaSkillExpPayload(skillStates),
		})
	}
	return client.SendMessage(63318, &response)
}
