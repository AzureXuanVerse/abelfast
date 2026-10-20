package game

import (
	"context"
	"slices"
	"time"

	"google.golang.org/protobuf/proto"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/protobuf"
)

// The speed-up tickets (IslandTicketAgency, island_speedup_ticket, kept by
// key {speed_id, end_time} in PB_ISLAND_PRIVATE.speed_tickets): each takes
// its speedup_time off something running (IslandUseTicketCommand).

// IslandUseTicketCommand.TYPES
const (
	islandTicketOrder      = 1 // a common order on its way
	islandTicketShipOrder  = 2 // a transport job sailing
	islandTicketManage     = 3 // a restaurant open
	islandTicketAppoint    = 4 // a production commission (CS_21427)
	islandTicketShipReload = 5 // the next draw of transport offers
)

// spendTickets takes the tickets used from the island: the seconds they
// take off, or ErrNotEnough (nothing taken) when it lacks any or one has
// expired (the client has them removed first, CS_21425).
func (state *islandState) spendTickets(tickets []*protobuf.PB_SPEEDUP_TICKET, now time.Time) (uint32, error) {
	private := state.private()
	seconds := uint32(0)
	needed := map[[2]uint32]uint32{}
	for _, ticket := range tickets {
		key := [2]uint32{ticket.GetKey().GetSpeedId(), ticket.GetKey().GetEndTime()}
		if key[1] != 0 && key[1] <= uint32(now.Unix()) || ticket.GetNum() == 0 {
			return 0, ErrNotEnough
		}
		needed[key] += ticket.GetNum()
		seconds += islandOrderTicketTime[key[0]].SpeedupTime * ticket.GetNum()
	}
	if len(needed) == 0 {
		return 0, ErrNotEnough
	}
	for key, count := range needed {
		index := slices.IndexFunc(private.SpeedTickets, func(owned *protobuf.PB_SPEEDUP_TICKET) bool {
			return owned.GetKey().GetSpeedId() == key[0] && owned.GetKey().GetEndTime() == key[1]
		})
		if index < 0 || private.SpeedTickets[index].GetNum() < count {
			return 0, ErrNotEnough
		}
	}
	for key, count := range needed {
		for _, owned := range private.SpeedTickets {
			if owned.GetKey().GetSpeedId() == key[0] && owned.GetKey().GetEndTime() == key[1] {
				owned.Num = proto.Uint32(owned.GetNum() - count)
			}
		}
	}
	private.SpeedTickets = slices.DeleteFunc(private.SpeedTickets, func(owned *protobuf.PB_SPEEDUP_TICKET) bool { return owned.GetNum() == 0 })
	return seconds, nil
}

// handleIslandTicketUse answers IslandUseTicketCommand (CS_21423) on a
// common order on its way (type 1, the order slot), a transport job sailing
// (2, the transport slot), a restaurant open (3, the restaurant) or the
// next draw of transport offers (5): the tickets go and their time comes
// off it, as the client takes it off (the order's or job's speed_list entry,
// the restaurant's speed_time, ship_refresh). Anything not running is
// refused.
func (game *Game) handleIslandTicketUse(client *connection.Client, req *protobuf.CS_21423) error {
	reply := &protobuf.SC_21424{Result: result(errRefused)}
	err := game.tx(context.Background(), client, func(tx *Tx) error {
		now := serverNow()
		unix := uint32(now.Unix())
		state, err := tx.ownIslandOrders(now)
		if err != nil {
			return err
		}
		orders := state.orders()
		var apply func(seconds uint32)
		switch req.GetType() {
		case islandTicketOrder:
			slot := state.orderSlot(req.GetTargetId())
			if slot == nil || slot.GetType() != islandOrderNormal || state.orderVisible(slot, now) {
				return nil
			}
			apply = func(seconds uint32) { state.addSpeedUp(slot.GetId(), seconds) }
		case islandTicketShipOrder:
			slot := state.shipSlot(req.GetTargetId())
			if slot == nil || slot.GetState() != islandShipSlotSubmited || slot.GetGetTime() <= unix+state.speedUp(slot.GetId()) {
				return nil
			}
			apply = func(seconds uint32) { state.addSpeedUp(slot.GetId(), seconds) }
		case islandTicketManage:
			trade := state.restaurant(req.GetTargetId())
			if trade == nil || trade.GetEndTime() == 0 || trade.GetEndTime() <= unix+trade.GetSpeedTime() {
				return nil
			}
			apply = func(seconds uint32) { trade.SpeedTime = proto.Uint32(trade.GetSpeedTime() + seconds) }
		case islandTicketShipReload:
			if orders.GetShipRefresh() <= unix {
				return nil
			}
			apply = func(seconds uint32) {
				orders.ShipRefresh = proto.Uint32(orders.GetShipRefresh() - min(seconds, orders.GetShipRefresh()))
			}
		default:
			return nil
		}
		seconds, err := state.spendTickets(req.GetTickets(), now)
		if err != nil {
			reply.Result = result(err)
			return nil
		}
		apply(seconds)
		reply.Result = proto.Uint32(0)
		return nil
	})
	if err != nil {
		return err
	}
	return client.Send(reply)
}

// handleIslandTicketUseAppoint answers IslandUseTicketCommand on a
// production commission (CS_21427, by its production slot, area_id): the
// tickets' time comes off the rounds still to run (cost_time_list, the
// running one first, never below what already ran), counted in its
// speed_time; time_list is the rounds as they now are (the client sets
// them, IslandRoleDelegationData.SetCostList), and its character rests from
// the new end (UpdateEnergyBeginRecoverTime, recover_time).
func (game *Game) handleIslandTicketUseAppoint(client *connection.Client, req *protobuf.CS_21427) error {
	reply := &protobuf.SC_21428{Result: result(errRefused)}
	err := game.tx(context.Background(), client, func(tx *Tx) error {
		now := serverNow()
		unix := uint32(now.Unix())
		state, err := tx.ownIslandOrders(now)
		if err != nil {
			return err
		}
		var appoint *protobuf.PB_ISLAND_SHIP_APPOINT
		for _, build := range state.public().GetBuildList() {
			for _, entry := range build.GetShipAppointList() {
				if entry.GetId() == req.GetAreaId() {
					appoint = entry
				}
			}
		}
		if appoint == nil {
			return nil
		}
		end := appoint.GetStartTime()
		for _, round := range appoint.GetCostTimeList() {
			end += round
		}
		if end <= unix {
			return nil
		}
		seconds, err := state.spendTickets(req.GetTickets(), now)
		if err != nil {
			reply.Result = result(err)
			return nil
		}
		left := seconds
		start := appoint.GetStartTime()
		for i, round := range appoint.CostTimeList {
			ran := uint32(0)
			if unix > start {
				ran = min(unix-start, round)
			}
			cut := min(left, round-ran)
			appoint.CostTimeList[i] = round - cut
			left -= cut
			start += round
		}
		appoint.SpeedTime = proto.Uint32(appoint.GetSpeedTime() + seconds)
		end = appoint.GetStartTime()
		for _, round := range appoint.CostTimeList {
			end += round
		}
		for _, ship := range state.public().GetShipSys().GetShipList() {
			if ship.GetId() == appoint.GetShipId() {
				ship.RecoverTime = proto.Uint32(end)
			}
		}
		reply.Result = proto.Uint32(0)
		reply.TimeList = appoint.CostTimeList
		return nil
	})
	if err != nil {
		return err
	}
	return client.Send(reply)
}

// handleIslandTicketRemoveExpired answers IslandRemoveExpiredTicketCommand
// (CS_21425): the expired tickets of those keys are thrown away
// (IslandTicketAgency.RemoveTicket); one still valid is kept.
func (game *Game) handleIslandTicketRemoveExpired(client *connection.Client, req *protobuf.CS_21425) error {
	err := game.tx(context.Background(), client, func(tx *Tx) error {
		state, err := tx.island()
		if err != nil {
			return err
		}
		unix := uint32(serverNow().Unix())
		private := state.private()
		private.SpeedTickets = slices.DeleteFunc(private.SpeedTickets, func(owned *protobuf.PB_SPEEDUP_TICKET) bool {
			end := owned.GetKey().GetEndTime()
			return end != 0 && end <= unix && slices.ContainsFunc(req.GetTicketKeys(), func(key *protobuf.PB_SPEEDUP_KEY) bool {
				return key.GetSpeedId() == owned.GetKey().GetSpeedId() && key.GetEndTime() == end
			})
		})
		return nil
	})
	if err != nil {
		return err
	}
	return client.Send(&protobuf.SC_21426{Result: proto.Uint32(0)})
}
