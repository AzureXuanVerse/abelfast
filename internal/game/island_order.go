package game

import (
	"context"
	"encoding/json"
	"maps"
	"math/rand/v2"
	"slices"
	"time"

	"google.golang.org/protobuf/proto"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/gamedata"
	"github.com/ggmolly/belfast/internal/packets"
	"github.com/ggmolly/belfast/internal/protobuf"
	"github.com/ggmolly/belfast/internal/protocol"
)

// The island's orders (IslandOrderAgency, "Island Requests"), transport jobs
// (ship orders, IslandShipOrderSlot), restaurants (IslandManageAgecny) and
// speed-up tickets (IslandTicketAgency): see island_core.go. All of it lives
// in the island's PB_ISLAND_PRIVATE.order_system and PB_ISLAND_PUBLIC
// trade_sys as the client reads them in SC_21201, plus islandOrderExtra.
//
// The orders themselves are the server's: which items an order asks for,
// when the next one comes, when an urgent one appears. Neither the Lua nor a
// capture says how, and the wiki only says that "Island Requests allow you
// to submit randomly selected resources to earn Development Coins", that
// "the number of normal Requests that can be completed is limited each day,
// and can also be rerolled with a short delay" and that "completing Requests
// can also occasionally spawn Urgent requests, which are available only for
// a limited time and cannot be rerolled" (Island Planner). Every rule below
// marked GUESS is belfast's own.
//
// The client computes an order's rewards itself (IslandOrder
// GetAwardItemAndExp: island_order_price by the order's level and
// difficulty, island_order for the fixed ones) and takes the items it asks
// for (GAME.CONSUME_ITEM), so the server applies exactly the same.

// IslandOrderAgency.*_ORDER_TYPE: island_order_list types
const (
	islandOrderListCommon = 1
	islandOrderListUrgent = 2
	islandOrderListShip   = 3
)

// IslandOrder.TYPE_*: PB_ISLAND_ORDER_SLOT type
const (
	islandOrderNormal = 1
	islandOrderUrgent = 2
	islandOrderFirm   = 4
)

// IslandFirmOrder.FIRM_ORDER_TYPE_*: island_order types
const (
	islandFirmCommon   = 1
	islandFirmUrgent   = 2
	islandFirmActivity = 3
)

// IslandOrderSlot.TENDENCY_TYPE_*
const (
	islandTendencyCommon = 0
	islandTendencyEasy   = 1
	islandTendencyHard   = 2
)

// IslandOrderSlot.SHOW_FLAG_TOMORROW: an order that only shows from the
// next day
const islandOrderTomorrow = 1

// IslandAblityAgency.TYPE_*
const (
	islandAbilityOrderSlot       = 8  // effect: island_order_list id
	islandAbilityRestaurant      = 17 // effect: island_manage_restaurant id
	islandAbilityAssistant       = 18 // effect: island_manage_assistant id
	islandAbilityOrderDailyCount = 20 // effect: more orders a day
	islandAbilityOrderExp        = 31 // the order level (favor) system
)

// IslandTaskTargetType and IslandAchievementType the server counts here
const (
	islandTargetOrder            = 3
	islandTargetShipOrder        = 12
	islandTargetRestaurantShelve = 27
	islandTargetRestaurantSell   = 28
	islandTargetRestaurantSales  = 29
	islandTargetRestaurantOpen   = 31
	islandRecordFinishOrder      = 4 // arg: the island_order_list type
	islandRecordRestaurantSell   = 29
	islandRecordRestaurantOpen   = 31
)

// island_item_data_template filter of the seeds (never asked for)
const islandItemFilterSeed = 10114

// the island's Development Funds (island_item_data_template 1): the orders'
// and restaurants' money (IslandOrder.GenAwards)
const islandItemFunds = 1

// islandOrderSlotConfig is an island_order_list row: an order slot.
type islandOrderSlotConfig struct {
	Type        uint32     `json:"type"`
	UnlockCost  [][]uint32 `json:"unlock_cost"`
	UnlockLevel uint32     `json:"unlock_level"`
}

// islandOrderFavorConfig is an island_order_favor row: an order level, the
// exp it takes and its reward.
type islandOrderFavorConfig struct {
	AwardDisplay [][]uint32 `json:"award_display"`
	Exp          uint32     `json:"exp"`
}

// islandOrderPriceConfig is an island_order_price row: an order level's
// rewards, [island exp, Development Funds], by difficulty.
type islandOrderPriceConfig struct {
	Common  []uint32 `json:"order_award"`
	Easy    []uint32 `json:"order_easy_award"`
	Hard    []uint32 `json:"order_award_challenge"`
	Special []uint32 `json:"order_award_special"`
}

// islandFirmOrderConfig is an island_order row: a fixed order (the guide's,
// the seasons' chains of activity orders).
type islandFirmOrderConfig struct {
	Type          uint32     `json:"type"`
	ActivityID    uint32     `json:"activity_id"`
	GroupID       uint32     `json:"group_id"`
	IsHead        uint32     `json:"is_head"`
	NextOrder     uint32     `json:"next_order"`
	EffectiveTime uint32     `json:"effective_time"`
	SeasonPtNum   uint32     `json:"season_pt_num"`
	Award         []uint32   `json:"award"`
	Request       [][]uint32 `json:"request"`
}

// islandOrderItemConfig is the part of an island_item_data_template row the
// orders and restaurants read.
type islandOrderItemConfig struct {
	Type            uint32          `json:"type"`
	OrderPrice      uint32          `json:"order_price"`
	TechID          uint32          `json:"tech_id"`
	Filter          json.RawMessage `json:"filter"`
	ManageInfluence uint32          `json:"manage_influence"`
}

// islandAbilityConfig is an island_ability_template row.
type islandAbilityConfig struct {
	Type   uint32 `json:"type"`
	Effect uint32 `json:"effect"`
}

var (
	islandOrderSlots     map[uint32]islandOrderSlotConfig
	islandOrderFavors    map[uint32]islandOrderFavorConfig
	islandOrderPrices    map[uint32]islandOrderPriceConfig
	islandFirmOrders     map[uint32]islandFirmOrderConfig
	islandOrderItems     map[uint32]islandOrderItemConfig
	islandOrderAbilities map[uint32]islandAbilityConfig
	islandOrderPositions map[uint32]struct{}
	islandOrderDialogs   map[uint32]struct{}
	islandOrderLevels    map[uint32]struct{}
	// derived
	islandOrderSeedItems map[uint32]bool
	islandOrderMaxLevel  uint32 // the highest island level (island_level)
)

func loadIslandOrderData() error {
	err := gamedata.LoadAll(
		gamedata.Load("island_order_list", &islandOrderSlots),
		gamedata.Load("island_order_favor", &islandOrderFavors),
		gamedata.Load("island_order_price", &islandOrderPrices),
		gamedata.Load("island_order", &islandFirmOrders),
		gamedata.Load("island_item_data_template", &islandOrderItems),
		gamedata.Load("island_ability_template", &islandOrderAbilities),
		gamedata.Load("island_order_position", &islandOrderPositions),
		gamedata.Load("island_order_publish_random", &islandOrderDialogs),
		gamedata.Load("island_level", &islandOrderLevels),
		loadIslandManageData,
	)
	if err != nil {
		return err
	}
	if len(islandOrderPrices) == 0 || len(islandOrderPositions) == 0 || len(islandOrderDialogs) == 0 {
		return gamedata.Missing("island_order_price", "rows")
	}
	islandOrderSeedItems = map[uint32]bool{}
	for id, item := range islandOrderItems {
		if slices.Contains(gamedata.IDs(item.Filter), islandItemFilterSeed) {
			islandOrderSeedItems[id] = true
		}
	}
	islandOrderMaxLevel = slices.Max(slices.Collect(maps.Keys(islandOrderLevels)))
	return nil
}

func (game *Game) registerIslandOrders() {
	packets.Handle(game.handleIslandOrderSubmit)
	packets.Handle(game.handleIslandOrderReplace)
	packets.Handle(game.handleIslandOrderSubmitUrgent)
	packets.Handle(game.handleIslandShipOrderOp)
	packets.Handle(game.handleIslandOrderTendency)
	packets.Handle(game.handleIslandOrderFavorAward)
	packets.Handle(game.handleIslandOrderSubmitFirm)
	packets.Handle(game.handleIslandShipOrderLoad)
	packets.Handle(game.handleIslandRestaurantOpen)
	packets.Handle(game.handleIslandRestaurantClose)
	packets.Handle(game.handleIslandTicketUse)
	packets.Handle(game.handleIslandTicketRemoveExpired)
	packets.Handle(game.handleIslandTicketUseAppoint)
	packets.Handle(game.handleIslandShipOrderReset)
	packets.Handle(game.handleIslandShipOrderExchange)
}

// islandOrderExtra is what only the server keeps of the orders.
type islandOrderExtra struct {
	Day         int64    `json:"day"`          // the server day the daily counts are of
	Week        int64    `json:"week"`         // the week the weekly counts are of
	EventDay    int64    `json:"event_day"`    // the day the restaurants' event was drawn
	FirmGiven   []uint32 `json:"firm_given"`   // the fixed non-activity orders handed out
	NextAppoint uint32   `json:"next_appoint"` // the last transport offer id
}

const islandOrderExtraKey = "order"

// islandOrderChanges are what an update changed that the client only learns
// from pushes: order slots (SC_21407) and the restaurants' day (SC_21422).
type islandOrderChanges struct {
	slots  []*protobuf.PB_ISLAND_ORDER_SLOT
	manage *protobuf.SC_21422
}

// orders is the island's order system.
func (state *islandState) orders() *protobuf.PB_ISLAND_ORDER_SYSTEM {
	return state.private().OrderSystem
}

// hasIslandAbility tells whether the island has an ability of type kind
// with that effect (any effect when effect is 0).
func (state *islandState) hasIslandAbility(kind, effect uint32) bool {
	for _, id := range state.public().GetAbilityList() {
		config, ok := islandOrderAbilities[id]
		if ok && config.Type == kind && (effect == 0 || config.Effect == effect) {
			return true
		}
	}
	return false
}

// islandAbilityTotal sums the effects of the island's abilities of type
// kind.
func (state *islandState) islandAbilityTotal(kind uint32) uint32 {
	total := uint32(0)
	for _, id := range state.public().GetAbilityList() {
		if config, ok := islandOrderAbilities[id]; ok && config.Type == kind {
			total += config.Effect
		}
	}
	return total
}

// addIslandExp adds island exp as IslandDropHelper.AddIslandExp does
// (BaseIsland.AddExp: nothing at the highest level).
func (state *islandState) addIslandExp(exp uint32) {
	public := state.public()
	if exp == 0 || public.GetLevel() >= islandOrderMaxLevel {
		return
	}
	public.Exp = proto.Uint32(public.GetExp() + exp)
}

// islandOrdersUpdate brings the island's orders and restaurants up to now:
// the daily counts (finished orders, restaurant openings) start again each
// server day and the weekly ones (urgent orders, order exp and its rewards)
// each Monday as IslandOrderAgency.UpdatePerDay does, the order slots the
// island has get their order, the fixed orders theirs, and the restaurants
// the island has their place, assistants and the day's event. Called on
// each order request and when the client loads its island (SC_21201).
func islandOrdersUpdate(state *islandState, now time.Time) (*islandOrderChanges, error) {
	changes := &islandOrderChanges{}
	extra := islandOrderExtra{}
	if _, err := state.getExtra(islandOrderExtraKey, &extra); err != nil {
		return nil, err
	}
	orders := state.orders()
	day := startOfDay(now).Unix()
	newDay := extra.Day != 0 && extra.Day != day
	if extra.Day != day {
		if extra.Day != 0 {
			orders.DailySlotNum = proto.Uint32(0)
			state.public().TradeSys.TodayNum = nil
		}
		extra.Day = day
	}
	if weekStart := periodStart(now, week).Unix(); extra.Week != weekStart {
		if extra.Week != 0 {
			orders.TimeSlotNum = proto.Uint32(0)
			orders.Favor = proto.Uint32(0)
			// GUESS: the order level rewards can be had again with the exp
			orders.GetFavor_ = nil
		}
		extra.Week = weekStart
	}

	// the common slots: the first from the start, the others with their
	// ability (GUESS: the abilities are what opens them, as for the
	// transport jobs, IslandAblityAgency.IsUnlockShipOrder)
	commonOpen := false
	first := true
	for _, id := range slices.Sorted(maps.Keys(islandOrderSlots)) {
		if islandOrderSlots[id].Type != islandOrderListCommon {
			continue
		}
		open := first || state.hasIslandAbility(islandAbilityOrderSlot, id)
		first = false
		if !open {
			continue
		}
		commonOpen = true
		if state.orderSlot(id) == nil {
			slot := state.newCommonOrder(id, now, 0)
			orders.SlotList = append(orders.SlotList, slot)
			changes.slots = append(changes.slots, slot)
		}
	}

	// the fixed orders: GUESS the non-activity ones (the guide's, task
	// target ORDER 2) come once with the order system, the activity chains
	// start at their head while their activity runs and haven't been
	// finished, and leave when it ends
	if commonOpen {
		for _, id := range slices.Sorted(maps.Keys(islandFirmOrders)) {
			config := islandFirmOrders[id]
			if config.ActivityID != 0 || slices.Contains(extra.FirmGiven, id) {
				continue
			}
			extra.FirmGiven = append(extra.FirmGiven, id)
			slot := state.newFirmOrder(id, now)
			orders.SlotList = append(orders.SlotList, slot)
			changes.slots = append(changes.slots, slot)
		}
	}
	orders.SlotList = slices.DeleteFunc(orders.SlotList, func(slot *protobuf.PB_ISLAND_ORDER_SLOT) bool {
		if slot.GetType() != islandOrderFirm {
			return false
		}
		config, ok := islandFirmOrders[slot.GetId()]
		if !ok {
			return true
		}
		_, open := openActivity(config.ActivityID, now)
		return config.ActivityID != 0 && !open
	})
	if commonOpen {
		for _, id := range slices.Sorted(maps.Keys(islandFirmOrders)) {
			config := islandFirmOrders[id]
			if config.ActivityID == 0 || config.IsHead != 1 {
				continue
			}
			if _, open := openActivity(config.ActivityID, now); !open || state.firmGroupFinished(config.ActivityID, config.GroupID) {
				continue
			}
			if slices.ContainsFunc(orders.SlotList, func(slot *protobuf.PB_ISLAND_ORDER_SLOT) bool {
				other, ok := islandFirmOrders[slot.GetId()]
				return slot.GetType() == islandOrderFirm && ok && other.ActivityID == config.ActivityID && other.GroupID == config.GroupID
			}) {
				continue
			}
			slot := state.newFirmOrder(id, now)
			orders.SlotList = append(orders.SlotList, slot)
			changes.slots = append(changes.slots, slot)
		}
	}

	manage, err := state.restaurantsUpdate(&extra, now, newDay)
	if err != nil {
		return nil, err
	}
	changes.manage = manage
	return changes, state.setExtra(islandOrderExtraKey, extra)
}

// pushOrderChanges queues the pushes for what an update changed.
func (tx *Tx) pushOrderChanges(state *islandState, changes *islandOrderChanges) {
	for _, slot := range changes.slots {
		tx.push(&protobuf.SC_21407{IslandId: proto.Uint32(state.commanderID), OrderInfo: slot})
	}
	if changes.manage != nil {
		tx.push(changes.manage)
	}
}

// ownIslandOrders is the commander's island brought up to now, the update's
// pushes queued.
func (tx *Tx) ownIslandOrders(now time.Time) (*islandState, error) {
	state, err := tx.island()
	if err != nil {
		return nil, err
	}
	changes, err := islandOrdersUpdate(state, now)
	if err != nil {
		return nil, err
	}
	tx.pushOrderChanges(state, changes)
	return state, nil
}

// islandOrderAbilityGained runs when the island gains an ability: an order
// slot gets its order (SC_21407, the client doesn't make one itself), a
// restaurant (IslandManageAgecny.UnlockNewRestaurant) or an assistant
// post is stored as the client adds it. Meant to be called from
// islandAbilityGained.
func (tx *Tx) islandOrderAbilityGained(state *islandState, id uint32) error {
	changes, err := islandOrdersUpdate(state, serverNow())
	if err != nil {
		return err
	}
	tx.pushOrderChanges(state, changes)
	return nil
}

// firmGroupFinished tells whether an activity's chain of orders was done.
func (state *islandState) firmGroupFinished(activityID, groupID uint32) bool {
	for _, group := range state.orders().GetActGroup() {
		if group.GetActId() == activityID && slices.Contains(group.GetGroups(), groupID) {
			return true
		}
	}
	return false
}

// orderSlot is the island's order slot id, nil if none.
func (state *islandState) orderSlot(id uint32) *protobuf.PB_ISLAND_ORDER_SLOT {
	for _, slot := range state.orders().GetSlotList() {
		if slot.GetId() == id {
			return slot
		}
	}
	return nil
}

// setOrderSlot puts slot in place of the island's slot of the same id (or
// adds it).
func (state *islandState) setOrderSlot(slot *protobuf.PB_ISLAND_ORDER_SLOT) {
	orders := state.orders()
	for i, current := range orders.SlotList {
		if current.GetId() == slot.GetId() {
			orders.SlotList[i] = slot
			return
		}
	}
	orders.SlotList = append(orders.SlotList, slot)
}

// removeOrderSlot takes order slot id off the island, with its speed-ups.
func (state *islandState) removeOrderSlot(id uint32) {
	orders := state.orders()
	orders.SlotList = slices.DeleteFunc(orders.SlotList, func(slot *protobuf.PB_ISLAND_ORDER_SLOT) bool { return slot.GetId() == id })
	state.clearSpeedUp(id)
}

// clearSpeedUp forgets the tickets used on order slot id (PB_SPEED_USE).
func (state *islandState) clearSpeedUp(id uint32) {
	orders := state.orders()
	orders.SpeedList = slices.DeleteFunc(orders.SpeedList, func(speed *protobuf.PB_SPEED_USE) bool { return speed.GetSlotId() == id })
}

// speedUp is the time the tickets took off order slot id.
func (state *islandState) speedUp(id uint32) uint32 {
	for _, speed := range state.orders().GetSpeedList() {
		if speed.GetSlotId() == id {
			return speed.GetSpeedTime()
		}
	}
	return 0
}

// addSpeedUp records seconds of tickets used on order slot id.
func (state *islandState) addSpeedUp(id, seconds uint32) {
	orders := state.orders()
	for _, speed := range orders.SpeedList {
		if speed.GetSlotId() == id {
			speed.SpeedTime = proto.Uint32(speed.GetSpeedTime() + seconds)
			return
		}
	}
	orders.SpeedList = append(orders.SpeedList, &protobuf.PB_SPEED_USE{SlotId: proto.Uint32(id), SpeedTime: proto.Uint32(seconds)})
}

// orderLevel is the level of the island's new orders (island_order_price):
// GUESS its island level.
func (state *islandState) orderLevel() uint32 {
	level := max(state.public().GetLevel(), 1)
	for level > 1 {
		if _, ok := islandOrderPrices[level]; ok {
			break
		}
		level--
	}
	return level
}

// orderPosition draws where a new order shows on the order board
// (island_order_position), one no other order of the island is at if any.
func (state *islandState) orderPosition() uint32 {
	used := map[uint32]bool{}
	for _, slot := range state.orders().GetSlotList() {
		used[slot.GetPosition()] = true
	}
	all := slices.Sorted(maps.Keys(islandOrderPositions))
	free := slices.DeleteFunc(slices.Clone(all), func(id uint32) bool { return used[id] })
	if len(free) == 0 {
		free = all
	}
	return free[rand.IntN(len(free))]
}

// orderAward is an order level's [island exp, Development Funds] for a
// difficulty (IslandOrder.GetAwardConfigByTendency).
func orderAward(level, tendency uint32, urgent bool) (exp, funds uint32) {
	config := islandOrderPrices[level]
	award := config.Common
	switch {
	case urgent:
		award = config.Special
	case tendency == islandTendencyEasy:
		award = config.Easy
	case tendency == islandTendencyHard:
		award = config.Hard
	}
	if len(award) < 2 {
		return 0, 0
	}
	return award[0], award[1]
}

// newCommonOrder is a new order for common slot id, shown after delay.
// GUESS: it asks for items worth (island_item_data_template order_price)
// the Development Funds it pays times island_set order_award_coefficient
// percent, at the island's difficulty (daily_select). Once the day's orders
// are done (order_daily_limit_num plus the TYPE_ORDER_DAILY_CNT abilities)
// it only shows the next day.
func (state *islandState) newCommonOrder(id uint32, now time.Time, delay time.Duration) *protobuf.PB_ISLAND_ORDER_SLOT {
	orders := state.orders()
	level := state.orderLevel()
	tendency := orders.GetDailySelect()
	_, funds := orderAward(level, tendency, false)
	start, submit, flag := now, now.Add(delay), uint32(0)
	if orders.GetDailySlotNum() >= state.dailyOrderLimit() {
		submit, flag = nextDay(now), islandOrderTomorrow
	}
	return protocol.FillRequired(&protobuf.PB_ISLAND_ORDER_SLOT{
		Id: proto.Uint32(id), Type: proto.Uint32(islandOrderNormal), CurSelect: proto.Uint32(tendency),
		StartTime: proto.Uint32(uint32(start.Unix())), SubmitTime: proto.Uint32(uint32(submit.Unix())),
		Position: proto.Uint32(state.orderPosition()), DialogId: proto.Uint32(randomOrderDialog()),
		Cost:    state.orderCost(funds*islandSetInt("order_award_coefficient")/100, level),
		OrderLv: proto.Uint32(level), ViewFlag: proto.Uint32(flag),
	})
}

// newUrgentOrder is a new urgent order for slot id: GUESS like a common one
// at the island_order_price urgent rewards, there for island_set
// order_special_duration.
func (state *islandState) newUrgentOrder(id uint32, now time.Time) *protobuf.PB_ISLAND_ORDER_SLOT {
	level := state.orderLevel()
	_, funds := orderAward(level, 0, true)
	return protocol.FillRequired(&protobuf.PB_ISLAND_ORDER_SLOT{
		Id: proto.Uint32(id), Type: proto.Uint32(islandOrderUrgent), CurSelect: proto.Uint32(0),
		StartTime:  proto.Uint32(uint32(now.Unix())),
		SubmitTime: proto.Uint32(uint32(now.Unix()) + islandSetInt("order_special_duration")),
		Position:   proto.Uint32(state.orderPosition()), DialogId: proto.Uint32(randomOrderDialog()),
		Cost:    state.orderCost(funds*islandSetInt("order_award_coefficient")/100, level),
		OrderLv: proto.Uint32(level), ViewFlag: proto.Uint32(0),
	})
}

// newFirmOrder is island_order id as an order slot: its slot id and dialog
// are the order's id (IslandOrderSlot.GenOrder, IslandFirmOrder), shown at
// once; an urgent one leaves after its effective_time.
func (state *islandState) newFirmOrder(id uint32, now time.Time) *protobuf.PB_ISLAND_ORDER_SLOT {
	config := islandFirmOrders[id]
	var cost []*protobuf.PB_ISLAND_ITEM
	for _, request := range config.Request {
		if len(request) >= 2 {
			cost = append(cost, &protobuf.PB_ISLAND_ITEM{Id: proto.Uint32(request[0]), Num: proto.Uint32(request[1])})
		}
	}
	submit := uint32(now.Unix())
	if config.Type == islandFirmUrgent {
		submit += config.EffectiveTime
	}
	return protocol.FillRequired(&protobuf.PB_ISLAND_ORDER_SLOT{
		Id: proto.Uint32(id), Type: proto.Uint32(islandOrderFirm), CurSelect: proto.Uint32(0),
		StartTime: proto.Uint32(uint32(now.Unix())), SubmitTime: proto.Uint32(submit),
		Position: proto.Uint32(state.orderPosition()), DialogId: proto.Uint32(id), Cost: cost,
		OrderLv: proto.Uint32(state.orderLevel()), ViewFlag: proto.Uint32(0),
	})
}

// randomOrderDialog draws an island_order_publish_random row: who asks and
// what they say.
func randomOrderDialog() uint32 {
	ids := slices.Sorted(maps.Keys(islandOrderDialogs))
	return ids[rand.IntN(len(ids))]
}

// dailyOrderLimit is how many common orders a day the island can do
// (IslandOrderAgency.GetMaxFinishCount).
func (state *islandState) dailyOrderLimit() uint32 {
	return islandSetInt("order_daily_limit_num") + state.islandAbilityTotal(islandAbilityOrderDailyCount)
}

// orderItemPool is the items an order can ask for. GUESS: the island's
// materials (island_item_data_template type 1 with an order_price, no
// seeds) it holds, and the raw ones (below 3000: crops, ore, wood, animal
// products) it can produce (no tech_id, or a finished technology).
func (state *islandState) orderItemPool() (owned, all []uint32) {
	finished := state.public().GetTech().GetFinishList()
	for _, id := range slices.Sorted(maps.Keys(islandOrderItems)) {
		item := islandOrderItems[id]
		if item.Type != 1 || item.OrderPrice == 0 || islandOrderSeedItems[id] {
			continue
		}
		have := state.islandItemCount(id) > 0
		if have {
			owned = append(owned, id)
		}
		if have || id < 3000 && (item.TechID == 0 || slices.Contains(finished, item.TechID)) {
			all = append(all, id)
		}
	}
	return owned, all
}

// orderCost draws what an order of an order level asks for: GUESS one to
// three kinds of items (more as the level rises), worth value together,
// rather ones the island holds.
func (state *islandState) orderCost(value, level uint32) []*protobuf.PB_ISLAND_ITEM {
	owned, all := state.orderItemPool()
	if len(all) == 0 {
		return nil
	}
	kinds := 1 + rand.IntN(min(3, 1+int(level)/10))
	share := max(value/uint32(kinds), 1)
	var cost []*protobuf.PB_ISLAND_ITEM
	taken := map[uint32]bool{}
	for range kinds {
		id, ok := pickOrderItem(owned, all, share, taken)
		if !ok {
			break
		}
		taken[id] = true
		price := islandOrderItems[id].OrderPrice
		count := min(max((share+price/2)/price, 1), 999)
		cost = append(cost, &protobuf.PB_ISLAND_ITEM{Id: proto.Uint32(id), Num: proto.Uint32(count)})
	}
	return cost
}

// pickOrderItem draws an item worth at most share if it can, among those
// not taken, one held two times out of three.
func pickOrderItem(owned, all []uint32, share uint32, taken map[uint32]bool) (uint32, bool) {
	fits := func(ids []uint32, cheap bool) []uint32 {
		return slices.DeleteFunc(slices.Clone(ids), func(id uint32) bool {
			return taken[id] || cheap && islandOrderItems[id].OrderPrice > share
		})
	}
	for _, cheap := range []bool{true, false} {
		pools := [][]uint32{fits(all, cheap)}
		if held := fits(owned, cheap); len(held) > 0 && rand.IntN(3) < 2 {
			pools = append([][]uint32{held}, pools...)
		}
		for _, pool := range pools {
			if len(pool) > 0 {
				return pool[rand.IntN(len(pool))], true
			}
		}
	}
	return 0, false
}

// orderDelay draws how long a slot's next order takes to come:
// island_set order_change_time [[after a replacement], [after a delivery],
// _]. GUESS: in minutes (the speed-up tickets go by the minute).
func orderDelay(replaced bool) time.Duration {
	var config []json.RawMessage
	var span []uint32
	if islandSetList("order_change_time", &config) != nil || len(config) < 2 {
		return 0
	}
	index := 1
	if replaced {
		index = 0
	}
	if json.Unmarshal(config[index], &span) != nil || len(span) < 2 || span[1] < span[0] {
		return 0
	}
	return time.Duration(span[0]+rand.Uint32N(span[1]-span[0]+1)) * time.Minute
}

// payOrder takes what an order asks for from the warehouse; ErrNotEnough
// (nothing taken) when it lacks any.
func (state *islandState) payOrder(cost []*protobuf.PB_ISLAND_ITEM) error {
	needed := map[uint32]uint32{}
	for _, item := range cost {
		needed[item.GetId()] += item.GetNum()
	}
	for id, count := range needed {
		if state.islandItemCount(id) < count {
			return ErrNotEnough
		}
	}
	for _, id := range slices.Sorted(maps.Keys(needed)) {
		if err := state.takeIslandItem(id, needed[id]); err != nil {
			return err
		}
	}
	return nil
}

// addOrderExp adds island_set order_favor to the order level's exp
// (IslandOrderAgency.AddExp: once the system is open, until the highest
// level).
func (state *islandState) addOrderExp() {
	orders := state.orders()
	if !state.hasIslandAbility(islandAbilityOrderExp, 0) || orders.GetFavor() >= orderFavorTarget(maxOrderFavorLevel()) {
		return
	}
	orders.Favor = proto.Uint32(orders.GetFavor() + islandSetInt("order_favor"))
}

// orderFavorTarget is the exp an order level takes from 0
// (IslandOrderAgency.StaticGetTargetExp).
func orderFavorTarget(level uint32) uint32 {
	total := uint32(0)
	for lv := uint32(1); lv <= level; lv++ {
		total += islandOrderFavors[lv].Exp
	}
	return total
}

func maxOrderFavorLevel() uint32 {
	return slices.Max(slices.Collect(maps.Keys(islandOrderFavors)))
}

// orderDrops are an order's rewards as drops, the island exp apart.
func orderDrops(funds, seasonPt uint32) []*protobuf.DROPINFO {
	var drops []*protobuf.DROPINFO
	if funds > 0 {
		drops = append(drops, &protobuf.DROPINFO{Type: proto.Uint32(dropTypeIslandItem), Id: proto.Uint32(islandItemFunds), Number: proto.Uint32(funds)})
	}
	if seasonPt > 0 {
		drops = append(drops, &protobuf.DROPINFO{Type: proto.Uint32(dropTypeIslandSeasonPt), Id: proto.Uint32(0), Number: proto.Uint32(seasonPt)})
	}
	return drops
}

// orderVisible tells whether an order is there to deliver: not still on its
// way (IslandOrder.IsLoading) nor an empty or gone one.
func (state *islandState) orderVisible(slot *protobuf.PB_ISLAND_ORDER_SLOT, now time.Time) bool {
	unix := uint32(now.Unix())
	switch slot.GetType() {
	case islandOrderUrgent:
		return slot.GetViewFlag() != islandOrderTomorrow && unix < slot.GetSubmitTime()
	case islandOrderFirm:
		config := islandFirmOrders[slot.GetId()]
		if config.Type == islandFirmUrgent {
			return slot.GetViewFlag() != islandOrderTomorrow && unix < slot.GetStartTime()+config.EffectiveTime
		}
		return slot.GetViewFlag() != islandOrderTomorrow
	}
	return unix+state.speedUp(slot.GetId()) >= slot.GetSubmitTime()
}

// finishOrder counts a delivered order for the tasks and achievements.
func (tx *Tx) finishOrder(listType uint32, keys ...uint32) error {
	if err := tx.islandTaskEvent(islandTargetOrder, 1, keys...); err != nil {
		return err
	}
	return tx.islandAchievementEvent(islandRecordFinishOrder, listType, 1)
}

// handleIslandOrderSubmit answers IslandSubmitOrderCommand for a common
// order (CS_21401): the items go, the order pays its island_order_price
// rewards at its level and difficulty (Development Funds in drop_list, island
// exp), counts for the day and the order level, and the slot gets its next
// order (slot), on its way for a while (orderDelay). GUESS: an order past the
// day's limit is refused, and when the limit is reached the other common
// slots' orders move to the next day (SC_21407). It may bring an urgent
// order (maybeUrgentOrder).
func (game *Game) handleIslandOrderSubmit(client *connection.Client, req *protobuf.CS_21401) error {
	reply := &protobuf.SC_21402{Result: result(errRefused)}
	err := game.tx(context.Background(), client, func(tx *Tx) error {
		now := serverNow()
		state, err := tx.ownIslandOrders(now)
		if err != nil {
			return err
		}
		orders := state.orders()
		slot := state.orderSlot(req.GetSlotId())
		if slot == nil || slot.GetType() != islandOrderNormal || !state.orderVisible(slot, now) || orders.GetDailySlotNum() >= state.dailyOrderLimit() {
			return nil
		}
		if err := state.payOrder(slot.GetCost()); err != nil {
			reply.Result = result(err)
			return nil
		}
		exp, funds := orderAward(slot.GetOrderLv(), slot.GetCurSelect(), false)
		drops := orderDrops(funds, 0)
		if err := tx.grantDrops(drops); err != nil {
			return err
		}
		state.addIslandExp(exp)
		state.addOrderExp()
		orders.DailySlotNum = proto.Uint32(orders.GetDailySlotNum() + 1)
		state.clearSpeedUp(slot.GetId())
		next := state.newCommonOrder(slot.GetId(), now, orderDelay(false))
		state.setOrderSlot(next)
		if orders.GetDailySlotNum() >= state.dailyOrderLimit() {
			for _, other := range slices.Clone(orders.SlotList) {
				if other.GetType() == islandOrderNormal && other.GetId() != next.GetId() && other.GetViewFlag() != islandOrderTomorrow {
					state.clearSpeedUp(other.GetId())
					moved := state.newCommonOrder(other.GetId(), now, 0)
					state.setOrderSlot(moved)
					tx.push(&protobuf.SC_21407{IslandId: proto.Uint32(state.commanderID), OrderInfo: moved})
				}
			}
		}
		if err := state.maybeUrgentOrder(tx, now); err != nil {
			return err
		}
		if err := tx.finishOrder(islandOrderListCommon); err != nil {
			return err
		}
		reply.Result = proto.Uint32(0)
		reply.Slot = next
		reply.DropList = drops
		return nil
	})
	if err != nil {
		return err
	}
	return client.Send(reply)
}

// maybeUrgentOrder may bring an urgent order (SC_21407) after a common one
// was delivered. GUESS: island_set order_special_probability[1] percent of
// the time, in the urgent slots the island has (their TYPE_ORDER ability)
// with no urgent order on, while fewer than order_special_limit_num were
// done this week.
func (state *islandState) maybeUrgentOrder(tx *Tx, now time.Time) error {
	orders := state.orders()
	if orders.GetTimeSlotNum() >= islandSetInt("order_special_limit_num") {
		return nil
	}
	var probability []uint32
	if islandSetList("order_special_probability", &probability) != nil || len(probability) == 0 || rand.Uint32N(100) >= probability[0] {
		return nil
	}
	for _, id := range slices.Sorted(maps.Keys(islandOrderSlots)) {
		if islandOrderSlots[id].Type != islandOrderListUrgent || !state.hasIslandAbility(islandAbilityOrderSlot, id) {
			continue
		}
		if current := state.orderSlot(id); current != nil && state.orderVisible(current, now) {
			continue
		}
		state.removeOrderSlot(id)
		slot := state.newUrgentOrder(id, now)
		orders.SlotList = append(orders.SlotList, slot)
		tx.push(&protobuf.SC_21407{IslandId: proto.Uint32(state.commanderID), OrderInfo: slot})
		return nil
	}
	return nil
}

// handleIslandOrderSubmitUrgent answers IslandSubmitOrderCommand for an
// urgent order (CS_21405): the items go, it pays the island_order_price
// urgent rewards (drop_list, island exp), counts for the week and the order
// level, and the slot goes (IslandOrderAgency.RemoveSlot) until the next
// urgent order.
func (game *Game) handleIslandOrderSubmitUrgent(client *connection.Client, req *protobuf.CS_21405) error {
	reply := &protobuf.SC_21406{Result: result(errRefused)}
	err := game.tx(context.Background(), client, func(tx *Tx) error {
		now := serverNow()
		state, err := tx.ownIslandOrders(now)
		if err != nil {
			return err
		}
		orders := state.orders()
		slot := state.orderSlot(req.GetSlotId())
		if slot == nil || slot.GetType() != islandOrderUrgent || !state.orderVisible(slot, now) {
			return nil
		}
		if err := state.payOrder(slot.GetCost()); err != nil {
			reply.Result = result(err)
			return nil
		}
		exp, funds := orderAward(slot.GetOrderLv(), 0, true)
		drops := orderDrops(funds, 0)
		if err := tx.grantDrops(drops); err != nil {
			return err
		}
		state.addIslandExp(exp)
		state.addOrderExp()
		orders.TimeSlotNum = proto.Uint32(orders.GetTimeSlotNum() + 1)
		state.removeOrderSlot(slot.GetId())
		if err := tx.finishOrder(islandOrderListUrgent); err != nil {
			return err
		}
		reply.Result = proto.Uint32(0)
		reply.DropList = drops
		return nil
	})
	if err != nil {
		return err
	}
	return client.Send(reply)
}

// handleIslandOrderSubmitFirm answers IslandSubmitOrderCommand for a fixed
// order (CS_21414, by its island_order id): the items go, it pays its
// award (island exp, Development Funds) and season points, and leaves
// (RemoveSlot). An activity chain's next order follows (SC_21407, GUESS at
// once); its last one marks the chain done (act_group, as
// AddFinishedActGroupId). Only the non-activity ones count for the order
// level.
func (game *Game) handleIslandOrderSubmitFirm(client *connection.Client, req *protobuf.CS_21414) error {
	reply := &protobuf.SC_21415{Result: result(errRefused)}
	err := game.tx(context.Background(), client, func(tx *Tx) error {
		now := serverNow()
		state, err := tx.ownIslandOrders(now)
		if err != nil {
			return err
		}
		slot := state.orderSlot(req.GetOrderId())
		config, ok := islandFirmOrders[req.GetOrderId()]
		if slot == nil || !ok || slot.GetType() != islandOrderFirm || !state.orderVisible(slot, now) {
			return nil
		}
		if err := state.payOrder(slot.GetCost()); err != nil {
			reply.Result = result(err)
			return nil
		}
		var exp, funds uint32
		if len(config.Award) >= 2 {
			exp, funds = config.Award[0], config.Award[1]
		}
		drops := orderDrops(funds, config.SeasonPtNum)
		if err := tx.grantDrops(drops); err != nil {
			return err
		}
		state.addIslandExp(exp)
		state.removeOrderSlot(slot.GetId())
		orders := state.orders()
		if config.Type == islandFirmActivity {
			if next, ok := islandFirmOrders[config.NextOrder]; ok && config.NextOrder != 0 && next.ActivityID == config.ActivityID {
				nextSlot := state.newFirmOrder(config.NextOrder, now)
				orders.SlotList = append(orders.SlotList, nextSlot)
				tx.push(&protobuf.SC_21407{IslandId: proto.Uint32(state.commanderID), OrderInfo: nextSlot})
			} else {
				var group *protobuf.PB_FINISH_ACT_GROUP
				for _, entry := range orders.ActGroup {
					if entry.GetActId() == config.ActivityID {
						group = entry
					}
				}
				if group == nil {
					group = &protobuf.PB_FINISH_ACT_GROUP{ActId: proto.Uint32(config.ActivityID)}
					orders.ActGroup = append(orders.ActGroup, group)
				}
				if !slices.Contains(group.Groups, config.GroupID) {
					group.Groups = append(group.Groups, config.GroupID)
				}
			}
		} else {
			state.addOrderExp()
		}
		if err := tx.finishOrder(islandOrderListCommon, req.GetOrderId()); err != nil {
			return err
		}
		reply.Result = proto.Uint32(0)
		reply.DropList = drops
		return nil
	})
	if err != nil {
		return err
	}
	return client.Send(reply)
}

// handleIslandOrderReplace answers IslandReplaceOrderCommand (CS_21403): a
// common order that is there (IslandOrder.CanReplace) is swapped for a new
// one on its way for a while (orderDelay), in slot.
func (game *Game) handleIslandOrderReplace(client *connection.Client, req *protobuf.CS_21403) error {
	reply := &protobuf.SC_21404{Result: result(errRefused)}
	err := game.tx(context.Background(), client, func(tx *Tx) error {
		now := serverNow()
		state, err := tx.ownIslandOrders(now)
		if err != nil {
			return err
		}
		slot := state.orderSlot(req.GetSlotId())
		if slot == nil || slot.GetType() != islandOrderNormal || !state.orderVisible(slot, now) || slot.GetViewFlag() == islandOrderTomorrow {
			return nil
		}
		state.clearSpeedUp(slot.GetId())
		next := state.newCommonOrder(slot.GetId(), now, orderDelay(true))
		state.setOrderSlot(next)
		reply.Result = proto.Uint32(0)
		reply.Slot = next
		return nil
	})
	if err != nil {
		return err
	}
	return client.Send(reply)
}

// handleIslandOrderTendency answers IslandSetOrderTendencyCommand
// (CS_21410): the difficulty (IslandOrderSlot.TENDENCY_TYPE_*) of the orders
// to come; the ones there keep theirs (GUESS).
func (game *Game) handleIslandOrderTendency(client *connection.Client, req *protobuf.CS_21410) error {
	if req.GetType() > islandTendencyHard {
		return client.Send(&protobuf.SC_21411{Result: result(errRefused)})
	}
	err := game.tx(context.Background(), client, func(tx *Tx) error {
		state, err := tx.ownIslandOrders(serverNow())
		if err != nil {
			return err
		}
		state.orders().DailySelect = proto.Uint32(req.GetType())
		return nil
	})
	if err != nil {
		return err
	}
	return client.Send(&protobuf.SC_21411{Result: proto.Uint32(0)})
}

// handleIslandOrderFavorAward answers GetIslandOrderExpAwardCommand
// (CS_21412): an order level reached (IslandOrderAgency.CanGetAward: its
// exp, not had yet) gives its island_order_favor award_display once.
func (game *Game) handleIslandOrderFavorAward(client *connection.Client, req *protobuf.CS_21412) error {
	reply := &protobuf.SC_21413{Result: result(errRefused)}
	err := game.tx(context.Background(), client, func(tx *Tx) error {
		state, err := tx.ownIslandOrders(serverNow())
		if err != nil {
			return err
		}
		orders := state.orders()
		config, ok := islandOrderFavors[req.GetLv()]
		if !ok || slices.Contains(orders.GetGetFavor_(), req.GetLv()) || orders.GetFavor() < orderFavorTarget(req.GetLv()) {
			return nil
		}
		var drops []*protobuf.DROPINFO
		for _, award := range config.AwardDisplay {
			if len(award) >= 3 {
				drops = append(drops, &protobuf.DROPINFO{Type: proto.Uint32(award[0]), Id: proto.Uint32(award[1]), Number: proto.Uint32(award[2])})
			}
		}
		if err := tx.grantDrops(drops); err != nil {
			return err
		}
		orders.GetFavor_ = append(orders.GetFavor_, req.GetLv())
		reply.Result = proto.Uint32(0)
		reply.DropList = drops
		return nil
	})
	if err != nil {
		return err
	}
	return client.Send(reply)
}
