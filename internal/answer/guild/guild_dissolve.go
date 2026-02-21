package guild

import (
	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/orm"
	"github.com/ggmolly/belfast/internal/protobuf"
	"google.golang.org/protobuf/proto"
)

func GuildDissolve(buffer *[]byte, client *connection.Client) (int, int, error) {
	var payload protobuf.CS_60010
	if err := proto.Unmarshal(*buffer, &payload); err != nil {
		return 0, 60011, err
	}
	response := protobuf.SC_60011{Result: proto.Uint32(guildResultFailure)}
	guildID := payload.GetId()
	if guildID == 0 {
		return client.SendMessage(60011, &response)
	}
	if err := orm.GuildDissolve(client.Commander.CommanderID, guildID); err != nil {
		return client.SendMessage(60011, &response)
	}
	response.Result = proto.Uint32(guildResultSuccess)
	return client.SendMessage(60011, &response)
}
