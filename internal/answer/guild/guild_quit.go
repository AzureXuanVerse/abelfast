package guild

import (
	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/orm"
	"github.com/ggmolly/belfast/internal/protobuf"
	"google.golang.org/protobuf/proto"
)

func GuildQuit(buffer *[]byte, client *connection.Client) (int, int, error) {
	var payload protobuf.CS_60018
	if err := proto.Unmarshal(*buffer, &payload); err != nil {
		return 0, 60019, err
	}
	response := protobuf.SC_60019{Result: proto.Uint32(guildResultFailure)}
	guildID := payload.GetId()
	if guildID == 0 {
		return client.SendMessage(60019, &response)
	}
	if err := orm.GuildQuit(client.Commander.CommanderID, guildID); err != nil {
		return client.SendMessage(60019, &response)
	}
	response.Result = proto.Uint32(guildResultSuccess)
	return client.SendMessage(60019, &response)
}
