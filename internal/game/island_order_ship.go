package game

import (
	"context"
	"maps"
	"math/rand/v2"
	"slices"
	"time"

	"google.golang.org/protobuf/proto"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/protobuf"
	"github.com/ggmolly/belfast/internal/protocol"
)

// The transport jobs (IslandShipOrderSlot, IslandShipOrderOpCommand, the
// island_order_list type 3 slots): a job asks for a few loads of items, each
// paying Development Funds as it is loaded (IslandShipOrder.GetConsumeAwards,
// island_set order_ship_award_coefficient), then sails for load_time and
// brings back its reward. Each slot sails island_set island_shiporder_limit
// times between two refresh times (island_shiporder_refresh, seconds after
// midnight). The offers (PB_SHIP_ORDER_APPOINT, IslandShipOrderDelegateSlot,
// island_shiporder_list_quantity of them) can be taken in place of a slot's
// job, and drawn again every island_shiporder_refresh_cd.
//
// GUESSES: what a job or offer asks for and brings (orderCost: items worth
// the island_order_price Development Funds of the island's level, both
// ways), how long it sails (that worth in seconds times
// order_ship_time_coefficient, from half an hour to four hours), and that a
// new job is there at once.

// IslandShipOrderSlot.STATE_*
const (
	islandShipSlotWaiting  = 1
	islandShipSlotSubmited = 2
)

// IslandShipOrder: a load is on board when its state is 1
const islandShipLoaded = 1

// IslandShipOrder.OP_TYPE_*
const (
	islandShipOpUnlock   = 1
	islandShipOpGetAward = 2
)

// shipSlot is the island's unlocked transport slot id, nil if locked.
func (state *islandState) shipSlot(id uint32) *protobuf.PB_ISLAND_ORDER_SHIP_SLOT {
	for _, slot := range state.orders().GetShipSlotList() {
		if slot.GetId() == id {
			return slot
		}
	}
	return nil
}

// shipOrderValue is what a transport job is worth: the Development Funds of
// a common order at the island's level.
func (state *islandState) shipOrderValue() (value, level uint32) {
	level = state.orderLevel()
	_, funds := orderAward(level, islandTendencyCommon, false)
	return max(funds, 1), level
}

// newShipGoods draws a job's loads and reward.
func (state *islandState) newShipGoods() (cost, reward []*protobuf.PB_ISLAND_ITEM) {
	value, level := state.shipOrderValue()
	cost = state.orderCost(value, level)
	for range 3 {
		reward = state.orderCost(value, level)
		if !slices.ContainsFunc(reward, func(item *protobuf.PB_ISLAND_ITEM) bool {
			return slices.ContainsFunc(cost, func(other *protobuf.PB_ISLAND_ITEM) bool { return other.GetId() == item.GetId() })
		}) {
			break
		}
	}
	return cost, reward
}

// shipLoadTime is how long a job with these loads sails.
func shipLoadTime(cost []*protobuf.PB_ISLAND_ITEM) uint32 {
	value := uint32(0)
	for _, item := range cost {
		value += islandOrderItems[item.GetId()].OrderPrice * item.GetNum()
	}
	return min(max(value*max(islandSetInt("order_ship_time_coefficient"), 1), 1800), 4*3600)
}

// fillShipSlot gives a transport slot a new job to load, there at once.
func fillShipSlot(slot *protobuf.PB_ISLAND_ORDER_SHIP_SLOT, cost, reward []*protobuf.PB_ISLAND_ITEM) {
	slot.State = proto.Uint32(islandShipSlotWaiting)
	slot.GetTime = proto.Uint32(0)
	slot.Cost = nil
	for _, item := range cost {
		slot.Cost = append(slot.Cost, &protobuf.PB_ISLAND_ORDER_SHIP_LOAD{Id: proto.Uint32(item.GetId()), Num: proto.Uint32(item.GetNum()), State: proto.Uint32(0)})
	}
	slot.Reward = reward
	slot.LoadTime = proto.Uint32(shipLoadTime(cost))
}

// newAppoint is a new transport offer, shown at once.
func (state *islandState) newAppoint(now time.Time) (*protobuf.PB_SHIP_ORDER_APPOINT, error) {
	extra := islandOrderExtra{}
	if _, err := state.getExtra(islandOrderExtraKey, &extra); err != nil {
		return nil, err
	}
	extra.NextAppoint++
	if err := state.setExtra(islandOrderExtraKey, extra); err != nil {
		return nil, err
	}
	cost, reward := state.newShipGoods()
	return protocol.FillRequired(&protobuf.PB_SHIP_ORDER_APPOINT{
		Id: proto.Uint32(extra.NextAppoint), ViewTime: proto.Uint32(uint32(now.Unix())), Cost: cost, Reward: reward,
	}), nil
}

// drawAppoints replaces the transport offers by
// island_shiporder_list_quantity new ones.
func (state *islandState) drawAppoints(now time.Time) error {
	orders := state.orders()
	orders.AppointList = nil
	for range islandSetInt("island_shiporder_list_quantity") {
		appoint, err := state.newAppoint(now)
		if err != nil {
			return err
		}
		orders.AppointList = append(orders.AppointList, appoint)
	}
	return nil
}

// nextShipRefresh is the next island_shiporder_refresh time after now
// (IslandShipOrderSlot.GetNextRefreshFinishCntTime).
func nextShipRefresh(now time.Time) uint32 {
	var points []uint32
	if islandSetList("island_shiporder_refresh", &points) != nil || len(points) == 0 {
		return uint32(nextDay(now).Unix())
	}
	midnight := uint32(startOfDay(now).Unix())
	for _, point := range points {
		if midnight+point > uint32(now.Unix()) {
			return midnight + point
		}
	}
	return uint32(nextDay(now).Unix()) + points[0]
}

// countShipFinish counts a sailing (IslandShipOrderSlot.IncreaseFinishCnt).
func countShipFinish(slot *protobuf.PB_ISLAND_ORDER_SHIP_SLOT, now time.Time) {
	slot.FinishNum = proto.Uint32(min(slot.GetFinishNum()+1, islandSetInt("island_shiporder_limit")))
	slot.AutoTime = proto.Uint32(nextShipRefresh(now))
}

// shipCanSail tells whether a slot may load now (CanTransport).
func shipCanSail(slot *protobuf.PB_ISLAND_ORDER_SHIP_SLOT, now time.Time) bool {
	return slot.GetFinishNum() < islandSetInt("island_shiporder_limit") || slot.GetAutoTime() <= uint32(now.Unix())
}

// handleIslandShipOrderOp answers IslandShipOrderOpCommand's unlock and
// collect (CS_21408). Unlock (type 1): a locked slot the island has the
// TYPE_ORDER ability of, paying its island_order_list unlock_cost, gets a
// job (slot) and, the first time, the offers (appoint_list). Collect (type
// 2): a job back from sailing gives its reward (drop_list) and the slot a
// new job (slot).
func (game *Game) handleIslandShipOrderOp(client *connection.Client, req *protobuf.CS_21408) error {
	reply := &protobuf.SC_21409{Result: result(errRefused)}
	err := game.tx(context.Background(), client, func(tx *Tx) error {
		now := serverNow()
		state, err := tx.ownIslandOrders(now)
		if err != nil {
			return err
		}
		orders := state.orders()
		id := req.GetShipSlotId()
		config, ok := islandOrderSlots[id]
		if !ok || config.Type != islandOrderListShip {
			return nil
		}
		slot := state.shipSlot(id)
		switch req.GetType() {
		case islandShipOpUnlock:
			if slot != nil || !state.hasIslandAbility(islandAbilityOrderSlot, id) {
				return nil
			}
			if len(config.UnlockCost) > 0 && len(config.UnlockCost[0]) >= 2 {
				if err := state.takeIslandItem(config.UnlockCost[0][0], config.UnlockCost[0][1]); err != nil {
					reply.Result = result(err)
					return nil
				}
			}
			slot = &protobuf.PB_ISLAND_ORDER_SHIP_SLOT{Id: proto.Uint32(id), FinishNum: proto.Uint32(0), AutoTime: proto.Uint32(0)}
			cost, reward := state.newShipGoods()
			fillShipSlot(slot, cost, reward)
			orders.ShipSlotList = append(orders.ShipSlotList, slot)
			if len(orders.AppointList) == 0 {
				if err := state.drawAppoints(now); err != nil {
					return err
				}
			}
			reply.AppointList = orders.AppointList
		case islandShipOpGetAward:
			if slot == nil || slot.GetState() != islandShipSlotSubmited || slot.GetGetTime() > uint32(now.Unix())+state.speedUp(id) {
				return nil
			}
			for _, item := range slot.GetReward() {
				if item.GetNum() > 0 {
					reply.DropList = append(reply.DropList, &protobuf.DROPINFO{Type: proto.Uint32(dropTypeIslandItem), Id: proto.Uint32(item.GetId()), Number: proto.Uint32(item.GetNum())})
				}
			}
			if err := tx.grantDrops(reply.DropList); err != nil {
				return err
			}
			state.clearSpeedUp(id)
			cost, reward := state.newShipGoods()
			fillShipSlot(slot, cost, reward)
			if err := tx.islandTaskEvent(islandTargetShipOrder, 1); err != nil {
				return err
			}
			if err := tx.islandAchievementEvent(islandRecordFinishOrder, islandOrderListShip, 1); err != nil {
				return err
			}
		default:
			return nil
		}
		reply.Result = proto.Uint32(0)
		reply.Slot = slot
		return nil
	})
	if err != nil {
		return err
	}
	return client.Send(islandStamped(reply))
}

// handleIslandShipOrderLoad answers IslandShipOrderOpCommand's loading
// (CS_21416, one load or all it can): loads of a job (by item id) the
// island holds go on board, each paying its share (GetConsumeAwards:
// Development Funds in drop_list, island exp); once all are on board the
// job sails until get_time and counts for the slot's sailings.
func (game *Game) handleIslandShipOrderLoad(client *connection.Client, req *protobuf.CS_21416) error {
	reply := &protobuf.SC_21417{Result: result(errRefused)}
	err := game.tx(context.Background(), client, func(tx *Tx) error {
		now := serverNow()
		state, err := tx.ownIslandOrders(now)
		if err != nil {
			return err
		}
		slot := state.shipSlot(req.GetShipSlotId())
		if slot == nil || slot.GetState() != islandShipSlotWaiting || slot.GetGetTime() > uint32(now.Unix()) || !shipCanSail(slot, now) || len(req.GetItemId()) == 0 {
			return nil
		}
		var coefficient []uint32
		if err := islandSetList("order_ship_award_coefficient", &coefficient); err != nil || len(coefficient) < 3 {
			return err
		}
		var loads []*protobuf.PB_ISLAND_ORDER_SHIP_LOAD
		var cost []*protobuf.PB_ISLAND_ITEM
		for _, itemID := range req.GetItemId() {
			index := slices.IndexFunc(slot.Cost, func(load *protobuf.PB_ISLAND_ORDER_SHIP_LOAD) bool {
				return load.GetId() == itemID && load.GetState() != islandShipLoaded && !slices.Contains(loads, load)
			})
			if index < 0 {
				return nil
			}
			loads = append(loads, slot.Cost[index])
			cost = append(cost, &protobuf.PB_ISLAND_ITEM{Id: proto.Uint32(itemID), Num: proto.Uint32(slot.Cost[index].GetNum())})
		}
		if err := state.payOrder(cost); err != nil {
			reply.Result = result(err)
			return nil
		}
		funds, exp := map[uint32]uint32{}, uint32(0)
		for _, load := range loads {
			load.State = proto.Uint32(islandShipLoaded)
			value := islandOrderItems[load.GetId()].OrderPrice * load.GetNum()
			funds[coefficient[0]] += value * coefficient[1] / 100
			exp += value * coefficient[2] / 100
		}
		for _, id := range slices.Sorted(maps.Keys(funds)) {
			if funds[id] > 0 {
				reply.DropList = append(reply.DropList, &protobuf.DROPINFO{Type: proto.Uint32(dropTypeIslandItem), Id: proto.Uint32(id), Number: proto.Uint32(funds[id])})
			}
		}
		if err := tx.grantDrops(reply.DropList); err != nil {
			return err
		}
		state.addIslandExp(exp)
		if !slices.ContainsFunc(slot.Cost, func(load *protobuf.PB_ISLAND_ORDER_SHIP_LOAD) bool { return load.GetState() != islandShipLoaded }) {
			slot.State = proto.Uint32(islandShipSlotSubmited)
			slot.GetTime = proto.Uint32(uint32(now.Unix()) + slot.GetLoadTime())
			state.clearSpeedUp(slot.GetId())
			countShipFinish(slot, now)
		}
		reply.Result = proto.Uint32(0)
		reply.GetTime = proto.Uint32(slot.GetGetTime())
		return nil
	})
	if err != nil {
		return err
	}
	return client.Send(islandStamped(reply))
}

// handleIslandShipOrderReset answers IslandResetShipOrderCommand (CS_21429
// slot 0): once ship_refresh has passed, the offers are drawn again and the
// next draw is island_shiporder_refresh_cd away (next_time). A single slot
// (IslandRefreshShipOrderCommand, no view sends it, and its reply has no
// slot to read) is refused.
func (game *Game) handleIslandShipOrderReset(client *connection.Client, req *protobuf.CS_21429) error {
	reply := &protobuf.SC_21430{Result: result(errRefused)}
	err := game.tx(context.Background(), client, func(tx *Tx) error {
		now := serverNow()
		state, err := tx.ownIslandOrders(now)
		if err != nil {
			return err
		}
		orders := state.orders()
		if req.GetSlotId() != 0 || orders.GetShipRefresh() > uint32(now.Unix()) || len(orders.GetShipSlotList()) == 0 {
			return nil
		}
		if err := state.drawAppoints(now); err != nil {
			return err
		}
		orders.ShipRefresh = proto.Uint32(uint32(now.Unix()) + islandSetInt("island_shiporder_refresh_cd"))
		reply.Result = proto.Uint32(0)
		reply.NextTime = orders.ShipRefresh
		reply.AppointList = orders.AppointList
		return nil
	})
	if err != nil {
		return err
	}
	return client.Send(islandStamped(reply))
}

// handleIslandShipOrderExchange answers IslandExchangeShipOrderCommand
// (CS_21431): a waiting slot takes an offer's job in place of its own (a
// job partly loaded still counts as a sailing, as the client counts it),
// and a new offer takes its place (appoint).
func (game *Game) handleIslandShipOrderExchange(client *connection.Client, req *protobuf.CS_21431) error {
	reply := &protobuf.SC_21432{Result: result(errRefused)}
	err := game.tx(context.Background(), client, func(tx *Tx) error {
		now := serverNow()
		state, err := tx.ownIslandOrders(now)
		if err != nil {
			return err
		}
		orders := state.orders()
		slot := state.shipSlot(req.GetSlotId())
		index := slices.IndexFunc(orders.AppointList, func(appoint *protobuf.PB_SHIP_ORDER_APPOINT) bool { return appoint.GetId() == req.GetAppointId() })
		if slot == nil || slot.GetState() != islandShipSlotWaiting || index < 0 || orders.AppointList[index].GetViewTime() > uint32(now.Unix()) {
			return nil
		}
		appoint := orders.AppointList[index]
		if slices.ContainsFunc(slot.Cost, func(load *protobuf.PB_ISLAND_ORDER_SHIP_LOAD) bool { return load.GetState() == islandShipLoaded }) {
			countShipFinish(slot, now)
		}
		fillShipSlot(slot, appoint.GetCost(), appoint.GetReward())
		next, err := state.newAppoint(now)
		if err != nil {
			return err
		}
		orders.AppointList[index] = next
		reply.Result = proto.Uint32(0)
		reply.Appoint = next
		return nil
	})
	if err != nil {
		return err
	}
	return client.Send(islandStamped(reply))
}

// randomPick draws n distinct entries of ids (all of them when fewer).
func randomPick(ids []uint32, n int) []uint32 {
	ids = slices.Clone(ids)
	rand.Shuffle(len(ids), func(i, j int) { ids[i], ids[j] = ids[j], ids[i] })
	return ids[:min(n, len(ids))]
}
