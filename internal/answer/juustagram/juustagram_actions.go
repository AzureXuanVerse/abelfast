package juustagram

import (
	"time"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/consts"
	"github.com/ggmolly/belfast/internal/orm"
	"github.com/ggmolly/belfast/internal/protobuf"
	"google.golang.org/protobuf/proto"
)

func HandleJuustagramAction(buffer *[]byte, client *connection.Client) (int, int, error) {
	var payload protobuf.CS_11701
	if err := proto.Unmarshal(*buffer, &payload); err != nil {
		return 0, consts.JuustagramPacketOpResponse, err
	}
	if client.Commander == nil {
		response := protobuf.SC_11702{Result: proto.Uint32(1)}
		return client.SendMessage(consts.JuustagramPacketOpResponse, &response)
	}
	now := uint32(time.Now().Unix())
	state, err := orm.GetOrCreateJuustagramMessageState(client.Commander.CommanderID, payload.GetId(), now)
	if err != nil {
		response := protobuf.SC_11702{Result: proto.Uint32(1)}
		return client.SendMessage(consts.JuustagramPacketOpResponse, &response)
	}
	state.UpdatedAt = now
	switch payload.GetCmd() {
	case consts.JuustagramOpActive, consts.JuustagramOpUpdate, consts.JuustagramOpShare:
		// no state changes
	case consts.JuustagramOpLike:
		if state.IsGood == 0 {
			state.IsGood = 1
			state.GoodCount += 1
		}
	case consts.JuustagramOpMarkRead:
		state.IsRead = 1
	default:
		response := protobuf.SC_11702{Result: proto.Uint32(1)}
		return client.SendMessage(consts.JuustagramPacketOpResponse, &response)
	}
	if err := orm.SaveJuustagramMessageState(state); err != nil {
		response := protobuf.SC_11702{Result: proto.Uint32(1)}
		return client.SendMessage(consts.JuustagramPacketOpResponse, &response)
	}
	message, err := BuildJuustagramMessage(client.Commander.CommanderID, payload.GetId(), now)
	if err != nil {
		response := protobuf.SC_11702{Result: proto.Uint32(1)}
		return client.SendMessage(consts.JuustagramPacketOpResponse, &response)
	}
	response := protobuf.SC_11702{
		Result: proto.Uint32(0),
		Data:   message,
	}
	return client.SendMessage(consts.JuustagramPacketOpResponse, &response)
}
