package game

import (
	"encoding/json"
	"math"
	"slices"

	"google.golang.org/protobuf/proto"

	"github.com/ggmolly/belfast/internal/gamedata"
	"github.com/ggmolly/belfast/internal/protobuf"
)

// What production needs to know of the island's characters (IslandShip,
// IslandProductTimeHelper, IslandProductCostHelper): their energy, which
// recovers over time once they're done working, their level from exp, their
// stats' grades and their skill. The timed buffs (buff_list) are left out:
// GUESS that they don't change what the server computes.

// IslandShip.STATE_*: what a character is busy with (PB_SHIP_WORK_PLACE)
const (
	islandShipNormal     = 0
	islandShipDelegation = 1
	islandShipTechnology = 2
)

// IslandBuffType
const (
	islandBuffShipPowerRecover   = 2
	islandBuffShipProductRatio   = 102
	islandBuffShipProductCostCut = 103
)

// productionCharaConfig is an island_chara_template row.
type productionCharaConfig struct {
	Power        uint32          `json:"power"`
	PowerRecover uint32          `json:"power_recover"`
	UpgradePower []uint32        `json:"upgrade_power"`
	UpgradeLevel []uint32        `json:"upgrade_level"`
	BaseAtt      [][]float64     `json:"base_att"`
	GrowthAtt    json.RawMessage `json:"growth_att"`
	SkillID      uint32          `json:"skill_id"`
	SkillUnlock  uint32          `json:"skill_unlock"`
}

type productionCharaLevelConfig struct {
	LevelUpExp uint32 `json:"level_up_exp"`
}

// productionGradeConfig is an island_chara_att row: a stat grade.
type productionGradeConfig struct {
	Effect       uint32   `json:"effect"`
	GatherEffect uint32   `json:"gather_effect"`
	Range        []uint32 `json:"range"`
}

type productionSkillConfig struct {
	SkillEffect [][]uint32 `json:"skill_effect"`
}

type productionBuffConfig struct {
	BuffType uint32          `json:"buff_type"`
	TypeUse  json.RawMessage `json:"type_use"`
}

var (
	productionCharas      map[uint32]productionCharaConfig
	productionCharaLevels map[uint32]productionCharaLevelConfig
	productionGrades      map[uint32]productionGradeConfig
	productionSkills      map[uint32]productionSkillConfig
	productionBuffs       map[uint32]productionBuffConfig
	// island_set base_efficiency: the work a character does per second
	productionBaseEfficiency uint32
)

func loadProductionShipData() error {
	return gamedata.LoadAll(
		gamedata.Load("island_chara_template", &productionCharas),
		gamedata.Load("island_chara_level", &productionCharaLevels),
		gamedata.Load("island_chara_att", &productionGrades),
		gamedata.Load("island_chara_skill", &productionSkills),
		gamedata.Load("island_buff_template", &productionBuffs),
	)
}

// shipMaxEnergy is a character's energy cap (IslandShip.InitMaxEnergy).
func shipMaxEnergy(ship *protobuf.PB_ISLAND_SHIP) uint32 {
	config := productionCharas[ship.GetId()]
	total := config.Power
	for i := uint32(0); i < ship.GetBreakLv() && int(i) < len(config.UpgradePower); i++ {
		total += config.UpgradePower[i]
	}
	return total
}

// shipBusy tells whether a character is still working (IslandShip.GetState).
func shipBusy(ship *protobuf.PB_ISLAND_SHIP, now uint32) bool {
	return ship.GetRecoverTime() > now && ship.GetWorkPlace().GetType() != islandShipNormal
}

// shipEnergy is a character's energy now (IslandShip.GetCurrentEnergy): what
// it had when it stopped working, plus one per power_recover seconds since
// (more with a recovery skill), up to its cap.
func shipEnergy(ship *protobuf.PB_ISLAND_SHIP, now uint32) uint32 {
	limit := shipMaxEnergy(ship)
	energy := ship.GetPower()
	recover := productionCharas[ship.GetId()].PowerRecover
	if ship.GetRecoverTime() <= now && recover > 0 {
		gained := (now - ship.GetRecoverTime()) / recover
		energy += gained
		if bonus := shipSkillEffect(ship, islandBuffShipPowerRecover, 0); bonus > 0 {
			energy += uint32(math.Floor(float64(gained) * float64(bonus) * 0.01))
		}
	}
	return min(energy, limit)
}

// setShipEnergy settles a character's energy at now: it recovers from there.
func setShipEnergy(ship *protobuf.PB_ISLAND_SHIP, energy, from uint32) {
	ship.Power = proto.Uint32(energy)
	ship.RecoverTime = proto.Uint32(from)
}

// shipAddExp gives exp to a character, leveling it up to its breakout's cap
// (IslandShip.AddExp).
func shipAddExp(ship *protobuf.PB_ISLAND_SHIP, exp uint32) {
	config := productionCharas[ship.GetId()]
	phase := uint32(0)
	if len(config.UpgradeLevel) > 0 {
		phase = config.UpgradeLevel[0]
	}
	maxLevel := ship.GetBreakLv() * phase
	if ship.GetLv() >= maxLevel {
		return
	}
	total := ship.GetExp() + exp
	level := ship.GetLv()
	for level < maxLevel {
		need := productionCharaLevels[level].LevelUpExp
		if need == 0 || total < need {
			break
		}
		total -= need
		level++
	}
	if level >= maxLevel {
		total = 0
	}
	ship.Lv = proto.Uint32(level)
	ship.Exp = proto.Uint32(total)
}

// shipStat is a character's stat (1 plant .. 6 machinery, IslandShipAttr)
// at its level (IslandShip.InitAttrs).
func shipStat(ship *protobuf.PB_ISLAND_SHIP, stat uint32) uint32 {
	config := productionCharas[ship.GetId()]
	phase := uint32(1)
	if len(config.UpgradeLevel) > 0 && config.UpgradeLevel[0] > 0 {
		phase = config.UpgradeLevel[0]
	}
	value := 0.0
	for _, base := range config.BaseAtt {
		if len(base) == 2 && uint32(base[0]) == stat {
			value = base[1]
		}
	}
	var growth [][]json.RawMessage
	_ = json.Unmarshal(config.GrowthAtt, &growth)
	phases, rest := ship.GetLv()/phase, ship.GetLv()%phase
	for _, entry := range growth {
		var id uint32
		var steps []float64
		if len(entry) != 2 || json.Unmarshal(entry[0], &id) != nil || id != stat || json.Unmarshal(entry[1], &steps) != nil {
			continue
		}
		for i := uint32(0); i < phases && int(i) < len(steps); i++ {
			value += steps[i] * float64(phase)
		}
		if int(phases) < len(steps) {
			value += steps[phases] * float64(rest)
		}
	}
	for _, extra := range ship.GetExtraAttrList() {
		if extra.GetId() == stat {
			value += float64(extra.GetValue())
		}
	}
	return uint32(math.Floor(value))
}

// statGrade is a stat value's grade (IslandShip.GetAttrGradeByValue): the
// last one when none fits.
func statGrade(value uint32) productionGradeConfig {
	ids := sortedKeys(productionGrades)
	if len(ids) == 0 {
		return productionGradeConfig{}
	}
	for _, id := range ids {
		grade := productionGrades[id]
		if len(grade.Range) == 2 && grade.Range[0] <= value && value <= grade.Range[1] {
			return grade
		}
	}
	return productionGrades[ids[len(ids)-1]]
}

// shipSkillEffect adds up a character's unlocked skill effects of a buff
// type (IslandShipSkill.GetUnlockShipEffectIds), those limited to places
// counting only at place (0: the unlimited ones, whose value is the first
// of type_use).
func shipSkillEffect(ship *protobuf.PB_ISLAND_SHIP, buffType, place uint32) uint32 {
	config := productionCharas[ship.GetId()]
	if config.SkillUnlock > ship.GetBreakLv() {
		return 0
	}
	effects := productionSkills[config.SkillID].SkillEffect
	level := max(ship.GetSkillLv(), 1)
	if int(level) > len(effects) {
		return 0
	}
	total := uint32(0)
	for _, id := range effects[level-1] {
		buff, ok := productionBuffs[id]
		if !ok || buff.BuffType != buffType {
			continue
		}
		var use []json.RawMessage
		if json.Unmarshal(buff.TypeUse, &use) != nil || len(use) == 0 {
			continue
		}
		var value float64
		if place == 0 {
			if json.Unmarshal(use[0], &value) == nil {
				total += uint32(value)
			}
			continue
		}
		if len(use) < 2 || json.Unmarshal(use[1], &value) != nil {
			continue
		}
		var places []uint32
		if json.Unmarshal(use[0], &places) == nil && slices.Contains(places, place) {
			total += uint32(value)
		}
	}
	return total
}

// productionSpeedAbility is the ability type speeding a site's work up
// (IslandProductTimeHelper.GetSpeedAddtionTypeByPlaceId), 0 for none.
func productionSpeedAbility(place uint32) uint32 {
	switch place {
	case productionPlaceFelling:
		return islandAbilityProductFelling
	case productionPlaceMine:
		return islandAbilityProductMining
	case productionPlaceFarm:
		return islandAbilityProductFarm
	case productionPlaceOrchard:
		return islandAbilityProductOrchard
	case productionPlaceGarden:
		return islandAbilityProductGarden
	case productionPlaceFishery:
		return islandAbilityProductFish
	}
	return 0
}

// cycleTimes is how long each of count cycles of a formula takes a
// character at a site (IslandProductTimeHelper.CalculateTimeToProductFormula
// without timed buffs): workload / (base_efficiency × (1 + its stat grade's
// effect + its skill + the site's speed abilities, in %)).
func cycleTimes(state *islandState, ship *protobuf.PB_ISLAND_SHIP, formulaID, count, place uint32) []uint32 {
	formula := productionFormulas[formulaID]
	bonus := statGrade(shipStat(ship, formula.Attribute)).Effect + shipSkillEffect(ship, islandBuffShipProductRatio, place)
	if kind := productionSpeedAbility(place); kind != 0 {
		bonus += state.islandAbilityEffects(kind)
	}
	one := uint32(math.Floor(float64(formula.Workload) / (float64(productionBaseEfficiency) * (1 + 0.01*float64(bonus)))))
	times := make([]uint32, count)
	for i := range times {
		times[i] = one
	}
	return times
}

// cycleEnergy is the energy a cycle of a formula costs a character at a site
// (IslandFormulaSelectPage.GetOneFormulaEnergyCost): at least 1.
func cycleEnergy(ship *protobuf.PB_ISLAND_SHIP, formulaID, place uint32) uint32 {
	cut := float64(shipSkillEffect(ship, islandBuffShipProductCostCut, place)) * 0.01
	return max(uint32(math.Floor(float64(productionFormulas[formulaID].StaminaCost)*(1-cut))), 1)
}

// workingShip is a character that can take a delegation: one of the
// island's, or the Workerjuu (islandNpcShipID, IslandNpcShip: every island
// has it without inviting it, always idle with full energy, on several
// slots at once), made up anew each time (never stored, its
// energy never runs out).
func (state *islandState) workingShip(id uint32) *protobuf.PB_ISLAND_SHIP {
	if id == islandNpcShipID {
		ship := &protobuf.PB_ISLAND_SHIP{Id: proto.Uint32(id), Lv: proto.Uint32(1), BreakLv: proto.Uint32(1)}
		ship.Power = proto.Uint32(shipMaxEnergy(ship))
		return ship
	}
	return state.islandShip(id)
}
