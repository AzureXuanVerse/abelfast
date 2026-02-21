package guild

import (
	"errors"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/db"
	"github.com/ggmolly/belfast/internal/orm"
	"github.com/ggmolly/belfast/internal/protobuf"
	"google.golang.org/protobuf/proto"
)

func GuildUpdateNodeAnimFlagCommandResponse(buffer *[]byte, client *connection.Client) (int, int, error) {
	var payload protobuf.CS_61025
	if err := proto.Unmarshal(*buffer, &payload); err != nil {
		return 0, 61026, err
	}
	response := &protobuf.SC_61026{Result: proto.Uint32(guildEventResultFailure)}
	if client.Commander == nil || len(payload.GetPerf()) == 0 {
		return client.SendMessage(61026, response)
	}
	guild, _, err := orm.GetGuildForCommander(client.Commander.CommanderID)
	if err != nil {
		return client.SendMessage(61026, response)
	}
	updates := make([]orm.GuildOperationPerf, 0, len(payload.GetPerf()))
	seen := make(map[uint32]struct{}, len(payload.GetPerf()))
	for _, perf := range payload.GetPerf() {
		if perf == nil || perf.GetEventId() == 0 {
			return client.SendMessage(61026, response)
		}
		if _, ok := seen[perf.GetEventId()]; ok {
			return client.SendMessage(61026, response)
		}
		seen[perf.GetEventId()] = struct{}{}
		event, err := orm.GetGuildOperationEvent(guild.ID, perf.GetEventId())
		if err != nil {
			if errors.Is(err, db.ErrNotFound) {
				return client.SendMessage(61026, response)
			}
			return 0, 61026, err
		}
		if perf.GetIndex() < event.Position {
			return client.SendMessage(61026, response)
		}
		updates = append(updates, orm.GuildOperationPerf{EventTid: perf.GetEventId(), Index: perf.GetIndex()})
	}
	if err := orm.UpsertGuildOperationPerfsMonotonic(guild.ID, updates); err != nil {
		if errors.Is(err, orm.ErrGuildPermission) {
			return client.SendMessage(61026, response)
		}
		return 0, 61026, err
	}
	response.Result = proto.Uint32(guildEventResultSuccess)
	return client.SendMessage(61026, response)
}
