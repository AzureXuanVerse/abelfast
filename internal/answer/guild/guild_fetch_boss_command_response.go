package guild

import (
	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/protobuf"
	"google.golang.org/protobuf/proto"
)

func GuildFetchBossCommandResponse(buffer *[]byte, client *connection.Client) (int, int, error) {
	var payload protobuf.CS_61015
	if err := proto.Unmarshal(*buffer, &payload); err != nil {
		return 0, 61016, err
	}
	result := guildEventResultFailure
	if payload.GetType() == 0 {
		result = guildEventResultSuccess
	}
	return client.SendMessage(61016, &protobuf.SC_61016{Result: proto.Uint32(result)})
}
