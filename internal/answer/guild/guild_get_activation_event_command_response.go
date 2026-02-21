package guild

import (
	"errors"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/db"
	"github.com/ggmolly/belfast/internal/orm"

	"github.com/ggmolly/belfast/internal/protobuf"
	"google.golang.org/protobuf/proto"
)

func GuildGetActivationEventCommandResponse(buffer *[]byte, client *connection.Client) (int, int, error) {
	var payload protobuf.CS_61005
	if err := proto.Unmarshal(*buffer, &payload); err != nil {
		return 0, 61006, err
	}
	if client.Commander == nil {
		return client.SendMessage(61006, &protobuf.SC_61006{Result: proto.Uint32(guildEventResultNoActiveOperation)})
	}

	state, err := orm.GetGuildOperationStateForCommander(client.Commander.CommanderID)
	if err != nil {
		if errors.Is(err, db.ErrNotFound) {
			return client.SendMessage(61006, &protobuf.SC_61006{Result: proto.Uint32(guildEventResultNoActiveOperation)})
		}
		return 0, 61006, err
	}
	if state.EndTime <= nowUnix() {
		return client.SendMessage(61006, &protobuf.SC_61006{Result: proto.Uint32(guildEventResultNoActiveOperation)})
	}
	response := &protobuf.SC_61006{
		Result:    proto.Uint32(guildEventResultSuccess),
		Operation: buildOperationResponse(state),
	}
	return client.SendMessage(61006, response)
}
