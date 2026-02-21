package guild

import (
	"errors"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/orm"
	"github.com/ggmolly/belfast/internal/protobuf"
	"google.golang.org/protobuf/proto"
)

func GuildUpdateAssaultFleetCommandResponse(buffer *[]byte, client *connection.Client) (int, int, error) {
	var payload protobuf.CS_61003
	if err := proto.Unmarshal(*buffer, &payload); err != nil {
		return 0, 61004, err
	}

	response := &protobuf.SC_61004{Result: proto.Uint32(guildEventResultFailure)}
	if client.Commander == nil || client.Commander.OwnedShipsMap == nil {
		return client.SendMessage(61004, response)
	}

	ctx, err := activeGuildEventContext(client.Commander.CommanderID)
	if err != nil {
		if isGuildEventContextError(err) {
			return client.SendMessage(61004, response)
		}
		return 0, 61004, err
	}

	bossMissionActive, err := orm.HasGuildBossMissionFleet(ctx.GuildID, ctx.OperationID)
	if err != nil {
		return 0, 61004, err
	}
	if bossMissionActive {
		return client.SendMessage(61004, response)
	}

	shipUpdates := payload.GetShipIds()
	if len(shipUpdates) == 0 || len(shipUpdates) > 2 {
		return client.SendMessage(61004, response)
	}

	usedPositions := make(map[uint32]struct{}, len(shipUpdates))
	usedShips := make(map[uint32]struct{}, len(shipUpdates))
	upserts := make([]orm.GuildAssaultFleetSlot, 0, len(shipUpdates))
	for _, entry := range shipUpdates {
		if entry == nil {
			return client.SendMessage(61004, response)
		}
		pos := entry.GetPos()
		shipID := entry.GetShipId()
		if pos < 1 || pos > 2 || shipID == 0 {
			return client.SendMessage(61004, response)
		}
		if _, ok := usedPositions[pos]; ok {
			return client.SendMessage(61004, response)
		}
		if _, ok := usedShips[shipID]; ok {
			return client.SendMessage(61004, response)
		}
		if _, ok := client.Commander.OwnedShipsMap[shipID]; !ok {
			return client.SendMessage(61004, response)
		}
		usedPositions[pos] = struct{}{}
		usedShips[shipID] = struct{}{}
		upserts = append(upserts, orm.GuildAssaultFleetSlot{
			GuildID:     ctx.GuildID,
			CommanderID: client.Commander.CommanderID,
			Pos:         pos,
			ShipID:      shipID,
		})
	}

	cooldownSeconds, err := orm.GetGuildSetUint("operation_assault_team_cd")
	if err != nil {
		return 0, 61004, err
	}

	slots, err := orm.ListGuildAssaultFleetSlotsByCommander(ctx.GuildID, client.Commander.CommanderID)
	if err != nil {
		return 0, 61004, err
	}
	slotByPos := make(map[uint32]orm.GuildAssaultFleetSlot, len(slots))
	for _, slot := range slots {
		slotByPos[slot.Pos] = slot
	}
	now := nowUnix()
	for _, update := range upserts {
		slot, ok := slotByPos[update.Pos]
		if !ok {
			continue
		}
		if now < slot.LastTime+cooldownSeconds {
			return client.SendMessage(61004, response)
		}
	}

	if err := orm.UpsertGuildAssaultFleetSlots(ctx.GuildID, client.Commander.CommanderID, upserts, now); err != nil {
		if errors.Is(err, orm.ErrGuildPermission) {
			return client.SendMessage(61004, response)
		}
		return 0, 61004, err
	}

	response.Result = proto.Uint32(guildEventResultSuccess)
	return client.SendMessage(61004, response)
}
