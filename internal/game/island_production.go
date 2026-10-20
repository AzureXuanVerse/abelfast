package game

import (
	"encoding/json"
	"slices"
	"time"

	"google.golang.org/protobuf/proto"

	"github.com/ggmolly/belfast/internal/gamedata"
	"github.com/ggmolly/belfast/internal/packets"
	"github.com/ggmolly/belfast/internal/protobuf"
)

// The island's production (IslandBuildingAgency, IslandBuilding, model/
// production): the production sites (island_production_place, PB_ISLAND_BUILD
// in PB_ISLAND_PUBLIC.build_list) and their slots (island_production_slot):
// characters delegated to a formula for a number of cycles (type 9, and 3 at
// the ranch with its animals: appoint_list, ship_appoint_list, award_list),
// fields sown by hand (type 1: hand_list) and mining / logging nodes
// collected by hand (type 2: build_collect). Island Technologies (place 702)
// is a production site too: researching a technology is a delegation of its
// formula. Also the wild resources (island_wild_gather) and collection
// fragments (island_collect_fragment) lying about the scenes, and the
// automatic collection of both. Built from the Lua and the tables alone: no
// capture covers the island and the wiki has no numbers for it, so every
// amount the tables don't give is a GUESS, said where it is made.

// island_production_slot.type
const (
	productionSlotHand       = 1 // sown by hand
	productionSlotCollect    = 2 // a mining / logging node
	productionSlotRanch      = 3 // a delegation with animals
	productionSlotDelegation = 9
)

// IslandAblityAgency.TYPE_*
const (
	islandAbilityPlace          = 2
	islandAbilityFormula        = 3
	islandAbilitySlot           = 9
	islandAbilityAnimal         = 15
	islandAbilityRecoverCamp    = 22
	islandAbilityRecoverOre     = 23
	islandAbilitySecondProduct  = 24
	islandAbilityProductFelling = 26
	islandAbilityProductMining  = 27
	islandAbilityPostManage     = 37
	islandAbilityProductFarm    = 38
	islandAbilityProductOrchard = 39
	islandAbilityProductGarden  = 40
	islandAbilityProductFish    = 42
)

// IslandProductConst
const (
	productionPlaceFarm      = 101
	productionPlacePasture   = 102
	productionPlaceFishery   = 201
	productionPlaceMine      = 401
	productionPlaceFelling   = 402
	productionPlaceOrchard   = 501
	productionPlaceGarden    = 502
	productionPlaceTechnoloy = 702 // IslandTechnologyAgency.PLACE_ID
)

// island task targets (IslandTaskTargetType) and achievement records
// (IslandAchievementType) production counts on the server
const (
	islandTaskHandGather     = 6
	islandTaskProduction     = 7
	islandTaskHandProd       = 14
	islandTaskProdItem       = 26
	islandTaskActivityGather = 40
	islandTaskStartDelegate  = 42
	islandTaskItemInPlace    = 47
	islandTaskDelegateIn     = 48
	islandTaskHandInPlace    = 49

	islandAchievementBuildingHand = 21
	islandAchievementShipDispatch = 24
)

// productionPlaceConfig is an island_production_place row.
type productionPlaceConfig struct {
	MapID         uint32          `json:"map_id"`
	GatheringSlot json.RawMessage `json:"gathering_slot"`
}

// productionSlotConfig is an island_production_slot row: a slot of a site,
// there when the site is (unlock_type 0) or unlocked by an ability.
type productionSlotConfig struct {
	Place           uint32          `json:"place"`
	Type            uint32          `json:"type"`
	UnlockType      uint32          `json:"unlock_type"`
	Formula         json.RawMessage `json:"formula"`
	ActivityFormula json.RawMessage `json:"activity_formula"`
	Animal          json.RawMessage `json:"animal"`
	ExclusionSlot   json.RawMessage `json:"exclusion_slot"`
}

// productionFormulaConfig is an island_formula row: what a cycle of
// production costs and makes.
type productionFormulaConfig struct {
	Attribute            uint32          `json:"attribute"`
	CommissionCost       json.RawMessage `json:"commission_cost"`
	CommissionProduct    json.RawMessage `json:"commission_product"`
	Cost                 json.RawMessage `json:"cost"`
	DropDisplay          json.RawMessage `json:"drop_display"`
	SecondProduct        json.RawMessage `json:"second_product"`
	SecondProductDisplay json.RawMessage `json:"second_product_display"`
	ProductionLimit      uint32          `json:"production_limit"`
	PtAward              uint32          `json:"pt_award"`
	ShipExp              uint32          `json:"ship_exp"`
	StaminaCost          uint32          `json:"stamina_cost"`
	UnlockType           uint32          `json:"unlock_type"`
	Workload             uint32          `json:"workload"`
	IsCondition          uint32          `json:"is_condition"`
	ItemID               uint32          `json:"item_id"`
}

// productionAbilityConfig is an island_ability_template row.
type productionAbilityConfig struct {
	Type   uint32 `json:"type"`
	Effect uint32 `json:"effect"`
}

// productionAnimalConfig is an island_ranch_animal row: an animal of a ranch
// slot and what it adds to each cycle there.
type productionAnimalConfig struct {
	SlotID                  uint32 `json:"slot_id"`
	Unlock                  uint32 `json:"unlock"`
	EfficiencyCost          uint32 `json:"efficiency_cost"`
	EfficiencyGainsNum      uint32 `json:"efficiency_gains_num"`
	EfficiencyGainsBonusNum uint32 `json:"efficiency_gains_bonus_num"`
}

var (
	productionPlaces    map[uint32]productionPlaceConfig
	productionSlots     map[uint32]productionSlotConfig
	productionFormulas  map[uint32]productionFormulaConfig
	productionAbilities map[uint32]productionAbilityConfig
	productionAnimals   map[uint32]productionAnimalConfig
	// island_set mining_default_slot: the nodes of a site collected by hand
	productionDefaultNodes map[uint32][]uint32
	// island_set mission_gather_point: the nodes a task puts there once
	productionTaskNodes map[uint32]bool
)

func (game *Game) registerIslandProduction() {
	packets.Handle(game.handleIslandStartDelegation)
	packets.Handle(game.handleIslandFinishDelegation)
	packets.Handle(game.handleIslandDelegationAward)
	packets.Handle(game.handleIslandAddDelegation)
	packets.Handle(game.handleIslandSlotCollect)
	packets.Handle(game.handleIslandHandPlant)
	packets.Handle(game.handleIslandHandPlantAward)
	packets.Handle(game.handleIslandStopHandPlant)
	packets.Handle(game.handleIslandAutoCollect)
	packets.Handle(game.handleIslandAutoCollectData)
	packets.Handle(game.handleIslandUnlockTech)
	packets.Handle(game.handleIslandFinishTech)
	packets.Handle(game.handleIslandWildGather)
	packets.Handle(game.handleIslandWildGatherSign)
	packets.Handle(game.handleIslandWildCollect)
	packets.Handle(game.handleIslandWildCollectSign)
}

// loadIslandProductionData reads production's tables, and its island_set
// keys itself: it loads alongside loadIslandCoreData, which fills
// islandSettings.
func loadIslandProductionData() error {
	settings, err := gamedata.Settings("island_set")
	if err != nil {
		return err
	}
	err = gamedata.LoadAll(
		gamedata.Load("island_production_place", &productionPlaces),
		gamedata.Load("island_production_slot", &productionSlots),
		gamedata.Load("island_formula", &productionFormulas),
		gamedata.Load("island_ability_template", &productionAbilities),
		gamedata.Load("island_ranch_animal", &productionAnimals),
		loadProductionShipData,
		loadProductionTechData,
		loadProductionWildData,
	)
	if err != nil {
		return err
	}
	var defaults [][]json.RawMessage
	var points, autoCosts [][]uint32
	for key, value := range map[string]any{
		"mining_default_slot":    &defaults,
		"mission_gather_point":   &points,
		"mining_auto_parameters": &autoCosts,
		"post_manage_produce":    &productionTipPlaces,
	} {
		if json.Unmarshal(settings[key].KeyValueVarchar, value) != nil {
			return gamedata.Missing("island_set", "%s key_value_varchar", key)
		}
	}
	productionDefaultNodes = map[uint32][]uint32{}
	for _, entry := range defaults {
		var place uint32
		if len(entry) != 2 || json.Unmarshal(entry[0], &place) != nil {
			return gamedata.Missing("island_set", "mining_default_slot [place, slots]")
		}
		productionDefaultNodes[place] = gamedata.IDs(entry[1])
	}
	productionTaskNodes = map[uint32]bool{}
	for _, point := range points {
		if len(point) > 0 {
			productionTaskNodes[point[0]] = true
		}
	}
	productionAutoCosts = map[uint32]productionAutoCost{}
	for _, entry := range autoCosts {
		if len(entry) < 4 {
			return gamedata.Missing("island_set", "mining_auto_parameters [place, coin, energy, exp]")
		}
		productionAutoCosts[entry[0]] = productionAutoCost{coin: entry[1], energy: entry[2], exp: entry[3]}
	}
	if productionNodeRecovery = settings["mining_recovery_tim_new"].KeyValueInt; productionNodeRecovery == 0 {
		return gamedata.Missing("island_set", "mining_recovery_tim_new")
	}
	if productionBaseEfficiency = settings["base_efficiency"].KeyValueInt; productionBaseEfficiency == 0 {
		return gamedata.Missing("island_set", "base_efficiency")
	}
	return nil
}

// productionPairs reads a list of [id, count] pairs ("" for none).
func productionPairs(raw json.RawMessage) [][2]uint32 {
	var list [][]uint32
	if json.Unmarshal(raw, &list) != nil {
		return nil
	}
	pairs := make([][2]uint32, 0, len(list))
	for _, pair := range list {
		if len(pair) >= 2 {
			pairs = append(pairs, [2]uint32{pair[0], pair[1]})
		}
	}
	return pairs
}

// productionItemDrops are island warehouse items as a reward.
func productionItemDrops(pairs [][2]uint32, times uint32, extra uint32) []*protobuf.DROPINFO {
	var drops []*protobuf.DROPINFO
	for _, pair := range pairs {
		if count := pair[1]*times + extra; count > 0 {
			drops = append(drops, &protobuf.DROPINFO{Type: proto.Uint32(dropTypeIslandItem), Id: proto.Uint32(pair[0]), Number: proto.Uint32(count)})
		}
	}
	return drops
}

// productionSlotFormulas are the formulas a slot can run.
func productionSlotFormulas(slot productionSlotConfig) []uint32 {
	return append(gamedata.IDs(slot.Formula), gamedata.IDs(slot.ActivityFormula)...)
}

// productionHasAbility tells whether the island has an ability (0: always).
func (state *islandState) productionHasAbility(id uint32) bool {
	return id == 0 || slices.Contains(state.public().GetAbilityList(), id)
}

// islandAbilityEffects adds up the effects of the island's abilities of a
// type (GetAdditionEffectByAblityType).
func (state *islandState) islandAbilityEffects(kind uint32) uint32 {
	total := uint32(0)
	for _, id := range state.public().GetAbilityList() {
		if ability := productionAbilities[id]; ability.Type == kind {
			total += ability.Effect
		}
	}
	return total
}

// formulaUnlocked tells whether the island can run a formula: unlock_type
// is the ability it needs, 0 for none.
func (state *islandState) formulaUnlocked(id uint32) bool {
	formula, ok := productionFormulas[id]
	return ok && state.productionHasAbility(formula.UnlockType)
}

// building is one of the island's production sites, nil when it has none.
func (state *islandState) building(placeID uint32) *protobuf.PB_ISLAND_BUILD {
	for _, build := range state.public().GetBuildList() {
		if build.GetId() == placeID {
			return build
		}
	}
	return nil
}

// newHandArea is a field slot at rest.
func newHandArea(slotID uint32) *protobuf.PB_ISLAND_HAND_AREA {
	return &protobuf.PB_ISLAND_HAND_AREA{
		Id: proto.Uint32(slotID), State: proto.Uint32(0), FormulaId: proto.Uint32(0),
		StartTime: proto.Uint32(0), EndTime: proto.Uint32(0),
	}
}

// newAppointArea is a delegation slot with no delegation yet: a ranch slot
// starts with its animals there from the start (unlock 0). GUESS: the
// client's own InitSlotRoleDataByAbility reads an unlock_type column the
// table doesn't have, so it starts ability-unlocked ranch slots empty; the
// sites' own slots get the free animals.
func newAppointArea(slotID uint32) *protobuf.PB_ISLAND_APPOINT_AREA {
	area := &protobuf.PB_ISLAND_APPOINT_AREA{Id: proto.Uint32(slotID)}
	if productionSlots[slotID].Type == productionSlotRanch {
		for _, animal := range gamedata.IDs(productionSlots[slotID].Animal) {
			if config, ok := productionAnimals[animal]; ok && config.Unlock == 0 {
				area.PartList = append(area.PartList, animal)
			}
		}
	}
	return area
}

// islandNewBuilding is a production site (island_production_place) as it is
// when unlocked: its slots there from the start (island_production_slot
// unlock_type 0), fields at rest and delegation slots free, and its hand
// collected nodes (island_set mining_default_slot), all ready. Every site
// carries a build_collect: IslandBuildingAgency.InitBuildData reads it
// unconditionally.
func islandNewBuilding(placeID uint32) *protobuf.PB_ISLAND_BUILD {
	build := &protobuf.PB_ISLAND_BUILD{
		Id: proto.Uint32(placeID),
		BuildCollect: &protobuf.PB_ISLAND_BUILD_COLLECT{
			GetNum: proto.Uint32(0), RefreshTime: proto.Uint32(0),
			CollectList: slices.Clone(productionDefaultNodes[placeID]),
		},
	}
	for _, id := range sortedKeys(productionSlots) {
		slot := productionSlots[id]
		if slot.Place != placeID || slot.UnlockType != 0 {
			continue
		}
		switch slot.Type {
		case productionSlotHand:
			build.HandList = append(build.HandList, newHandArea(id))
		case productionSlotDelegation, productionSlotRanch:
			build.AppointList = append(build.AppointList, newAppointArea(id))
		}
	}
	return build
}

// repairIslandBuilding gives a site stored before islandNewBuilding filled
// one in what it lacks: its build_collect and the slots it has from the
// start.
func repairIslandBuilding(build *protobuf.PB_ISLAND_BUILD) {
	fresh := islandNewBuilding(build.GetId())
	if build.BuildCollect == nil {
		build.BuildCollect = fresh.BuildCollect
	}
	for _, area := range fresh.HandList {
		if handArea(build, area.GetId()) == nil {
			build.HandList = append(build.HandList, area)
		}
	}
	for _, area := range fresh.AppointList {
		if appointArea(build, area.GetId()) == nil {
			build.AppointList = append(build.AppointList, area)
		}
	}
}

// islandAbilityGained runs when the island gains an ability
// (island_ability_template), from any reward, doing what the client does
// itself (IslandDropHelper.HandleIslandAbilityByType) or is told:
// - a production site (TYPE_PLACE): the site, pushed in SC_21519
// - a slot (TYPE_SLOT): a field (type 1) or a delegation slot (9, 3) on its
// site, which the client adds itself
// - an animal (TYPE_ANIMAL) on its ranch slot, added by the client too
// - a faster regrowth of the woods' / mine's nodes (TYPE_RECOVER_CAMP /
// ORE): they are back by that time of the day at the latest.
func (tx *Tx) islandAbilityGained(state *islandState, id uint32) error {
	// order slots, restaurants and assistant posts (island_order_manage.go)
	if err := tx.islandOrderAbilityGained(state, id); err != nil {
		return err
	}
	ability, ok := productionAbilities[id]
	if !ok {
		return nil
	}
	switch ability.Type {
	case islandAbilityPlace:
		if _, ok := productionPlaces[ability.Effect]; !ok || state.building(ability.Effect) != nil {
			return nil
		}
		build := islandNewBuilding(ability.Effect)
		public := state.public()
		public.BuildList = append(public.BuildList, build)
		tx.push(&protobuf.SC_21519{Build: proto.Clone(build).(*protobuf.PB_ISLAND_BUILD)})
	case islandAbilitySlot:
		slot, ok := productionSlots[ability.Effect]
		build := state.building(slot.Place)
		if !ok || build == nil {
			return nil
		}
		switch slot.Type {
		case productionSlotHand:
			if !slices.ContainsFunc(build.HandList, func(area *protobuf.PB_ISLAND_HAND_AREA) bool { return area.GetId() == ability.Effect }) {
				build.HandList = append(build.HandList, newHandArea(ability.Effect))
			}
		case productionSlotDelegation, productionSlotRanch:
			if appointArea(build, ability.Effect) == nil {
				area := &protobuf.PB_ISLAND_APPOINT_AREA{Id: proto.Uint32(ability.Effect)}
				build.AppointList = append(build.AppointList, area)
			}
		}
	case islandAbilityAnimal:
		animal, ok := productionAnimals[ability.Effect]
		build := state.building(productionSlots[animal.SlotID].Place)
		if !ok || build == nil {
			return nil
		}
		if area := appointArea(build, animal.SlotID); area != nil && !slices.Contains(area.PartList, ability.Effect) {
			area.PartList = append(area.PartList, ability.Effect)
		}
	case islandAbilityRecoverCamp, islandAbilityRecoverOre:
		place := uint32(productionPlaceFelling)
		if ability.Type == islandAbilityRecoverOre {
			place = productionPlaceMine
		}
		if build := state.building(place); build != nil && build.BuildCollect != nil {
			by := uint32(startOfDay(serverNow()).Unix()) + ability.Effect
			if by < build.BuildCollect.GetRefreshTime() {
				build.BuildCollect.RefreshTime = proto.Uint32(by)
			}
		}
	}
	return nil
}

// appointArea is a delegation slot of a site, nil if it has none.
func appointArea(build *protobuf.PB_ISLAND_BUILD, slotID uint32) *protobuf.PB_ISLAND_APPOINT_AREA {
	for _, area := range build.GetAppointList() {
		if area.GetId() == slotID {
			return area
		}
	}
	return nil
}

// productionNow is the current server time in seconds.
func productionNow() uint32 {
	return uint32(time.Now().Unix())
}
