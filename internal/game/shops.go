package game

import (
	"time"

	"google.golang.org/protobuf/proto"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/protobuf"
)

// handleChargeList answers GetChargeListCommand: nothing bought, so every
// first-purchase bonus is still available.
func (game *Game) handleChargeList(client *connection.Client, req *protobuf.CS_16104) error {
	return client.Send(&protobuf.SC_16105{})
}

// medalShopGoods is this month's medal exchange rotation, picked by the
// server among honormedal_goods_list (the captured one), in the official
// order.
var medalShopGoods = []uint32{
	45, 32, 28, 27, 55, 54, 53, 52, 51, 48, 41, 26, 25, 22, 21, 20, 19, 18,
	17, 16, 15, 14, 13, 12, 11, 10, 9, 8, 7, 6, 5, 4, 3, 2, 1,
}

// honormedal_goods_list row
type medalGoodsConfig struct {
	PurchaseLimit uint32 `json:"goods_purchase_limit"`
}

var medalGoods map[uint32]medalGoodsConfig

// handleMedalShop answers GetMedalShopCommand: the medal exchange, every
// purchase left (count is what can still be bought); it refreshes with the
// month.
func (game *Game) handleMedalShop(client *connection.Client, req *protobuf.CS_16106) error {
	reply := &protobuf.SC_16107{
		Result:        result(nil),
		ItemFlashTime: proto.Uint32(uint32(nextMonth(time.Now()).Unix())),
	}
	for _, id := range medalShopGoods {
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
