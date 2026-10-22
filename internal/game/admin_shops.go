package game

// The back-office's Shops screen: every shop's stock as the client shows it,
// and the admin edits of what was bought, the manual refreshes, the oil
// bought today, the requisitions and Akashi's affection, each checked
// against the bounds the client and the shop handlers keep.

import (
	"context"
	"database/sql"
	"errors"
	"maps"
	"slices"
	"sync"
	"time"

	"github.com/ggmolly/belfast/internal/db"
	"github.com/ggmolly/belfast/internal/gamedata"
)

// BoardGoods is a goods of a shop with the bought count an admin may set:
// 0 to Max, or only reset when Max is 0 (goods without a purchase limit,
// and the new commander shop goods sold once each, whose purchases are the
// sub-goods taken).
type BoardGoods struct {
	AdminGoods
	Max uint32
}

// BoardShop is a shop of the Shops screen: its stock, its manual refreshes
// (RefreshMax 0: it has none) and what the next one costs.
type BoardShop struct {
	AdminShop
	Goods        []BoardGoods
	RefreshMax   uint32
	RefreshPrice uint32 // gems (merit) or guild tokens (guild); 0 when none is left
}

// CatalogGoods is a shop_template goods not bought yet, in the shop Kind.
type CatalogGoods struct {
	BoardGoods
	Kind string
}

// AkashiLevel is a pay_level_award level and the affection exp it takes
// (Player:getChargeLevel).
type AkashiLevel struct {
	Level uint32 `json:"level"`
	Exp   uint32 `json:"exp"`
}

// ShopBoard is the commander's Shops screen.
type ShopBoard struct {
	Shops   []BoardShop
	GuildID uint32 // 0 outside a guild
	// the oil bought today (SC_11003.buy_oil_count), at most gameset buy_oil_limit
	OilBought, OilLimit uint32
	OilResetAt          int64
	// shop_template rows on sale with a purchase limit, that can be marked
	// bought in the packs and Akashi's shop
	Catalog []CatalogGoods
	// the resources and items the shops' prices are in: drop type -> id -> owned
	Wallet map[uint32]map[uint32]uint32
	// Akashi's levels and the exp past which taps add nothing to the level
	AkashiLevels []AkashiLevel
	AkashiExpMax uint32
}

// akashiLevels reads pay_level_award: level 1 from 0 exp, level id+1 from
// row id's exp, up to the last level (Player:getChargeLevel).
var akashiLevels = sync.OnceValues(func() ([]AkashiLevel, error) {
	rows, err := gamedata.Table[struct {
		Exp uint32 `json:"exp"`
	}]("pay_level_award")
	if err != nil {
		return nil, err
	}
	ids := slices.Sorted(maps.Keys(rows))
	levels := []AkashiLevel{{Level: 1}}
	for _, id := range ids[:max(len(ids)-1, 0)] {
		levels = append(levels, AkashiLevel{Level: id + 1, Exp: rows[id].Exp})
	}
	if len(ids) > 0 {
		levels = append(levels, AkashiLevel{Level: 0, Exp: rows[ids[len(ids)-1]].Exp})
	}
	return levels, nil
})

// AdminShopBoard is the commander's Shops screen at now.
func (game *Game) AdminShopBoard(ctx context.Context, commanderID uint32) (ShopBoard, error) {
	now := time.Now()
	q := game.DB.Queries
	board := ShopBoard{OilLimit: buyOilLimit, OilResetAt: nextDay(now).Unix(), Wallet: map[uint32]map[uint32]uint32{}}
	shops, err := game.AdminShops(ctx, commanderID)
	if err != nil {
		return board, err
	}
	purchases, err := shopPurchases(ctx, q, commanderID, now)
	if err != nil {
		return board, err
	}
	member, inGuild, err := guildMembership(ctx, q, commanderID)
	if err != nil {
		return board, err
	}
	if inGuild {
		board.GuildID = member.GuildID
	}
	for _, shop := range shops {
		row := BoardShop{AdminShop: shop}
		switch shop.Kind {
		case AdminShopOil:
			for _, goods := range shop.Goods {
				board.OilBought += goods.Bought
			}
			continue
		case AdminShopStreet:
			row.Goods = streetBoard(shop, purchases, now)
		case AdminShopArena:
			state, err := loadExercise(ctx, q, commanderID, now)
			if err != nil {
				return board, err
			}
			row.Refreshes, row.RefreshMax = state.ShopRefreshes, uint32(len(arenaShop.RefreshPrice))
			if int(state.ShopRefreshes) < len(arenaShop.RefreshPrice) {
				row.RefreshPrice = arenaShop.RefreshPrice[state.ShopRefreshes]
			}
			row.Goods = meritBoard(shop, arenaShopGoods(arenaShop, commanderID, nextDay(now), state.ShopRefreshes), purchases)
		case AdminShopGuild:
			row.RefreshMax = guildShopResets
			if g, err := guildBoard(ctx, q, commanderID, inGuild, now); err != nil {
				return board, err
			} else if g != nil {
				row.AdminShop = *g
			}
			if row.Refreshes < guildShopResets {
				row.RefreshPrice = guildShopResetCost
			}
			fallthrough
		default:
			for _, goods := range row.AdminShop.Goods {
				row.Goods = append(row.Goods, BoardGoods{AdminGoods: goods, Max: goodsMax(shop, goods)})
			}
		}
		row.AdminShop.Goods = nil
		if row.Goods == nil {
			row.Goods = []BoardGoods{}
		}
		board.Shops = append(board.Shops, row)
	}

	// the limited goods one can mark bought in the packs and Akashi's shop
	for _, id := range slices.Sorted(maps.Keys(shopTemplate)) {
		goods := shopTemplate[id]
		switch {
		case goods.times == 0 || purchases[id] > 0 || !goods.Time.Open(now) || !slices.Contains(sellableGenres, goods.Genre):
			continue
		case goods.Genre == genreShoppingStreet || goods.Genre == genreArenaShop || goods.Genre == genreBuyOil:
			continue
		}
		board.Catalog = append(board.Catalog, CatalogGoods{BoardGoods: BoardGoods{AdminGoods: templateGoods(id, 0), Max: goods.times}, Kind: lifeShopKind(goods.Genre)})
	}

	// the wallet
	resources, err := q.ListResources(ctx, commanderID)
	if err != nil {
		return board, err
	}
	board.Wallet[dropTypeResource] = map[uint32]uint32{}
	for _, row := range resources {
		board.Wallet[dropTypeResource][row.ResourceID] = row.Amount
	}
	items, err := q.ListItems(ctx, commanderID)
	if err != nil {
		return board, err
	}
	board.Wallet[dropTypeItem] = map[uint32]uint32{}
	for _, row := range items {
		board.Wallet[dropTypeItem][row.ItemID] = row.Count
	}

	levels, err := akashiLevels()
	if err != nil {
		return board, err
	}
	board.AkashiLevels = levels[:max(len(levels)-1, 0)]
	if len(levels) > 0 {
		board.AkashiExpMax = levels[len(levels)-1].Exp
	}
	return board, nil
}

// templateGoods is a shop_template row as a goods bought count times.
func templateGoods(id, count uint32) AdminGoods {
	goods := shopTemplate[id]
	kind, item := lifeShopDrop(goods)
	return AdminGoods{
		ID: id, Type: kind, Item: item, Num: uint32(max(goods.Num, 0)), Effect: goods.effect,
		PriceType: dropTypeResource, PriceID: goods.ResourceType, Price: uint32(max(goods.ResourceNum, 0)),
		Bought: count, Limit: goods.times,
	}
}

// streetBoard is the general shop's stock (handleShopStreet): one of each
// goods, at its discount; then what was bought since the refresh that isn't
// in it.
func streetBoard(shop AdminShop, purchases map[uint32]uint32, now time.Time) []BoardGoods {
	list := []BoardGoods{}
	for _, entry := range shopStreetGoods {
		goods := templateGoods(entry[0], purchases[entry[0]])
		if price, ok := shopTemplate[entry[0]].price(1, entry[1], now); ok {
			goods.Price = price
		}
		goods.Limit = 1
		list = append(list, BoardGoods{AdminGoods: goods, Max: 1})
	}
	return appendBought(list, shop)
}

// meritBoard is the merit shop's stock today (handleMilitaryShop): each
// goods once; then what was bought today that isn't in it.
func meritBoard(shop AdminShop, stock []uint32, purchases map[uint32]uint32) []BoardGoods {
	list := []BoardGoods{}
	for _, id := range stock {
		goods := templateGoods(id, purchases[id])
		goods.Limit = 1
		list = append(list, BoardGoods{AdminGoods: goods, Max: 1})
	}
	return appendBought(list, shop)
}

func appendBought(list []BoardGoods, shop AdminShop) []BoardGoods {
	for _, goods := range shop.Goods {
		if !slices.ContainsFunc(list, func(g BoardGoods) bool { return g.ID == goods.ID }) {
			list = append(list, BoardGoods{AdminGoods: goods})
		}
	}
	return list
}

// guildBoard is the guild shop the commander sees at now (tx.guildShop): the
// stored stock, or the next one, nothing bought, once it has refreshed; nil
// when nothing is stored yet.
func guildBoard(ctx context.Context, q *db.Queries, commanderID uint32, inGuild bool, now time.Time) (*AdminShop, error) {
	stored, err := q.GetGuildShop(ctx, commanderID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	} else if err != nil {
		return nil, err
	}
	shop := AdminShop{Kind: AdminShopGuild, InGuild: inGuild, ResetAt: stored.NextRefreshTime, Refreshes: stored.RefreshCount}
	bought := map[uint32]uint32{}
	if next := nextGuildShopRefresh(now).Unix(); stored.NextRefreshTime != next {
		shop.ResetAt, shop.Refreshes = next, 0
	} else {
		slots, err := q.ListGuildShopPurchases(ctx, commanderID)
		if err != nil {
			return nil, err
		}
		for _, row := range slots {
			bought[row.Slot] = row.Count
		}
	}
	for i, id := range guildShopGoods(commanderID, shop.ResetAt, shop.Refreshes) {
		goods := guildGoods[id]
		shop.Goods = append(shop.Goods, AdminGoods{
			ID: uint32(i + 1), Type: goods.Type, Item: lifeFirst(goods.Goods), Num: goods.Num,
			PriceType: dropTypeResource, PriceID: ResourceGuildCoin, Price: goods.Price,
			Bought: bought[uint32(i+1)], Limit: goods.PurchaseLimit,
		})
	}
	return &shop, nil
}

// goodsMax is the most an admin may set a goods' bought count to: its
// purchase limit, none (reset only) without one or for the new commander
// shop goods sold once each.
func goodsMax(shop AdminShop, goods AdminGoods) uint32 {
	if shop.Kind == AdminShopNewServer {
		if newServerShopGoods[shop.ActType][goods.ID].GoodsType == newServerGoodsOnceEach {
			return 0
		}
	}
	return goods.Limit
}

// AdminSetShopBought sets how many times the commander bought a goods (its
// BoardGoods ID, group for a shop_template group) in a shop's current stock,
// at most its BoardGoods.Max: what the shop's handler counts against the
// limit is replaced by count. 0 forgets the purchases, as AdminResetShop.
// A shop_template id not bought yet can be set from the catalog.
func (game *Game) AdminSetShopBought(ctx context.Context, commanderID uint32, kind string, activity, id uint32, group bool, count uint32) (bool, error) {
	board, err := game.AdminShopBoard(ctx, commanderID)
	if err != nil {
		return false, err
	}
	index := slices.IndexFunc(board.Shops, func(shop BoardShop) bool {
		return shop.Kind == kind && (kind != AdminShopNewServer || shop.Activity == activity)
	})
	if index < 0 {
		return false, adminRefused("shop %s: unknown", kind)
	}
	shop := board.Shops[index]
	at := slices.IndexFunc(shop.Goods, func(g BoardGoods) bool { return g.ID == id && g.Group == group })
	var goods BoardGoods
	switch {
	case at >= 0:
		goods = shop.Goods[at]
	case !group && (kind == AdminShopPacks || kind == AdminShopOther):
		at = slices.IndexFunc(board.Catalog, func(g CatalogGoods) bool { return g.ID == id && g.Kind == kind })
		if at < 0 {
			return false, adminRefused("goods %d: unknown", id)
		}
		goods = board.Catalog[at].BoardGoods
	default:
		return false, adminRefused("goods %d: unknown", id)
	}
	if count > goods.Max && count != 0 {
		return false, adminRefused("goods %d: at most %d: refused", id, goods.Max)
	}
	now := time.Now()
	return game.AdminWrite(ctx, commanderID, func(tx *Tx) error {
		c := tx.commanderID
		switch kind {
		case AdminShopStreet, AdminShopArena, AdminShopPacks, AdminShopOther:
			if group {
				var head shopGoods
				for _, row := range slices.Sorted(maps.Keys(shopTemplate)) {
					if shopTemplate[row].Group == id {
						head = shopTemplate[row]
						break
					}
				}
				if err := tx.LifeDeleteShopGroupPurchases(ctx, db.LifeDeleteShopGroupPurchasesParams{CommanderID: c, GroupID: id}); err != nil || count == 0 {
					return err
				}
				return tx.AddShopGroupPurchase(ctx, db.AddShopGroupPurchaseParams{CommanderID: c, GroupID: id, ExpiresAt: head.groupResetAt(now), Count: count})
			}
			if err := tx.LifeDeleteShopPurchases(ctx, db.LifeDeleteShopPurchasesParams{CommanderID: c, ShopID: id}); err != nil || count == 0 {
				return err
			}
			return tx.AddShopPurchase(ctx, db.AddShopPurchaseParams{CommanderID: c, ShopID: id, ExpiresAt: shopTemplate[id].resetAt(now), Count: count})
		case AdminShopMedal:
			if err := tx.LifeDeleteMedalShopPurchase(ctx, db.LifeDeleteMedalShopPurchaseParams{CommanderID: c, GoodsID: id, FlashTime: shop.ResetAt}); err != nil || count == 0 {
				return err
			}
			return tx.AddMedalShopPurchase(ctx, db.AddMedalShopPurchaseParams{CommanderID: c, GoodsID: id, FlashTime: shop.ResetAt, Count: count})
		case AdminShopCore, AdminShopPrototype:
			month := monthKey(now)
			if goods.Never {
				month = 0
			}
			if err := tx.LifeDeleteMonthShopPurchase(ctx, db.LifeDeleteMonthShopPurchaseParams{CommanderID: c, GoodsID: id, Month: month}); err != nil || count == 0 {
				return err
			}
			return tx.AddMonthShopPurchase(ctx, db.AddMonthShopPurchaseParams{CommanderID: c, GoodsID: id, Month: month, Count: count})
		case AdminShopGuild:
			// the stock shown, stored first when it has refreshed
			if _, err := tx.guildShop(now); err != nil {
				return err
			}
			if err := tx.LifeDeleteGuildShopPurchase(ctx, db.LifeDeleteGuildShopPurchaseParams{CommanderID: c, Slot: id}); err != nil || count == 0 {
				return err
			}
			return tx.AddGuildShopPurchase(ctx, db.AddGuildShopPurchaseParams{CommanderID: c, Slot: id, Count: count})
		case AdminShopPrize:
			// the purchases of earlier months no longer count for the goods
			// with month_re, and count as this month's for the others
			if err := tx.LifeDeleteMiniGameShopPurchases(ctx, db.LifeDeleteMiniGameShopPurchasesParams{CommanderID: c, GoodsID: id}); err != nil || count == 0 {
				return err
			}
			return tx.AddMiniGameShopPurchase(ctx, db.AddMiniGameShopPurchaseParams{CommanderID: c, GoodsID: id, MonthStart: startOfMonth(now).Unix(), Count: count})
		case AdminShopNewServer:
			if err := tx.LifeDeleteNewServerShopGoodsItem(ctx, db.LifeDeleteNewServerShopGoodsItemParams{CommanderID: c, ActivityID: activity, GoodsID: id}); err != nil {
				return err
			}
			if err := tx.LifeDeleteNewServerShopSubGoodsItem(ctx, db.LifeDeleteNewServerShopSubGoodsItemParams{CommanderID: c, ActivityID: activity, GoodsID: id}); err != nil || count == 0 {
				return err
			}
			return tx.AddNewServerShopGoods(ctx, db.AddNewServerShopGoodsParams{CommanderID: c, ActivityID: activity, GoodsID: id, Bought: count})
		}
		return adminRefused("shop %s: unknown", kind)
	})
}

// AdminSetOilBought sets the oil bought today, at most buy_oil_limit: each
// purchase is counted on the buy_oil row priced for the oil bought before it
// (limit_args count, ShoppingStreet.getRiseShopId), as handleShopping
// counts them.
func (game *Game) AdminSetOilBought(ctx context.Context, commanderID, count uint32) (bool, error) {
	if count > buyOilLimit {
		return false, adminRefused("oil: at most %d a day: refused", buyOilLimit)
	}
	var rows []uint32
	for _, id := range slices.Sorted(maps.Keys(shopTemplate)) {
		if shopTemplate[id].Genre == genreBuyOil {
			rows = append(rows, id)
		}
	}
	now := time.Now()
	return game.AdminWrite(ctx, commanderID, func(tx *Tx) error {
		for _, id := range rows {
			if err := tx.LifeDeleteShopPurchases(ctx, db.LifeDeleteShopPurchasesParams{CommanderID: commanderID, ShopID: id}); err != nil {
				return err
			}
		}
		for n := range count {
			at := slices.IndexFunc(rows, func(id uint32) bool { return shopTemplate[id].countAllows(n) })
			if at < 0 {
				return adminRefused("oil: no buy_oil row for purchase %d: refused", n+1)
			}
			if err := tx.AddShopPurchase(ctx, db.AddShopPurchaseParams{CommanderID: commanderID, ShopID: rows[at], ExpiresAt: shopTemplate[rows[at]].resetAt(now), Count: 1}); err != nil {
				return err
			}
		}
		return nil
	})
}

// AdminSetShopRefreshes sets the manual refreshes of the merit shop today
// (at most arena_data_shop refresh_price's length) or of the guild shop's
// stock (at most guildset store_reset_time). The stock is drawn from them,
// so it changes and what was bought from the old one is forgotten, as when
// the player refreshes (handleRefreshMilitaryShop, handleGuildShop).
func (game *Game) AdminSetShopRefreshes(ctx context.Context, commanderID uint32, kind string, count uint32) (bool, error) {
	now := time.Now()
	switch kind {
	case AdminShopArena:
		if count > uint32(len(arenaShop.RefreshPrice)) {
			return false, adminRefused("merit shop: at most %d refreshes a day: refused", len(arenaShop.RefreshPrice))
		}
		return game.AdminWrite(ctx, commanderID, func(tx *Tx) error {
			state, err := loadExercise(ctx, tx.Queries, commanderID, now)
			if err != nil {
				return err
			}
			if state.ShopRefreshes == count {
				return nil
			}
			old := arenaShopGoods(arenaShop, commanderID, nextDay(now), state.ShopRefreshes)
			if len(old) > 0 {
				if err := tx.DeleteShopPurchases(ctx, db.DeleteShopPurchasesParams{CommanderID: commanderID, ExpiresAt: nextDay(now).Unix(), ShopIds: old}); err != nil {
					return err
				}
			}
			state.ShopRefreshes = count
			return saveExercise(ctx, tx.Queries, state)
		})
	case AdminShopGuild:
		if count > guildShopResets {
			return false, adminRefused("guild shop: at most %d refreshes: refused", guildShopResets)
		}
		return game.AdminWrite(ctx, commanderID, func(tx *Tx) error {
			if _, inGuild, err := guildMembership(ctx, tx.Queries, commanderID); err != nil {
				return err
			} else if !inGuild {
				return adminRefused("guild shop: not in a guild: refused")
			}
			shop, err := tx.guildShop(now)
			if err != nil || shop.RefreshCount == count {
				return err
			}
			shop.RefreshCount = count
			return tx.resetGuildShop(shop)
		})
	}
	return false, adminRefused("shop %s has no refreshes: refused", kind)
}

// AdminSetRequisitions sets this month's requisitions, at most
// supports_config's ships per month.
func (game *Game) AdminSetRequisitions(ctx context.Context, commanderID, count uint32) (bool, error) {
	if count > requisitionMonthly {
		return false, adminRefused("requisitions: at most %d a month: refused", requisitionMonthly)
	}
	next := nextMonth(time.Now()).Unix()
	return game.AdminWrite(ctx, commanderID, func(tx *Tx) error {
		if err := tx.LifeDeleteRequisitions(ctx, db.LifeDeleteRequisitionsParams{CommanderID: commanderID, Month: next}); err != nil || count == 0 {
			return err
		}
		return tx.AddRequisitions(ctx, db.AddRequisitionsParams{CommanderID: commanderID, Month: next, Count: count})
	})
}
