package admin

import (
	"context"
	"net/http"
	"slices"

	"github.com/ggmolly/belfast/internal/game"
)

// registerShops adds the back-office's Shops routes.
func (server *Server) registerShops(api apiFunc) {
	api("GET /api/commanders/{id}/shops/board", server.handleShopBoard)
	api("POST /api/commanders/{id}/shops/bought", server.handleShopBought)
	api("POST /api/commanders/{id}/shops/refreshes", server.handleShopRefreshes)
	api("POST /api/commanders/{id}/shops/oil", server.handleShopOil)
	api("POST /api/commanders/{id}/requisitions", server.handleRequisitions)
}

// boardGoods is a goods with what it gives and costs named, and the most
// its bought count can be set to (0: only reset).
type boardGoods struct {
	lifeGoods
	Max uint32 `json:"max"`
}

type catalogGoods struct {
	boardGoods
	Kind string `json:"kind"`
}

type boardShop struct {
	Kind         string       `json:"kind"`
	Name         string       `json:"name"`
	Activity     uint32       `json:"activity,omitempty"`
	ResetAt      int64        `json:"reset_at"`
	Refreshes    uint32       `json:"refreshes"`
	RefreshMax   uint32       `json:"refresh_max"`
	RefreshPrice uint32       `json:"refresh_price"`
	InGuild      bool         `json:"in_guild"`
	Bought       uint32       `json:"bought"`
	Currencies   []lifeThing  `json:"currencies"`
	Goods        []boardGoods `json:"goods"`
}

type shopBoard struct {
	Shops   []boardShop    `json:"shops"`
	GuildID uint32         `json:"guild_id"`
	Catalog []catalogGoods `json:"catalog"`
	Oil     struct {
		Bought  uint32 `json:"bought"`
		Limit   uint32 `json:"limit"`
		ResetAt int64  `json:"reset_at"`
	} `json:"oil"`
	Gems   lifeThing `json:"gems"`
	Akashi struct {
		Exp      uint32             `json:"exp"`
		ExpMax   uint32             `json:"exp_max"`
		Levels   []game.AkashiLevel `json:"levels"`
		Taps     uint32             `json:"taps"`
		Daily    uint32             `json:"daily"`
		TappedAt int64              `json:"tapped_at"`
	} `json:"akashi"`
	Requisitions struct {
		Count   uint32    `json:"count"`
		Limit   uint32    `json:"limit"`
		ResetAt int64     `json:"reset_at"`
		Medals  lifeThing `json:"medals"`
	} `json:"requisitions"`
}

// itemHonorMedal is the Medal of Honor requisitions cost
// (SupportShipCommand). A gem price is paid from the bought and the earned
// gems (resourceGem, resourceFreeGem), the earned ones first
// (SpendShopResource), so the wallet shows them summed as the earned gems.
const itemHonorMedal = 15001

func (server *Server) boardGoods(goods game.BoardGoods) boardGoods {
	return boardGoods{
		lifeGoods: lifeGoods{
			ID: goods.ID, Group: goods.Group, Bought: goods.Bought, Limit: goods.Limit, Never: goods.Never,
			Goods: server.thing(goods.Type, goods.Item, goods.Num, goods.Effect),
			Price: server.thing(goods.PriceType, goods.PriceID, goods.Price, ""),
		},
		Max: goods.Max,
	}
}

// owned names a resource or item with the commander's count of it, the
// gems both kinds together.
func owned(server *Server, wallet map[uint32]map[uint32]uint32, kind, id uint32) lifeThing {
	if kind == dropResource && (id == resourceGem || id == resourceFreeGem) {
		return server.thing(kind, resourceFreeGem, wallet[kind][resourceGem]+wallet[kind][resourceFreeGem], "")
	}
	return server.thing(kind, id, wallet[kind][id], "")
}

// handleShopBoard is the commander's Shops screen: every shop's stock with
// the bounds of its edits, the currencies its prices are in, the oil bought
// today, Akashi's affection and this month's requisitions.
func (server *Server) handleShopBoard(r *http.Request) (any, error) {
	commander, err := server.commander(r)
	if err != nil {
		return nil, err
	}
	ctx := r.Context()
	board, err := server.Game.AdminShopBoard(ctx, commander.ID)
	if err != nil {
		return nil, err
	}
	reply := shopBoard{Shops: []boardShop{}, Catalog: []catalogGoods{}, GuildID: board.GuildID}
	for _, shop := range board.Shops {
		row := boardShop{
			Kind: shop.Kind, Name: shopNames[shop.Kind], Activity: shop.Activity, ResetAt: shop.ResetAt,
			Refreshes: shop.Refreshes, RefreshMax: shop.RefreshMax, RefreshPrice: shop.RefreshPrice, InGuild: shop.InGuild,
			Currencies: []lifeThing{}, Goods: []boardGoods{},
		}
		if shop.ActType == 120 { // ACTIVITY_TYPE_BLACK_FRIDAY_SHOP
			row.Name = "Black Friday shop"
		}
		type currency struct{ kind, id uint32 }
		var seen []currency
		for _, goods := range shop.Goods {
			row.Bought += goods.Bought
			row.Goods = append(row.Goods, server.boardGoods(goods))
			c := currency{goods.PriceType, goods.PriceID}
			if c.kind == dropResource && c.id == resourceGem {
				c.id = resourceFreeGem
			}
			if goods.PriceID != 0 && !slices.Contains(seen, c) {
				seen = append(seen, c)
			}
		}
		if shop.Kind == game.AdminShopArena && !slices.Contains(seen, currency{dropResource, resourceFreeGem}) {
			seen = append(seen, currency{dropResource, resourceFreeGem})
		}
		for _, c := range seen {
			row.Currencies = append(row.Currencies, owned(server, board.Wallet, c.kind, c.id))
		}
		reply.Shops = append(reply.Shops, row)
	}
	for _, goods := range board.Catalog {
		reply.Catalog = append(reply.Catalog, catalogGoods{boardGoods: server.boardGoods(goods.BoardGoods), Kind: goods.Kind})
	}
	reply.Oil.Bought, reply.Oil.Limit, reply.Oil.ResetAt = board.OilBought, board.OilLimit, board.OilResetAt
	reply.Gems = owned(server, board.Wallet, dropResource, resourceFreeGem)

	akashi, daily, err := server.Game.AdminAkashi(ctx, commander.ID)
	if err != nil {
		return nil, err
	}
	reply.Akashi.Exp, reply.Akashi.Taps, reply.Akashi.Daily, reply.Akashi.TappedAt = akashi.Exp, akashi.Taps, daily, akashi.TappedAt
	reply.Akashi.Levels, reply.Akashi.ExpMax = board.AkashiLevels, max(board.AkashiExpMax, akashi.Exp)
	count, limit, resetAt, err := server.Game.AdminRequisitions(ctx, commander.ID)
	if err != nil {
		return nil, err
	}
	reply.Requisitions.Count, reply.Requisitions.Limit, reply.Requisitions.ResetAt = count, limit, resetAt
	reply.Requisitions.Medals = owned(server, board.Wallet, dropItem, itemHonorMedal)
	return reply, nil
}

// handleShopBought sets how many times a goods was bought: {kind, activity,
// id, group, count}.
func (server *Server) handleShopBought(r *http.Request) (any, error) {
	type body struct {
		Kind     string `json:"kind"`
		Activity uint32 `json:"activity"`
		ID       uint32 `json:"id"`
		Group    bool   `json:"group"`
		Count    uint32 `json:"count"`
	}
	return commanderEdit(server, r, func(ctx context.Context, id uint32, b body) (bool, error) {
		return server.Game.AdminSetShopBought(ctx, id, b.Kind, b.Activity, b.ID, b.Group, b.Count)
	})
}

// handleShopRefreshes sets the manual refreshes of the merit or guild shop:
// {kind, count}.
func (server *Server) handleShopRefreshes(r *http.Request) (any, error) {
	type body struct {
		Kind  string `json:"kind"`
		Count uint32 `json:"count"`
	}
	return commanderEdit(server, r, func(ctx context.Context, id uint32, b body) (bool, error) {
		return server.Game.AdminSetShopRefreshes(ctx, id, b.Kind, b.Count)
	})
}

// handleShopOil sets the oil bought today: {count}.
func (server *Server) handleShopOil(r *http.Request) (any, error) {
	return server.countEdit(r, server.Game.AdminSetOilBought)
}

// handleRequisitions sets this month's requisitions: {count}.
func (server *Server) handleRequisitions(r *http.Request) (any, error) {
	return server.countEdit(r, server.Game.AdminSetRequisitions)
}

// countEdit sets one count of the commander: {count}.
func (server *Server) countEdit(r *http.Request, edit func(ctx context.Context, commanderID, count uint32) (bool, error)) (any, error) {
	type body struct {
		Count uint32 `json:"count"`
	}
	return commanderEdit(server, r, func(ctx context.Context, id uint32, b body) (bool, error) {
		return edit(ctx, id, b.Count)
	})
}
