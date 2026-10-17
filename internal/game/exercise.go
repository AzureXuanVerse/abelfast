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

// arenaShopCommon and arenaShopLists are arena_data_shop[1]'s commodities
// (arena_data_shop ids and their weight): five picks from the common list,
// then one from each other list, in that order (as the captured shop is).
var (
	arenaShopCommon = [][2]uint32{{23001, 50}, {33011, 400}, {33012, 200}, {33013, 100}, {33014, 50}, {33021, 400}, {33022, 200}, {33023, 100}, {33024, 50}, {33031, 400}, {33032, 200}, {33033, 100}, {33034, 50}, {33041, 400}, {33042, 200}, {33043, 100}, {33044, 50}, {33060, 100}, {33070, 100}, {33080, 100}}
	arenaShopLists  = [][][2]uint32{
		{{43004, 300}},
		{{43003, 300}},
		{{43001, 1000}, {43002, 1000}, {43005, 300}, {43011, 300}, {43016, 300}, {43020, 300}, {43028, 300}, {43040, 300}, {43041, 300}, {43045, 300}, {43051, 300}},
		{{43008, 300}, {43009, 300}, {43010, 300}, {43013, 300}, {43014, 300}, {43015, 300}, {43017, 300}, {43021, 300}, {43022, 300}, {43023, 300}, {43025, 300}, {43026, 300}, {43029, 300}, {43030, 300}, {43033, 300}, {43034, 300}, {43035, 300}, {43036, 300}, {43037, 300}, {43038, 300}, {43039, 300}, {43042, 300}, {43043, 300}, {43044, 300}, {43046, 300}, {43047, 300}, {43048, 300}, {43049, 300}, {43050, 300}, {43052, 300}, {43053, 300}, {43054, 300}, {43055, 300}, {43056, 300}, {43057, 300}, {43058, 300}, {43059, 300}, {43060, 300}, {43061, 300}, {43062, 300}, {43063, 300}},
		{{43006, 300}, {43007, 300}, {43012, 300}, {43018, 300}, {43019, 300}, {43024, 300}, {43027, 300}, {43031, 300}, {43032, 300}},
	}
)

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
func arenaShopGoods(commanderID uint32, day time.Time) []uint32 {
	random := rand.New(rand.NewPCG(uint64(commanderID), uint64(day.Unix())))
	var goods []uint32
	pool := arenaShopCommon
	for range arenaShopCommonPicks {
		var id uint32
		id, pool = weightedPick(random, pool)
		goods = append(goods, id)
	}
	for _, list := range arenaShopLists {
		id, _ := weightedPick(random, list)
		goods = append(goods, id)
	}
	return goods
}

// handleMilitaryShop answers GetMilitaryShopCommand, the merit shop: today's
// goods, nothing bought, no manual refresh; it refreshes at midnight.
func (game *Game) handleMilitaryShop(client *connection.Client, req *protobuf.CS_18100) error {
	now := time.Now()
	next := nextDay(now)
	reply := &protobuf.SC_18101{
		FlashCount:    proto.Uint32(0),
		NextFlashTime: proto.Uint32(uint32(next.Unix())),
	}
	for _, id := range arenaShopGoods(client.CommanderID, next) {
		reply.ArenaShopList = append(reply.ArenaShopList, &protobuf.ARENASHOP{ShopId: proto.Uint32(id), Count: proto.Uint32(0)})
	}
	return client.Send(reply)
}
