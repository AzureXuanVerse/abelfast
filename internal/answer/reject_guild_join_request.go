package answer

import (
	"errors"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/db"
	"github.com/ggmolly/belfast/internal/orm"
	"github.com/ggmolly/belfast/internal/protobuf"
	"google.golang.org/protobuf/proto"
)

func RejectGuildJoinRequest(buffer *[]byte, client *connection.Client) (int, int, error) {
	request := &protobuf.CS_60022{}
	if err := proto.Unmarshal(*buffer, request); err != nil {
		return 0, 60023, err
	}

	response := &protobuf.SC_60023{Result: proto.Uint32(guildResultFailure)}
	if client.Commander == nil || request.GetPlayerId() == 0 {
		return client.SendMessage(60023, response)
	}

	membership, err := orm.GetCommanderGuildMembership(client.Commander.CommanderID)
	if errors.Is(err, db.ErrNotFound) {
		return client.SendMessage(60023, response)
	}
	if err != nil {
		return 0, 60023, err
	}
	if membership.Duty != orm.GuildDutyCommander && membership.Duty != orm.GuildDutyDeputy {
		return client.SendMessage(60023, response)
	}

	if _, err := orm.DeleteGuildJoinRequest(membership.GuildID, request.GetPlayerId()); err != nil {
		return 0, 60023, err
	}

	response.Result = proto.Uint32(guildResultSuccess)
	return client.SendMessage(60023, response)
}
