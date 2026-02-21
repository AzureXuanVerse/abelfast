package answer

import (
	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/orm"
	"github.com/ggmolly/belfast/internal/protobuf"
	"google.golang.org/protobuf/proto"
)

func GuildFire(buffer *[]byte, client *connection.Client) (int, int, error) {
	var payload protobuf.CS_60014
	if err := proto.Unmarshal(*buffer, &payload); err != nil {
		return 0, 60015, err
	}
	response := protobuf.SC_60015{Result: proto.Uint32(guildResultFailure)}
	targetCommanderID := payload.GetPlayerId()
	if targetCommanderID == 0 || targetCommanderID == client.Commander.CommanderID {
		return client.SendMessage(60015, &response)
	}
	if err := orm.FireGuildMember(client.Commander.CommanderID, targetCommanderID); err != nil {
		return client.SendMessage(60015, &response)
	}
	response.Result = proto.Uint32(guildResultSuccess)
	return client.SendMessage(60015, &response)
}
