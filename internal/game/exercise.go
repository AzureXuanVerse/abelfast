package game

import (
	"context"
	"math/rand/v2"
	"time"

	"google.golang.org/protobuf/proto"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/protobuf"
)

// serverZone is the EN servers' clock, the one mondayReference is midnight in.
var serverZone = time.FixedZone("UTC-7", -7*60*60)

// exerciseResetHours are when exercise attempts refill, server time.
var exerciseResetHours = []int{0, 12, 18}

// exerciseFightCount is SeasonInfo.MAX_FIGHTCOUNT, the attempts a refill
// tops up to.
const exerciseFightCount = 10

// exerciseSeasonLength is SeasonInfo.ONE_SEASON_TIME. Seasons start every
// two weeks from mondayReference (the captured season started 153 seasons
// after it).
const exerciseSeasonLength = 14 * 24 * time.Hour

// nextExerciseReset returns the first attempt refill after now.
func nextExerciseReset(now time.Time) time.Time {
	local := now.In(serverZone)
	day := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, serverZone)
	for _, hour := range exerciseResetHours {
		if reset := day.Add(time.Duration(hour) * time.Hour); reset.After(now) {
			return reset
		}
	}
	return day.AddDate(0, 0, 1)
}

// exerciseSeasonStart returns when the exercise season running at now began.
func exerciseSeasonStart(now time.Time) time.Time {
	reference := time.Unix(mondayReference, 0)
	return reference.Add(now.Sub(reference).Truncate(exerciseSeasonLength))
}

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
		Score:               proto.Uint32(0),
		Rank:                proto.Uint32(uint32(rank)),
		FightCount:          proto.Uint32(exerciseFightCount),
		FightCountResetTime: proto.Uint32(uint32(nextExerciseReset(time.Now()).Unix())),
		FlashTargetCount:    proto.Uint32(0),
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

// arenaShopID is the arena_data_shop row the merit shop draws from.
const arenaShopID = 1

var arenaShopConfigs = &configTable[arenaShopConfig]{name: "arena_data_shop"}

const arenaShopCommonPicks = 5

// weightedPick removes and returns one id of pool, picked by weight.
func weightedPick(random *rand.Rand, pool [][2]uint32) (uint32, [][2]uint32) {
	total := uint32(0)
	for _, entry := range pool {
		total += entry[1]
	}
	roll := random.Uint32N(total)
	for i, entry := range pool {
		if roll < entry[1] {
			rest := append(append([][2]uint32{}, pool[:i]...), pool[i+1:]...)
			return entry[0], rest
		}
		roll -= entry[1]
	}
	panic("unreachable")
}

// arenaShopGoods draws the merit shop's goods for a commander and day. The
// draw is seeded by both, so the shop stays the same until it refreshes
// without being stored.
func arenaShopGoods(shop arenaShopConfig, commanderID uint32, day time.Time) []uint32 {
	random := rand.New(rand.NewPCG(uint64(commanderID), uint64(day.Unix())))
	var goods []uint32
	pool := shop.Common
	for range arenaShopCommonPicks {
		if len(pool) == 0 {
			break
		}
		var id uint32
		id, pool = weightedPick(random, pool)
		goods = append(goods, id)
	}
	for _, list := range [][][2]uint32{shop.List1, shop.List2, shop.List3, shop.List4, shop.List5} {
		if len(list) > 0 {
			id, _ := weightedPick(random, list)
			goods = append(goods, id)
		}
	}
	return goods
}

// handleMilitaryShop answers GetMilitaryShopCommand, the merit shop: today's
// goods, nothing bought, no manual refresh; it refreshes at midnight.
func (game *Game) handleMilitaryShop(client *connection.Client, req *protobuf.CS_18100) error {
	shops, err := arenaShopConfigs.get()
	if err != nil {
		return err
	}
	now := time.Now()
	next := nextDay(now)
	reply := &protobuf.SC_18101{
		FlashCount:    proto.Uint32(0),
		NextFlashTime: proto.Uint32(uint32(next.Unix())),
	}
	for _, id := range arenaShopGoods(shops[arenaShopID], client.CommanderID, next) {
		reply.ArenaShopList = append(reply.ArenaShopList, &protobuf.ARENASHOP{ShopId: proto.Uint32(id), Count: proto.Uint32(0)})
	}
	return client.Send(reply)
}
