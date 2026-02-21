package guild

import (
	"errors"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/db"
	"github.com/ggmolly/belfast/internal/orm"
	"github.com/ggmolly/belfast/internal/protobuf"
	"google.golang.org/protobuf/proto"
)

func GuildRefreshMissionCommandResponse(buffer *[]byte, client *connection.Client) (int, int, error) {
	var payload protobuf.CS_61023
	if err := proto.Unmarshal(*buffer, &payload); err != nil {
		return 0, 61024, err
	}
	response := &protobuf.SC_61024{Result: proto.Uint32(guildEventResultFailure)}
	if client.Commander == nil || payload.GetEventTid() == 0 {
		return client.SendMessage(61024, response)
	}
	state, err := orm.GetGuildOperationStateForCommander(client.Commander.CommanderID)
	if err != nil {
		if errors.Is(err, db.ErrNotFound) {
			return client.SendMessage(61024, response)
		}
		return 0, 61024, err
	}
	if state.EndTime <= nowUnix() {
		return client.SendMessage(61024, response)
	}
	for _, event := range state.Events {
		if event.EventTid != payload.GetEventTid() {
			continue
		}
		if err := orm.UpdateGuildOperationEventRefresh(state.GuildID, event.EventTid, nowUnix()); err != nil {
			return 0, 61024, err
		}
		response.Result = proto.Uint32(guildEventResultSuccess)
		if event.Completed {
			response.CompletedInfo = &protobuf.EVENT_BASE_COMPLETED{EventId: proto.Uint32(event.EventTid), Position: proto.Uint32(event.Position)}
			return client.SendMessage(61024, response)
		}
		response.EventInfo = buildEventBase(event)
		return client.SendMessage(61024, response)
	}
	return client.SendMessage(61024, response)
}
