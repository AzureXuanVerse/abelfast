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
// server among honormedal_goods_list (the captured one): ids, in the
// official order, with their goods_purchase_limit.
var medalShopGoods = [][2]uint32{
	{45, 1}, {32, 1}, {28, 10}, {27, 2}, {55, 5}, {54, 15}, {53, 20}, {52, 20}, {51, 20}, {48, 10},
	{41, 20}, {26, 12}, {25, 24}, {22, 2}, {21, 2}, {20, 2}, {19, 6}, {18, 6}, {17, 6}, {16, 4},
	{15, 12}, {14, 24}, {13, 5}, {12, 4}, {11, 4}, {10, 4}, {9, 4}, {8, 4}, {7, 5}, {6, 5},
	{5, 5}, {4, 5}, {3, 5}, {2, 5}, {1, 30},
}

// handleMedalShop answers GetMedalShopCommand: the medal exchange, every
// purchase left (count is what can still be bought); it refreshes with the
// month.
func (game *Game) handleMedalShop(client *connection.Client, req *protobuf.CS_16106) error {
	reply := &protobuf.SC_16107{
		Result:        result(nil),
		ItemFlashTime: proto.Uint32(uint32(nextMonth(time.Now()).Unix())),
	}
	for _, goods := range medalShopGoods {
		reply.GoodList = append(reply.GoodList, &protobuf.GOODS_INFO_P16{Id: proto.Uint32(goods[0]), Count: proto.Uint32(goods[1])})
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
