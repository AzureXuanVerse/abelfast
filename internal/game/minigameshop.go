package game

import (
	"context"
	"maps"
	"slices"
	"time"

	"google.golang.org/protobuf/proto"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/db"
	"github.com/ggmolly/belfast/internal/logger"
	"github.com/ggmolly/belfast/internal/protobuf"
)

// miniGameGoodsSelectable is the gameroom_shop_template goods_type whose
// purchases each give one of goods, picked by the player
// (MiniGameGoods:Selectable).
const miniGameGoodsSelectable = 2

// miniGameShopPurchases is how many of each gameroom_shop_template goods
// the commander bought that still count against its limit at now: every
// purchase, or this month's for the goods with month_re.
func miniGameShopPurchases(ctx context.Context, queries *db.Queries, commanderID uint32, now time.Time) (map[uint32]uint32, error) {
	rows, err := queries.ListMiniGameShopPurchases(ctx, commanderID)
	if err != nil {
		return nil, err
	}
	month := startOfMonth(now).Unix()
	bought := make(map[uint32]uint32, len(rows))
	for _, row := range rows {
		if miniGameGoods[row.GoodsID].MonthRe != 0 && row.MonthStart != month {
			continue
		}
		bought[row.GoodsID] += row.Count
	}
	return bought, nil
}

// handleMiniGameShop answers GetMiniGameShopCommand
// (controller/command/shop/getminigameshopcommand.lua), the Arcade's Prize
// Shop (MiniGameShop): the client lists every gameroom_shop_template row,
// goods is how many of each were bought (MiniGameGoods:UpdateCnt takes them
// off goods_purchase_limit), next_flash_time when the shop refreshes: the
// next month, as captured.
func (game *Game) handleMiniGameShop(client *connection.Client, req *protobuf.CS_26150) error {
	now := time.Now()
	bought, err := miniGameShopPurchases(context.Background(), game.DB.Queries, client.CommanderID, now)
	if err != nil {
		return err
	}
	reply := &protobuf.SC_26151{NextFlashTime: proto.Uint32(uint32(nextMonth(now).Unix()))}
	for _, id := range slices.Sorted(maps.Keys(bought)) {
		reply.Goods = append(reply.Goods, &protobuf.GOODS_INFO_P26{Id: proto.Uint32(id), Count: proto.Uint32(bought[id])})
	}
	return client.Send(reply)
}

// handleMiniGameShopFlush answers MiniGameShopFlushCommand and
// FlushMiniGameShopCommand (controller/command/shop/), sent once the shop's
// next_flash_time passed (ZeroHourCommand, ShopsProxy's day call, opening
// the shop) or when the day changes with the shop open: on result 0 the
// client takes next_flash_time[1] as the next refresh and forgets the
// purchases of the goods with month_re. Purchases are kept by month, so
// nothing is stored. No official capture has it: the reply is built from
// the Lua alone.
func (game *Game) handleMiniGameShopFlush(client *connection.Client, req *protobuf.CS_26154) error {
	return client.Send(&protobuf.SC_26155{
		Result:        result(nil),
		NextFlashTime: []uint32{uint32(nextMonth(time.Now()).Unix())},
	})
}

// handleMiniGameShopBuy answers MiniGameShopBuyCommand
// (controller/command/shop/minigameshopbuycommand.lua): goodsid is the
// gameroom_shop_template id, selected the drop ids with how many of each
// (goods[1] with the count picked, or each of goods once for the goods
// sold one at a time, MiniGameShopPage:OnClickCommodity). On result 0 the
// client takes price times the purchases off its Game Tickets, counts them
// against the goods' limit and adds drop_list. The client checks the
// tickets and the count left; so does the server, along with the goods'
// time window (the client lists every row, whatever its time). No official
// capture has it: the reply is built from the Lua alone.
func (game *Game) handleMiniGameShopBuy(client *connection.Client, req *protobuf.CS_26152) error {
	var drops []*protobuf.DROPINFO
	err := game.tx(context.Background(), client, func(tx *Tx) (err error) {
		drops, err = tx.buyMiniGameGoods(req.GetGoodsid(), req.GetSelected(), time.Now())
		return err
	})
	if err != nil && !refused(err) {
		return err
	}
	if err != nil {
		logger.WithFields("MiniGameShop", logger.FieldValue("id", req.GetGoodsid())).Warn("purchase refused")
		drops = nil
	}
	return client.Send(&protobuf.SC_26153{Result: result(err), DropList: drops})
}

// buyMiniGameGoods buys the selected drops of Prize Shop goods: checks they
// are on sale with enough purchases left, spends the tickets, gives the
// drops and counts the purchases. It returns the drops given.
func (tx *Tx) buyMiniGameGoods(id uint32, selected []*protobuf.SELECT_INFO, now time.Time) ([]*protobuf.DROPINFO, error) {
	goods, ok := miniGameGoods[id]
	if !ok || !goods.Time.Open(now) || len(selected) == 0 {
		return nil, errRefused
	}
	choices := goods.Goods
	if goods.GoodsType != miniGameGoodsSelectable && goods.PurchaseLimit > 1 {
		choices = choices[:min(len(choices), 1)]
	}
	total := uint32(0)
	var drops []*protobuf.DROPINFO
	for _, pick := range selected {
		if pick.GetNum() == 0 || pick.GetNum() > goods.PurchaseLimit || !slices.Contains(choices, pick.GetId()) {
			return nil, errRefused
		}
		if total += pick.GetNum(); total > goods.PurchaseLimit {
			return nil, errRefused
		}
		drops = mergeDrop(drops, goods.DropType, pick.GetId(), goods.Num*pick.GetNum())
	}
	bought, err := miniGameShopPurchases(tx.ctx, tx.Queries, tx.commanderID, now)
	if err != nil {
		return nil, err
	}
	// MiniGameGoods:CanPurchaseCnt
	if total > goods.PurchaseLimit-min(bought[id], goods.PurchaseLimit) {
		return nil, errRefused
	}
	if err := tx.SpendResource(resourceGameTicket, goods.Price*total); err != nil {
		return nil, err
	}
	if err := tx.grantDrops(drops); err != nil {
		return nil, err
	}
	err = tx.AddMiniGameShopPurchase(tx.ctx, db.AddMiniGameShopPurchaseParams{
		CommanderID: tx.commanderID, GoodsID: id, MonthStart: startOfMonth(now).Unix(), Count: total,
	})
	return drops, err
}
