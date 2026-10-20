package game

import (
	"context"
	"encoding/json"
	"slices"

	"google.golang.org/protobuf/proto"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/gamedata"
	"github.com/ggmolly/belfast/internal/protobuf"
)

// Delegations (IslandStartDelegationCommand and the rest of command/
// production): a character works a slot's formula for a number of cycles,
// each taking cycleTimes and costing the formula's commission_cost and
// energy up front. Finished cycles are collected on the way (type 1) or
// once it ends (type 2, from award_list). Island Technologies' research is
// a delegation of the technology's formula on place 702.

// productionReturn is what is collected for a run of finished cycles: the
// cycles (formula_drop_list[1]), the second product's count
// (formula_drop_list[2], every second_product[1] cycles of the formula on
// this slot, once its TYPE_SECOND_PRODUCT ability is there) and the
// animals' extras (main_num, other_num).
type productionReturn struct {
	cycles, second, main, other uint32
}

// shipAppoint is the delegation running on a slot of a site, nil if none.
func shipAppoint(build *protobuf.PB_ISLAND_BUILD, slotID uint32) *protobuf.PB_ISLAND_SHIP_APPOINT {
	for _, appoint := range build.GetShipAppointList() {
		if appoint.GetId() == slotID {
			return appoint
		}
	}
	return nil
}

// appointAward is the finished delegation of a slot waiting to be
// collected, nil if none.
func appointAward(build *protobuf.PB_ISLAND_BUILD, slotID uint32) *protobuf.PB_ISLAND_APPOINT_AREA_AWARD {
	for _, award := range build.GetAwardList() {
		if award.GetId() == slotID {
			return award
		}
	}
	return nil
}

// cyclesDone counts the cycles of a delegation finished by now
// (IslandRoleDelegationData.GetCountByTimestamp).
func cyclesDone(appoint *protobuf.PB_ISLAND_SHIP_APPOINT, now uint32) uint32 {
	elapsed := int64(now) - int64(appoint.GetStartTime())
	sum, done := int64(0), uint32(0)
	for _, length := range appoint.GetCostTimeList() {
		sum += int64(length)
		if sum > elapsed {
			break
		}
		done++
	}
	return done
}

// appointEnd is when a delegation's last cycle ends.
func appointEnd(appoint *protobuf.PB_ISLAND_SHIP_APPOINT) uint32 {
	end := appoint.GetStartTime()
	for _, length := range appoint.GetCostTimeList() {
		end += length
	}
	return end
}

// formulaCount is how many cycles of a formula a slot has made
// (PB_ISLAND_APPOINT_AREA.formula_list, IslandRoleDelegationSlot.formula_dic).
func formulaCount(area *protobuf.PB_ISLAND_APPOINT_AREA, formulaID uint32) uint32 {
	for _, entry := range area.GetFormulaList() {
		if entry.GetId() == formulaID {
			return entry.GetNum()
		}
	}
	return 0
}

// secondEvery is a formula's second product period (second_product[1]), 0
// for none.
func secondEvery(formulaID uint32) uint32 {
	var raw []json.RawMessage
	if json.Unmarshal(productionFormulas[formulaID].SecondProduct, &raw) != nil || len(raw) == 0 {
		return 0
	}
	var every uint32
	_ = json.Unmarshal(raw[0], &every)
	return every
}

// secondUnlocked tells whether the island has a formula's second product
// (IslandAblityAgency.IsUnlcokSecondProduct).
func (state *islandState) secondUnlocked(formulaID uint32) bool {
	for _, id := range state.public().GetAbilityList() {
		if ability := productionAbilities[id]; ability.Type == islandAbilitySecondProduct && ability.Effect == formulaID {
			return true
		}
	}
	return false
}

// delegationReturn is what cycles [from, to) of a delegation make.
func (state *islandState) delegationReturn(area *protobuf.PB_ISLAND_APPOINT_AREA, appoint *protobuf.PB_ISLAND_SHIP_APPOINT, from, to uint32) productionReturn {
	formulaID := appoint.GetFormulaId()
	made := productionReturn{cycles: to - from}
	every := secondEvery(formulaID)
	second := every > 0 && state.secondUnlocked(formulaID)
	total := formulaCount(area, formulaID)
	extras := appoint.GetTimesExtra()
	for i := from; i < to; i++ {
		if int(i) < len(extras) {
			made.main += extras[i].GetMainExtra()
		}
		if second && (total+i-from+1)%every == 0 {
			made.second++
			if int(i) < len(extras) {
				made.other += extras[i].GetOtherExtra()
			}
		}
	}
	return made
}

// returnDrops are the rewards of finished cycles: the main product
// (commission_product) and the second (second_product_display, GUESS: the
// display is what is given).
func returnDrops(formulaID uint32, made productionReturn) []*protobuf.DROPINFO {
	formula := productionFormulas[formulaID]
	drops := productionItemDrops(productionPairs(formula.CommissionProduct), made.cycles, made.main)
	if made.second > 0 {
		drops = append(drops, productionItemDrops(productionPairs(formula.SecondProductDisplay), made.second, made.other)...)
	}
	return drops
}

// partEffects are the ranch animals' extras on each of count cycles
// (IslandFormulaSelectPage.GetAniExtraGainByConfigName): only at the
// pasture. GUESS: num is 1 per cycle.
func partEffects(place uint32, area *protobuf.PB_ISLAND_APPOINT_AREA, count uint32) []*protobuf.PB_ISLAND_PART_EFFECT {
	if place != productionPlacePasture {
		return nil
	}
	var cost, main, other uint32
	for _, animal := range area.GetPartList() {
		config := productionAnimals[animal]
		cost += config.EfficiencyCost
		main += config.EfficiencyGainsNum
		other += config.EfficiencyGainsBonusNum
	}
	effects := make([]*protobuf.PB_ISLAND_PART_EFFECT, count)
	for i := range effects {
		effects[i] = &protobuf.PB_ISLAND_PART_EFFECT{
			Num: proto.Uint32(1), CostExtra: proto.Uint32(cost), MainExtra: proto.Uint32(main), OtherExtra: proto.Uint32(other),
		}
	}
	return effects
}

// animalCost is the extra commission_cost per cycle the animals take.
func animalCost(place uint32, area *protobuf.PB_ISLAND_APPOINT_AREA) uint32 {
	if place != productionPlacePasture {
		return 0
	}
	cost := uint32(0)
	for _, animal := range area.GetPartList() {
		cost += productionAnimals[animal].EfficiencyCost
	}
	return cost
}

// handBusy tells whether one of the fields sharing a delegation slot's
// ground (exclusion_slot) is sown (IslandRoleDelegationSlot.CheckChildSlotIsInWork).
func handBusy(build *protobuf.PB_ISLAND_BUILD, slotID uint32) bool {
	exclusion := productionSlotExclusion(slotID)
	for _, area := range build.GetHandList() {
		if area.GetState() == 1 && slices.Contains(exclusion, area.GetId()) {
			return true
		}
	}
	return false
}

// takeCommissionCost takes count cycles' commission_cost (plus extra per
// cycle and item) from the warehouse.
func (state *islandState) takeCommissionCost(formulaID, extra, count uint32) error {
	pairs := productionPairs(productionFormulas[formulaID].CommissionCost)
	for _, pair := range pairs {
		if state.islandItemCount(pair[0]) < (pair[1]+extra)*count {
			return ErrNotEnough
		}
	}
	for _, pair := range pairs {
		if err := state.takeIslandItem(pair[0], (pair[1]+extra)*count); err != nil {
			return err
		}
	}
	return nil
}

// handleIslandStartDelegation answers IslandStartDelegationCommand
// (CS_21501): a character starts num cycles of a formula on a free
// delegation slot of a site. The slot's fields must be empty, the formula
// one of the slot's and unlocked, num within its production_limit, the
// character idle with the energy for every cycle, the warehouse holding
// every cycle's commission_cost (plus the ranch animals' extra). The reply
// has the character's energy left (ship_power) and the delegation; the
// client takes the costs itself and the others on the island see it start
// (SC_21220).
func (game *Game) handleIslandStartDelegation(client *connection.Client, req *protobuf.CS_21501) error {
	reply := &protobuf.SC_21502{}
	var appoint *protobuf.PB_ISLAND_SHIP_APPOINT
	err := game.tx(context.Background(), client, func(tx *Tx) error {
		state, err := tx.island()
		if err != nil {
			return err
		}
		now := productionNow()
		place, slotID, formulaID, num := req.GetBuildId(), req.GetAreaId(), req.GetFormulaId(), req.GetNum()
		build := state.building(place)
		slot := productionSlots[slotID]
		formula, known := productionFormulas[formulaID]
		ship := state.workingShip(req.GetShipId())
		if build == nil || slot.Place != place || appointArea(build, slotID) == nil || shipAppoint(build, slotID) != nil ||
			appointAward(build, slotID) != nil || handBusy(build, slotID) || !known ||
			!slices.Contains(productionSlotFormulas(slot), formulaID) || !state.formulaUnlocked(formulaID) ||
			num == 0 || num > max(formula.ProductionLimit, 1) || ship == nil || ship.GetId() != islandNpcShipID && (shipBusy(ship, now) || state.shipWorking(ship.GetId())) {
			reply.Result = result(errRefused)
			return nil
		}
		// a technology researched as often as it can be (IslandTechnology.CheckRemainCnt)
		if place == productionPlaceTechnoloy {
			tech, ok := productionTechs[productionTechByFormula[formulaID]]
			done := state.techFinished(productionTechByFormula[formulaID])
			if !ok || (tech.once() && done > 0) || (!tech.once() && len(tech.TechRepeat) > 1 && tech.TechRepeat[1] > 0 && done >= tech.TechRepeat[1]) {
				reply.Result = result(errRefused)
				return nil
			}
		}
		area := appointArea(build, slotID)
		energy := shipEnergy(ship, now)
		need := cycleEnergy(ship, formulaID, place) * num
		if energy < need {
			reply.Result = result(ErrNotEnough)
			return nil
		}
		if err := state.takeCommissionCost(formulaID, animalCost(place, area), num); err != nil {
			reply.Result = result(err)
			return nil
		}
		appoint = &protobuf.PB_ISLAND_SHIP_APPOINT{
			Id: proto.Uint32(slotID), ShipId: proto.Uint32(ship.GetId()), MaxTimes: proto.Uint32(num), GetTimes: proto.Uint32(0),
			FormulaId: proto.Uint32(formulaID), StartTime: proto.Uint32(now), SpeedTime: proto.Uint32(0),
			CostTimeList: cycleTimes(state, ship, formulaID, num, place), TimesExtra: partEffects(place, area, num),
		}
		build.ShipAppointList = append(build.ShipAppointList, appoint)
		setShipEnergy(ship, energy-need, appointEnd(appoint))
		// the client's own choice (IslandStartDelegationCommand)
		work := uint32(islandShipTechnology)
		if place == productionPlaceTechnoloy {
			work = islandShipDelegation
		}
		ship.WorkPlace = &protobuf.PB_SHIP_WORK_PLACE{Type: proto.Uint32(work), Place: proto.Uint32(place)}
		reply.Result = result(nil)
		reply.ShipPower = proto.Uint32(energy - need)
		reply.ShipAppoint = proto.Clone(appoint).(*protobuf.PB_ISLAND_SHIP_APPOINT)
		if err := tx.islandTaskEvent(islandTaskStartDelegate, 1, place, formulaID); err != nil {
			return err
		}
		if err := tx.islandTaskEvent(islandTaskDelegateIn, 1, place); err != nil {
			return err
		}
		if err := tx.islandAchievementEvent(islandAchievementShipDispatch, 0, 1); err != nil {
			return err
		}
		tx.pushProductionTips(state)
		return nil
	})
	if err != nil {
		return err
	}
	if appoint != nil {
		game.tellIsland(client, client.CommanderID, &protobuf.SC_21220{IslandId: proto.Uint32(client.CommanderID), AppointData: appoint})
	}
	return client.Send(islandStamped(reply))
}

// shipWorking tells whether a character runs a delegation anywhere.
func (state *islandState) shipWorking(shipID uint32) bool {
	for _, build := range state.public().GetBuildList() {
		for _, appoint := range build.GetShipAppointList() {
			if appoint.GetShipId() == shipID {
				return true
			}
		}
	}
	return false
}

// handleIslandAddDelegation answers IslandAddDelegationCommand (CS_21537):
// add_num more cycles of the formula a delegation runs, paid like at the
// start. The reply has the new cycles' times and the animals' extras; the
// client takes the costs and the energy itself. GUESS: the cycles left may
// not exceed production_limit.
func (game *Game) handleIslandAddDelegation(client *connection.Client, req *protobuf.CS_21537) error {
	reply := &protobuf.SC_21538{}
	err := game.tx(context.Background(), client, func(tx *Tx) error {
		state, err := tx.island()
		if err != nil {
			return err
		}
		now := productionNow()
		place, slotID, add := req.GetBuildId(), req.GetAreaId(), req.GetAddNum()
		build := state.building(place)
		var appoint *protobuf.PB_ISLAND_SHIP_APPOINT
		if build != nil {
			appoint = shipAppoint(build, slotID)
		}
		if appoint == nil || add == 0 {
			reply.Result = result(errRefused)
			return nil
		}
		formulaID := appoint.GetFormulaId()
		left := uint32(len(appoint.GetCostTimeList())) - cyclesDone(appoint, now)
		ship := state.workingShip(appoint.GetShipId())
		if ship == nil || left == 0 || left+add > max(productionFormulas[formulaID].ProductionLimit, 1) {
			reply.Result = result(errRefused)
			return nil
		}
		need := cycleEnergy(ship, formulaID, place) * add
		energy := min(ship.GetPower(), shipMaxEnergy(ship))
		if energy < need {
			reply.Result = result(ErrNotEnough)
			return nil
		}
		area := appointArea(build, slotID)
		if err := state.takeCommissionCost(formulaID, animalCost(place, area), add); err != nil {
			reply.Result = result(err)
			return nil
		}
		times := cycleTimes(state, ship, formulaID, add, place)
		extras := partEffects(place, area, add)
		appoint.CostTimeList = append(appoint.CostTimeList, times...)
		appoint.TimesExtra = append(appoint.TimesExtra, extras...)
		appoint.MaxTimes = proto.Uint32(appoint.GetMaxTimes() + add)
		setShipEnergy(ship, energy-need, appointEnd(appoint))
		reply.Result = result(nil)
		reply.CostTimeList = times
		for _, extra := range extras {
			reply.TimesExtra = append(reply.TimesExtra, proto.Clone(extra).(*protobuf.PB_ISLAND_PART_EFFECT))
		}
		tx.pushProductionTips(state)
		return nil
	})
	if err != nil {
		return err
	}
	return client.Send(islandStamped(reply))
}

// handleIslandFinishDelegation answers IslandFinishDelegationCommand
// (CS_21503): a delegation ends, by itself (the client sends it once its
// last cycle is done) or stopped. The character gets the exp of every cycle
// it finished (ship_exp each, GUESS: given here, the award only shows it),
// and for the cycles it didn't run (return_num) its energy (GUESS) and the
// commission_cost back to the warehouse (added by the client too); the
// finished cycles not collected yet wait in award_list.
func (game *Game) handleIslandFinishDelegation(client *connection.Client, req *protobuf.CS_21503) error {
	reply := &protobuf.SC_21504{}
	err := game.tx(context.Background(), client, func(tx *Tx) error {
		state, err := tx.island()
		if err != nil {
			return err
		}
		now := productionNow()
		place, slotID := req.GetBuildId(), req.GetAreaId()
		build := state.building(place)
		var appoint *protobuf.PB_ISLAND_SHIP_APPOINT
		if build != nil {
			appoint = shipAppoint(build, slotID)
		}
		if appoint == nil {
			reply.Result = result(errRefused)
			return nil
		}
		formulaID := appoint.GetFormulaId()
		formula := productionFormulas[formulaID]
		total := uint32(len(appoint.GetCostTimeList()))
		done := cyclesDone(appoint, now)
		left := total - done
		ship := state.workingShip(appoint.GetShipId())
		reply.Result = result(nil)
		reply.ShipId = proto.Uint32(appoint.GetShipId())
		reply.ReturnNum = proto.Uint32(left)
		exp := formula.ShipExp * (appoint.GetGetTimes() + done)
		reply.AddExp = proto.Uint32(exp)
		if done > 0 {
			made := state.delegationReturn(appointArea(build, slotID), appoint, 0, done)
			award := &protobuf.PB_ISLAND_APPOINT_AREA_AWARD{
				Id: proto.Uint32(slotID), ShipId: proto.Uint32(appoint.GetShipId()), Exp: proto.Uint32(exp),
				FormulaId: proto.Uint32(formulaID), MainNum: proto.Uint32(made.main), OtherNum: proto.Uint32(made.other),
				FormulaDropList: []*protobuf.PB_FORMULA_DROP_INFO{{Id: proto.Uint32(formulaID), Num: proto.Uint32(made.cycles)}},
			}
			if made.second > 0 {
				award.FormulaDropList = append(award.FormulaDropList, &protobuf.PB_FORMULA_DROP_INFO{Id: proto.Uint32(formulaID), Num: proto.Uint32(made.second)})
			}
			build.AwardList = append(build.AwardList, award)
			reply.Award = []*protobuf.PB_ISLAND_APPOINT_AREA_AWARD{proto.Clone(award).(*protobuf.PB_ISLAND_APPOINT_AREA_AWARD)}
		}
		if left > 0 {
			// IslandFinishDelegationCommand adds the same back itself
			extra := uint32(0)
			for i := done; i < total && int(i) < len(appoint.GetTimesExtra()); i++ {
				extra += appoint.GetTimesExtra()[i].GetCostExtra()
			}
			var refund []*protobuf.DROPINFO
			for _, pair := range productionPairs(formula.CommissionCost) {
				if count := pair[1]*left + extra; count > 0 {
					refund = append(refund, &protobuf.DROPINFO{Type: proto.Uint32(dropTypeIslandItem), Id: proto.Uint32(pair[0]), Number: proto.Uint32(count)})
				}
			}
			if err := tx.grantDrops(refund); err != nil {
				return err
			}
		}
		if ship != nil {
			shipAddExp(ship, exp)
			if left > 0 {
				energy := min(ship.GetPower()+cycleEnergy(ship, formulaID, place)*left, shipMaxEnergy(ship))
				setShipEnergy(ship, energy, now)
			}
			ship.WorkPlace = &protobuf.PB_SHIP_WORK_PLACE{Type: proto.Uint32(islandShipNormal), Place: proto.Uint32(0)}
			reply.CurEnergy = proto.Uint32(ship.GetPower())
			reply.RecoverTime = proto.Uint32(ship.GetRecoverTime())
		}
		build.ShipAppointList = slices.DeleteFunc(build.ShipAppointList, func(entry *protobuf.PB_ISLAND_SHIP_APPOINT) bool { return entry == appoint })
		tx.pushProductionTips(state)
		return nil
	})
	if err != nil {
		return err
	}
	return client.Send(islandStamped(reply))
}

// handleIslandDelegationAward answers IslandGetDelegationAwardCommand
// (CS_21505): the finished cycles of a delegation collected, on the way
// (type 1: the delegation goes on with the cycles left, from start_time
// with rest_time_list) or after it ended (type 2: its award_list entry).
// drop_list has the products, pt_award the season points (both added by the
// client); the slot's formula count goes up (SC_21518), and a technology
// researched on place 702 counts as finished once more.
func (game *Game) handleIslandDelegationAward(client *connection.Client, req *protobuf.CS_21505) error {
	reply := &protobuf.SC_21506{}
	err := game.tx(context.Background(), client, func(tx *Tx) error {
		state, err := tx.island()
		if err != nil {
			return err
		}
		now := productionNow()
		place, slotID := req.GetBuildId(), req.GetAreaId()
		build := state.building(place)
		if build == nil || appointArea(build, slotID) == nil {
			reply.Result = result(errRefused)
			return nil
		}
		area := appointArea(build, slotID)
		var formulaID uint32
		var made productionReturn
		switch req.GetType() {
		case 1:
			appoint := shipAppoint(build, slotID)
			if appoint == nil {
				reply.Result = result(errRefused)
				return nil
			}
			done := cyclesDone(appoint, now)
			if done == 0 {
				reply.Result = result(errRefused)
				return nil
			}
			formulaID = appoint.GetFormulaId()
			made = state.delegationReturn(area, appoint, 0, done)
			// IslandRoleDelegationData.OnGetAwardMidway
			start := appoint.GetStartTime()
			for _, length := range appoint.CostTimeList[:done] {
				start += length
			}
			appoint.StartTime = proto.Uint32(start)
			appoint.CostTimeList = slices.Clone(appoint.CostTimeList[done:])
			if int(done) <= len(appoint.TimesExtra) {
				appoint.TimesExtra = appoint.TimesExtra[done:]
			} else {
				appoint.TimesExtra = nil
			}
			appoint.GetTimes = proto.Uint32(appoint.GetGetTimes() + done)
			reply.StartTime = proto.Uint32(start)
			reply.RestTimeList = slices.Clone(appoint.CostTimeList)
		case 2:
			award := appointAward(build, slotID)
			if award == nil {
				reply.Result = result(errRefused)
				return nil
			}
			formulaID = award.GetFormulaId()
			made = productionReturn{main: award.GetMainNum(), other: award.GetOtherNum()}
			if list := award.GetFormulaDropList(); len(list) > 0 {
				made.cycles = list[0].GetNum()
				if len(list) > 1 {
					made.second = list[1].GetNum()
				}
			}
			build.AwardList = slices.DeleteFunc(build.AwardList, func(entry *protobuf.PB_ISLAND_APPOINT_AREA_AWARD) bool { return entry == award })
		default:
			reply.Result = result(errRefused)
			return nil
		}
		formula := productionFormulas[formulaID]
		drops := returnDrops(formulaID, made)
		if place == productionPlaceTechnoloy {
			techDrops, err := tx.finishTech(state, productionTechByFormula[formulaID])
			if err != nil {
				return err
			}
			drops = append(drops, techDrops...)
		}
		pt := formula.PtAward * made.cycles
		granted := slices.Clone(drops)
		if pt > 0 {
			granted = append(granted, &protobuf.DROPINFO{Type: proto.Uint32(dropTypeIslandSeasonPt), Id: proto.Uint32(0), Number: proto.Uint32(pt)})
		}
		if err := tx.grantDrops(granted); err != nil {
			return err
		}
		// the slot's and the island's formula counts (SC_21518)
		found := false
		for _, entry := range area.FormulaList {
			if entry.GetId() == formulaID {
				entry.Num = proto.Uint32(entry.GetNum() + made.cycles)
				found = true
			}
		}
		if !found {
			area.FormulaList = append(area.FormulaList, &protobuf.PB_USE_FORMULA{Id: proto.Uint32(formulaID), Num: proto.Uint32(made.cycles)})
		}
		if formula.IsCondition == 1 {
			private := state.private()
			found = false
			for _, entry := range private.FormulaNum {
				if entry.GetId() == formulaID {
					entry.Num = proto.Uint32(entry.GetNum() + made.cycles)
					found = true
				}
			}
			if !found {
				private.FormulaNum = append(private.FormulaNum, &protobuf.PB_USE_FORMULA{Id: proto.Uint32(formulaID), Num: proto.Uint32(made.cycles)})
			}
		}
		tx.push(&protobuf.SC_21518{
			AreaId: proto.Uint32(slotID), FormulaId: proto.Uint32(formulaID),
			Num: proto.Uint32(made.cycles), CombNum: proto.Uint32(made.cycles),
		})
		if err := tx.productionMadeEvents(place, formulaID, made.cycles, drops); err != nil {
			return err
		}
		tx.pushProductionTips(state)
		reply.Result = result(nil)
		reply.DropList = drops
		reply.GetTimes = proto.Uint32(made.cycles)
		reply.PtAward = proto.Uint32(pt)
		reply.FormulaId = proto.Uint32(formulaID)
		return nil
	})
	if err != nil {
		return err
	}
	return client.Send(islandStamped(reply))
}

// productionMadeEvents counts what a site made for the island tasks: the
// formula's cycles (PRODUCTION), the items made (PROD_ITEM) and made at
// the site (ITEM_IN_PLACE).
func (tx *Tx) productionMadeEvents(place, formulaID, cycles uint32, drops []*protobuf.DROPINFO) error {
	if cycles > 0 {
		if err := tx.islandTaskEvent(islandTaskProduction, cycles, formulaID); err != nil {
			return err
		}
	}
	for _, drop := range drops {
		if err := tx.islandTaskEvent(islandTaskProdItem, drop.GetNumber(), drop.GetId()); err != nil {
			return err
		}
		if err := tx.islandTaskEvent(islandTaskItemInPlace, drop.GetNumber(), place); err != nil {
			return err
		}
	}
	return nil
}

// productionSlotExclusion is a slot's exclusion_slot.
func productionSlotExclusion(slotID uint32) []uint32 {
	return gamedata.IDs(productionSlots[slotID].ExclusionSlot)
}

// productionTipPlaces are island_set post_manage_produce: the sites the
// main screen's tip counts.
var productionTipPlaces []uint32

// pushProductionTips pushes the main screen's island tip (SC_21536,
// SystemTipProxy): the finished delegations waiting (get_num), the free
// slots (empty_num) and when the running ones end (get_times) on the
// post_manage_produce sites (IslandBuildingAgency.GetTipInfos), and whether
// the island has the management ability (mange_flag). GUESS: pushed after
// each change of a delegation.
func (tx *Tx) pushProductionTips(state *islandState) {
	tip := &protobuf.SC_21536{GetNum: proto.Uint32(0), EmptyNum: proto.Uint32(0), MangeFlag: proto.Uint32(0)}
	for _, place := range productionTipPlaces {
		build := state.building(place)
		if build == nil {
			continue
		}
		for _, area := range build.GetAppointList() {
			appoint := shipAppoint(build, area.GetId())
			switch {
			case appointAward(build, area.GetId()) != nil:
				tip.GetNum = proto.Uint32(tip.GetGetNum() + 1)
			case appoint == nil && !handBusy(build, area.GetId()):
				tip.EmptyNum = proto.Uint32(tip.GetEmptyNum() + 1)
			case appoint != nil:
				tip.GetTimes = append(tip.GetTimes, appointEnd(appoint))
			}
		}
	}
	if state.hasAbilityType(islandAbilityPostManage) {
		tip.MangeFlag = proto.Uint32(1)
	}
	tx.push(tip)
}

// hasAbilityType tells whether the island has an ability of a type.
func (state *islandState) hasAbilityType(kind uint32) bool {
	for _, id := range state.public().GetAbilityList() {
		if productionAbilities[id].Type == kind {
			return true
		}
	}
	return false
}
