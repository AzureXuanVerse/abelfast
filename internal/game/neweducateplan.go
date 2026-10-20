package game

import (
	"encoding/json"
	"slices"

	"google.golang.org/protobuf/proto"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/protobuf"
)

// The schedule (NewEducateScheduleScene), the assessment and the next round.

// availablePlans are the plans the schedule offers (NewEducateChar:
// GetPlanList): each of the round's plan groups at its level, and the extra
// plans of the buffs (effect 7).
func (state *neState) availablePlans() []uint32 {
	var plans []uint32
	for _, group := range state.roundConfig().PlanGroups {
		ids := nePlanGroups[group]
		switch {
		case len(ids) == 0:
		case state.Plans[group] != 0:
			plans = append(plans, state.Plans[group])
		default:
			plans = append(plans, ids[0])
		}
	}
	state.benefits(nil, func(_ uint32, effect neEffect) {
		if effect.kind != 7 || len(effect.params) == 0 {
			return
		}
		var ids []uint32
		if json.Unmarshal(effect.params[0], &ids) == nil {
			plans = append(plans, ids...)
		}
	}, neTrigPassive, neTrigPlan, neTrigTurn, neTrigSchedule)
	return plans
}

// free tells whether the round's free part is on (map, topic, mind):
// nothing running.
func (state *neState) free() bool {
	if state.Node != 0 || len(state.Priority) > 0 || len(state.TarotSelects) > 0 {
		return false
	}
	switch state.System {
	case neSysChoose, neSysTalent, neSysTopic, neSysMind, neSysMap:
		return true
	}
	return false
}

// handleNewEducateSchedule answers NewEducateScheduleCommand: the round's
// plans by slot, paid now with the buffs' reductions (as the client does);
// the schedule trigger's effects come with it.
func (game *Game) handleNewEducateSchedule(client *connection.Client, req *protobuf.CS_29040) error {
	var drops *protobuf.TBDROPS
	return game.neReply(client, req.GetId(), func(tx *Tx, ne *neGame) error {
		if !ne.free() {
			return errRefused
		}
		available := ne.availablePlans()
		round := ne.roundConfig()
		var list [][2]uint32
		var costs []neDrop
		for _, kv := range req.GetPlans() {
			slot, plan := kv.GetKey(), kv.GetValue()
			if slot == 0 || slot > max(round.PlanNum, 1) || !slices.Contains(available, plan) || slices.ContainsFunc(list, func(p [2]uint32) bool { return p[0] == slot }) {
				return errRefused
			}
			list = append(list, [2]uint32{slot, plan})
			costs = append(costs, ne.planCost(plan)...)
		}
		if err := ne.pay(costs); err != nil {
			return err
		}
		slices.SortFunc(list, func(a, b [2]uint32) int { return int(a[0]) - int(b[0]) })
		ne.System = neSysPlan
		ne.PlanSet, ne.PlanCur, ne.PlanList = true, 0, list
		drops = ne.give(nil, ne.triggered(neTrigSchedule, nil))
		return nil
	}, func(err error) proto.Message {
		return &protobuf.SC_29041{Result: result(err), Plans: req.GetPlans(), Drop: drops}
	})
}

// planDone tells whether every plan of the schedule was performed
// (NewEducatePlanState:IsFinish).
func (state *neState) planDone() bool {
	return len(state.PlanList) == 0 || state.PlanCur == state.PlanList[len(state.PlanList)-1][0]
}

// nextPlan moves to the schedule's next plan (NewEducatePlanState:
// SetNextPlanIdx).
func (state *neState) nextPlan() (slot, plan uint32) {
	for i, kv := range state.PlanList {
		if state.PlanCur == 0 || i > 0 && state.PlanList[i-1][0] == state.PlanCur {
			state.PlanCur = kv[0]
			return kv[0], kv[1]
		}
	}
	return 0, 0
}

// handleNewEducateNextPlan answers NewEducateNextPlanCommand: the next plan
// of the schedule plays its chain (child2_plan result_node), whose drop
// node gives its result_display with the gain bonuses and the activity
// trigger's effects.
func (game *Game) handleNewEducateNextPlan(client *connection.Client, req *protobuf.CS_29042) error {
	var node uint32
	var drops *protobuf.TBDROPS
	return game.neReply(client, req.GetId(), func(tx *Tx, ne *neGame) error {
		if ne.System != neSysPlan || !ne.PlanSet || ne.Node != 0 || ne.planDone() {
			return errRefused
		}
		slot, plan := ne.nextPlan()
		config := nePlans[plan]
		node = config.Node
		drops = ne.startReward(node, neDropsOf(config.Result), neTrigPlan, &neCtx{plan: plan, slot: slot})
		return nil
	}, func(err error) proto.Message {
		return &protobuf.SC_29043{Result: result(err), FirstNode: proto.Uint32(node), Drop: drops}
	})
}

// handleNewEducateScheduleSkip answers NewEducateScheduleSkipCommand (the
// schedule's skip toggle): every plan left is performed at once.
func (game *Game) handleNewEducateScheduleSkip(client *connection.Client, req *protobuf.CS_29046) error {
	drops := neDrops(nil, nil)
	return game.neReply(client, req.GetId(), func(tx *Tx, ne *neGame) error {
		if ne.System != neSysPlan || !ne.PlanSet {
			return errRefused
		}
		ne.startChain(0, nil, nil)
		for !ne.planDone() {
			slot, plan := ne.nextPlan()
			got := ne.reward(neDropsOf(nePlans[plan].Result), neTrigPlan, &neCtx{plan: plan, slot: slot})
			drops.BaseDrop = append(drops.BaseDrop, got.BaseDrop...)
			drops.BenefitDrop = append(drops.BenefitDrop, got.BenefitDrop...)
		}
		return nil
	}, func(err error) proto.Message { return &protobuf.SC_29047{Result: result(err), Drop: drops} })
}

// handleNewEducateGetExtraDrop answers NewEducateGetExtraDropCommand, after
// the schedule's last plan: what the buffs give once the plans are done.
func (game *Game) handleNewEducateGetExtraDrop(client *connection.Client, req *protobuf.CS_29048) error {
	var drops *protobuf.TBDROPS
	var res *protobuf.TBRES
	return game.neReply(client, req.GetId(), func(tx *Tx, ne *neGame) error {
		if ne.System != neSysPlan || !ne.planDone() || ne.Node != 0 {
			return errRefused
		}
		drops = neDrops(nil, nil)
		if !ne.ExtraDone {
			ne.ExtraDone = true
			drops = ne.give(nil, ne.triggered(neTrigEndPlans, nil))
		}
		res = ne.res()
		return nil
	}, func(err error) proto.Message { return &protobuf.SC_29049{Result: result(err), Drop: drops, Res: res} })
}

// handleNewEducateEnterAssess answers NewEducateEnterAssessCommand, after
// the schedule: the assessment starts (done at once in a round without
// one), with what the buffs give before it.
func (game *Game) handleNewEducateEnterAssess(client *connection.Client, req *protobuf.CS_29050) error {
	drops := neDrops(nil, nil)
	return game.neReply(client, req.GetId(), func(tx *Tx, ne *neGame) error {
		if ne.System == neSysAssess {
			return nil
		}
		if ne.System != neSysPlan || !ne.planDone() || ne.Node != 0 {
			return errRefused
		}
		ne.System = neSysAssess
		done := ne.assessRank() == 0
		ne.Eval = &done
		if !done {
			drops = ne.give(nil, ne.triggered(neTrigPreEval, nil))
		}
		return nil
	}, func(err error) proto.Message { return &protobuf.SC_29051{Result: result(err), Drop: drops} })
}

// handleNewEducateAssess answers NewEducateAssessCommand: the assessment's
// rank (child2_target attr_sum_level, recomputed) and its chain
// (result_node), kept by round; in endless mode a total under the target
// fails the run.
func (game *Game) handleNewEducateAssess(client *connection.Client, req *protobuf.CS_29013) error {
	var node uint32
	drops := neDrops(nil, nil)
	return game.neReply(client, req.GetId(), func(tx *Tx, ne *neGame) error {
		if ne.System != neSysAssess || ne.Eval == nil || *ne.Eval || ne.Node != 0 {
			return errRefused
		}
		rank := ne.assessRank()
		target := neTargets[ne.roundConfig().Target]
		if rank == 0 || rank > len(target.Nodes) {
			return errRefused
		}
		done := true
		ne.Eval = &done
		ne.Evaluations[ne.Round] = uint32(rank)
		if ne.endless() && float64(ne.attrSum()) < float64(target.AttrSum)*ne.extraFactor() {
			ne.EvalFail = true
		}
		node = target.Nodes[rank-1]
		ne.startChain(node, nil, nil)
		drops = ne.give(nil, ne.triggered(neTrigPostEval, nil))
		return nil
	}, func(err error) proto.Message {
		return &protobuf.SC_29014{Result: result(err), FirstNode: proto.Uint32(node), Drop: drops}
	})
}

// handleNewEducateChangePhase answers NewEducateChangePhaseCommand: the next
// round starts (after the assessment, or from an ending into endless mode),
// with its stage chain (stage_change_event_node_id) and what the buffs give
// at the turn's start.
func (game *Game) handleNewEducateChangePhase(client *connection.Client, req *protobuf.CS_29025) error {
	var node uint32
	drops := neDrops(nil, nil)
	return game.neReply(client, req.GetId(), func(tx *Tx, ne *neGame) error {
		if ne.Node != 0 || ne.EvalFail {
			return errRefused
		}
		switch {
		case ne.System == neSysAssess && ne.Eval != nil && *ne.Eval && (!ne.lastRound() || ne.endless() || ne.TempRounds > 0):
		case ne.System == neSysPhase:
		case ne.System == neSysEnding && ne.End != nil && ne.End.Select != 0 && len(neEndlessIDs(ne.ID, ne.Difficulty)) > 0:
		default:
			return errRefused
		}
		ne.System = neSysPhase
		ne.nextRound()
		node = ne.roundConfig().StageNode
		ne.startChain(node, nil, nil)
		drops = ne.give(nil, ne.triggered(neTrigTurn, nil))
		return nil
	}, func(err error) proto.Message {
		return &protobuf.SC_29026{Result: result(err), FirstNode: proto.Uint32(node), Drop: drops}
	})
}

// handleNewEducateUpgradePlan answers NewEducateUpgradePlanCommand, sent by
// the schedule when plans can level up: each one goes up while the level
// it reached has its level_condition met (NewEducateUpgradePlanCommand
// does the same).
func (game *Game) handleNewEducateUpgradePlan(client *connection.Client, req *protobuf.CS_29044) error {
	return game.neReply(client, req.GetId(), func(tx *Tx, ne *neGame) error {
		available := ne.availablePlans()
		for _, id := range req.GetPlanIds() {
			plan, ok := nePlans[id]
			if !ok || !slices.Contains(available, id) {
				return errRefused
			}
			levels := nePlanGroups[plan.Group]
			i := slices.Index(levels, id)
			if i+1 >= len(levels) || !ne.match(plan.Condition, nil) {
				return errRefused
			}
			for i+1 < len(levels) {
				i++
				ne.Plans[plan.Group] = levels[i]
				if !ne.match(nePlans[levels[i]].Condition, nil) {
					break
				}
			}
		}
		return nil
	}, func(err error) proto.Message { return &protobuf.SC_29045{Result: result(err)} })
}
