package answer

import (
	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/orm"

	"github.com/ggmolly/belfast/internal/protobuf"
	"google.golang.org/protobuf/proto"
)

func GetMyAssaultFleetCommandResponse(buffer *[]byte, client *connection.Client) (int, int, error) {
	response := protobuf.SC_61010{Result: proto.Uint32(guildEventResultFailure)}
	if client.Commander == nil {
		return client.SendMessage(61010, &response)
	}

	ctx, err := activeGuildEventContext(client.Commander.CommanderID)
	if err != nil {
		if isGuildEventContextError(err) {
			return client.SendMessage(61010, &response)
		}
		return 0, 61010, err
	}

	slots, err := orm.ListGuildAssaultFleetSlotsByCommander(ctx.GuildID, client.Commander.CommanderID)
	if err != nil {
		return 0, 61010, err
	}

	personShips := make([]*protobuf.SHIPID_POS_INFO, 0, len(slots))
	for _, slot := range slots {
		ownedShip, err := orm.GetOwnedShipByOwnerAndID(client.Commander.CommanderID, slot.ShipID)
		if err != nil {
			continue
		}
		personShips = append(personShips, &protobuf.SHIPID_POS_INFO{
			Pos:      proto.Uint32(slot.Pos),
			Ship:     orm.ToProtoOwnedShip(*ownedShip, nil, nil),
			LastTime: proto.Uint32(slot.LastTime),
		})
	}

	response.Result = proto.Uint32(guildEventResultSuccess)
	response.PersonShips = personShips
	return client.SendMessage(61010, &response)
}
