package answer

import (
	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/orm"

	"github.com/ggmolly/belfast/internal/protobuf"
	"google.golang.org/protobuf/proto"
)

func GuildGetAssaultFleetCommandResponse(buffer *[]byte, client *connection.Client) (int, int, error) {
	response := protobuf.SC_61012{Result: proto.Uint32(guildEventResultFailure)}
	if client.Commander == nil {
		return client.SendMessage(61012, &response)
	}

	ctx, err := activeGuildEventContext(client.Commander.CommanderID)
	if err != nil {
		if isGuildEventContextError(err) {
			return client.SendMessage(61012, &response)
		}
		return 0, 61012, err
	}

	members, err := orm.ListGuildMembers(ctx.GuildID)
	if err != nil {
		return 0, 61012, err
	}
	allSlots, err := orm.ListGuildAssaultFleetSlotsByGuild(ctx.GuildID)
	if err != nil {
		return 0, 61012, err
	}
	slotsByCommander := make(map[uint32][]orm.GuildAssaultFleetSlot)
	for _, slot := range allSlots {
		slotsByCommander[slot.CommanderID] = append(slotsByCommander[slot.CommanderID], slot)
	}

	ships := make([]*protobuf.TEAM_CHUNK, 0, len(members))
	for _, member := range members {
		memberSlots := slotsByCommander[member.CommanderID]
		memberShips := make([]*protobuf.SHIPINFO, 0, len(memberSlots))
		for _, slot := range memberSlots {
			ownedShip, err := orm.GetOwnedShipByOwnerAndID(member.CommanderID, slot.ShipID)
			if err != nil {
				continue
			}
			memberShips = append(memberShips, orm.ToProtoOwnedShip(*ownedShip, nil, nil))
		}
		ships = append(ships, &protobuf.TEAM_CHUNK{UserId: proto.Uint32(member.CommanderID), Ships: memberShips})
	}

	recommendations, err := orm.ListGuildAssaultRecommendations(ctx.GuildID)
	if err != nil {
		return 0, 61012, err
	}
	recommends := make([]*protobuf.TEAM_CELL, 0, len(recommendations))
	for _, recommendation := range recommendations {
		recommends = append(recommends, &protobuf.TEAM_CELL{
			UserId: proto.Uint32(recommendation.CommanderID),
			ShipId: proto.Uint32(recommendation.ShipID),
		})
	}

	response.Result = proto.Uint32(guildEventResultSuccess)
	response.Ships = ships
	response.Recommends = recommends
	return client.SendMessage(61012, &response)
}
