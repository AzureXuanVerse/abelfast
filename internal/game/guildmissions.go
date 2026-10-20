package game

import (
	"database/sql"
	"errors"
	"math"
	"slices"
	"time"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/db"
	"github.com/ggmolly/belfast/internal/protobuf"
)

// handleGuildDispatch sends a fleet of the commander's ships to a mission of
// the open operation (GuildJoinMissionCommand, CS_61007): a member that
// joined the operation, the mission unlocked and running, up to
// GuildConst.MISSION_MAX_SHIP_CNT ships not already out on another of the
// operation's missions, one more fleet (up to MISSION_MAX_FLEET_CNT) per
// dispatch window (guildset operation_member_dispatch_reset hours,
// GuildMission:CanFormation). The first fleet starts the mission; each
// raises its efficiency, so its speed (guildMissionSpeed). On result 0 the
// client adds the fleet at the next page and the dispatch time itself.
// Built from the Lua alone.
func (game *Game) handleGuildDispatch(client *connection.Client, req *protobuf.CS_61007) error {
	err := game.guildOperationRequest(client, func(tx *Tx, member db.GuildMember, state *guildOperationState, now time.Time) error {
		ctx := tx.ctx
		mission := state.mission(req.GetEventTid())
		if mission == nil || mission.FinishedAt != 0 || !state.unlocked(mission) {
			return errRefused
		}
		if _, err := tx.GetGuildOperationMember(ctx, db.GetGuildOperationMemberParams{OperationID: state.op.ID, CommanderID: client.CommanderID}); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return errRefused
			}
			return err
		}
		ids := req.GetShipIds()
		if len(ids) == 0 || len(ids) > guildMissionMaxShips || len(slices.Compact(slices.Sorted(slices.Values(ids)))) != len(ids) {
			return errRefused
		}
		fleets, last := map[uint32]bool{}, uint32(0)
		for _, ship := range state.ships {
			if ship.CommanderID != client.CommanderID {
				continue
			}
			if ship.EventID == mission.EventID {
				fleets[ship.Fleet] = true
				last = max(last, ship.DispatchedAt)
			}
			if busy := state.mission(ship.EventID); busy != nil && busy.FinishedAt == 0 && slices.Contains(ids, ship.ShipID) {
				return errRefused
			}
		}
		if len(fleets) >= guildMissionMaxFleets || last != 0 && !guildDispatchWindow(now).After(time.Unix(int64(last), 0)) {
			return errRefused
		}
		owned, err := shipsByID(ctx, tx.Queries, client.CommanderID, ids)
		if err != nil {
			return err
		}
		fleet := uint32(len(fleets) + 1)
		at := uint32(now.Unix())
		rows := make([]db.GuildOperationShip, 0, len(ids))
		for i, id := range ids {
			ship, ok := owned[id]
			if !ok {
				return errRefused
			}
			rows = append(rows, db.GuildOperationShip{
				OperationID: state.op.ID, EventID: mission.EventID, CommanderID: client.CommanderID,
				Fleet: fleet, Slot: uint32(i + 1), ShipID: ship.ID, TemplateID: ship.TemplateID,
				SkinID: ship.SkinID, Level: ship.Level, DispatchedAt: at,
			})
		}
		rows[0].Efficiency = guildFleetEfficiency(rows)
		if mission.StartTime == 0 {
			mission.StartTime, mission.UpdatedAt = at, at
		}
		mission.Efficiency += rows[0].Efficiency
		if err := saveGuildMission(ctx, tx.Queries, *mission); err != nil {
			return err
		}
		for _, row := range rows {
			if err := tx.AddGuildMissionShip(ctx, db.AddGuildMissionShipParams{
				OperationID: row.OperationID, EventID: row.EventID, CommanderID: row.CommanderID, Fleet: row.Fleet,
				Slot: row.Slot, ShipID: row.ShipID, TemplateID: row.TemplateID, SkinID: row.SkinID,
				Level: row.Level, Efficiency: row.Efficiency, DispatchedAt: row.DispatchedAt,
			}); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil && !refused(err) {
		return err
	}
	return client.Send(&protobuf.SC_61008{Result: result(err)})
}

// guildFleetEfficiency is a dispatched fleet's efficiency as the client
// shows it (GuildMission.CalcMyEffect): (20 + total level^0.7) x
// (1 + power / (power + 12500)), the power being each ship's combat power
// (Ship:getShipCombatPower) from its level's attributes, without gear.
func guildFleetEfficiency(ships []db.GuildOperationShip) uint32 {
	levels, power := 0.0, 0.0
	for _, ship := range ships {
		levels += float64(ship.Level)
		power += shipCombatPower(ship)
	}
	return uint32(math.Floor((20 + math.Pow(levels, 0.7)) * (1 + power/(power+12500))))
}

// shipCombatPower is a ship's combat power (Ship:getShipCombatPower) from
// its level's attributes, without gear.
func shipCombatPower(ship db.GuildOperationShip) float64 {
	attr := func(id uint32) float64 { return guildShipAttr(ship, id) }
	// attribute_info_by_type: durability 1, cannon 2, torpedo 3,
	// antiaircraft 4, air 5, reload 6, hit 8, dodge 9, speed 10,
	// antisub 12
	return math.Floor(attr(1)/5 + attr(2) + attr(3) + attr(4) + attr(5) + attr(12) + attr(6) + attr(8)*2 + attr(9)*2 + attr(10))
}
