package game

import (
	"context"
	"encoding/json"
	"slices"
	"time"

	"google.golang.org/protobuf/proto"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/gamedata"
	"github.com/ggmolly/belfast/internal/packets"
	"github.com/ggmolly/belfast/internal/protobuf"
)

// The island's shops (IslandShopAgency, IslandShopp, IslandCommodity,
// IslandShopOpCommand): island_shop_template lists them, the ones with
// shop_type 1 (normal) have their opening time, unlocking abilities and
// refresh rules in island_shop_normal_template and sell island_shop_goods.
// Each shop the commander may see is kept in private.shop_list with how
// many of each goods it bought (PB_SHOP goods_list).

// island_shop_template.shop_type of a normal shop
const islandShopNormal = 1

// IslandAchievementType.SHOP_COST_ITEM / SHOP_BUY_ITEM
const (
	islandAchievementShopCost = 17
	islandAchievementShopBuy  = 18
)

// islandShopConfig is an island_shop_template row.
type islandShopConfig struct {
	ShopType uint32          `json:"shop_type"`
	Goods    json.RawMessage `json:"goods_id"`
}

// islandShopNormalConfig is an island_shop_normal_template row.
type islandShopNormalConfig struct {
	ExistTime     gamedata.Window `json:"exist_time"`
	Unlock        json.RawMessage `json:"unlock"`
	RefreshSet    uint32          `json:"refresh_set"`
	RefreshFree   uint32          `json:"refresh_free"`
	RefreshPlayer json.RawMessage `json:"refresh_player"`
}

// islandGoodsConfig is an island_shop_goods row.
type islandGoodsConfig struct {
	Items        [][]uint32      `json:"items"`
	Cost         []uint32        `json:"resource_consume"`
	Discount     uint32          `json:"discount"`
	DiscountTime gamedata.Window `json:"discount_time"`
	Limit        uint32          `json:"limited_num"`
	PayID        uint32          `json:"pay_id"`
	PtAward      uint32          `json:"pt_award"`
	Time         gamedata.Window `json:"time"`
	Unlock       json.RawMessage `json:"unlock"`
}

var (
	islandShops       map[uint32]islandShopConfig
	islandNormalShops map[uint32]islandShopNormalConfig
	islandGoods       map[uint32]islandGoodsConfig
)

func loadIslandShopData() error {
	return gamedata.LoadAll(
		gamedata.Load("island_shop_template", &islandShops),
		gamedata.Load("island_shop_normal_template", &islandNormalShops),
		gamedata.Load("island_shop_goods", &islandGoods),
	)
}

func (game *Game) registerIslandShops() {
	packets.Handle(game.handleIslandShop)
	packets.Handle(game.handleIslandShopBuy)
	packets.Handle(game.handleIslandShopRefresh)
}

// hasIslandAbilities tells whether the island has every ability of an
// unlock field (empty: none needed).
func (state *islandState) hasIslandAbilities(unlock json.RawMessage) bool {
	for _, ability := range gamedata.IDs(unlock) {
		if !slices.Contains(state.public().GetAbilityList(), ability) {
			return false
		}
	}
	return true
}

// islandShopOpen tells whether the commander may see a shop now: a normal
// shop within its exist_time whose unlock abilities the island has
// (IslandShopp:IsInTime, IslandDropHelper.HandleIslandShopAbility). GUESS:
// the temporary shops (shop_type 2, random goods) are never open; no
// island_shop_template row uses them.
func (state *islandState) islandShopOpen(id uint32, now time.Time) bool {
	shop, ok := islandShops[id]
	normal, okNormal := islandNormalShops[id]
	return ok && okNormal && shop.ShopType == islandShopNormal && normal.ExistTime.Open(now) && state.hasIslandAbilities(normal.Unlock)
}

// islandShop is the commander's stored shop, added when missing.
func (state *islandState) islandShop(id uint32) *protobuf.PB_SHOP {
	private := state.private()
	for _, shop := range private.ShopList {
		if shop.GetId() == id {
			return shop
		}
	}
	shop := &protobuf.PB_SHOP{
		Id: proto.Uint32(id), ExistTime: proto.Uint32(0), RefreshTime: proto.Uint32(0), RefreshCount: proto.Uint32(0),
	}
	private.ShopList = append(private.ShopList, shop)
	return shop
}

// dropIslandShop forgets a shop the commander may no longer see.
func (state *islandState) dropIslandShop(id uint32) {
	private := state.private()
	private.ShopList = slices.DeleteFunc(private.ShopList, func(shop *protobuf.PB_SHOP) bool { return shop.GetId() == id })
}

// handleIslandShop answers IslandShopOpCommand's SHOP_GET_DATA (CS_21016,
// for each shop IslandShopAgency:GetNewOrOverdueShopIds names when the shop
// page opens, the shown one when it is shown, a shop an ability unlocked):
// the shop with what was bought in it, or a non-zero result for a shop the
// commander can't see (the client then drops it, without a tip). GUESS:
// exist_time and refresh_time are 0 (the client reads a normal shop's time
// from its config).
func (game *Game) handleIslandShop(client *connection.Client, req *protobuf.CS_21016) error {
	reply := &protobuf.SC_21017{}
	err := game.tx(context.Background(), client, func(tx *Tx) error {
		state, err := tx.island()
		if err != nil {
			return err
		}
		if !state.islandShopOpen(req.GetShopId(), time.Now()) {
			state.dropIslandShop(req.GetShopId())
			return nil
		}
		reply.ShopInfo = state.islandShop(req.GetShopId())
		return nil
	})
	if err != nil {
		return err
	}
	if reply.ShopInfo == nil {
		reply.Result = result(errRefused)
	} else {
		reply.Result = proto.Uint32(0)
	}
	return client.Send(reply)
}

// handleIslandShopBuy answers IslandShopOpCommand's SHOP_BUY_COMMODITY
// (CS_21018, the shopping cart: key the shop, value1 the goods, value2 how
// many). Each goods must be on sale in an open shop (its time, unlock
// abilities and limited_num); the price is its resource_consume times the
// count less the running discount, rounded up (as the client computes it),
// paid in island items or player resources, which the client takes itself.
// drop_list is the goods' items, which the client adds; it adds the goods'
// pt_award in season points itself too, so they are stored but not in
// drop_list. Island items can't be bought while some wait on the overflow
// list (island_3Dshop_bag_max). Paid goods (pay_id) are refused. Counts the
// shop achievements (Development Funds spent, goods bought).
func (game *Game) handleIslandShopBuy(client *connection.Client, req *protobuf.CS_21018) error {
	reply := &protobuf.SC_21019{}
	now := time.Now()
	err := game.tx(context.Background(), client, func(tx *Tx) error {
		state, err := tx.island()
		if err != nil {
			return err
		}
		var costs [][]uint32
		var drops []*protobuf.DROPINFO
		points := uint32(0)
		for _, entry := range req.GetGoodsList() {
			shopID, goodsID, count := entry.GetKey(), entry.GetValue1(), entry.GetValue2()
			goods, ok := islandGoods[goodsID]
			if !ok || count == 0 || goods.PayID != 0 || len(goods.Cost) < 3 || !state.islandShopOpen(shopID, now) ||
				!slices.Contains(gamedata.IDs(islandShops[shopID].Goods), goodsID) || !goods.Time.Open(now) || !state.hasIslandAbilities(goods.Unlock) {
				return errRefused
			}
			shop := state.islandShop(shopID)
			var bought *protobuf.PB_GOODS
			for _, record := range shop.GoodsList {
				if record.GetId() == goodsID {
					bought = record
				}
			}
			if bought == nil {
				bought = &protobuf.PB_GOODS{Id: proto.Uint32(goodsID), Num: proto.Uint32(0)}
				shop.GoodsList = append(shop.GoodsList, bought)
			}
			if goods.Limit != 0 && bought.GetNum()+count > goods.Limit {
				return errRefused
			}
			bought.Num = proto.Uint32(bought.GetNum() + count)
			discount := uint32(0)
			if goods.DiscountTime.Open(now) {
				discount = min(goods.Discount, 100)
			}
			price := ((100-discount)*goods.Cost[2]*count + 99) / 100
			merged := false
			for _, cost := range costs {
				if cost[0] == goods.Cost[0] && cost[1] == goods.Cost[1] {
					cost[2] += price
					merged = true
				}
			}
			if !merged {
				costs = append(costs, []uint32{goods.Cost[0], goods.Cost[1], price})
			}
			for _, item := range islandItemDrops(goods.Items, count) {
				drops = addDrop(drops, item.GetType(), item.GetId(), item.GetNumber())
			}
			points += goods.PtAward * count
			if err := tx.islandAchievementEvent(islandAchievementShopBuy, goodsID, count); err != nil {
				return err
			}
		}
		if len(costs) == 0 {
			return errRefused
		}
		if len(state.private().GetItemListCache()) > 0 && slices.ContainsFunc(drops, func(drop *protobuf.DROPINFO) bool { return drop.GetType() == dropTypeIslandItem }) {
			return errRefused
		}
		for _, cost := range costs {
			switch cost[0] {
			case dropTypeIslandItem:
				err = state.takeIslandItem(cost[1], cost[2])
				if err == nil {
					err = tx.islandAchievementEvent(islandAchievementShopCost, cost[1], cost[2])
				}
			case dropTypeResource:
				err = tx.SpendShopResource(cost[1], cost[2])
			default:
				err = tx.payDrops([][]uint32{cost})
			}
			if err != nil {
				return err
			}
		}
		reply.DropList = drops
		if err := tx.grantDrops(drops); err != nil {
			return err
		}
		if points == 0 {
			return nil
		}
		return tx.grantDrops([]*protobuf.DROPINFO{{
			Type: proto.Uint32(dropTypeIslandSeasonPt), Id: proto.Uint32(0), Number: proto.Uint32(points),
		}})
	})
	if reply.Result, err = islandReply(err); err != nil {
		return err
	}
	if reply.GetResult() != 0 {
		reply.DropList = nil
	}
	return client.Send(reply)
}

// handleIslandShopRefresh answers IslandShopOpCommand's
// SHOP_REFRESH_BY_PLAYER (CS_21020): a normal shop refreshed by hand, up to
// refresh_set times, for refresh_player (the first one free with
// refresh_free), which the client takes itself; the reply is the shop.
// No island_shop_normal_template row allows it today (refresh_set 0), so it
// is refused. GUESS: a refresh forgets what was bought.
func (game *Game) handleIslandShopRefresh(client *connection.Client, req *protobuf.CS_21020) error {
	reply := &protobuf.SC_21021{}
	err := game.tx(context.Background(), client, func(tx *Tx) error {
		state, err := tx.island()
		if err != nil {
			return err
		}
		normal := islandNormalShops[req.GetShopId()]
		if !state.islandShopOpen(req.GetShopId(), time.Now()) {
			return errRefused
		}
		shop := state.islandShop(req.GetShopId())
		if shop.GetRefreshCount() >= normal.RefreshSet {
			return errRefused
		}
		if normal.RefreshFree != 1 || shop.GetRefreshCount() > 0 {
			var cost []uint32
			if json.Unmarshal(normal.RefreshPlayer, &cost) == nil && len(cost) >= 3 && cost[2] > 0 {
				if err := tx.payIslandCosts(state, [][]uint32{cost}); err != nil {
					return err
				}
			}
		}
		shop.RefreshCount = proto.Uint32(shop.GetRefreshCount() + 1)
		shop.GoodsList = nil
		reply.ShopInfo = shop
		return nil
	})
	if reply.Result, err = islandReply(err); err != nil {
		return err
	}
	if reply.GetResult() != 0 {
		reply.ShopInfo = nil
	}
	return client.Send(reply)
}
