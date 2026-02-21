package answer

import (
	"errors"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/orm"
	"github.com/ggmolly/belfast/internal/protobuf"
	"google.golang.org/protobuf/proto"
)

func GuildActiveEventCommandResponse(buffer *[]byte, client *connection.Client) (int, int, error) {
	var payload protobuf.CS_61001
	if err := proto.Unmarshal(*buffer, &payload); err != nil {
		return 0, 61002, err
	}

	response := &protobuf.SC_61002{Result: proto.Uint32(guildEventResultFailure)}
	if client.Commander == nil || payload.GetChapterId() == 0 {
		return client.SendMessage(61002, response)
	}

	guild, member, err := orm.GetGuildForCommander(client.Commander.CommanderID)
	if err != nil {
		return client.SendMessage(61002, response)
	}
	if member.Duty != orm.GuildDutyCommander && member.Duty != orm.GuildDutyDeputy {
		return client.SendMessage(61002, response)
	}

	chapter, err := loadGuildOperationTemplate(payload.GetChapterId())
	if err != nil {
		return client.SendMessage(61002, response)
	}
	if guild.Level < chapter.UnlockGuildLevel {
		return client.SendMessage(61002, response)
	}

	durationSeconds, err := orm.GetGuildSetUint("operation_duration_time")
	if err != nil {
		return 0, 61002, err
	}

	err = orm.ActivateGuildOperation(client.Commander.CommanderID, payload.GetChapterId(), chapter.Consume, durationSeconds, nowUnix())
	if err != nil {
		switch {
		case errors.Is(err, orm.ErrGuildPermission):
			response.Result = proto.Uint32(guildEventResultFailure)
		case errors.Is(err, orm.ErrGuildInsufficientCap):
			response.Result = proto.Uint32(guildEventResultInsufficientCapital)
		default:
			response.Result = proto.Uint32(guildEventResultInternal)
		}
		return client.SendMessage(61002, response)
	}

	response.Result = proto.Uint32(guildEventResultSuccess)
	return client.SendMessage(61002, response)
}
