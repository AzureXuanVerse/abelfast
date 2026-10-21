package game

import (
	"encoding/json"
	"math/rand/v2"
	"slices"

	"google.golang.org/protobuf/proto"

	"github.com/ggmolly/belfast/internal/protobuf"
)

// Conditions, drops and buff effects of Project Identity (NewEducateChar:
// IsMatchComplex, NewEducateDropHelper, NewEducateBenefit).

// child2_benefit triggers, read from the buff descriptions (child2_benefit_list
// desc) and the wiki's ability / status / tarot lists.
const (
	neTrigPassive  = 1  // always on: gain bonuses
	neTrigPlan     = 2  // when performing an activity (a plan)
	neTrigSchedule = 3  // when setting the schedule (ends the turn)
	neTrigEndPlans = 4  // when the schedule's plans are all done
	neTrigTurn     = 5  // when the turn starts
	neTrigMap      = 6  // when spending AP on the map (site, event)
	neTrigShop     = 7  // when shopping
	neTrigObtain   = 13 // when the buff is obtained
	neTrigPreEval  = 16 // before the assessment
	neTrigPostEval = 17 // after the assessment
	neTrigLevelUp  = 18 // when leveling a reading up
)

// child2_benefit effect types
const (
	neEffAdd       = 1  // {type, id, n}: gain n
	neEffAddFixed  = 2  // same, a fixed amount
	neEffAddPct    = 3  // {type, id, x}: +x/10000 of what's gained (ADD_PPT)
	neEffMultPct   = 4  // {type, id, x}: final gain *(1 + x/10000) (MULT_PPT)
	neEffShare     = 5  // {{type, id, x}, {type, id}}: x/10000 of what was gained of the first, as the second
	neEffOwnPct    = 6  // {{type, id, x}, {type, id}}: x/10000 of what is owned of the second, as the first
	neEffGainBuff  = 14 // buff / pool id: gain it, level up (its own id) or draw from a pool
	neEffPlanCost  = 22 // {type, id, 1 flat / 2 ratio, value}: plan cost (REDUCE_PLAN_COST)
	neEffGoodsCost = 23 // same for the shop (REDUCE_GOODS_COST)
)

// neCtx is what a request is doing, for conditions and effects.
type neCtx struct {
	plan  uint32              // plan being performed
	slot  uint32              // its schedule slot
	gains map[[2]uint32]int64 // what the action gained so far
}

// own is what the game holds of a drop (NewEducateChar:GetOwnCnt).
func (state *neState) own(kind, id uint32) int64 {
	switch kind {
	case neDropAttr:
		return state.Attrs[id]
	case neDropRes:
		return state.Res[id]
	case neDropBuff:
		if state.buff(id) != nil {
			return 1
		}
	}
	return 0
}

func neCompare(a int64, op string, b int64) bool {
	switch op {
	case ">=":
		return a >= b
	case ">":
		return a > b
	case "<=":
		return a <= b
	case "<":
		return a < b
	case "=", "==":
		return a == b
	case "~=", "!=":
		return a != b
	}
	return false
}

// condition evaluates a child2_condition. Types 1 to 5 are the client's
// (NewEducateChar:IsMatchCondition); the others are server-side and guessed
// from the buffs using them: 6 a chance in 10000, 8 the plan performed is
// one of a list, 12 always met, 15 the schedule holds plans of a list, 16
// every n rounds, 17 like 1, 21 owning buffs of a list; the rest (counters
// of what happened during the round) are never met.
func (state *neState) condition(id uint32, ctx *neCtx) bool {
	config, ok := neConditions[id]
	if !ok {
		return false
	}
	var p []json.RawMessage
	_ = json.Unmarshal(config.Param, &p)
	num := func(i int) int64 {
		if i < len(p) {
			return neNumber(p[i])
		}
		return 0
	}
	str := func(i int) string {
		var s string
		if i < len(p) {
			_ = json.Unmarshal(p[i], &s)
		}
		return s
	}
	ids := func(i int) []uint32 {
		var list []uint32
		if i < len(p) {
			_ = json.Unmarshal(p[i], &list)
		}
		return list
	}
	switch config.Type {
	case 1, 17:
		return neCompare(state.own(uint32(num(0)), uint32(num(1))), str(2), num(3))
	case 2:
		return neCompare(state.attrSum(), str(0), num(1))
	case 3:
		return neCompare(int64(state.EventCount[uint32(num(0))]), str(1), num(2))
	case 4:
		return neCompare(int64(state.Round), str(0), num(1))
	case 5:
		var sum int64
		for _, normal := range ids(0) {
			sum += int64(state.WorkCounter[normal])
		}
		return neCompare(sum, str(1), num(2))
	case 6:
		return rand.Int64N(10000) < num(0)
	case 8:
		count := int64(0)
		if ctx != nil && slices.Contains(ids(0), ctx.plan) {
			count = 1
		}
		return neCompare(count, str(1), num(2))
	case 12:
		return true
	case 15:
		var count int64
		for _, kv := range state.PlanList {
			if slices.Contains(ids(0), kv[1]) {
				count++
			}
		}
		return neCompare(count, str(1), num(2))
	case 16:
		n := num(0)
		return n > 0 && int64(state.Round)%n == 0
	case 21:
		var count int64
		for _, buff := range ids(0) {
			if state.buff(buff) != nil {
				count++
			}
		}
		return neCompare(count, str(1), num(2))
	}
	return false
}

// match evaluates a condition field: a condition id, [] (always), or
// {"&&" | "||", [ids or nested fields]}; non-numeric entries ("${num}",
// a count the server scales effects by) are skipped.
func (state *neState) match(raw json.RawMessage, ctx *neCtx) bool {
	var id uint32
	if json.Unmarshal(raw, &id) == nil {
		return state.condition(id, ctx)
	}
	var parts []json.RawMessage
	if json.Unmarshal(raw, &parts) != nil || len(parts) == 0 {
		return true
	}
	var op string
	if json.Unmarshal(parts[0], &op) != nil || len(parts) < 2 {
		return true
	}
	var items []json.RawMessage
	if json.Unmarshal(parts[1], &items) != nil {
		return true
	}
	any, all := false, true
	for _, item := range items {
		var s string
		if json.Unmarshal(item, &s) == nil {
			continue
		}
		if state.match(item, ctx) {
			any = true
		} else {
			all = false
		}
	}
	if op == "||" {
		return any
	}
	return all
}

// benefits calls fn for the effects of the active buffs' benefits with one
// of the triggers whose condition holds. Pending buffs (gained during the
// schedule or the assessment) only act from the next round.
func (state *neState) benefits(ctx *neCtx, fn func(buff uint32, effect neEffect), triggers ...uint32) {
	for _, buff := range slices.Clone(state.Buffs) {
		if buff.Pending {
			continue
		}
		for _, id := range neBuffs[buff.ID].content {
			benefit, ok := neBenefits[id]
			if !ok || !slices.Contains(triggers, benefit.Trigger) || !state.match(benefit.Condition, ctx) {
				continue
			}
			for _, effect := range benefit.effects {
				fn(buff.ID, effect)
			}
		}
	}
}

// bonus is what the buffs' gain bonuses add to drops (effects 3 and 4:
// gained * (1 + sum of 3) * product of (1 + each 4), as
// NewEducateBenefit:GetFinalPct shows them), as benefit drops.
func (state *neState) bonus(drops []neDrop, ctx *neCtx) []neDrop {
	adds := map[[2]uint32]int64{}
	mults := map[[2]uint32][]int64{}
	state.benefits(ctx, func(_ uint32, effect neEffect) {
		if effect.kind != neEffAddPct && effect.kind != neEffMultPct || len(effect.params) == 0 {
			return
		}
		target, ok := neDropOf(effect.params[0])
		if !ok {
			return
		}
		key := [2]uint32{target.Type, target.ID}
		if effect.kind == neEffAddPct {
			adds[key] += target.Number
		} else {
			mults[key] = append(mults[key], target.Number)
		}
	}, neTrigPassive, neTrigPlan, neTrigTurn)
	var extra []neDrop
	for _, drop := range drops {
		if drop.Number <= 0 || (drop.Type != neDropAttr && drop.Type != neDropRes) {
			continue
		}
		key := [2]uint32{drop.Type, drop.ID}
		final := float64(drop.Number) * float64(10000+adds[key]) / 10000
		for _, m := range mults[key] {
			final = final * float64(10000+m) / 10000
		}
		if diff := int64(final) - drop.Number; diff != 0 {
			extra = append(extra, neDrop{drop.Type, drop.ID, max(diff, -drop.Number)})
		}
	}
	return extra
}

// triggered are the drops the active buffs give on a trigger (effects 1, 2,
// 5, 6 and 14).
func (state *neState) triggered(trigger uint32, ctx *neCtx) []neDrop {
	var drops []neDrop
	state.benefits(ctx, func(buff uint32, effect neEffect) {
		drops = append(drops, state.effectDrops(buff, effect, ctx)...)
	}, trigger)
	return drops
}

func (state *neState) effectDrops(buff uint32, effect neEffect, ctx *neCtx) []neDrop {
	switch effect.kind {
	case neEffAdd, neEffAddFixed:
		if len(effect.params) > 0 {
			if drop, ok := neDropOf(effect.params[0]); ok && drop.Number != 0 {
				return []neDrop{drop}
			}
		}
	case neEffShare, neEffOwnPct:
		if len(effect.params) < 2 {
			return nil
		}
		first, ok := neDropOf(effect.params[0])
		var second []uint32
		if !ok || json.Unmarshal(effect.params[1], &second) != nil || len(second) < 2 {
			return nil
		}
		if effect.kind == neEffShare {
			if ctx == nil {
				return nil
			}
			if n := ctx.gains[[2]uint32{first.Type, first.ID}] * first.Number / 10000; n != 0 {
				return []neDrop{{second[0], second[1], n}}
			}
			return nil
		}
		if n := state.own(second[0], second[1]) * first.Number / 10000; n != 0 {
			return []neDrop{{first.Type, first.ID, n}}
		}
	case neEffGainBuff:
		if len(effect.params) == 0 {
			return nil
		}
		var id uint32
		if json.Unmarshal(effect.params[0], &id) != nil {
			return nil
		}
		if id == buff {
			if next := neBuffs[buff].NextLevel; next != 0 {
				return []neDrop{{neDropBuff, buff, -1}, {neDropBuff, next, 1}}
			}
			return nil
		}
		config, ok := neBuffs[id]
		if !ok {
			return nil
		}
		if config.reading {
			return []neDrop{{neDropChoose, id, 1}}
		}
		return []neDrop{{neDropBuff, id, 1}}
	}
	return nil
}

// apply applies a drop as NewEducateDropHelper.HandleDrops does, and tells
// whether it gave a buff.
func (game *neGame) apply(drop neDrop) bool {
	state := game.neState
	switch drop.Type {
	case neDropAttr:
		if _, ok := state.Attrs[drop.ID]; ok || neAttrs[drop.ID].Character == state.ID {
			state.Attrs[drop.ID] += drop.Number
		}
	case neDropRes:
		if _, ok := state.Res[drop.ID]; ok || neResources[drop.ID].Character == state.ID {
			state.Res[drop.ID] += drop.Number
		}
	case neDropPolaroid:
		game.perm.Polaroids = append(game.perm.Polaroids, drop.ID)
	case neDropBuff, neDropTarot:
		config, ok := neBuffs[drop.ID]
		if !ok {
			return false
		}
		if drop.Type == neDropBuff && drop.Number > 0 && config.Type == neBuffTarot && state.tarot() != 0 {
			// NewEducateDropHelper.CheckReplaceTarot: the player picks
			for range drop.Number {
				state.TarotSelects = append(state.TarotSelects, drop.ID)
			}
			return false
		}
		if drop.Number <= 0 {
			state.Buffs = slices.DeleteFunc(state.Buffs, func(b neBuff) bool { return b.ID == drop.ID })
			return false
		}
		if !slices.Contains(game.perm.Archive, drop.ID) {
			game.perm.Archive = append(game.perm.Archive, drop.ID)
		}
		buff := neBuff{ID: drop.ID, Round: state.Round, Pending: !state.immediate()}
		if b := state.buff(drop.ID); b != nil {
			*b = buff
		} else {
			state.Buffs = append(state.Buffs, buff)
		}
		return true
	case neDropChoose:
		for range drop.Number {
			state.Priority = slices.Insert(state.Priority, 0, nePriority{System: neSysPickChoice, Pool: drop.ID})
		}
	case neDropUpEntry:
		for range drop.Number {
			state.Priority = slices.Insert(state.Priority, 0, nePriority{System: neSysUpEntry})
		}
	case neDropTempRound:
		if drop.Number > 0 {
			state.TempRounds += uint32(drop.Number)
		}
	}
	state.clamp()
	return false
}

// give applies drops and their bonuses in the order the client does (base,
// then benefit drops), then the "when obtained" effects (trigger 13) of the
// buffs they gave, and is them as the reply's TBDROPS.
func (game *neGame) give(base, extra []neDrop) *protobuf.TBDROPS {
	var gained []uint32
	for _, list := range [][]neDrop{base, extra} {
		for _, drop := range list {
			if game.apply(drop) {
				gained = append(gained, drop.ID)
			}
		}
	}
	for _, buff := range gained {
		for _, drop := range game.obtained(buff) {
			game.apply(drop)
			extra = append(extra, drop)
		}
	}
	return neDrops(base, extra)
}

// obtained are the drops of a buff's "when obtained" benefits.
func (game *neGame) obtained(buff uint32) []neDrop {
	var drops []neDrop
	for _, id := range neBuffs[buff].content {
		benefit, ok := neBenefits[id]
		if !ok || benefit.Trigger != neTrigObtain || !game.match(benefit.Condition, nil) {
			continue
		}
		for _, effect := range benefit.effects {
			drops = append(drops, game.effectDrops(buff, effect, nil)...)
		}
	}
	return drops
}

func neDropList(drops []neDrop) []*protobuf.TBDROP {
	list := make([]*protobuf.TBDROP, 0, len(drops))
	for _, drop := range drops {
		list = append(list, &protobuf.TBDROP{Type: proto.Uint32(drop.Type), Id: proto.Uint32(drop.ID), Number: proto.Int32(int32(drop.Number))})
	}
	return list
}

func neDrops(base, extra []neDrop) *protobuf.TBDROPS {
	return &protobuf.TBDROPS{BaseDrop: neDropList(base), BenefitDrop: neDropList(extra), Display: &protobuf.TBDISPLAY{}}
}

// reward gives what an action yields: its drops with the gain bonuses, and
// what the buffs give on its trigger (0 none).
func (game *neGame) reward(base []neDrop, trigger uint32, ctx *neCtx) *protobuf.TBDROPS {
	if ctx == nil {
		ctx = &neCtx{}
	}
	extra := game.bonus(base, ctx)
	ctx.gains = map[[2]uint32]int64{}
	for _, list := range [][]neDrop{base, extra} {
		for _, drop := range list {
			ctx.gains[[2]uint32{drop.Type, drop.ID}] += drop.Number
		}
	}
	if trigger != 0 {
		extra = append(extra, game.triggered(trigger, ctx)...)
	}
	return game.give(base, extra)
}

// discount is a cost after the buffs' reductions
// (NewEducateBenefit:GetCommonDiscountInfos, NewEducateHelper.GetBenefitValue).
func neDiscount(cost neDrop, effects []neEffect) neDrop {
	var value, ratio int64
	found := false
	for _, effect := range effects {
		if len(effect.params) == 0 {
			continue
		}
		var p []int64
		if json.Unmarshal(effect.params[0], &p) != nil || len(p) < 4 || uint32(p[0]) != cost.Type || uint32(p[1]) != cost.ID {
			continue
		}
		found = true
		if p[2] == 1 {
			value += p[3]
		} else if p[2] == 2 {
			ratio += p[3]
		}
	}
	if found {
		cost.Number = max(0, int64(float64(cost.Number)*(1+float64(ratio)/10000))+value)
	}
	return cost
}

// planCost is what a plan costs (NewEducatePlan:GetCostWithBenefit with
// NewEducateBenefit:GetPlanDiscountInfos: the REDUCE_PLAN_COST effects of
// every buff whose condition names the plan, the conditions themselves
// unchecked as in the client).
func (state *neState) planCost(plan uint32) []neDrop {
	var effects []neEffect
	for _, buff := range state.Buffs {
		for _, id := range neBuffs[buff.ID].content {
			benefit := neBenefits[id]
			var reduce []neEffect
			for _, effect := range benefit.effects {
				if effect.kind == neEffPlanCost {
					reduce = append(reduce, effect)
				}
			}
			if len(reduce) == 0 {
				continue
			}
			for _, cond := range neConditionIDs(benefit.Condition) {
				c := neConditions[cond]
				if c.Type != 8 && c.Type != 15 {
					continue
				}
				var p []json.RawMessage
				var plans []uint32
				if json.Unmarshal(c.Param, &p) == nil && len(p) > 0 && json.Unmarshal(p[0], &plans) == nil && slices.Contains(plans, plan) {
					effects = append(effects, reduce...)
				}
			}
		}
	}
	var costs []neDrop
	for _, cost := range neDropsOf(nePlans[plan].Cost) {
		costs = append(costs, neDiscount(cost, effects))
	}
	return costs
}

// goodsCost is what one of a shop's goods costs (NewEducateGoods:
// GetCostWithBenefit with the REDUCE_GOODS_COST effects whose condition
// holds).
func (state *neState) goodsCost(goods uint32) neDrop {
	config := neGoods[goods]
	cost := neDrop{neDropRes, state.resOfType(neResMoney), config.Price}
	var effects []neEffect
	state.benefits(nil, func(_ uint32, effect neEffect) {
		if effect.kind == neEffGoodsCost {
			effects = append(effects, effect)
		}
	}, neTrigPassive, neTrigShop, neTrigTurn, neTrigPlan)
	return neDiscount(cost, effects)
}

// neConditionIDs lists the condition ids of a condition field
// (NewEducateChar:GetConditionIdsFromComplex).
func neConditionIDs(raw json.RawMessage) []uint32 {
	var id uint32
	if json.Unmarshal(raw, &id) == nil {
		return []uint32{id}
	}
	var parts []json.RawMessage
	if json.Unmarshal(raw, &parts) != nil || len(parts) < 2 {
		return nil
	}
	var items []json.RawMessage
	if json.Unmarshal(parts[1], &items) != nil {
		return nil
	}
	var ids []uint32
	for _, item := range items {
		ids = append(ids, neConditionIDs(item)...)
	}
	return ids
}

// pay takes costs, refused when one can't be paid.
func (state *neState) pay(costs []neDrop) error {
	for _, cost := range costs {
		if cost.Number > 0 && state.own(cost.Type, cost.ID) < cost.Number {
			return ErrNotEnough
		}
	}
	for _, cost := range costs {
		switch cost.Type {
		case neDropAttr:
			state.Attrs[cost.ID] -= cost.Number
		case neDropRes:
			state.Res[cost.ID] -= cost.Number
		}
	}
	state.clamp()
	return nil
}
