package answer

import (
	"errors"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/db"
	"github.com/ggmolly/belfast/internal/orm"
	"github.com/ggmolly/belfast/internal/protobuf"
	"google.golang.org/protobuf/proto"
)

func GuildGetBossInfoCommandResponse(buffer *[]byte, client *connection.Client) (int, int, error) {
	var payload protobuf.CS_61027
	if err := proto.Unmarshal(*buffer, &payload); err != nil {
		return 0, 61028, err
	}

	response := &protobuf.SC_61028{
		Result:    proto.Uint32(guildEventResultFailure),
		BossEvent: defaultGuildBossEvent(),
	}
	if client.Commander == nil {
		response.Result = proto.Uint32(guildEventResultNoActiveOperation)
		return client.SendMessage(61028, response)
	}
	if payload.GetType() != 0 {
		return client.SendMessage(61028, response)
	}

	state, err := orm.GetGuildOperationStateForCommander(client.Commander.CommanderID)
	if err != nil {
		if errors.Is(err, db.ErrNotFound) {
			response.Result = proto.Uint32(guildEventResultNoActiveOperation)
			return client.SendMessage(61028, response)
		}
		return 0, 61028, err
	}
	if state.EndTime <= nowUnix() {
		response.Result = proto.Uint32(guildEventResultNoActiveOperation)
		return client.SendMessage(61028, response)
	}

	bossState, err := orm.GetGuildOperationBossState(state.GuildID, state.ChapterID)
	if err != nil {
		if errors.Is(err, db.ErrNotFound) {
			response.Result = proto.Uint32(guildEventResultNoActiveOperation)
			return client.SendMessage(61028, response)
		}
		return 0, 61028, err
	}

	response.Result = proto.Uint32(guildEventResultSuccess)
	response.BossEvent = &protobuf.EVENT_BOSS{
		BossId: proto.Uint32(bossState.BossID),
		Damage: proto.Uint32(bossState.Damage),
		Hp:     proto.Uint32(bossState.HP),
	}
	return client.SendMessage(61028, response)
}

func defaultGuildBossEvent() *protobuf.EVENT_BOSS {
	return &protobuf.EVENT_BOSS{
		BossId: proto.Uint32(0),
		Damage: proto.Uint32(0),
		Hp:     proto.Uint32(0),
	}
}
