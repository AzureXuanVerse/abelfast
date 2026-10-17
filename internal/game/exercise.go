package game

import (
	"context"
	"math/rand/v2"
	"slices"
	"time"

	"google.golang.org/protobuf/proto"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/gamedata"
	"github.com/ggmolly/belfast/internal/protobuf"
)

// exerciseFightCount is SeasonInfo.MAX_FIGHTCOUNT, the attempts a refill
// tops up to.
const exerciseFightCount = 10

// handleSeasonInfo answers the exercise (PvP) season (GetSeasonInfoCommand):
// no points, ranked behind every commander of the server (they all start at
// the same score), full attempts, and the defense fleet. There are no rivals
// to fight yet.
//
// The defense fleet can't be edited yet; it is the one the official server
// sets up: the starter ship in the vanguard, the other starting ships (the
// prologue's carrier) in the main fleet.
func (game *Game) handleSeasonInfo(client *connection.Client, req *protobuf.CS_18001) error {
	ctx := context.Background()
	commander, err := game.DB.GetCommander(ctx, client.CommanderID)
	if err != nil {
		return err
	}
	ships, err := game.DB.ListShipsByCommander(ctx, commander.ID)
	if err != nil {
		return err
	}
	rank, err := game.DB.CountCommanders(ctx)
	if err != nil {
		return err
	}
	reply := &protobuf.SC_18002{
		Rank:                proto.Uint32(uint32(rank)),
		FightCount:          proto.Uint32(exerciseFightCount),
		FightCountResetTime: proto.Uint32(uint32(nextExerciseReset(time.Now()).Unix())),
	}
	if starter, ok := starterShip(commander, ships); ok {
		reply.VanguardShipIdList = []uint32{starter.ID}
		for _, ship := range ships {
			if ship.ID != starter.ID && len(reply.MainShipIdList) < 3 {
				reply.MainShipIdList = append(reply.MainShipIdList, ship.ID)
			}
		}
	}
	return client.Send(reply)
}

// arena_data_shop row: the merit shop's commodities (arena_data_shop ids and
// their weight), five picks from the common list, then one from each other
// list, in that order (as the captured shop is).
type arenaShopConfig struct {
	Common [][2]uint32 `json:"commodity_list_common"`
	List1  [][2]uint32 `json:"commodity_list_1"`
	List2  [][2]uint32 `json:"commodity_list_2"`
	List3  [][2]uint32 `json:"commodity_list_3"`
	List4  [][2]uint32 `json:"commodity_list_4"`
	List5  [][2]uint32 `json:"commodity_list_5"`
}

// arenaShopID is the arena_data_shop row the merit shop draws from, the one
// the client reads its refresh prices from (MilitaryShopPage,
// RefreshMilitaryShopCommand: pg.arena_data_shop[1]).
const arenaShopID = 1

var arenaShop arenaShopConfig

func loadArenaShop() error {
	shops, err := gamedata.Table[arenaShopConfig]("arena_data_shop")
	if err != nil {
		return err
	}
	shop, ok := shops[arenaShopID]
	if !ok {
		return gamedata.Missing("arena_data_shop", "shop %d", arenaShopID)
	}
	arenaShop = shop
	return nil
}

// arenaShopCommonPicks is how many goods come from commodity_list_common:
// five in the captured shop (ten goods), which no table or Lua says.
const arenaShopCommonPicks = 5

// arenaShopGoods draws the merit shop's goods (each pick by weight, the
// common ones without repeats) for a commander and day. The draw is seeded by
// both, so the shop stays the same until it refreshes without being stored.
func arenaShopGoods(shop arenaShopConfig, commanderID uint32, day time.Time) []uint32 {
	random := rand.New(rand.NewPCG(uint64(commanderID), uint64(day.Unix())))
	weight := func(entry [2]uint32) uint32 { return entry[1] }
	var goods []uint32
	pool := slices.Clone(shop.Common)
	for range arenaShopCommonPicks {
		i := weightedPick(random.Uint32N, pool, weight)
		if i < 0 {
			break
		}
		goods = append(goods, pool[i][0])
		pool = slices.Delete(pool, i, i+1)
	}
	for _, list := range [][][2]uint32{shop.List1, shop.List2, shop.List3, shop.List4, shop.List5} {
		if i := weightedPick(random.Uint32N, list, weight); i >= 0 {
			goods = append(goods, list[i][0])
		}
	}
	return goods
}

// handleMilitaryShop answers GetMilitaryShopCommand, the merit shop: today's
// goods, nothing bought, no manual refresh; it refreshes at midnight.
func (game *Game) handleMilitaryShop(client *connection.Client, req *protobuf.CS_18100) error {
	now := time.Now()
	next := nextDay(now)
	reply := &protobuf.SC_18101{
		NextFlashTime: proto.Uint32(uint32(next.Unix())),
	}
	for _, id := range arenaShopGoods(arenaShop, client.CommanderID, next) {
		reply.ArenaShopList = append(reply.ArenaShopList, &protobuf.ARENASHOP{ShopId: proto.Uint32(id)})
	}
	return client.Send(reply)
}
