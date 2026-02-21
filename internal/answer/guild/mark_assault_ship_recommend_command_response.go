package guild

import (
	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/orm"
	"github.com/ggmolly/belfast/internal/protobuf"
	"google.golang.org/protobuf/proto"
)

func MarkAssaultShipRecommendCommandResponse(buffer *[]byte, client *connection.Client) (int, int, error) {
	var payload protobuf.CS_61033
	if err := proto.Unmarshal(*buffer, &payload); err != nil {
		return 0, 61034, err
	}

	response := &protobuf.SC_61034{Result: proto.Uint32(guildEventResultFailure)}
	if client.Commander == nil {
		return client.SendMessage(61034, response)
	}

	ctx, err := activeGuildEventContext(client.Commander.CommanderID)
	if err != nil {
		if isGuildEventContextError(err) {
			return client.SendMessage(61034, response)
		}
		return 0, 61034, err
	}
	if !isGuildAdminDuty(ctx.Duty) {
		return client.SendMessage(61034, response)
	}

	targetCommanderID := payload.GetRecommendUid()
	targetShipID := payload.GetRecommendShipid()
	cmd := payload.GetCmd()
	if targetCommanderID == 0 || targetShipID == 0 {
		return client.SendMessage(61034, response)
	}
	if cmd != 0 && cmd != 1 {
		return client.SendMessage(61034, response)
	}

	if cmd == 0 {
		members, err := orm.ListGuildMembers(ctx.GuildID)
		if err != nil {
			return 0, 61034, err
		}
		targetMemberExists := false
		for _, member := range members {
			if member.CommanderID == targetCommanderID {
				targetMemberExists = true
				break
			}
		}
		if !targetMemberExists {
			return client.SendMessage(61034, response)
		}

		slots, err := orm.ListGuildAssaultFleetSlotsByCommander(ctx.GuildID, targetCommanderID)
		if err != nil {
			return 0, 61034, err
		}
		hasShip := false
		for _, slot := range slots {
			if slot.ShipID == targetShipID {
				hasShip = true
				break
			}
		}
		if !hasShip {
			return client.SendMessage(61034, response)
		}
	}

	if err := orm.SetGuildAssaultRecommendation(ctx.GuildID, targetCommanderID, targetShipID, cmd == 0); err != nil {
		if err == orm.ErrGuildPermission {
			return client.SendMessage(61034, response)
		}
		return 0, 61034, err
	}

	response.Result = proto.Uint32(guildEventResultSuccess)
	return client.SendMessage(61034, response)
}
