package guild

import (
	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/orm"
	"github.com/ggmolly/belfast/internal/protobuf"
	"google.golang.org/protobuf/proto"
)

func GuildUpdateBossMissionFleetCommandResponse(buffer *[]byte, client *connection.Client) (int, int, error) {
	var payload protobuf.CS_61013
	if err := proto.Unmarshal(*buffer, &payload); err != nil {
		return 0, 61014, err
	}

	response := &protobuf.SC_61014{Result: proto.Uint32(guildEventResultFailure)}
	if client.Commander == nil {
		return client.SendMessage(61014, response)
	}

	ctx, err := activeGuildEventContext(client.Commander.CommanderID)
	if err != nil {
		if isGuildEventContextError(err) {
			return client.SendMessage(61014, response)
		}
		return 0, 61014, err
	}

	fleets := payload.GetFleet()
	if len(fleets) == 0 {
		return client.SendMessage(61014, response)
	}

	members, err := orm.ListGuildMembers(ctx.GuildID)
	if err != nil {
		return 0, 61014, err
	}
	memberSet := make(map[uint32]struct{}, len(members))
	for _, member := range members {
		memberSet[member.CommanderID] = struct{}{}
	}

	usedFleetIDs := make(map[uint32]struct{}, len(fleets))
	usedShips := make(map[[2]uint32]struct{})
	usedCommanders := make(map[uint32]struct{})
	normalized := make([]orm.GuildBossMissionFleet, 0, len(fleets))
	for _, fleet := range fleets {
		if fleet == nil {
			return client.SendMessage(61014, response)
		}
		fleetID := fleet.GetFleetId()
		if fleetID != 1 && fleetID != 11 {
			return client.SendMessage(61014, response)
		}
		if _, ok := usedFleetIDs[fleetID]; ok {
			return client.SendMessage(61014, response)
		}
		usedFleetIDs[fleetID] = struct{}{}

		ships := fleet.GetShips()
		if fleetID == 1 && len(ships) > 6 {
			return client.SendMessage(61014, response)
		}
		if fleetID == 11 && len(ships) > 3 {
			return client.SendMessage(61014, response)
		}

		normalizedShips := make([]orm.GuildBossMissionShip, 0, len(ships))
		requesterContributed := false
		for _, ship := range ships {
			if ship == nil {
				return client.SendMessage(61014, response)
			}
			userID := ship.GetUserId()
			shipID := ship.GetShipId()
			if userID == 0 || shipID == 0 {
				return client.SendMessage(61014, response)
			}
			if _, ok := memberSet[userID]; !ok {
				return client.SendMessage(61014, response)
			}
			pair := [2]uint32{userID, shipID}
			if _, ok := usedShips[pair]; ok {
				return client.SendMessage(61014, response)
			}
			usedShips[pair] = struct{}{}

			slots, err := orm.ListGuildAssaultFleetSlotsByCommander(ctx.GuildID, userID)
			if err != nil {
				return 0, 61014, err
			}
			ownsShip := false
			for _, slot := range slots {
				if slot.ShipID == shipID {
					ownsShip = true
					break
				}
			}
			if !ownsShip {
				return client.SendMessage(61014, response)
			}

			if userID == client.Commander.CommanderID {
				requesterContributed = true
			}
			normalizedShips = append(normalizedShips, orm.GuildBossMissionShip{UserID: userID, ShipID: shipID})
		}
		if fleetID == 1 && !requesterContributed {
			return client.SendMessage(61014, response)
		}

		normalizedCommanders := make([]orm.GuildBossMissionCommander, 0, len(fleet.GetCommanders()))
		for _, commander := range fleet.GetCommanders() {
			if commander == nil || commander.GetPos() == 0 || commander.GetId() == 0 {
				return client.SendMessage(61014, response)
			}
			if _, ok := usedCommanders[commander.GetId()]; ok {
				return client.SendMessage(61014, response)
			}
			usedCommanders[commander.GetId()] = struct{}{}
			normalizedCommanders = append(normalizedCommanders, orm.GuildBossMissionCommander{Pos: commander.GetPos(), ID: commander.GetId()})
		}

		normalized = append(normalized, orm.GuildBossMissionFleet{
			GuildID:     ctx.GuildID,
			OperationID: ctx.OperationID,
			FleetID:     fleetID,
			Ships:       normalizedShips,
			Commanders:  normalizedCommanders,
		})
	}

	if err := orm.UpsertGuildBossMissionFleets(ctx.GuildID, ctx.OperationID, normalized); err != nil {
		return 0, 61014, err
	}

	response.Result = proto.Uint32(guildEventResultSuccess)
	return client.SendMessage(61014, response)
}
