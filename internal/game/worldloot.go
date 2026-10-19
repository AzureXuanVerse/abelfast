package game

import (
	"encoding/json"
	"maps"
	"math/rand/v2"
	"slices"

	"google.golang.org/protobuf/proto"

	"github.com/ggmolly/belfast/internal/protobuf"
)

// Operation Siren's loot. The drop tables are the official server's: the
// client's tables only name them (world_effect_data EffectEventDrop /
// DropTreasure parameters such as 1100400, behind a branch on the map's
// difficulty: effect 10000 is a resource node's, 11001...11010 the drop of
// each difficulty) and show what may come out (world_expedition_data
// award_display_world, gameset world_catsearchdrop_show). What is given
// here comes from the wiki (Operation Siren): the Secured zone table (an
// enemy fleet and a resource node give Operation Supply Coins that add up
// to about 500 / 950 / 1400 / 1750 per C3 / C4 / C5 / C6 Secured clear,
// a resource node 5 / 10 / 10 / 10 / 10 / 15 META Coordinates), the Abyssal
// and Siren Stronghold reward tables, the meowfficer search loot lists;
// the rest (chances, the Promoted "Pawns"' extras) is a guess in their
// spirit.

// worldLoot is an entry of a loot table: count lo to hi of an item, with
// chance (in 1/100; 0 is always).
type worldLoot struct {
	kind, id uint32
	lo, hi   uint32
	chance   uint32
}

// world items (world_item_data_template) and other ids of the loot
const (
	worldItemCoins      = 100  // Operation Supply Coins
	worldItemTokens     = 101  // Special Item Tokens
	worldItemRepair     = 201  // Emergency Repair Pack
	worldItemLogger     = 2000 // + corrosion: R2-R6 Coordinate Logger (2002-2006)
	worldItemSirenLog   = 2010 // + corrosion: R4-R6 Siren Coordinate Logger (2014-2016)
	worldItemT4Part     = 2113 // T4 Random Gear Part
	worldItemSRPlans    = 2120 // SR Gear Design Plans
	worldItemURPlans    = 2121 // UR Gear Design Plans
	vitemMetaCoords     = 100000
	vitemT1Container    = 150000 // T1-T3 Development Materials Container: one material
	itemT1Report        = 18125
	itemT2Report        = 18126
	worldLootPercent    = 100
	worldLootHazardsMax = 7
)

// by corrosion (difficulty) 0-7: an enemy fleet's coins, a resource node's
// coins and (in a Secured zone) META Coordinates
var (
	worldEnemyCoins = [...]uint32{50, 50, 70, 90, 120, 150, 180, 200}
	worldNodeCoins  = [...]uint32{60, 60, 80, 105, 120, 135, 170, 180}
	worldNodeMeta   = [...]uint32{5, 5, 10, 10, 10, 10, 15, 15}
)

func worldHazard(difficulty uint32) uint32 {
	return min(max(difficulty, 1), worldLootHazardsMax)
}

// worldContainer is the Development Materials Container of a corrosion
// level: T1 at C1-2, T2 at C3-4, T3 from C5.
func worldContainer(hazard uint32) uint32 {
	return vitemT1Container + min((hazard-1)/2, 2)
}

// spread is n give or take a tenth.
func spread(n uint32) (uint32, uint32) {
	return n * 9 / 10, n * 11 / 10
}

// roll draws a loot table into drops: each entry by its chance, its count
// between lo and hi; containers (virtual items listing their content) open
// into one of their materials each (resolveContent).
func rollWorldLoot(rng *rand.Rand, loot []worldLoot) []*protobuf.DROPINFO {
	var drops []*protobuf.DROPINFO
	for _, entry := range loot {
		if entry.chance > 0 && rng.Uint32N(worldLootPercent) >= entry.chance {
			continue
		}
		n := entry.lo
		if entry.hi > entry.lo {
			n += rng.Uint32N(entry.hi - entry.lo + 1)
		}
		if n == 0 {
			continue
		}
		if entry.kind == dropTypeItem && len(dropPools[entry.id]) > 0 {
			drops = resolveContent(drops, entry.kind, entry.id, n)
			continue
		}
		drops = mergeDrop(drops, entry.kind, entry.id, n)
	}
	return drops
}

// worldEnemyClass is what an enemy's battle gives.
type worldEnemyClass uint8

const (
	worldEnemyNothing  worldEnemyClass = iota // exp_world 0: story, reinforcements
	worldEnemyFleet                           // a "Pawn" or other fleet
	worldEnemyPromoted                        // a Promoted "Pawn", an elite (type 98/99)
	worldEnemyBoss                            // a boss fought over several battles (phases)
)

func worldEnemyKind(id, difficulty uint32) worldEnemyClass {
	world := worldExpeditions[id]
	if len(world.phases()) > 0 {
		return worldEnemyBoss
	}
	exp := uint32(0)
	for _, entry := range world.ExpWorld {
		if entry[0] == difficulty || exp == 0 {
			exp = entry[1]
		}
	}
	if exp == 0 {
		return worldEnemyNothing
	}
	if expedition := expeditionTemplates[id]; expedition.Type == 98 || expedition.Type == 99 {
		return worldEnemyPromoted
	}
	return worldEnemyFleet
}

// worldBattleLoot is a won battle's loot against enemy on a map of
// difficulty: an enemy fleet gives coins, sometimes a materials container
// or a repair pack; a promoted one twice the coins, Special Item Tokens,
// META Coordinates, a container, and a chance of a T4 Random Gear Part and
// of the corrosion's Coordinate Loggers (what the wiki says Handover gives
// fewer of); the enemies that give no exp (story, reinforcements) nothing.
// A boss's loot comes with its hp phases (worldBossPhaseLoot).
func worldBattleLoot(rng *rand.Rand, enemy, difficulty uint32) []*protobuf.DROPINFO {
	h := worldHazard(difficulty)
	lo, hi := spread(worldEnemyCoins[h])
	switch worldEnemyKind(enemy, difficulty) {
	case worldEnemyFleet:
		return rollWorldLoot(rng, []worldLoot{
			{dropTypeWorldItem, worldItemCoins, lo, hi, 0},
			{dropTypeItem, worldContainer(h), 1, 1, 30},
			{dropTypeWorldItem, worldItemRepair, 1, 1, 10},
		})
	case worldEnemyPromoted:
		loot := []worldLoot{
			{dropTypeWorldItem, worldItemCoins, 2 * lo, 2 * hi, 0},
			{dropTypeWorldItem, worldItemTokens, 3 * h, 5 * h, 0},
			{dropTypeVItem, vitemMetaCoords, worldNodeMeta[h], worldNodeMeta[h], 0},
			{dropTypeItem, worldContainer(h), 1, 1, 0},
		}
		if h >= 4 {
			loot = append(loot, worldLoot{dropTypeWorldItem, worldItemT4Part, 1, 1, 20},
				worldLoot{dropTypeWorldItem, worldItemSirenLog + min(h, 6), 1, 1, 5})
		}
		if h >= 2 {
			loot = append(loot, worldLoot{dropTypeWorldItem, worldItemLogger + min(h, 6), 1, 1, 15})
		}
		return rollWorldLoot(rng, loot)
	}
	return nil
}

// worldAbyssalLoot is the wiki's Abyssal Rewards: per corrosion, what
// each hp phase of the Abyssal Siren gives (C4 at 70% and 40%, C5 and C6
// at 80%, 50%, 20%) and what its death does.
var worldAbyssalLoot = map[uint32][2][]worldLoot{
	4: {
		{{dropTypeWorldItem, worldItemCoins, 1035, 1994, 0}, {dropTypeWorldItem, worldItemTokens, 5, 10, 0}, {dropTypeWorldItem, worldItemT4Part, 1, 1, 0}, {dropTypeItem, vitemT1Container + 2, 2, 2, 0}},
		{{dropTypeWorldItem, worldItemCoins, 2044, 3767, 0}, {dropTypeWorldItem, worldItemTokens, 50, 97, 0}, {dropTypeWorldItem, worldItemT4Part, 2, 2, 0}, {dropTypeItem, vitemT1Container + 2, 4, 4, 0},
			{dropTypeWorldItem, worldItemSRPlans, 2, 2, 0}, {dropTypeWorldItem, worldItemURPlans, 1, 1, 50}, {dropTypeItem, itemT1Report, 0, 2, 0}, {dropTypeItem, itemT2Report, 1, 1, 50}},
	},
	5: {
		{{dropTypeWorldItem, worldItemCoins, 1524, 2934, 0}, {dropTypeWorldItem, worldItemTokens, 5, 10, 0}, {dropTypeWorldItem, worldItemT4Part, 2, 2, 0}, {dropTypeItem, vitemT1Container + 2, 2, 2, 0}},
		{{dropTypeWorldItem, worldItemCoins, 3259, 5972, 0}, {dropTypeWorldItem, worldItemTokens, 53, 99, 0}, {dropTypeWorldItem, worldItemT4Part, 4, 4, 0}, {dropTypeItem, vitemT1Container + 2, 4, 4, 0},
			{dropTypeWorldItem, worldItemSRPlans, 3, 3, 0}, {dropTypeWorldItem, worldItemURPlans, 1, 1, 50}, {dropTypeItem, itemT1Report, 1, 1, 50}, {dropTypeItem, itemT2Report, 1, 1, 50}},
	},
	6: {
		{{dropTypeWorldItem, worldItemCoins, 2144, 3968, 0}, {dropTypeWorldItem, worldItemTokens, 5, 10, 0}, {dropTypeWorldItem, worldItemT4Part, 3, 3, 0}, {dropTypeItem, vitemT1Container + 2, 3, 3, 0}},
		{{dropTypeWorldItem, worldItemCoins, 4015, 7805, 0}, {dropTypeWorldItem, worldItemTokens, 57, 100, 0}, {dropTypeWorldItem, worldItemT4Part, 6, 6, 0}, {dropTypeItem, vitemT1Container + 2, 6, 6, 0},
			{dropTypeWorldItem, worldItemSRPlans, 4, 4, 0}, {dropTypeWorldItem, worldItemURPlans, 1, 1, 50}, {dropTypeItem, itemT1Report, 1, 1, 0}, {dropTypeItem, itemT2Report, 1, 1, 0}},
	},
}

// worldBossPhaseLoot is SC_40004 hp_drop_info for a boss whose hp went
// from before to after (1/10000): the loot of each phase_story threshold
// crossed, the last (0) its death's. The Abyssal Sirens' come from the
// wiki's table by the map's corrosion (C4 below, C6 above); the other
// bosses with phases (none on a generated map) give the C4 one.
func worldBossPhaseLoot(rng *rand.Rand, enemy, difficulty, before, after uint32) []*protobuf.HPDROPINFO {
	table, ok := worldAbyssalLoot[min(max(difficulty, 4), 6)]
	if !ok {
		return nil
	}
	var infos []*protobuf.HPDROPINFO
	for _, hp := range worldExpeditions[enemy].phases() {
		if hp >= before || hp < after {
			continue
		}
		loot := table[0]
		if hp == 0 {
			loot = table[1]
		}
		infos = append(infos, &protobuf.HPDROPINFO{Hp: proto.Uint32(hp), DropInfo: rollWorldLoot(rng, loot)})
	}
	return infos
}

// worldDropSource is the branch a world drop belongs to: the type 35
// effect (10000-10042, one per kind of node or enemy event) that picks it
// by difficulty, and that difficulty.
type worldDropSource struct {
	branch, difficulty uint32
}

// worldDropSources are the drops (EffectEventDrop / DropTreasure
// parameters) the difficulty branches lead to.
var worldDropSources map[uint32]worldDropSource

// the branches of the drops (world_effect_data ids) the loot knows
const (
	worldBranchNode         = 10000 // an Explorable zone's resource node
	worldBranchSafePawn     = 10001 // a Safe Zone "Pawn" once beaten
	worldBranchPromoted     = 10003 // a Promoted "Pawn" once beaten
	worldBranchContainer    = 10004 // a special container
	worldBranchSafeNode     = 10006 // a Safe Zone resource node, a hidden cache
	worldBranchControl      = 10007 // the Zone Control Reward of a secured zone
	worldBranchRally        = 10008 // a Siren Rally Point destroyed
	worldBranchRare         = 10009 // switch mechanisms, special resource nodes
	worldBranchCrate        = 10012 // resource crates
	worldBranchCrate2       = 10013
	worldBranchRareCrate    = 10014
	worldBranchOtherNode    = 10021
	worldBranchMassProduced = 10022 // a Mass-Produced Siren Fleet once beaten
	worldBranchStronghold   = 10024 // a Siren Stronghold Flagship Fleet
	worldBranchPawnFirst    = 10027 // the Explorable "Pawns" once beaten, by fleet type
	worldBranchPawnLast     = 10033
)

// loadWorldDropSources reads which branch each drop is the end of.
func loadWorldDropSources() {
	worldDropSources = map[uint32]worldDropSource{}
	for id, effect := range worldEffects {
		if id < 10000 || id > 10099 || effect.EffectType != worldEffectCondition {
			continue
		}
		params := effectParams(effect)
		for i := 0; i+1 < len(params); i += 2 {
			var cond []json.RawMessage
			if json.Unmarshal(params[i], &cond) != nil || len(cond) < 2 {
				continue
			}
			var kind string
			if json.Unmarshal(cond[0], &kind) != nil || kind != "difficulty" {
				continue
			}
			target, ok := worldEffects[rawUint(params[i+1])]
			if !ok || (target.EffectType != worldEffectDrop && target.EffectType != worldEffectDropTreasure) {
				continue
			}
			if drop := paramUints(effectParams(target)); len(drop) > 0 {
				worldDropSources[drop[0]] = worldDropSource{branch: id, difficulty: rawUint(cond[1])}
			}
		}
	}
}

// worldEventLoot is an EffectEventDrop / DropTreasure's loot (the drop
// id its effect names): by the branch it ends (worldDropSources) and the
// map's difficulty otherwise. The enemies' event drops after their battle
// give nothing: the battle's loot (worldBattleLoot) is theirs.
func worldEventLoot(rng *rand.Rand, drop, difficulty uint32, zone worldZone) []*protobuf.DROPINFO {
	source, ok := worldDropSources[drop]
	if ok {
		difficulty = source.difficulty
	}
	h := worldHazard(difficulty)
	lo, hi := spread(worldNodeCoins[h])
	coins := worldLoot{dropTypeWorldItem, worldItemCoins, lo, hi, 0}
	switch source.branch {
	case worldBranchSafePawn, worldBranchPromoted, worldBranchMassProduced:
		return nil
	case worldBranchNode, worldBranchSafeNode, worldBranchOtherNode:
		loot := []worldLoot{coins}
		if zone == worldZoneSafe || source.branch == worldBranchSafeNode {
			loot = append(loot, worldLoot{dropTypeVItem, vitemMetaCoords, worldNodeMeta[h], worldNodeMeta[h], 0})
		} else {
			loot = append(loot, worldLoot{dropTypeItem, worldContainer(h), 1, 1, 25})
		}
		return rollWorldLoot(rng, loot)
	case worldBranchControl:
		return rollWorldLoot(rng, []worldLoot{
			{dropTypeWorldItem, worldItemCoins, 4 * lo, 4 * hi, 0},
			{dropTypeWorldItem, worldItemTokens, 5 * h, 10 * h, 0},
			{dropTypeItem, worldContainer(h), 1, 2, 0},
		})
	case worldBranchRally:
		return rollWorldLoot(rng, []worldLoot{
			{dropTypeWorldItem, worldItemCoins, 2 * lo, 2 * hi, 0},
			{dropTypeWorldItem, worldItemTokens, 3 * h, 5 * h, 0},
			{dropTypeItem, worldContainer(h), 1, 1, 0},
		})
	case worldBranchCrate, worldBranchCrate2:
		return rollWorldLoot(rng, []worldLoot{
			{dropTypeWorldItem, worldItemCoins, 2 * lo, 2 * hi, 0},
			{dropTypeItem, worldContainer(h), 1, 1, 50},
		})
	case worldBranchRareCrate, worldBranchRare:
		return rollWorldLoot(rng, []worldLoot{
			{dropTypeWorldItem, worldItemTokens, 5 * h, 10 * h, 0},
			{dropTypeWorldItem, worldItemT4Part, 1, 2, 0},
			{dropTypeWorldItem, worldItemSRPlans, 1, 1, 30},
			{dropTypeWorldItem, worldItemLogger + min(max(h, 2), 6), 1, 1, 10},
		})
	case worldBranchContainer:
		return rollWorldLoot(rng, []worldLoot{coins, {dropTypeWorldItem, worldItemTokens, 2 * h, 4 * h, 0}})
	case worldBranchStronghold:
		// the wiki's Siren Stronghold boss rewards
		return rollWorldLoot(rng, []worldLoot{
			{dropTypeWorldItem, worldItemCoins, 2056, 3984, 0},
			{dropTypeWorldItem, worldItemTokens, 10, 20, 0},
			{dropTypeWorldItem, worldItemT4Part, 0, 2, 0},
			{dropTypeItem, vitemT1Container + 1, 2, 2, 0},
			{dropTypeItem, vitemT1Container + 2, 2, 2, 0},
		})
	}
	if source.branch >= worldBranchPawnFirst && source.branch <= worldBranchPawnLast {
		return nil
	}
	return rollWorldLoot(rng, []worldLoot{coins})
}

// The meowfficer search (EffectEventCatSalvage): the fleet's meowfficers
// go looking for treasure while it sails (cmd_collection): progress_list
// are the world_catsearch_node ids found, one per turn of movement, the
// search done when progress reaches their number (WorldMapFleet:
// IsSalvageFinish; the wiki: "the Meowfficers return after 8 or more turns
// of movement"); the client then asks for the loot (OpReqCatSalvage), also
// when the fleet leaves the map or the search is called off, the nodes
// reached so far giving theirs. A node's score adds up to the search's
// rarity (gameset world_catsearch_score: blue, purple, yellow paw); its
// step (1-3) is how good its loot is, 3 (special_drop) the wiki's rare
// loot. The four sets of nodes (101-120, 201-220...) are by the best
// meowfficer's rarity; a node's own drop (1000311...) is server-side: the
// loot here is the wiki's regular / rare lists, in tiers.
const worldSalvageNodes = 8

// worldSalvageLoot by node step: 1 the common materials and coins, 2 the
// better ones, 3 the rare loot
var worldSalvageLoot = [4][]worldLoot{
	1: {
		{dropTypeWorldItem, worldItemCoins, 100, 300, 0},
		{dropTypeItem, vitemT1Container, 1, 2, 50},
		{dropTypeResource, 1, 200, 600, 30}, // coins (gold)
	},
	2: {
		{dropTypeWorldItem, worldItemTokens, 10, 30, 0},
		{dropTypeItem, vitemT1Container + 1, 1, 2, 60},
		{dropTypeItem, vitemT1Container + 2, 1, 1, 30},
		{dropTypeWorldItem, worldItemT4Part, 1, 1, 25},
		{dropTypeItem, itemT1Report, 1, 1, 20},
		{dropTypeItem, 20012, 1, 1, 10}, // Elite Cat Box
	},
	3: {
		{dropTypeWorldItem, worldItemTokens, 30, 60, 0},
		{dropTypeWorldItem, worldItemT4Part, 1, 2, 60},
		{dropTypeWorldItem, 2114, 1, 1, 30}, // Prototype Gear Upgrade Part
		{dropTypeWorldItem, 2115, 1, 1, 10}, // Specialized Gear Upgrade Part
		{dropTypeItem, itemT2Report, 1, 1, 30},
		{dropTypeWorldItem, worldItemSRPlans, 1, 1, 25},
		{dropTypeWorldItem, worldItemURPlans, 1, 1, 8},
		{dropTypeItem, 20013, 1, 1, 15}, // Super Rare Cat Box
		{dropTypeWorldItem, 2005, 1, 1, 10},
		{dropTypeWorldItem, 2006, 1, 1, 8},
		{dropTypeWorldItem, 2014, 1, 1, 6},
		{dropTypeWorldItem, 2015, 1, 1, 4},
		{dropTypeWorldItem, 2016, 1, 1, 2},
	},
}

// drawSalvage draws a search's nodes: set (1-4) by the meowfficers'
// rarity, each node's step by weight, better meowfficers and the zone's
// corrosion finding the better ones more often.
func drawSalvage(rng *rand.Rand, set, hazard uint32) []uint32 {
	var nodes []uint32
	byStep := map[uint32][]uint32{}
	for _, id := range slices.Sorted(maps.Keys(worldCatSearchNodes)) {
		if id/100 == set {
			byStep[worldCatSearchNodes[id].Step] = append(byStep[worldCatSearchNodes[id].Step], id)
		}
	}
	weights := []uint32{0, 70, 25 + 2*set + hazard, 3 + set + hazard/2}
	for range worldSalvageNodes {
		steps := []uint32{1, 2, 3}
		pick := weightedPick(func(n uint32) uint32 { return rng.Uint32N(n) }, steps, func(step uint32) uint32 {
			if len(byStep[step]) == 0 {
				return 0
			}
			return weights[step]
		})
		if pick < 0 {
			break
		}
		pool := byStep[steps[pick]]
		nodes = append(nodes, pool[rng.IntN(len(pool))])
	}
	return nodes
}

// salvageLoot is the loot of the nodes a search reached.
func salvageLoot(rng *rand.Rand, nodes []uint32) []*protobuf.DROPINFO {
	var drops []*protobuf.DROPINFO
	for _, id := range nodes {
		step := min(max(worldCatSearchNodes[id].Step, 1), 3)
		for _, drop := range rollWorldLoot(rng, worldSalvageLoot[step]) {
			drops = mergeDrop(drops, drop.GetType(), drop.GetId(), drop.GetNumber())
		}
	}
	return drops
}
