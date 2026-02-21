package answer

import (
	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/orm"
	"github.com/ggmolly/belfast/internal/protobuf"
	"google.golang.org/protobuf/proto"
)

func SetGuildDuty(buffer *[]byte, client *connection.Client) (int, int, error) {
	var payload protobuf.CS_60012
	if err := proto.Unmarshal(*buffer, &payload); err != nil {
		return 0, 60013, err
	}
	response := protobuf.SC_60013{Result: proto.Uint32(guildResultFailure)}
	targetCommanderID := payload.GetPlayerId()
	dutyID := payload.GetDutyId()
	if targetCommanderID == 0 || !orm.IsValidGuildDuty(dutyID) {
		return client.SendMessage(60013, &response)
	}
	if err := orm.UpdateGuildDuty(client.Commander.CommanderID, targetCommanderID, dutyID); err != nil {
		return client.SendMessage(60013, &response)
	}
	response.Result = proto.Uint32(guildResultSuccess)
	return client.SendMessage(60013, &response)
}
