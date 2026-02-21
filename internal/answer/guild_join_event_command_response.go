package answer

import (
	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/orm"
	"github.com/ggmolly/belfast/internal/protobuf"
	"google.golang.org/protobuf/proto"
)

func GuildJoinEventCommandResponse(buffer *[]byte, client *connection.Client) (int, int, error) {
	var payload protobuf.CS_61031
	if err := proto.Unmarshal(*buffer, &payload); err != nil {
		return 0, 61032, err
	}
	response := &protobuf.SC_61032{Result: proto.Uint32(guildEventResultFailure)}
	if client.Commander == nil || payload.GetType() != 0 {
		return client.SendMessage(61032, response)
	}
	maxJoin, err := orm.GetGuildSetUint("efficiency_param_times")
	if err != nil {
		return 0, 61032, err
	}
	livenessGain, err := orm.GetGuildSetUint("operation_event_guild_active")
	if err != nil {
		return 0, 61032, err
	}
	if err := orm.UpdateGuildOperationParticipation(client.Commander.CommanderID, nowUnix(), maxJoin, livenessGain); err != nil {
		return client.SendMessage(61032, response)
	}
	response.Result = proto.Uint32(guildEventResultSuccess)
	return client.SendMessage(61032, response)
}
