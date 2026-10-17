package game

import (
	"maps"
	"math/rand/v2"
	"slices"
	"time"

	"google.golang.org/protobuf/proto"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/gamedata"
	"github.com/ggmolly/belfast/internal/protobuf"
)

// handleChargeList answers GetChargeListCommand: nothing bought, so every
// first-purchase bonus is still available.
func (game *Game) handleChargeList(client *connection.Client, req *protobuf.CS_16104) error {
	return client.Send(&protobuf.SC_16105{})
}

// honormedal_goods_list row
type medalGoodsConfig struct {
	Group         uint32 `json:"group"`
	Order         uint32 `json:"order"`
	PurchaseLimit uint32 `json:"goods_purchase_limit"`
}

var medalGoods map[uint32]medalGoodsConfig

// medalShopGroups are the honormedal_goods_list groups the medal exchange
// stocks, group descending (the official order), each with its rows by id.
// The table doesn't say which rows are on sale: its order is the slot
// MedalShop sorts the goods by (model/vo/shop/medalshop.lua), and newer rows
// reuse the slot of the ones they replace (each General Blueprint series
// takes slots 6 and 7). The official shop stocks, for every slot, its newest
// group (highest group id), which is exactly the captured shop: the series 8
// blueprints but no older series, and one of the SR ship boxes of groups
// 20003 and 20004.
var medalShopGroups [][]uint32

func loadMedalShop() error {
	goods, err := gamedata.Table[medalGoodsConfig]("honormedal_goods_list")
	if err != nil {
		return err
	}
	newest := map[uint32]uint32{}
	for _, row := range goods {
		newest[row.Order] = max(newest[row.Order], row.Group)
	}
	rows := map[uint32][]uint32{}
	for id, row := range goods {
		if newest[row.Order] == row.Group {
			rows[row.Group] = append(rows[row.Group], id)
		}
	}
	groups := slices.Sorted(maps.Keys(rows))
	medalGoods, medalShopGroups = goods, nil
	for _, group := range slices.Backward(groups) {
		slices.Sort(rows[group])
		medalShopGroups = append(medalShopGroups, rows[group])
	}
	return nil
}

// medalShopGoods draws a commander's medal exchange for the month ending at
// next: one row of each stocked group, uniformly (the table has no weights;
// the captured shop is one draw for groups 20003 and 20004). The draw is
// seeded by both, so the shop stays the same until it refreshes without
// being stored.
func medalShopGoods(commanderID uint32, next time.Time) []uint32 {
	random := rand.New(rand.NewPCG(uint64(commanderID), uint64(next.Unix())))
	goods := make([]uint32, 0, len(medalShopGroups))
	for _, rows := range medalShopGroups {
		goods = append(goods, rows[random.IntN(len(rows))])
	}
	return goods
}

// handleMedalShop answers GetMedalShopCommand: this month's medal exchange,
// every purchase left (count is what can still be bought); it refreshes with
// the month.
func (game *Game) handleMedalShop(client *connection.Client, req *protobuf.CS_16106) error {
	next := nextMonth(time.Now())
	reply := &protobuf.SC_16107{
		Result:        result(nil),
		ItemFlashTime: proto.Uint32(uint32(next.Unix())),
	}
	for _, id := range medalShopGoods(client.CommanderID, next) {
		reply.GoodList = append(reply.GoodList, &protobuf.GOODS_INFO_P16{Id: proto.Uint32(id), Count: proto.Uint32(medalGoods[id].PurchaseLimit)})
	}
	return client.Send(reply)
}

// shopStreetGoods is the general shop's stock (shop_template ids of genre
// shopping_street and their discount): navalacademy_shoppingstreet_template
// level 1 sells 10 goods. The server's draw isn't in the client, so this is
// the captured stock.
var shopStreetGoods = [][2]uint32{
	{31132, 90}, {31022, 100}, {31112, 100}, {31111, 80}, {31142, 100},
	{31122, 100}, {24002, 90}, {31011, 100}, {21029, 100}, {31012, 100},
}

// handleShopStreet answers GetShopStreetCommand, the general shop: level 1,
// every item in stock once, no manual refresh. It refreshes at the same hours
// as the exercise attempts (the captured refresh was the 12:00 one).
func (game *Game) handleShopStreet(client *connection.Client, req *protobuf.CS_22101) error {
	street := &protobuf.SHOPPINGSTREET{
		Lv:            proto.Uint32(1),
		NextFlashTime: proto.Uint32(uint32(nextExerciseReset(time.Now()).Unix())),
	}
	for _, goods := range shopStreetGoods {
		street.GoodsList = append(street.GoodsList, &protobuf.STREETGOODS{
			GoodsId: proto.Uint32(goods[0]), Discount: proto.Uint32(goods[1]), BuyCount: proto.Uint32(1),
		})
	}
	return client.Send(&protobuf.SC_22102{Street: street})
}

// handleMiniGameShop answers GetMiniGameShopCommand, the game room shop
// (gameroom_shop_template): goods lists the purchases, none yet; it
// refreshes with the month.
func (game *Game) handleMiniGameShop(client *connection.Client, req *protobuf.CS_26150) error {
	return client.Send(&protobuf.SC_26151{
		NextFlashTime: proto.Uint32(uint32(nextMonth(time.Now()).Unix())),
	})
}
