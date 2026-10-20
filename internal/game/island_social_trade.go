package game

import (
	"context"
	"encoding/binary"
	"errors"
	"hash/fnv"
	"slices"
	"time"

	"google.golang.org/protobuf/proto"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/db"
	"github.com/ggmolly/belfast/internal/protobuf"
)

// The pearl market (IslandTradegency, PB_ISLAND_PUBLIC.treasure): each
// island has a pearl selling price for the market day (03:00 to 03:00,
// server time); pearls (island item 9900) are sold there for development
// funds (item 1) at that price, and bought at treasure_price_buy % of it.
// A commander buys treasure_week_limit[1] pearls a week at most and sells
// treasure_week_limit[2] a day on each other island (its own sell_list),
// none on its own.

// IslandConst.TRADE_*: CS_21240.type.
const (
	islandTradeBuy  = 1
	islandTradeSell = 2
)

// IslandItem.GOLD_ID and PEARL_ID.
const (
	islandItemGold  = 1
	islandItemPearl = 9900
)

// The pearl trade's unlock (handleIslandTrade).
const (
	islandTradeUnlockLevel = 22
	islandTradeUnlockTask  = 20017001
	// IslandTaskConditionType.FINISH_TASK
	islandConditionFinishTask = 2
)

const (
	islandTradeDayStart = 3 * time.Hour
	// the price trend shows the 6 days before today (CollectDate)
	islandTradeHistory = 7
)

// islandTradeDay is when the market day running at now began.
func islandTradeDay(now time.Time) time.Time {
	return periodStart(now.Add(-islandTradeDayStart), day).Add(islandTradeDayStart)
}

// islandTradePrice is an island's pearl selling price for the market day
// starting at start. GUESS: uniform in treasure_price_initial, kept under
// treasure_price_limit; the tables' other treasure_price_* parameters
// (a random walk, weighted bands) aren't understood. It is drawn from the
// island and the day, so every reader of the island agrees on it before its
// owner's next save.
func islandTradePrice(commanderID uint32, start time.Time) uint32 {
	low, high := islandSocialConfig.tradeInitial[0], islandSocialConfig.tradeInitial[1]
	if high <= low {
		return low
	}
	hash := fnv.New64a()
	var key [12]byte
	binary.LittleEndian.PutUint32(key[:4], commanderID)
	binary.LittleEndian.PutUint64(key[4:], uint64(start.Unix()))
	hash.Write(key[:])
	price := low + uint32(hash.Sum64()%uint64(high-low+1))
	if limit := islandSocialConfig.tradePriceCap; limit[1] > 0 {
		price = min(max(price, limit[0]), limit[1])
	}
	return price
}

// refreshIslandTrade starts a new market day (the day's price, for
// IslandTradegency.UpdateTodayPrice: the day's sales and invitations
// forgotten) and a new market week (Monday 03:00: the pearls bought) when
// they began.
func refreshIslandTrade(state *islandState, now time.Time) {
	treasure := state.public().Treasure
	if treasure == nil {
		return
	}
	start := islandTradeDay(now)
	end := uint32(start.Add(day).Unix())
	if latest := islandTradeLatest(treasure); latest == nil || latest.GetTimestamp() < end {
		treasure.PriceList = append(treasure.PriceList, &protobuf.PB_TRE_HISTORY_PRICE{
			Timestamp: proto.Uint32(end), Price: proto.Uint32(islandTradePrice(state.commanderID, start)),
		})
		if extra := len(treasure.PriceList) - islandTradeHistory; extra > 0 {
			treasure.PriceList = slices.Delete(treasure.PriceList, 0, extra)
		}
		treasure.SellList = nil
		treasure.InviteList = nil
	}
	social, err := state.social()
	if err != nil {
		return
	}
	week := periodStart(now.Add(-islandTradeDayStart), week).Add(islandTradeDayStart).Unix()
	if social.TradeWeek < week {
		social.TradeWeek = week
		treasure.WeekBuyNum = proto.Uint32(0)
		_ = state.setSocial(social)
	}
}

// islandTradeLatest is the island's latest price, nil when none.
func islandTradeLatest(treasure *protobuf.PB_ISLAND_TREASURE) *protobuf.PB_TRE_HISTORY_PRICE {
	var latest *protobuf.PB_TRE_HISTORY_PRICE
	for _, price := range treasure.GetPriceList() {
		if latest == nil || price.GetTimestamp() > latest.GetTimestamp() {
			latest = price
		}
	}
	return latest
}

// handleIslandTrade answers IslandTradeCommand (CS_21240): buy (type 1) or
// sell (type 2) num pearls at the price of the island the commander is on.
// The client takes the funds (num times the purchase price) or the pearls
// itself; the reply drops the pearls or the funds. Trading needs the
// commander's island at Development Lv. 22 and the "Research Into the Pearl
// Trade" task done (wiki "Recent News", Season II notes; the client's
// island_trade_activity_unlock tip says 22 too, island_set treasure_unlock's
// 20 is not what players are told). That task is a chain of three
// (island_task 20017001-20017003); the first one, after which Peary trades,
// opens it.
func (game *Game) handleIslandTrade(client *connection.Client, req *protobuf.CS_21240) error {
	reply := &protobuf.SC_21241{}
	islandID, num := req.GetIslandId(), req.GetNum()
	err := game.tx(context.Background(), client, func(tx *Tx) error {
		current, here := game.islandOf(client)
		if !here || current != islandID || num == 0 {
			reply.Result = result(errRefused)
			return nil
		}
		own, err := tx.island()
		if err != nil {
			return err
		}
		market, err := tx.otherIsland(islandID)
		if err != nil {
			return err
		}
		if market == nil || own.public().GetLevel() < islandTradeUnlockLevel || !islandConditionMet(own, []uint32{islandConditionFinishTask, islandTradeUnlockTask}) {
			reply.Result = result(errRefused)
			return nil
		}
		latest := islandTradeLatest(market.public().Treasure)
		if latest == nil || latest.GetTimestamp() <= uint32(serverNow().Unix()) {
			reply.Result = result(errRefused)
			return nil
		}
		treasure := own.public().Treasure
		var drop *protobuf.DROPINFO
		switch req.GetType() {
		case islandTradeBuy:
			price := latest.GetPrice() * islandSocialConfig.tradeBuy / 100
			if treasure.GetWeekBuyNum()+num > islandSocialConfig.tradeLimit[0] {
				reply.Result = result(errRefused)
				return nil
			}
			if err := own.takeIslandItem(islandItemGold, price*num); err != nil {
				reply.Result = result(err)
				return nil
			}
			treasure.WeekBuyNum = proto.Uint32(treasure.GetWeekBuyNum() + num)
			drop = &protobuf.DROPINFO{Type: proto.Uint32(dropTypeIslandItem), Id: proto.Uint32(islandItemPearl), Number: proto.Uint32(num)}
		case islandTradeSell:
			if islandID != client.CommanderID {
				var sold *protobuf.PB_TRE_SELL_LIST
				for _, entry := range treasure.SellList {
					if entry.GetIslandId() == islandID {
						sold = entry
					}
				}
				if sold == nil {
					sold = &protobuf.PB_TRE_SELL_LIST{IslandId: proto.Uint32(islandID), Num: proto.Uint32(0)}
					treasure.SellList = append(treasure.SellList, sold)
				}
				if sold.GetNum()+num > islandSocialConfig.tradeLimit[1] {
					reply.Result = result(errRefused)
					return errIslandUndo
				}
				sold.Num = proto.Uint32(sold.GetNum() + num)
			}
			if err := own.takeIslandItem(islandItemPearl, num); err != nil {
				reply.Result = result(err)
				return errIslandUndo
			}
			drop = &protobuf.DROPINFO{Type: proto.Uint32(dropTypeIslandItem), Id: proto.Uint32(islandItemGold), Number: proto.Uint32(num * latest.GetPrice())}
		default:
			reply.Result = result(errRefused)
			return nil
		}
		drops := []*protobuf.DROPINFO{drop}
		if err := tx.grantDrops(drops); err != nil {
			return err
		}
		reply.Result = result(nil)
		reply.DropList = drops
		return nil
	})
	if err != nil && !errors.Is(err, errIslandUndo) {
		return err
	}
	return client.Send(islandStamped(reply))
}

// handleIslandTradePrice answers IslandGetFriendTradeRankCommand (CS_21243,
// one per friend): the island's latest price (until when, and the selling
// price: the client counts it 0 once past) and its level, for the friends'
// price ranking. An island nobody has yet has neither.
func (game *Game) handleIslandTradePrice(client *connection.Client, req *protobuf.CS_21243) error {
	reply := &protobuf.SC_21244{
		TodayPrice: &protobuf.PB_TRE_HISTORY_PRICE{Timestamp: proto.Uint32(0), Price: proto.Uint32(0)},
		IslandLv:   proto.Uint32(0),
	}
	err := game.tx(context.Background(), client, func(tx *Tx) error {
		state, err := tx.otherIsland(req.GetIslandId())
		if err != nil || state == nil {
			return err
		}
		if latest := islandTradeLatest(state.public().Treasure); latest != nil {
			reply.TodayPrice = latest
		}
		reply.IslandLv = proto.Uint32(state.public().GetLevel())
		return nil
	})
	if err != nil {
		return err
	}
	return client.Send(islandStamped(reply))
}

// handleIslandTradeInvite answers IslandInviteTradeCommand (CS_21245):
// friends invited to sell their pearls on the commander's island
// (treasure.invite_list, until the next market day). Each one online is
// told (SC_21247: "invitation from $1, their selling price is $3", with
// the scene it names). GUESS: only friends, as the invite page lists them,
// and the price told is the island's own, not the one the client sent.
func (game *Game) handleIslandTradeInvite(client *connection.Client, req *protobuf.CS_21245) error {
	reply := &protobuf.SC_21246{}
	var notice *protobuf.SC_21247
	var invited []uint32
	err := game.tx(context.Background(), client, func(tx *Tx) error {
		if _, ok := islandMaps[req.GetMapId()]; !ok || len(req.GetFriendList()) == 0 {
			reply.Result = result(errRefused)
			return nil
		}
		state, err := tx.island()
		if err != nil {
			return err
		}
		treasure := state.public().Treasure
		latest := islandTradeLatest(treasure)
		if latest == nil {
			reply.Result = result(errRefused)
			return nil
		}
		for _, id := range req.GetFriendList() {
			friends, err := tx.CountFriend(tx.ctx, db.CountFriendParams{CommanderID: client.CommanderID, FriendID: id})
			if err != nil {
				return err
			}
			if friends == 0 {
				reply.Result = result(errRefused)
				return errIslandUndo
			}
			if !slices.Contains(treasure.InviteList, id) {
				treasure.InviteList = append(treasure.InviteList, id)
			}
			invited = append(invited, id)
		}
		notice = &protobuf.SC_21247{
			IslandId: proto.Uint32(client.CommanderID), MapId: proto.Uint32(req.GetMapId()), Price: proto.Uint32(latest.GetPrice()),
		}
		reply.Result = result(nil)
		return nil
	})
	if errors.Is(err, errIslandUndo) {
		return client.Send(islandStamped(reply))
	}
	if err != nil {
		return err
	}
	for _, id := range invited {
		game.deliver(client, id, notice)
	}
	return client.Send(islandStamped(reply))
}
