package game

import (
	"context"
	"maps"
	"math/rand/v2"
	"regexp"
	"slices"
	"strconv"

	"google.golang.org/protobuf/proto"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/gamedata"
	"github.com/ggmolly/belfast/internal/packets"
	"github.com/ggmolly/belfast/internal/protobuf"
)

// Fishing (IslandFishingAgency, IslandFishingOpView, mod/island/command/
// fishing): the commander casts at a fishing point (island_fish_point) of
// the island it is on, the server says which fish bites and how heavy it is
// (grams), the client plays the reeling minigame and reports how it ended.
// A catch is recorded on the commander's own island (fish_sys.fish_weight:
// lightest and heaviest, cups) and the fish's item goes to its warehouse.
// The others on the island watch it (SC_21232 cast, SC_21233 end).
//
// Nothing in the tables or on the wiki says which fish bite where, how
// often, or how heavy they are: the pool, rates and weights below are
// GUESSES.

// IslandConst.FISHING_OP_*: how a cast ended (CS_21062 end_result)
const islandFishingSuccess = 3

// IslandTaskTargetType.START_FISHING / FISHING
const (
	islandTaskStartFishing = 57
	islandTaskFishing      = 58
)

// IslandItemUsage.usage_fishing: a bait
const islandUsageFishing = "usage_fish"

// IslandFishingAgency's bait before the commander picked one (old_bait 0)
const islandDefaultBait = 1501

// IslandAblityAgency.TYPE_FISHING_ROD: an ability giving the rod its effect
// names
const islandAbilityFishingRod = 41

// IslandFish.CUP_STATE_*: a catch's cup (SC_21061 gold_state)
const (
	islandCupMin = 1
	islandCupMax = 2
)

// the extra key of the cast waiting for its end
const islandFishingKey = "inventory_fishing"

// islandFishConfig is an island_fish row: a fish and its warehouse item.
type islandFishConfig struct {
	ItemID uint32 `json:"item_id"`
	Rarity uint32 `json:"rarity"`
	Hard   uint32 `json:"hard"`
}

// islandCast is a cast waiting for its end (CS_21062).
type islandCast struct {
	Island uint32 `json:"island"`
	Point  uint32 `json:"point"`
	Fish   uint32 `json:"fish"`
	Weight uint32 `json:"weight"`
	Cup    uint32 `json:"cup"`
}

var (
	islandFishes     map[uint32]islandFishConfig
	islandFishPoints map[uint32]struct{}
	islandFishRods   map[uint32]struct{}
	// a bait's usage_arg pairs {fish, rate change}
	islandBaitPattern = regexp.MustCompile(`\{(\d+),(-?\d+)\}`)
)

func loadIslandFishingData() error {
	return gamedata.LoadAll(
		gamedata.Load("island_fish", &islandFishes),
		gamedata.Load("island_fish_point", &islandFishPoints),
		gamedata.Load("island_fish_rod", &islandFishRods),
	)
}

func (game *Game) registerIslandFishing() {
	packets.Handle(game.handleIslandFishingCast)
	packets.Handle(game.handleIslandFishingEnd)
	packets.Handle(game.handleIslandFishingBait)
}

// islandFishingDefaults gives an island the first fishing rod: the client
// can't fish without one (IslandFishingOpView:InitArgs asserts
// island_fish_rod[fish_rod]) and the better ones come from abilities.
// GUESS: the first rod is there from the start.
func islandFishingDefaults(state *islandState) {
	fish := state.private().FishSys
	if fish == nil || fish.GetFishRod() != 0 || len(islandFishRods) == 0 {
		return
	}
	fish.FishRod = proto.Uint32(slices.Min(slices.Collect(maps.Keys(islandFishRods))))
}

// islandFishingAbility sets the rod a fishing rod ability gives
// (IslandDropHelper.HandleIslandAbilityByType: UpdateFishRodId).
func islandFishingAbility(state *islandState, ability uint32) {
	config := islandInvAbilities[ability]
	if config.Type == islandAbilityFishingRod && state.private().FishSys != nil {
		state.private().FishSys.FishRod = proto.Uint32(config.Effect)
	}
}

// islandBait is the commander's bait (IslandFishingAgency:GetBaitId).
func (state *islandState) islandBait() uint32 {
	if bait := state.private().GetFishSys().GetOldBait(); bait != 0 {
		return bait
	}
	return islandDefaultBait
}

// pickIslandFish draws the fish that bites and its weight in grams with its
// cup. GUESSES: every fish bites anywhere, at 1000 / 700 / 400 / 150 for
// rarity 1 to 4, plus the bait's rate changes (its usage_arg {fish, change}
// pairs, read as the same scale); the weight is uniform within
// 300 x hard to 1500 x hard grams, the bottom 5% a min cup, the top 5% a max
// cup.
func pickIslandFish(bait uint32) (uint32, uint32, uint32) {
	rates := map[uint32]int{}
	for id, fish := range islandFishes {
		rates[id] = []int{1000, 1000, 700, 400, 150}[min(fish.Rarity, 4)]
	}
	for _, match := range islandBaitPattern.FindAllStringSubmatch(string(islandItems[bait].UsageArg), -1) {
		id, _ := strconv.ParseUint(match[1], 10, 32)
		change, _ := strconv.Atoi(match[2])
		if _, ok := rates[uint32(id)]; ok {
			rates[uint32(id)] += change
		}
	}
	total := 0
	ids := make([]uint32, 0, len(rates))
	for id, rate := range rates {
		if rate > 0 {
			total += rate
			ids = append(ids, id)
		}
	}
	if total == 0 {
		return 0, 0, 0
	}
	slices.Sort(ids)
	roll := rand.IntN(total)
	fish := ids[len(ids)-1]
	for _, id := range ids {
		if roll < rates[id] {
			fish = id
			break
		}
		roll -= rates[id]
	}
	hard := max(islandFishes[fish].Hard, 1)
	low, high := 300*hard, 1500*hard
	weight := low + uint32(rand.IntN(int(high-low)+1))
	cup := uint32(0)
	switch span := high - low; {
	case weight-low <= span/20:
		cup = islandCupMin
	case high-weight <= span/20:
		cup = islandCupMax
	}
	return fish, weight, cup
}

// handleIslandFishingCast answers IslandGoFishingCommand (CS_21060, a cast
// at a fishing point of the island the commander is on): the fish that
// bites, its weight and cup (pickIslandFish, with the commander's bait). The
// cast waits for its end on the commander's own island; the others on the
// island see it start (SC_21232). Counts START_FISHING.
func (game *Game) handleIslandFishingCast(client *connection.Client, req *protobuf.CS_21060) error {
	islandID, here := game.islandOf(client)
	_, point := islandFishPoints[req.GetPointId()]
	if !here || islandID != req.GetIslandId() || !point {
		return client.Send(&protobuf.SC_21061{Result: result(errRefused), FishId: proto.Uint32(0), Weight: proto.Uint32(0), GoldState: proto.Uint32(0)})
	}
	reply := &protobuf.SC_21061{}
	rod := uint32(0)
	err := game.tx(context.Background(), client, func(tx *Tx) error {
		state, err := tx.island()
		if err != nil {
			return err
		}
		islandFishingDefaults(state)
		rod = state.private().GetFishSys().GetFishRod()
		fish, weight, cup := pickIslandFish(state.islandBait())
		if fish == 0 {
			return errRefused
		}
		reply.FishId, reply.Weight, reply.GoldState = proto.Uint32(fish), proto.Uint32(weight), proto.Uint32(cup)
		if err := state.setExtra(islandFishingKey, islandCast{Island: islandID, Point: req.GetPointId(), Fish: fish, Weight: weight, Cup: cup}); err != nil {
			return err
		}
		return tx.islandTaskEvent(islandTaskStartFishing, 1, req.GetPointId())
	})
	if reply.Result, err = islandReply(err); err != nil {
		return err
	}
	if reply.GetResult() != 0 {
		reply.FishId, reply.Weight, reply.GoldState = proto.Uint32(0), proto.Uint32(0), proto.Uint32(0)
		return client.Send(reply)
	}
	game.tellIsland(client, islandID, &protobuf.SC_21232{
		IslandId: proto.Uint32(islandID), UserId: proto.Uint32(client.CommanderID), PointId: req.PointId,
		RodId: proto.Uint32(rod), FishId: reply.FishId,
	})
	return client.Send(reply)
}

// handleIslandFishingEnd answers IslandFishingResultCommand (CS_21062): how
// the waiting cast ended (1 cancelled, 2 got away, 3 caught). A catch is
// recorded as the client records it (IslandFishingAgency:AddFish: a new fish
// at that weight, else IslandFish:SetWeight and SetCupState) and its item
// goes to the warehouse (or overflows); the client doesn't add that item
// itself (it sees it at the next island data). The others on the island see
// it end (SC_21233). Counts FISHING by fish. GUESS: a catch gives one of
// the fish's item.
func (game *Game) handleIslandFishingEnd(client *connection.Client, req *protobuf.CS_21062) error {
	islandID := uint32(0)
	err := game.tx(context.Background(), client, func(tx *Tx) error {
		state, err := tx.island()
		if err != nil {
			return err
		}
		var cast islandCast
		ok, err := state.getExtra(islandFishingKey, &cast)
		if err != nil {
			return err
		}
		if !ok || cast.Fish == 0 || cast.Island != req.GetIslandId() || cast.Point != req.GetPointId() {
			return errRefused
		}
		islandID = cast.Island
		if err := state.setExtra(islandFishingKey, islandCast{}); err != nil {
			return err
		}
		if req.GetEndResult() != islandFishingSuccess {
			return nil
		}
		state.recordIslandFish(cast)
		if err := tx.islandTaskEvent(islandTaskFishing, 1, cast.Fish); err != nil {
			return err
		}
		return tx.grantDrops([]*protobuf.DROPINFO{{
			Type: proto.Uint32(dropTypeIslandItem), Id: proto.Uint32(islandFishes[cast.Fish].ItemID), Number: proto.Uint32(1),
		}})
	})
	ret, err := islandReply(err)
	if err != nil {
		return err
	}
	if *ret == 0 {
		if current, here := game.islandOf(client); here && current == islandID {
			game.tellIsland(client, islandID, &protobuf.SC_21233{
				IslandId: proto.Uint32(islandID), UserId: proto.Uint32(client.CommanderID), PointId: req.PointId, Type: req.EndResult,
			})
		}
	}
	return client.Send(&protobuf.SC_21063{Result: ret})
}

// recordIslandFish records a catch in fish_sys.fish_weight as the client
// does (IslandFishingAgency:AddFish).
func (state *islandState) recordIslandFish(cast islandCast) {
	fish := state.private().FishSys
	for _, record := range fish.FishWeight {
		if record.GetFishId() != cast.Fish {
			continue
		}
		if cast.Weight > record.GetMaxWeight() {
			record.MaxWeight = proto.Uint32(cast.Weight)
		} else if cast.Weight < record.GetMinWeight() {
			record.MinWeight = proto.Uint32(cast.Weight)
		}
		record.GoldState = proto.Uint32(islandCupState(record.GetGoldState(), cast.Cup))
		return
	}
	fish.FishWeight = append(fish.FishWeight, &protobuf.PB_FISH_WEIGHT{
		FishId: proto.Uint32(cast.Fish), MinWeight: proto.Uint32(cast.Weight), MaxWeight: proto.Uint32(cast.Weight),
		GoldState: proto.Uint32(islandCupState(0, cast.Cup)),
	})
}

// islandCupState is a fish's cups after a catch with cup
// (IslandFish:SetCupState: 3 is both).
func islandCupState(state, cup uint32) uint32 {
	const both = islandCupMin | islandCupMax
	switch {
	case state == both:
		return both
	case cup == islandCupMin:
		if state == islandCupMax {
			return both
		}
		return islandCupMin
	case cup == islandCupMax:
		if state == islandCupMin {
			return both
		}
		return islandCupMax
	}
	return state
}

// handleIslandFishingBait answers IslandExchangeLureCommand (CS_21064, the
// bait picked before a cast, sent when it differs from the stored one):
// stored as fish_sys.old_bait. Only baits (usage_fish items) are taken.
// GUESS: a bait needn't be owned and is never used up (the client never
// takes one).
func (game *Game) handleIslandFishingBait(client *connection.Client, req *protobuf.CS_21064) error {
	if islandItems[req.GetBaitId()].Usage != islandUsageFishing {
		return client.Send(&protobuf.SC_21065{Result: result(errRefused)})
	}
	err := game.tx(context.Background(), client, func(tx *Tx) error {
		state, err := tx.island()
		if err != nil {
			return err
		}
		state.private().FishSys.OldBait = proto.Uint32(req.GetBaitId())
		return nil
	})
	if err != nil {
		return err
	}
	return client.Send(&protobuf.SC_21065{Result: proto.Uint32(0)})
}
