package answer

import (
	"time"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/orm"
	"github.com/ggmolly/belfast/internal/protobuf"
	"google.golang.org/protobuf/proto"
)

func GuildImpeach(buffer *[]byte, client *connection.Client) (int, int, error) {
	var payload protobuf.CS_60016
	if err := proto.Unmarshal(*buffer, &payload); err != nil {
		return 0, 60017, err
	}
	response := protobuf.SC_60017{Result: proto.Uint32(guildResultFailure)}
	targetCommanderID := payload.GetPlayerId()
	if targetCommanderID == 0 || targetCommanderID == client.Commander.CommanderID {
		return client.SendMessage(60017, &response)
	}
	if err := orm.GuildImpeach(client.Commander.CommanderID, targetCommanderID, time.Now().UTC()); err != nil {
		return client.SendMessage(60017, &response)
	}
	response.Result = proto.Uint32(guildResultSuccess)
	return client.SendMessage(60017, &response)
}
