package game

import (
	"encoding/json"
	"math/rand/v2"
	"slices"

	"google.golang.org/protobuf/proto"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/gamedata"
	"github.com/ggmolly/belfast/internal/protobuf"
)

// The week of Project Identity: TB: the schedule is planned
// (EducateScheduleScene, CS_27012) then run (CS_27002), which moves TB to the
// next week.

// educateDays and educateSlots are the schedule's size (EducateScheduleScene:
// 6 days of up to 3 slots).
const (
	educateDays  = 6
	educateSlots = 3
)

// childEventPlan is child_event type 1, the events a plan may lead to
// (type_param: the plans); type 2 are the site options' (EducateEvent).
const (
	childEventPlan   = 1
	childEventOption = 2
)

// child_event_special types (EducateSpecialEvent)
const (
	childSpecialPlan     = 1
	childSpecialSite     = 2
	childSpecialMind     = 3
	childSpecialDiscount = 4
)

// handleEducatePlans answers EducateGetPlansCommand (CS_27012, the
// schedule's Next button): the week's schedule, saved to be run by
// CS_27002 (and run on entering the scene if it never was,
// EducateScene:SeriesCheck). Every slot is checked as
// EducateScheduleScene:CheckCondition does: a plan of next week's stage
// (EducatePlan:IsInStage), offered in that slot (time), whose attribute
// requirements (ability) and prerequisite plan count (pre) TB meets, in
// slots that are open next week (stage_plan_number) and free, with the
// money for all of them. Built from the Lua alone.
func (game *Game) handleEducatePlans(client *connection.Client, req *protobuf.CS_27012) error {
	var plans []educatePlanCell
	err := game.educateTx(client, func(tx *Tx, state *educateState) error {
		if !state.playing() {
			return errRefused
		}
		next := educateStage(state.Time.afterWeeks(1))
		open := educateTB.StagePlanNumber[next-1]
		taken := map[[2]uint32]bool{}
		money := uint32(0)
		for _, cell := range req.GetPlans() {
			day, index := cell.GetDay(), cell.GetIndex()
			if day < 1 || day > educateDays || index < 1 || len(cell.GetValue()) == 0 {
				return errRefused
			}
			value := cell.GetValue()[0]
			plan := educatePlanCell{Day: day, Index: index, Plan: value.GetPlanId(), Special: value.GetSpecEventId()}
			cells := uint32(1)
			switch {
			case plan.Special != 0:
				special, ok := educateSpecials[plan.Special]
				if !ok || special.Type != childSpecialPlan || slices.Contains(state.Specials, plan.Special) {
					return errRefused
				}
				plan.Plan = 0
			case plan.Plan != 0:
				config, ok := educatePlans[plan.Plan]
				if !ok || !state.canPlan(config, next, day, index) {
					return errRefused
				}
				cells = max(config.Cells, 1)
				money += config.Money
			default:
				continue
			}
			if (plan.Plan != 0 && index+cells-1 > open) || index > educateSlots {
				return errRefused
			}
			for slot := index; slot < index+cells; slot++ {
				if taken[[2]uint32{day, slot}] {
					return errRefused
				}
				taken[[2]uint32{day, slot}] = true
			}
			plans = append(plans, plan)
		}
		if money > state.Money {
			return ErrNotEnough
		}
		plans = append(plans, state.freeTime(next, open, taken)...)
		state.Plans = plans
		return nil
	})
	if err != nil && !refused(err) {
		return err
	}
	reply := &protobuf.SC_27013{Result: result(err)}
	if err == nil {
		reply.Plans = planCells(plans)
	}
	return client.Send(reply)
}

// childPlanFreeTime is EducatePlan.TYPE_FREETIME.
const childPlanFreeTime = 4

// freeTime fills the open slots left empty (EducateScheduleScene warns
// "TB will decide how to use this free time"): each gets one of the free
// time plans (child_plan type 4, Relax At Home) she can do there, at
// random. A guess at what she picks: the type 5 ones (Beginner Housekeeper,
// mood for money) are left to the player.
func (state *educateState) freeTime(next, open uint32, taken map[[2]uint32]bool) []educatePlanCell {
	var cells []educatePlanCell
	for day := uint32(1); day <= educateDays; day++ {
		for index := uint32(1); index <= open; index++ {
			if taken[[2]uint32{day, index}] {
				continue
			}
			var choices []uint32
			for _, id := range sortedKeys(educatePlans) {
				if plan := educatePlans[id]; plan.Type == childPlanFreeTime && max(plan.Cells, 1) == 1 && plan.Money == 0 && state.canPlan(plan, next, day, index) {
					choices = append(choices, id)
				}
			}
			if len(choices) > 0 {
				cells = append(cells, educatePlanCell{Day: day, Index: index, Plan: choices[rand.IntN(len(choices))]})
			}
		}
	}
	return cells
}

// canPlan is whether TB may do a plan in a slot of next week (stage next).
func (state *educateState) canPlan(plan educatePlanConfig, next, day, index uint32) bool {
	if len(plan.Stage) > 0 && !slices.Contains(plan.Stage, next) {
		return false
	}
	if !slices.ContainsFunc(plan.Time, func(slot []uint32) bool { return len(slot) >= 2 && slot[0] == day && slot[1] == index }) {
		return false
	}
	for _, ability := range plan.Ability {
		if int64(state.Attrs[uint32(ability[1])]) < ability[2] {
			return false
		}
	}
	return len(plan.Pre) < 2 || state.History[plan.Pre[0]] >= plan.Pre[1]
}

// handleEducateRunPlans answers EducateExecutePlansCommand (CS_27002): runs
// the saved schedule and moves TB to the next week. The plans' costs
// (cost_resource1 money, cost_resource2 mood) are paid first
// (EducateProxy:ReduceResForPlans), then every slot gives its plan's
// result_display, scaled by TB's mood at the start of the week and her
// statuses (EducateScheduleResultLayer applies them all at the end). A plan
// may lead to one of its child_event (ratio in 1/10000, at most
// child_event_number_weekly[stage][2] a week); their results are server
// content belfast doesn't have, they give nothing. plan_results covers
// every slot of the 6 days (EducateSchedulePerformLayer indexes each day).
// Then the week turns (EducateProxy:OnNextWeek). Built from the Lua alone.
func (game *Game) handleEducateRunPlans(client *connection.Client, req *protobuf.CS_27002) error {
	reply := &protobuf.SC_27003{}
	err := game.educateTx(client, func(tx *Tx, state *educateState) error {
		if !state.playing() {
			return errRefused
		}
		changes := &educateTaskChanges{}
		next := state.Time.afterWeeks(1)
		ratio := state.moodRatio()
		var money, mood int64
		for _, cell := range state.Plans {
			if plan, ok := educatePlans[cell.Plan]; ok && cell.Plan != 0 {
				money += int64(plan.Money)
				mood += int64(plan.Mood)
			}
		}
		state.addResource(educateResMoney, -money)
		state.addResource(educateResMood, -mood)
		results := map[[2]uint32]*protobuf.CHILD_PLAN_RESULT{}
		for day := uint32(1); day <= educateDays; day++ {
			for index := uint32(1); index <= educateSlots; index++ {
				result := &protobuf.CHILD_PLAN_RESULT{Day: proto.Uint32(day), Index: proto.Uint32(index)}
				results[[2]uint32{day, index}] = result
				reply.PlanResults = append(reply.PlanResults, result)
			}
		}
		weekly := uint32(0)
		if stage := int(educateStage(next)); stage <= len(educateEventsWeekly) && len(educateEventsWeekly[stage-1]) >= 2 {
			weekly = educateEventsWeekly[stage-1][1]
		}
		var events uint32
		var drops []*protobuf.CHILD_DROP
		for _, cell := range state.Plans {
			result := results[[2]uint32{cell.Day, cell.Index}]
			if cell.Special != 0 {
				state.Specials = append(state.Specials, cell.Special)
				continue
			}
			plan, ok := educatePlans[cell.Plan]
			if !ok {
				continue
			}
			result.PlanDrops = state.display(plan.Result, ratio)
			drops = append(drops, result.PlanDrops...)
			state.History[cell.Plan]++
			state.count(childTaskPlan, cell.Plan, 1, changes)
			if events >= weekly {
				continue
			}
			at := tbTime{Month: next.Month, Week: next.Week, Day: cell.Day}
			if event := state.drawEvent(childEventPlan, cell.Plan, at); event != 0 {
				events++
				reply.Events = append(reply.Events, &protobuf.CHILD_PLAN_CELL{
					Day:   proto.Uint32(cell.Day),
					Index: proto.Uint32(cell.Index),
					Value: []*protobuf.CHILD_PLAN_VAL{{PlanId: proto.Uint32(0), EventId: proto.Uint32(event), SpecEventId: proto.Uint32(0)}},
				})
			}
		}
		state.apply(drops)
		state.count(childTaskSchedule, 0, 1, changes)
		state.Plans = nil
		state.nextWeek(changes)
		changes.push(tx, state)
		return nil
	})
	if err != nil && !refused(err) {
		return err
	}
	reply.Result = result(err)
	if err != nil {
		reply.PlanResults, reply.Events = nil, nil
	}
	return client.Send(reply)
}

// drawEvent rolls the events of a plan or a site option (child_event of a
// type whose type_param has it) open at t whose conditions TB meets, each
// at its ratio (1/10000); 0 when none happens.
func (state *educateState) drawEvent(kind, source uint32, t tbTime) uint32 {
	for _, id := range sortedKeys(educateEvents) {
		event := educateEvents[id]
		if event.Type != kind || !slices.Contains(gamedata.IDs(event.TypeParam), source) || !event.Date.open(t) || !state.meets(event.Attr) {
			continue
		}
		if rand.Uint32N(10000) < event.Ratio {
			return id
		}
	}
	return 0
}

// meets is whether TB's attributes pass a [[attr, ">", value], ...]
// condition (child_event, child_site_option_branch attr); an empty one
// always passes.
func (state *educateState) meets(raw json.RawMessage) bool {
	var conditions [][]json.RawMessage
	if json.Unmarshal(raw, &conditions) != nil {
		return true
	}
	for _, condition := range conditions {
		if len(condition) < 3 {
			continue
		}
		var attr uint32
		var op string
		var value int64
		if json.Unmarshal(condition[0], &attr) != nil || json.Unmarshal(condition[1], &op) != nil || json.Unmarshal(condition[2], &value) != nil {
			continue
		}
		have := int64(state.Attrs[attr])
		var ok bool
		switch op {
		case ">":
			ok = have > value
		case ">=":
			ok = have >= value
		case "<":
			ok = have < value
		case "<=":
			ok = have <= value
		default:
			ok = have == value
		}
		if !ok {
			return false
		}
	}
	return true
}

// nextWeek is EducateProxy:OnNextWeek on the server's side: the week
// turns, action points refill for the new stage, the site options and shops
// whose refresh week it is restock (EducateSiteOption:CheckCntReset,
// EducateShop:IsRefreshShop: from week 9, every count_limit[2] /
// goods_refresh_time weeks), ended statuses go, home events can be asked
// for again, and the week's tasks are handed out.
func (state *educateState) nextWeek(changes *educateTaskChanges) {
	state.Time = state.Time.afterWeeks(1)
	state.Site = state.siteNumber()
	week := state.Time.index()
	refresh := func(period int64) bool {
		return period > 0 && week >= 9 && int64(week-9)%period == 0
	}
	for _, id := range sortedKeys(state.Options) {
		option := educateOptions[id]
		if _, period, ok := option.countLimit(); ok && option.Type == 2 && refresh(int64(period)) {
			delete(state.Options, id)
		}
	}
	for _, id := range educateShopIDs {
		if shop := educateShopConfigs[id]; refresh(shop.Refresh) {
			state.Shops[id] = stockEducateShop(shop)
		}
	}
	for _, id := range sortedKeys(state.Buffs) {
		if state.Time.daysTo(state.Buffs[id]) < 0 {
			delete(state.Buffs, id)
		}
	}
	state.HomeDue = true
	state.HomeEvents = nil
	state.issueTasks(changes)
	state.measureAll(changes)
}

// handleEducateHomeEvents answers EducateGetEventsCommand (CS_27014,
// EducateScene once a week): the week's home events. The client of this
// version never triggers any (EducateEventProxy:GetHomeEventIds has no
// caller) and child_event has no home type: none.
func (game *Game) handleEducateHomeEvents(client *connection.Client, req *protobuf.CS_27014) error {
	var events []uint32
	err := game.educateTx(client, func(tx *Tx, state *educateState) error {
		state.HomeDue = false
		events = state.HomeEvents
		return nil
	})
	if err != nil && !refused(err) {
		return err
	}
	return client.Send(&protobuf.SC_27015{Result: result(err), Events: events})
}

// handleEducateHomeEvent answers EducateTriggerEventCommand (CS_27016): plays
// one of the week's home events (EducateEventProxy:RemoveEvent). Their
// results are server content: nothing. No UI sends it in this client.
func (game *Game) handleEducateHomeEvent(client *connection.Client, req *protobuf.CS_27016) error {
	err := game.educateTx(client, func(tx *Tx, state *educateState) error {
		index := slices.Index(state.HomeEvents, req.GetEventid())
		if index < 0 {
			return errRefused
		}
		state.HomeEvents = slices.Delete(state.HomeEvents, index, index+1)
		return nil
	})
	if err != nil && !refused(err) {
		return err
	}
	return client.Send(&protobuf.SC_27017{Result: result(err)})
}

// handleEducateSpecialEvent answers EducateTriggerSpecEventCommand
// (CS_27027): a special event played once, at home (the bubbles,
// EducateEventProxy:GetHomeSpecEvents: the mind type once the favor system
// is unlocked, the discount type) or on entering a site
// (GetSiteSpecEvents), in its dates, for TB's personality (child_attr2).
// Their results (child_event_special.result) are server content: nothing
// (this version's table has no discount event).
func (game *Game) handleEducateSpecialEvent(client *connection.Client, req *protobuf.CS_27027) error {
	err := game.educateTx(client, func(tx *Tx, state *educateState) error {
		id := req.GetSpecEventsId()
		special, ok := educateSpecials[id]
		if !ok || special.Type == childSpecialPlan || slices.Contains(state.Specials, id) || !special.Date.open(state.Time) {
			return errRefused
		}
		if special.Attr2 != 0 && special.Attr2 != state.personality() {
			return errRefused
		}
		state.Specials = append(state.Specials, id)
		return nil
	})
	if err != nil && !refused(err) {
		return err
	}
	return client.Send(&protobuf.SC_27028{Result: result(err)})
}
