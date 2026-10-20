package game

import (
	"context"
	"maps"
	"math/rand/v2"
	"slices"
	"time"

	"google.golang.org/protobuf/proto"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/gamedata"
	"github.com/ggmolly/belfast/internal/protobuf"
	"github.com/ggmolly/belfast/internal/protocol"
)

// The island's restaurants (IslandManageAgecny, IslandRestaurant,
// island_manage_*): a restaurant the island has (its TYPE_RESTAURANT
// ability) opens island_manage_restaurant opening_number times a day for
// opening_time, its assistant posts staffed and its shelves stocked with its
// dishes (item_id); once closed it pays what it sold (Development Funds),
// gives back the rest, and its sales raise its rank (island_manage_rank
// level_up_exp). Each day one restaurant has an event (island_manage_event)
// that raises some of its dishes' prices (SC_21422).
//
// GUESSES: how much sells (between the estimate the client sends,
// PB_TRADE_PRESELL, spread over the dishes as stocked; at least
// island_manage_sale_limit of each), the price (order_price times
// island_manage_price_coefficient percent, plus the event's add_per, without
// the assistants' buffs), the money (Development Funds), that the
// assistants spend no energy, and the daily event (any event, any
// restaurant, event_effect [add_per, dishes] on that many of its dishes).

// islandRestaurantConfig is an island_manage_restaurant row.
type islandRestaurantConfig struct {
	AbilityID     uint32     `json:"ability_id"`
	AssistantSlot []uint32   `json:"assistant_slot"`
	ItemID        [][]uint32 `json:"item_id"`
	OpeningNumber uint32     `json:"opening_number"`
	OpeningTime   uint32     `json:"opening_time"`
}

// islandManageEventConfig is an island_manage_event row.
type islandManageEventConfig struct {
	EventEffect    [][]uint32 `json:"event_effect"`
	InfluenceBonus uint32     `json:"influence_bonus"`
}

// islandManageRankConfig is an island_manage_rank row: a restaurant rank.
type islandManageRankConfig struct {
	LevelUpExp [][]uint32 `json:"level_up_exp"`
	SlotNum    []uint32   `json:"slot_num"`
}

// islandAssistantConfig is an island_manage_assistant row: a restaurant's
// assistant post, there from the start (unlock_type 0) or with an ability.
type islandAssistantConfig struct {
	Restaurant uint32 `json:"restaurant"`
	UnlockType uint32 `json:"unlock_type"`
}

// islandTicketTimeConfig is the time an island_speedup_ticket takes off.
type islandTicketTimeConfig struct {
	SpeedupTime uint32 `json:"speedup_time"`
}

var (
	islandRestaurants     map[uint32]islandRestaurantConfig
	islandManageEvents    map[uint32]islandManageEventConfig
	islandManageRanks     map[uint32]islandManageRankConfig
	islandAssistants      map[uint32]islandAssistantConfig
	islandOrderTicketTime map[uint32]islandTicketTimeConfig
)

func loadIslandManageData() error {
	return gamedata.LoadAll(
		gamedata.Load("island_manage_restaurant", &islandRestaurants),
		gamedata.Load("island_manage_event", &islandManageEvents),
		gamedata.Load("island_manage_rank", &islandManageRanks),
		gamedata.Load("island_manage_assistant", &islandAssistants),
		gamedata.Load("island_speedup_ticket", &islandOrderTicketTime),
	)
}

// dishes are a restaurant's dishes (item_id: [dish, its look]).
func (config islandRestaurantConfig) dishes() []uint32 {
	var dishes []uint32
	for _, entry := range config.ItemID {
		if len(entry) > 0 {
			dishes = append(dishes, entry[0])
		}
	}
	return dishes
}

// restaurant is the island's restaurant id, nil if it has none.
func (state *islandState) restaurant(id uint32) *protobuf.PB_ISLAND_TRADE {
	for _, trade := range state.public().GetTradeSys().GetTradeList() {
		if trade.GetId() == id {
			return trade
		}
	}
	return nil
}

// restaurantsUpdate gives the island the restaurants and assistant posts
// its abilities open, and draws the day's event once a day: a new day's
// draw is pushed as a daily refresh (type 1, the client starts the day's
// openings over too), a draw for a first restaurant later in the day as an
// unlock (type 2).
func (state *islandState) restaurantsUpdate(extra *islandOrderExtra, now time.Time, newDay bool) (*protobuf.SC_21422, error) {
	trades := state.public().TradeSys
	abilities := state.public().GetAbilityList()
	for _, id := range slices.Sorted(maps.Keys(islandRestaurants)) {
		config := islandRestaurants[id]
		if state.restaurant(id) == nil && slices.Contains(abilities, config.AbilityID) {
			trades.TradeList = append(trades.TradeList, protocol.FillRequired(&protobuf.PB_ISLAND_TRADE{
				Id: proto.Uint32(id), Lv: proto.Uint32(1), TotalSell: proto.Uint32(0), EndTime: proto.Uint32(0), SpeedTime: proto.Uint32(0),
			}))
		}
	}
	for _, trade := range trades.TradeList {
		for _, post := range islandRestaurants[trade.GetId()].AssistantSlot {
			assistant := islandAssistants[post]
			if assistant.UnlockType != 0 && !slices.Contains(abilities, assistant.UnlockType) {
				continue
			}
			if !slices.ContainsFunc(trade.PostList, func(entry *protobuf.PB_TRADE_POST) bool { return entry.GetPostId() == post }) {
				trade.PostList = append(trade.PostList, &protobuf.PB_TRADE_POST{PostId: proto.Uint32(post), ShipId: proto.Uint32(0)})
			}
		}
	}
	day := startOfDay(now).Unix()
	if extra.EventDay == day || len(trades.TradeList) == 0 || len(islandManageEvents) == 0 {
		return nil, nil
	}
	extra.EventDay = day
	eventID := randomPick(slices.Sorted(maps.Keys(islandManageEvents)), 1)[0]
	trade := trades.TradeList[rand.IntN(len(trades.TradeList))]
	trades.TodayEvent = proto.Uint32(eventID)
	trades.TodayTrade = proto.Uint32(trade.GetId())
	trades.Effect = nil
	for _, effect := range islandManageEvents[eventID].EventEffect {
		if len(effect) < 2 {
			continue
		}
		for _, dish := range randomPick(islandRestaurants[trade.GetId()].dishes(), int(effect[1])) {
			trades.Effect = append(trades.Effect, &protobuf.PB_EVENT_EFFECT{FoodId: proto.Uint32(dish), AddPer: proto.Uint32(effect[0])})
		}
	}
	kind := uint32(2)
	if newDay {
		kind = 1
	}
	return &protobuf.SC_21422{Type: proto.Uint32(kind), TodayEvent: trades.TodayEvent, TodayTrade: trades.TodayTrade, Effect: trades.Effect}, nil
}

// openedToday is how many times restaurant id opened today.
func (state *islandState) openedToday(id uint32) uint32 {
	for _, entry := range state.public().GetTradeSys().GetTodayNum() {
		if entry.GetTradeId() == id {
			return entry.GetNum()
		}
	}
	return 0
}

// dishBonus is the event's price raise of a dish of restaurant id, in
// percent.
func (state *islandState) dishBonus(id, dish uint32) uint32 {
	trades := state.public().GetTradeSys()
	if trades.GetTodayTrade() != id {
		return 0
	}
	for _, effect := range trades.GetEffect() {
		if effect.GetFoodId() == dish {
			return effect.GetAddPer()
		}
	}
	return 0
}

// dishPrice is what count of a dish sell for, before the event
// (IslandRestaurantPage.CaclGroupPrice without the buffs).
func dishPrice(dish, count uint32) uint32 {
	return islandOrderItems[dish].OrderPrice * islandSetInt("island_manage_price_coefficient") / 100 * count
}

// handleIslandRestaurantOpen answers IslandOpenRestaurantCommand
// (CS_21418): a restaurant ready to open (IslandRestaurant STATUS.PREPARE:
// closed, openings left today) takes the dishes stocked (from the
// warehouse) and its assistants (posts, island characters) and opens for
// opening_time; trade_data is what it will have sold when it closes, and
// ship_power the assistants' energy. The others on the island see it open
// (SC_21225).
func (game *Game) handleIslandRestaurantOpen(client *connection.Client, req *protobuf.CS_21418) error {
	reply := &protobuf.SC_21419{Result: result(errRefused)}
	err := game.tx(context.Background(), client, func(tx *Tx) error {
		now := serverNow()
		state, err := tx.ownIslandOrders(now)
		if err != nil {
			return err
		}
		id := req.GetTradeId()
		trade, config := state.restaurant(id), islandRestaurants[id]
		if trade == nil || trade.GetEndTime() != 0 || state.openedToday(id) >= config.OpeningNumber || len(req.GetFoodList()) == 0 {
			return nil
		}
		if rank := islandManageRanks[trade.GetLv()]; len(rank.SlotNum) > 0 && len(req.GetFoodList()) > int(rank.SlotNum[0]) {
			return nil
		}
		dishes := config.dishes()
		var stock []*protobuf.PB_ISLAND_ITEM
		total := uint32(0)
		for _, food := range req.GetFoodList() {
			if !slices.Contains(dishes, food.GetFoodId()) || food.GetNum() == 0 || slices.ContainsFunc(stock, func(item *protobuf.PB_ISLAND_ITEM) bool { return item.GetId() == food.GetFoodId() }) {
				return nil
			}
			stock = append(stock, &protobuf.PB_ISLAND_ITEM{Id: proto.Uint32(food.GetFoodId()), Num: proto.Uint32(food.GetNum())})
			total += food.GetNum()
		}
		ships := map[uint32]*protobuf.PB_ISLAND_SHIP{}
		for _, ship := range state.public().GetShipSys().GetShipList() {
			ships[ship.GetId()] = ship
		}
		staff := map[uint32]uint32{}
		seen := map[uint32]bool{}
		for _, post := range req.GetPostList() {
			ship := post.GetShipId()
			if !slices.ContainsFunc(trade.PostList, func(entry *protobuf.PB_TRADE_POST) bool { return entry.GetPostId() == post.GetPostId() }) ||
				ship != 0 && (ships[ship] == nil || seen[ship]) {
				return nil
			}
			seen[ship] = ship != 0
			staff[post.GetPostId()] = ship
		}
		if err := state.payOrder(stock); err != nil {
			reply.Result = result(err)
			return nil
		}

		presell := req.GetPresell()
		low, high := min(presell.GetSellNumMin(), total), min(presell.GetSellNumMax(), total)
		low = min(low, high)
		sold := low + rand.Uint32N(high-low+1)
		trade.SellList, trade.RestList = nil, nil
		left := sold
		amounts := make([]uint32, len(stock))
		for i, item := range stock {
			amounts[i] = min(item.GetNum()*sold/total, item.GetNum())
			left -= amounts[i]
		}
		for i, item := range stock {
			extra := min(left, item.GetNum()-amounts[i])
			amounts[i] += extra
			left -= extra
			amounts[i] = max(amounts[i], min(islandSetInt("island_manage_sale_limit"), item.GetNum()))
		}
		for i, item := range stock {
			dish := item.GetId()
			if amounts[i] > 0 {
				price := dishPrice(dish, amounts[i])
				price += price * state.dishBonus(id, dish) / 100
				trade.SellList = append(trade.SellList, &protobuf.PB_TRADE_SELL_FOOD{FoodId: proto.Uint32(dish), Num: proto.Uint32(amounts[i]), SellMoney: proto.Uint32(price)})
			}
			if rest := item.GetNum() - amounts[i]; rest > 0 {
				trade.RestList = append(trade.RestList, &protobuf.PB_TRADE_FOOD{FoodId: proto.Uint32(dish), Num: proto.Uint32(rest)})
			}
		}
		for _, post := range trade.PostList {
			post.ShipId = proto.Uint32(staff[post.GetPostId()])
			if ship := ships[post.GetShipId()]; ship != nil {
				reply.ShipPower = append(reply.ShipPower, &protobuf.PB_ISLAND_SHIP_POWER{ShipId: ship.Id, Power: proto.Uint32(ship.GetPower())})
			}
		}
		trade.EndTime = proto.Uint32(uint32(now.Unix()) + config.OpeningTime)
		trade.SpeedTime = proto.Uint32(0)

		trades := state.public().TradeSys
		counted := false
		for _, entry := range trades.TodayNum {
			if entry.GetTradeId() == id {
				entry.Num = proto.Uint32(entry.GetNum() + 1)
				counted = true
			}
		}
		if !counted {
			trades.TodayNum = append(trades.TodayNum, &protobuf.PB_TRADE_NUM{TradeId: proto.Uint32(id), Num: proto.Uint32(1)})
		}
		kept := protocol.FillRequired(proto.Clone(presell).(*protobuf.PB_TRADE_PRESELL))
		kept.TradeId = proto.Uint32(id)
		trades.PresellList = append(slices.DeleteFunc(trades.PresellList, func(entry *protobuf.PB_TRADE_PRESELL) bool { return entry.GetTradeId() == id }), kept)

		for _, item := range stock {
			if err := tx.islandTaskEvent(islandTargetRestaurantShelve, item.GetNum(), item.GetId()); err != nil {
				return err
			}
		}
		if err := tx.islandTaskEvent(islandTargetRestaurantOpen, 1, id); err != nil {
			return err
		}
		if err := tx.islandAchievementEvent(islandRecordRestaurantOpen, 0, 1); err != nil {
			return err
		}
		reply.Result = proto.Uint32(0)
		reply.TradeData = trade
		return nil
	})
	if err != nil {
		return err
	}
	if reply.GetResult() == 0 {
		if islandID, here := game.islandOf(client); here && islandID == client.CommanderID {
			game.tellIsland(client, islandID, &protobuf.SC_21225{IslandId: proto.Uint32(islandID), Trade: reply.TradeData})
		}
	}
	return client.Send(reply)
}

// handleIslandRestaurantClose answers IslandCloseRestaurantCommand
// (CS_21420): a restaurant whose opening is over (IslandRestaurant
// STATUS.CLOSE) pays what it sold and gives back the rest (drop_list), its
// sales count for its rank (one rank at a time, IslandRestaurant
// CheckUpgrade), and it is ready again; event_id and event_add are the
// day's event and what it added to the takings, when it was this
// restaurant's.
func (game *Game) handleIslandRestaurantClose(client *connection.Client, req *protobuf.CS_21420) error {
	reply := &protobuf.SC_21421{Result: result(errRefused)}
	err := game.tx(context.Background(), client, func(tx *Tx) error {
		now := serverNow()
		state, err := tx.ownIslandOrders(now)
		if err != nil {
			return err
		}
		id := req.GetTradeId()
		trade := state.restaurant(id)
		if trade == nil || trade.GetEndTime() == 0 || trade.GetEndTime() > uint32(now.Unix())+trade.GetSpeedTime() {
			return nil
		}
		money, sold, bonus := uint32(0), uint32(0), uint32(0)
		for _, food := range trade.GetSellList() {
			money += food.GetSellMoney()
			sold += food.GetNum()
			bonus += dishPrice(food.GetFoodId(), food.GetNum()) * state.dishBonus(id, food.GetFoodId()) / 100
		}
		if money > 0 {
			reply.DropList = append(reply.DropList, &protobuf.DROPINFO{Type: proto.Uint32(dropTypeIslandItem), Id: proto.Uint32(islandItemFunds), Number: proto.Uint32(money)})
		}
		for _, food := range trade.GetRestList() {
			reply.DropList = append(reply.DropList, &protobuf.DROPINFO{Type: proto.Uint32(dropTypeIslandItem), Id: proto.Uint32(food.GetFoodId()), Number: proto.Uint32(food.GetNum())})
		}
		if err := tx.grantDrops(reply.DropList); err != nil {
			return err
		}
		for _, food := range trade.GetSellList() {
			if err := tx.islandTaskEvent(islandTargetRestaurantSell, food.GetNum(), food.GetFoodId()); err != nil {
				return err
			}
		}
		if err := tx.islandTaskEvent(islandTargetRestaurantSales, money, id); err != nil {
			return err
		}
		if err := tx.islandAchievementEvent(islandRecordRestaurantSell, 0, sold); err != nil {
			return err
		}
		trade.TotalSell = proto.Uint32(trade.GetTotalSell() + money)
		for _, threshold := range islandManageRanks[trade.GetLv()].LevelUpExp {
			if len(threshold) >= 2 && threshold[0] == id && threshold[1] != 0 && trade.GetTotalSell() >= threshold[1] {
				if _, ok := islandManageRanks[trade.GetLv()+1]; ok {
					trade.Lv = proto.Uint32(trade.GetLv() + 1)
				}
			}
		}
		trade.SellList, trade.RestList = nil, nil
		for _, post := range trade.PostList {
			post.ShipId = proto.Uint32(0)
		}
		trade.EndTime = proto.Uint32(0)
		trade.SpeedTime = proto.Uint32(0)
		trades := state.public().TradeSys
		trades.PresellList = slices.DeleteFunc(trades.PresellList, func(entry *protobuf.PB_TRADE_PRESELL) bool { return entry.GetTradeId() == id })
		reply.Result = proto.Uint32(0)
		reply.EventId = proto.Uint32(0)
		reply.EventAdd = proto.Uint32(bonus)
		if trades.GetTodayTrade() == id {
			reply.EventId = proto.Uint32(trades.GetTodayEvent())
		}
		return nil
	})
	if err != nil {
		return err
	}
	return client.Send(reply)
}
