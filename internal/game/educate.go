package game

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand/v2"
	"regexp"
	"slices"
	"strings"

	"google.golang.org/protobuf/proto"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/db"
	"github.com/ggmolly/belfast/internal/gamedata"
	"github.com/ggmolly/belfast/internal/protobuf"
)

// Project Identity: TB, the first child-raising game (EducateProxy, protocol
// module 27; NewEducateSelectScene's first entry). TB lives through 48
// weeks of game time, from month 2 week 4 to month 14 week 4
// (child_data.stage, gameset child_end_data), each week planned on the
// schedule (child_plan, 6 days of 1 to 3 slots) and spent outside
// (child_site / child_site_option, paid with action points). The client
// mirrors almost every rule (EducateChar, EducatePlanProxy, EducateTaskProxy,
// EducateShopProxy...): the server holds the same state and draws what's
// random.
//
// What the official server gives for a plan event, a special event, a site
// branch or a favor level (the child_event / child_site_option_branch /
// child_event_special result ids and child_data trigger_drop) is in no
// table: belfast gives the result_display values (the client's own preview
// of a plan or an option), scaled by TB's mood (wiki: Project Identity: TB,
// Mood) and the active statuses, and the extras listed in the handlers.

// educateChild is the child_data row of TB.
const educateChild = 1

// EducateConst.DROP_TYPE_*: CHILD_DROP.type
const (
	childDropAttr     = 1
	childDropRes      = 2
	childDropItem     = 3
	childDropMemory   = 4
	childDropPolaroid = 5
	childDropBuff     = 6
)

// EducateChar.RES_*_ID, child_resource ids
const (
	educateResMoney = 1
	educateResMood  = 2
	educateResSite  = 3
	educateResFavor = 4
)

// EducateTask types (child_task.type_1 and type_2)
const (
	childTaskMind   = 1
	childTaskTarget = 2
	childTaskMain   = 3

	childTaskPlan      = 1
	childTaskAttr      = 2
	childTaskSiteCost  = 3
	childTaskPurchase  = 4
	childTaskSiteEnter = 5
	childTaskSetTarget = 6
	childTaskPerform   = 7
	childTaskItem      = 8
	childTaskSchedule  = 10
)

// tbTime is a point of TB's calendar (CHILD_TIME): month (2 to 14), week
// (1 to 4) and day (1 to 7).
type tbTime struct {
	Month uint32 `json:"month"`
	Week  uint32 `json:"week"`
	Day   uint32 `json:"day"`
}

func (t tbTime) key() uint32 { return t.Month*100 + t.Week*10 + t.Day }

// in is EducateHelper.InTime: start <= t <= end.
func (t tbTime) in(start, end tbTime) bool { return start.key() <= t.key() && t.key() <= end.key() }

// index is EducateHelper.GetWeekIdxWithTime.
func (t tbTime) index() uint32 { return (t.Month-1)*4 + t.Week }

// afterWeeks is EducateHelper.GetTimeAfterWeeks.
func (t tbTime) afterWeeks(n uint32) tbTime {
	t.Week += n
	for t.Week > 4 {
		t.Week -= 4
		t.Month++
	}
	return t
}

// daysTo is EducateHelper.GetDaysBetweenTimes(t, other).
func (t tbTime) daysTo(other tbTime) int {
	return (int(other.Month)-int(t.Month))*28 + (int(other.Week)-int(t.Week))*7 + int(other.Day) - int(t.Day)
}

func (t tbTime) proto() *protobuf.CHILD_TIME {
	return &protobuf.CHILD_TIME{Month: proto.Uint32(t.Month), Week: proto.Uint32(t.Week), Day: proto.Uint32(t.Day)}
}

// tbWindow is a config time range (EducateHelper.CfgTime2Time):
// [[month, week, day], [month, week, day]], the start week and day
// defaulting to 1, the end ones to 4 and 7. Special events write a fourth
// number (the slot), ignored here.
type tbWindow struct {
	start, end tbTime
	set        bool
}

func (window *tbWindow) UnmarshalJSON(raw []byte) error {
	var bounds [][]uint32
	if json.Unmarshal(raw, &bounds) != nil || len(bounds) < 2 || len(bounds[0]) == 0 || len(bounds[1]) == 0 {
		*window = tbWindow{}
		return nil
	}
	at := func(bound []uint32, i int, fallback uint32) uint32 {
		if i < len(bound) {
			return bound[i]
		}
		return fallback
	}
	window.start = tbTime{bounds[0][0], at(bounds[0], 1, 1), at(bounds[0], 2, 1)}
	window.end = tbTime{bounds[1][0], at(bounds[1], 1, 4), at(bounds[1], 2, 7)}
	window.set = true
	return nil
}

// open is whether t is in the window; a window that isn't set is always
// open.
func (window tbWindow) open(t tbTime) bool { return !window.set || t.in(window.start, window.end) }

// tbPoint is a config time [month, week, day] (EducateHelper.GetTimeFromCfg).
func tbPoint(values []uint32) tbTime {
	var t tbTime
	if len(values) >= 3 {
		t = tbTime{values[0], values[1], values[2]}
	}
	return t
}

// child_data row
type educateChildConfig struct {
	Attrs1          []uint32   `json:"attr_1_list"`
	Attrs2          []uint32   `json:"attr_2_list"`
	Attrs3          []uint32   `json:"attr_3_list"`
	Values1         []uint32   `json:"attr_1_value"`
	Values2         []uint32   `json:"attr_2_value"`
	Values3         []uint32   `json:"attr_3_value"`
	Attr2Add        uint32     `json:"attr_2_add"`
	Stages          []tbWindow `json:"stage"`
	StagePlanNumber []uint32   `json:"stage_plan_number"`
	StageSiteNumber []uint32   `json:"stage_site_number"`
	FavorExp        []uint32   `json:"favor_exp"`
	FavorLevel      uint32     `json:"favor_level"`
	TriggerDrop     []uint32   `json:"trigger_drop"`
}

// child_resource row
type educateResourceConfig struct {
	DefaultValue uint32 `json:"default_value"`
	Min          int64  `json:"min_value"`
	Max          int64  `json:"max_value"`
}

// child_plan row
type educatePlanConfig struct {
	Ability [][3]int64 `json:"ability"` // {DROP_TYPE_ATTR, attr, minimum}
	Money   uint32     `json:"cost_resource1"`
	Mood    uint32     `json:"cost_resource2"`
	Cells   uint32     `json:"cost_resource3"`
	Pre     []uint32   `json:"pre"` // {plan, times done}
	Result  [][3]int64 `json:"result_display"`
	Stage   []uint32   `json:"stage"`
	Time    [][]uint32 `json:"time"` // {day, slot}
	Type    uint32     `json:"type"`
}

// child_site row
type educateSiteConfig struct {
	Type    uint32          `json:"type"`
	Option  []uint32        `json:"option"`
	Random  [][][]uint32    `json:"-"`
	RawRand json.RawMessage `json:"option_random"`
	Unlock1 []uint32        `json:"unlock_time_1"`
	Unlock2 []uint32        `json:"unlock_time_2"`
}

// child_site_option row
type educateOptionConfig struct {
	Type       uint32          `json:"type"`
	Name       string          `json:"name"`
	Cost       [][3]int64      `json:"cost"`
	CountLimit json.RawMessage `json:"count_limit"`
	Param      json.RawMessage `json:"param"`
	Polaroids  []uint32        `json:"polarid_list"`
	Result     []uint32        `json:"result"`
	Display    [][3]int64      `json:"result_display"`
	Time       tbWindow        `json:"time_limit"`
}

// countLimit is the option's {uses, refresh period in weeks}, ok false when
// it has none (EducateSiteOption:IsCountLimit).
func (option educateOptionConfig) countLimit() (uses, period uint32, ok bool) {
	var limit []uint32
	if json.Unmarshal(option.CountLimit, &limit) != nil || len(limit) != 2 {
		return 0, 0, false
	}
	return limit[0], limit[1], true
}

// child_site_option_branch row
type educateBranchConfig struct {
	Attr  json.RawMessage `json:"attr"` // [[attr, ">", value], ...]
	Ratio uint32          `json:"ratio"`
}

// child_event row
type educateEventConfig struct {
	Attr      json.RawMessage `json:"attr"`
	Date      tbWindow        `json:"date"`
	Ratio     uint32          `json:"ratio"`
	Type      uint32          `json:"type"`
	TypeParam json.RawMessage `json:"type_param"`
}

// child_event_special row
type educateSpecialConfig struct {
	Attr2     uint32          `json:"child_attr2"`
	Date      tbWindow        `json:"date"`
	Type      uint32          `json:"type"`
	TypeParam json.RawMessage `json:"type_param"`
}

// child_task row
type educateTaskConfig struct {
	Arg            uint32          `json:"arg"`
	Drop           []int64         `json:"drop_display"`
	SubType        json.RawMessage `json:"sub_type"`
	TargetProgress uint32          `json:"task_target_progress"`
	Time           tbWindow        `json:"time_limit"`
	System         uint32          `json:"type_1"`
	Type           uint32          `json:"type_2"`
}

// matches is whether the task counts sub (EducateHelper.IsMatchSubType; "0"
// counts anything).
func (task educateTaskConfig) matches(sub uint32) bool {
	ids := gamedata.IDs(task.SubType)
	return len(ids) == 0 || slices.Contains(ids, sub)
}

// child_target_set row
type educateTargetConfig struct {
	Condition json.RawMessage `json:"condition"` // [[recommended attr], [personality]]
	Drop      []int64         `json:"drop_display"`
	IDs       []uint32        `json:"ids"`
	Progress  uint32          `json:"target_progress"`
	Stage     uint32          `json:"stage"`
}

// child_shop row
type educateShopConfig struct {
	Num     uint32              `json:"goods_num"`
	Pool    [][]json.RawMessage `json:"goods_pool"` // [[goods, count, weight, condition], ...]
	Refresh int64               `json:"goods_refresh_time"`
}

// child_shop_template row
type educateGoodsConfig struct {
	BuyNum   uint32          `json:"buy_num"`
	Item     uint32          `json:"item_id"`
	Resource uint32          `json:"resource"`
	Price    uint32          `json:"resource_num"`
	RawTime  json.RawMessage `json:"time"`
	time     tbWindow
}

// child_ending row
type educateEndingConfig struct {
	Price     uint32          `json:"child_cg_basic_price"`
	Condition [][]uint32      `json:"-"`
	RawCond   json.RawMessage `json:"condition"`
}

// child_memory row
type educateMemoryConfig struct {
	Price uint32 `json:"child_cg_basic_price"`
}

// child_polaroid row
type educatePolaroidConfig struct {
	Group  uint32          `json:"group"`
	Stage  []uint32        `json:"stage"`
	Xingge json.RawMessage `json:"xingge"`
}

// child_item row
type educateItemConfig struct {
	Display [][3]int64 `json:"display"`
}

// child_buff row
type educateBuffConfig struct {
	During uint32     `json:"during_time"`
	Effect [][4]int64 `json:"effect"` // {1 attr / 2 resource, id, 2 flat, value}
}

// secretary_special_ship row
type educateSecretaryConfig struct {
	UnlockType uint32          `json:"unlock_type"`
	Unlock     json.RawMessage `json:"unlock"`
}

var (
	educateTB          educateChildConfig
	educateAttrs       []uint32 // TB's attributes (child_data attr_1..3_list)
	educateAttrStart   map[uint32]uint32
	educateResources   map[uint32]educateResourceConfig
	educatePlans       map[uint32]educatePlanConfig
	educateSites       map[uint32]educateSiteConfig
	educateOptions     map[uint32]educateOptionConfig
	educateBranches    map[uint32]educateBranchConfig
	educateEvents      map[uint32]educateEventConfig
	educateSpecials    map[uint32]educateSpecialConfig
	educateTasks       map[uint32]educateTaskConfig
	educateTaskIDs     []uint32 // child_task ids in order
	educateTargets     map[uint32]educateTargetConfig
	educateShopConfigs map[uint32]educateShopConfig
	educateShopIDs     []uint32
	educateGoods       map[uint32]educateGoodsConfig
	educateEndings     map[uint32]educateEndingConfig
	educateMemories    map[uint32]educateMemoryConfig
	educatePolaroids   map[uint32]educatePolaroidConfig
	educateItems       map[uint32]educateItemConfig
	educateBuffs       map[uint32]educateBuffConfig
	educateSecretaries map[uint32]educateSecretaryConfig

	// educateOptionItems are the items (child_item) a site option gives
	// (see educateItemsOf).
	educateOptionItems map[uint32][]uint32

	// gameset values
	educateEndTime       tbTime     // child_end_data
	educateTargetDays    []tbTime   // child_target_set_date, by stage
	educateMoodRatios    [][3]int64 // child_emotion: {from, to, ratio}
	educateMoodAttrs     []uint32   // child_emotion_attr
	educateMoodResources []uint32   // child_emotion_resource
	educateEventsWeekly  [][]uint32 // child_event_number_weekly, by stage
	educateTasksWeekly   []uint32   // child_task_number_weekly, by stage
	educateTaskGroups    [][]uint32 // child_task_group, by week from the start
	educateMindStart     tbTime     // child_intimacy_task_first
	educateOutUnlock     tbTime     // child_out_unlock
	educateAttr2AddTime  tbTime     // child_attr_2_add
	educatePrices        map[string]uint32
	educateSecretaryTask uint32 // the task unlocking the special secretary
)

// educateStart is when a game starts (EducateChar default cur_time,
// captured).
var educateStart = tbTime{Month: 2, Week: 4, Day: 7}

func loadEducateData() error {
	var children map[uint32]educateChildConfig
	var attrs map[uint32]struct{}
	var items map[uint32]struct {
		Usage json.RawMessage `json:"usage_arg"`
		Name  string          `json:"name"`
	}
	var mindFirst []json.RawMessage
	var emotion []json.RawMessage
	var targetDays [][]uint32
	var endTime, outUnlock []uint32
	var attr2Add []uint32
	gameset := map[string]uint32{}
	err := gamedata.LoadAll(
		gamedata.Load("child_data", &children),
		gamedata.Load("child_attr", &attrs),
		gamedata.Load("child_resource", &educateResources),
		gamedata.Load("child_plan", &educatePlans),
		gamedata.Load("child_site", &educateSites),
		gamedata.Load("child_site_option", &educateOptions),
		gamedata.Load("child_site_option_branch", &educateBranches),
		gamedata.Load("child_event", &educateEvents),
		gamedata.Load("child_event_special", &educateSpecials),
		gamedata.Load("child_task", &educateTasks),
		gamedata.Load("child_target_set", &educateTargets),
		gamedata.Load("child_shop", &educateShopConfigs),
		gamedata.Load("child_shop_template", &educateGoods),
		gamedata.Load("child_ending", &educateEndings),
		gamedata.Load("child_memory", &educateMemories),
		gamedata.Load("child_polaroid", &educatePolaroids),
		gamedata.Load("child_item", &educateItems),
		gamedata.Load("child_item", &items),
		gamedata.Load("child_buff", &educateBuffs),
		gamedata.Load("secretary_special_ship", &educateSecretaries),
		func() error { return gamedata.GamesetDescription("child_end_data", &endTime) },
		func() error { return gamedata.GamesetDescription("child_target_set_date", &targetDays) },
		func() error { return gamedata.GamesetDescription("child_emotion", &emotion) },
		func() error { return gamedata.GamesetDescription("child_emotion_attr", &educateMoodAttrs) },
		func() error { return gamedata.GamesetDescription("child_emotion_resource", &educateMoodResources) },
		func() error {
			return gamedata.GamesetDescription("child_event_number_weekly", &educateEventsWeekly)
		},
		func() error { return gamedata.GamesetDescription("child_task_number_weekly", &educateTasksWeekly) },
		func() error { return gamedata.GamesetDescription("child_task_group", &educateTaskGroups) },
		func() error { return gamedata.GamesetDescription("child_intimacy_task_first", &mindFirst) },
		func() error { return gamedata.GamesetDescription("child_out_unlock", &outUnlock) },
		func() error { return gamedata.GamesetDescription("child_attr_2_add", &attr2Add) },
		func() (err error) {
			values, err := gamedata.Gameset()
			for _, key := range []string{"child_cg_add_price", "child_cg_max_price", "child_polaroid_basic_price", "child_polaroid_add_price", "child_polaroid_max_price"} {
				gameset[key] = values[key]
			}
			return err
		},
	)
	if err != nil {
		return err
	}
	child, ok := children[educateChild]
	if !ok {
		return gamedata.Missing("child_data", "child %d", educateChild)
	}
	if len(child.Stages) == 0 || len(child.StagePlanNumber) < len(child.Stages) || len(child.StageSiteNumber) < len(child.Stages) {
		return gamedata.Missing("child_data", "stages of child %d", educateChild)
	}
	educateTB = child
	educateAttrs = slices.Concat(child.Attrs1, child.Attrs2, child.Attrs3)
	educateAttrStart = map[uint32]uint32{}
	for _, pair := range [][2][]uint32{{child.Attrs1, child.Values1}, {child.Attrs2, child.Values2}, {child.Attrs3, child.Values3}} {
		for i, id := range pair[0] {
			if _, ok := attrs[id]; !ok {
				return gamedata.Missing("child_attr", "attribute %d", id)
			}
			if i < len(pair[1]) {
				educateAttrStart[id] = pair[1][i]
			}
		}
	}
	for _, id := range []uint32{educateResMoney, educateResMood, educateResSite, educateResFavor} {
		if _, ok := educateResources[id]; !ok {
			return gamedata.Missing("child_resource", "resource %d", id)
		}
	}
	for id, site := range educateSites {
		if len(site.RawRand) > 0 && json.Unmarshal(site.RawRand, &site.Random) != nil {
			site.Random = nil
		}
		educateSites[id] = site
	}
	educateShopIDs = sortedKeys(educateShopConfigs)
	for id, goods := range educateGoods {
		_ = json.Unmarshal(goods.RawTime, &goods.time) // "always": never set
		educateGoods[id] = goods
	}
	for id, ending := range educateEndings {
		_ = json.Unmarshal(ending.RawCond, &ending.Condition)
		educateEndings[id] = ending
	}
	for id := range educateTasks {
		educateTaskIDs = append(educateTaskIDs, id)
	}
	slices.Sort(educateTaskIDs)
	educateOptionItems = educateItemsOf(items)
	if len(endTime) < 3 {
		return gamedata.Missing("gameset", "child_end_data")
	}
	educateEndTime = tbPoint(endTime)
	educateTargetDays = nil
	for _, day := range targetDays {
		educateTargetDays = append(educateTargetDays, tbPoint(day))
	}
	educateMoodRatios = nil
	for _, raw := range emotion {
		var entry []json.RawMessage
		var span []int64
		var ratio int64
		if json.Unmarshal(raw, &entry) != nil || len(entry) < 2 || json.Unmarshal(entry[0], &span) != nil || len(span) < 2 || json.Unmarshal(entry[1], &ratio) != nil {
			return fmt.Errorf("gameset child_emotion: %s", raw)
		}
		educateMoodRatios = append(educateMoodRatios, [3]int64{span[0], span[1], ratio})
	}
	if len(mindFirst) >= 2 {
		var at []uint32
		if json.Unmarshal(mindFirst[1], &at) == nil {
			educateMindStart = tbPoint(at)
		}
	}
	educateOutUnlock = tbPoint(outUnlock)
	educateAttr2AddTime = tbPoint(attr2Add)
	educatePrices = gameset
	educateSecretaryTask = 0
	for _, id := range sortedKeys(educateSecretaries) {
		if secretary := educateSecretaries[id]; secretary.UnlockType == 1 {
			if ids := gamedata.IDs(secretary.Unlock); len(ids) > 0 {
				educateSecretaryTask = ids[0]
				break
			}
		}
	}
	return nil
}

// educateItemsOf links the items to the site options giving them. An
// item's usage_arg is the id of its drop on the official server, whose
// first digits are the option's id (Trophy - Great Cook-Off 110504: the
// cooking competition 1105, Photograph 131111: the Everyday Life Photo
// 1311...). A competition's later rounds have the first round's name
// with another ordinal ("The 2nd Art Contest"), or the same name, and give
// the same trophy.
func educateItemsOf(items map[uint32]struct {
	Usage json.RawMessage `json:"usage_arg"`
	Name  string          `json:"name"`
}) map[uint32][]uint32 {
	round := func(name string) string {
		var words []string
		for _, word := range strings.Fields(name) {
			if !educateOrdinal.MatchString(word) {
				words = append(words, word)
			}
		}
		return strings.Join(words, " ")
	}
	linked := map[uint32][]uint32{}
	for _, id := range sortedKeys(items) {
		drops := gamedata.IDs(items[id].Usage)
		if len(drops) == 0 || drops[0] < 100000 {
			continue
		}
		first, ok := educateOptions[drops[0]/100]
		if !ok || first.Type != childOptionEvent {
			continue
		}
		for _, optionID := range sortedKeys(educateOptions) {
			option := educateOptions[optionID]
			if optionID == drops[0]/100 || (len(first.Result) > 1 && len(option.Result) > 1 && option.Type == childOptionEvent && round(option.Name) == round(first.Name)) {
				linked[optionID] = append(linked[optionID], id)
			}
		}
	}
	return linked
}

// educateOrdinal matches "1st", "2nd", "3rd", "4th"...
var educateOrdinal = regexp.MustCompile(`^\d+(st|nd|rd|th)$`)

// sortedKeys are a table's ids in order.
func sortedKeys[T any](table map[uint32]T) []uint32 {
	ids := make([]uint32, 0, len(table))
	for id := range table {
		ids = append(ids, id)
	}
	slices.Sort(ids)
	return ids
}

// educatePlanCell is a filled schedule slot (CHILD_PLAN_CELL): a plan or a
// special event (child_event_special of the plan type).
type educatePlanCell struct {
	Day     uint32 `json:"day"`
	Index   uint32 `json:"index"`
	Plan    uint32 `json:"plan,omitempty"`
	Special uint32 `json:"special,omitempty"`
}

// educateState is a commander's TB game (CHILD_INFO) and what the server
// alone keeps (which tasks were already handed out). The collections
// (memories, polaroids, endings) and the special secretary carry over to the
// next game.
type educateState struct {
	Time        tbTime                       `json:"time"`
	Mood        uint32                       `json:"mood"`
	Money       uint32                       `json:"money"`
	Site        uint32                       `json:"site"`
	Attrs       map[uint32]uint32            `json:"attrs"`
	FavorLv     uint32                       `json:"favor_lv"`
	FavorExp    uint32                       `json:"favor_exp"`
	Items       map[uint32]uint32            `json:"items"`
	History     map[uint32]uint32            `json:"plan_history"`
	Plans       []educatePlanCell            `json:"plans"`
	Target      uint32                       `json:"target"`
	TargetAward bool                         `json:"target_award"`
	Tasks       map[uint32]uint32            `json:"tasks"`  // id: progress
	Issued      []uint32                     `json:"issued"` // tasks handed out this game
	Wishes      []uint32                     `json:"wishes"` // mind tasks done (realized_wish)
	Buffs       map[uint32]tbTime            `json:"buffs"`  // id: end
	Name        string                       `json:"name"`
	Specials    []uint32                     `json:"specials"` // special events done
	HomeDue     bool                         `json:"home_due"`
	HomeEvents  []uint32                     `json:"home_events"`
	Shops       map[uint32]map[uint32]uint32 `json:"shops"`   // shop: goods: stock
	Options     map[uint32]uint32            `json:"options"` // site option: uses
	Ending      bool                         `json:"ending"`
	Adjusted    bool                         `json:"adjusted"`

	Games        uint32            `json:"games"`
	Secretary    bool              `json:"secretary"`
	Memories     []uint32          `json:"memories"`
	Polaroids    map[uint32]tbTime `json:"polaroids"`
	Completed    []uint32          `json:"completed_endings"`
	Endings      []uint32          `json:"endings"` // unlocked (reached or bought)
	EndingBuys   uint32            `json:"ending_buys"`
	MemoryBuys   uint32            `json:"memory_buys"`
	PolaroidBuys uint32            `json:"polaroid_buys"`
}

// newEducateGame is a game that hasn't started, as the official server
// sends it to a new commander (captured): month 2 week 4 day 7, the
// child_resource defaults, the child_data starting attributes, the main
// tasks open then and every shop stocked. previous carries the
// collections over.
func newEducateGame(previous *educateState) *educateState {
	state := &educateState{
		Time:      educateStart,
		Mood:      educateResources[educateResMood].DefaultValue,
		Money:     educateResources[educateResMoney].DefaultValue,
		FavorLv:   1,
		FavorExp:  educateResources[educateResFavor].DefaultValue,
		Attrs:     map[uint32]uint32{},
		Items:     map[uint32]uint32{},
		History:   map[uint32]uint32{},
		Tasks:     map[uint32]uint32{},
		Buffs:     map[uint32]tbTime{},
		Shops:     map[uint32]map[uint32]uint32{},
		Options:   map[uint32]uint32{},
		Polaroids: map[uint32]tbTime{},
		HomeDue:   true,
		Games:     1,
	}
	for _, id := range educateAttrs {
		state.Attrs[id] = educateAttrStart[id]
	}
	state.Site = state.siteNumber()
	if previous != nil {
		state.Games = previous.Games + 1
		state.Name = previous.Name
		state.Secretary = previous.Secretary
		state.Memories = previous.Memories
		state.Polaroids = previous.Polaroids
		state.Completed = previous.Completed
		state.Endings = previous.Endings
		state.EndingBuys = previous.EndingBuys
		state.MemoryBuys = previous.MemoryBuys
		state.PolaroidBuys = previous.PolaroidBuys
	}
	for _, id := range educateShopIDs {
		state.Shops[id] = stockEducateShop(educateShopConfigs[id])
	}
	state.issueTasks(nil)
	return state
}

// stockEducateShop draws a shop's goods: goods_num of its pool, weighted,
// each in the pool's count.
func stockEducateShop(shop educateShopConfig) map[uint32]uint32 {
	type entry struct{ id, count, weight uint32 }
	var pool []entry
	for _, row := range shop.Pool {
		if len(row) < 3 {
			continue
		}
		var e entry
		if json.Unmarshal(row[0], &e.id) != nil || json.Unmarshal(row[1], &e.count) != nil || json.Unmarshal(row[2], &e.weight) != nil {
			continue
		}
		pool = append(pool, e)
	}
	goods := map[uint32]uint32{}
	for len(goods) < int(shop.Num) && len(pool) > 0 {
		total := uint32(0)
		for _, e := range pool {
			total += e.weight
		}
		pick := 0
		if total > 0 {
			roll := rand.Uint32N(total)
			for i, e := range pool {
				if roll < e.weight {
					pick = i
					break
				}
				roll -= e.weight
			}
		}
		goods[pool[pick].id] = pool[pick].count
		pool = slices.Delete(pool, pick, pick+1)
	}
	return goods
}

// stage is TB's growth stage at t (EducateChar:GetStageByTime, from 1).
func educateStage(t tbTime) uint32 {
	for i, window := range educateTB.Stages {
		if window.open(t) && window.set {
			return uint32(i + 1)
		}
	}
	return 1
}

func (state *educateState) stage() uint32 { return educateStage(state.Time) }

// siteNumber is the week's action points (child_data stage_site_number).
func (state *educateState) siteNumber() uint32 {
	return educateTB.StageSiteNumber[state.stage()-1]
}

// firstGame is EducateProxy:IsFirstGame.
func (state *educateState) firstGame() bool { return state.Games <= 1 }

// endingTime is EducateProxy:IsEndingTime.
func (state *educateState) endingTime() bool {
	return educateEndTime.Month <= state.Time.Month && educateEndTime.Week <= state.Time.Week && educateEndTime.Day <= state.Time.Day
}

// targetDue is EducateTaskProxy:CheckTargetSet: no growth goal yet, or a
// goal setting day with the previous stage's goal.
func (state *educateState) targetDue() bool {
	if state.Target == 0 {
		return true
	}
	for i, day := range educateTargetDays {
		if day == state.Time {
			return educateTargets[state.Target].Stage != uint32(i+1)
		}
	}
	return false
}

// playing is EducateProxy:UpdateGameStatus giving STATUES_NORMAL: weeks
// can be planned and spent.
func (state *educateState) playing() bool {
	return !state.Ending && !state.endingTime() && !state.targetDue()
}

// personality is EducateChar:GetPersonalityId: the highest attr_2 (the
// first one on a tie).
func (state *educateState) personality() uint32 {
	best := educateTB.Attrs2[0]
	for _, id := range educateTB.Attrs2 {
		if state.Attrs[best] < state.Attrs[id] {
			best = id
		}
	}
	return best
}

// moodRatio is the mood's bonus on gains, in 1/10000 (gameset
// child_emotion; wiki: Depressed -40%, Sad -20%, Normal 0, Happy +40%).
func (state *educateState) moodRatio() int64 {
	mood := int64(state.Mood)
	for _, entry := range educateMoodRatios {
		if entry[0] <= mood && mood < entry[1] {
			return entry[2]
		}
	}
	if len(educateMoodRatios) > 0 && mood >= educateMoodRatios[len(educateMoodRatios)-1][1] {
		return educateMoodRatios[len(educateMoodRatios)-1][2]
	}
	return 0
}

// gain is what a result_display value becomes for TB now: gains of the
// child_emotion attributes and resources scaled by her mood, then every
// status on it added (wiki: statuses add a flat amount to every gain; a
// mood status changes the mood cost).
func (state *educateState) gain(kind, id uint32, value int64, ratio int64) int64 {
	if value > 0 && ((kind == childDropAttr && slices.Contains(educateMoodAttrs, id)) || (kind == childDropRes && slices.Contains(educateMoodResources, id))) {
		value = value * (10000 + ratio) / 10000
	}
	for buff := range state.Buffs {
		for _, effect := range educateBuffs[buff].Effect {
			if uint32(effect[0]) != kind || uint32(effect[1]) != id || effect[2] != 2 {
				continue
			}
			switch {
			case kind == childDropRes && id == educateResMood && value < 0:
				value -= effect[3]
			case value > 0:
				value = max(value+effect[3], 0)
			}
		}
	}
	return value
}

// display turns result_display values into the drops TB gets (the -1
// placeholders of random results are left out).
func (state *educateState) display(values [][3]int64, ratio int64) []*protobuf.CHILD_DROP {
	var drops []*protobuf.CHILD_DROP
	for _, value := range values {
		if value[1] <= 0 || value[2] == 0 {
			continue
		}
		n := state.gain(uint32(value[0]), uint32(value[1]), value[2], ratio)
		if n == 0 {
			continue
		}
		drops = append(drops, childDrop(uint32(value[0]), uint32(value[1]), n))
	}
	return drops
}

func childDrop(kind, id uint32, number int64) *protobuf.CHILD_DROP {
	return &protobuf.CHILD_DROP{Type: proto.Uint32(kind), Id: proto.Uint32(id), Number: proto.Int32(int32(number))}
}

// addResource is EducateChar:UpdateRes: money, mood and action points stay
// within child_resource's bounds, favor exp doesn't.
func (state *educateState) addResource(id uint32, n int64) {
	clamp := func(value uint32) uint32 {
		bounds := educateResources[id]
		return uint32(min(max(int64(value)+n, bounds.Min, 0), bounds.Max))
	}
	switch id {
	case educateResMoney:
		state.Money = clamp(state.Money)
	case educateResMood:
		state.Mood = clamp(state.Mood)
	case educateResSite:
		state.Site = clamp(state.Site)
	case educateResFavor:
		state.FavorExp = uint32(max(int64(state.FavorExp)+n, 0))
	}
}

// apply is EducateHelper.UpdateDropsData: an item also gives its display
// values, once per item.
func (state *educateState) apply(drops []*protobuf.CHILD_DROP) {
	for _, drop := range drops {
		id, n := drop.GetId(), int64(drop.GetNumber())
		switch drop.GetType() {
		case childDropAttr:
			if _, ok := state.Attrs[id]; ok {
				state.Attrs[id] = uint32(max(int64(state.Attrs[id])+n, 0))
			}
		case childDropRes:
			state.addResource(id, n)
		case childDropItem:
			state.Items[id] = uint32(max(int64(state.Items[id])+n, 0))
			var effects []*protobuf.CHILD_DROP
			for _, value := range educateItems[id].Display {
				effects = append(effects, childDrop(uint32(value[0]), uint32(value[1]), value[2]*n))
			}
			state.apply(effects)
		case childDropMemory:
			if !slices.Contains(state.Memories, id) {
				state.Memories = append(state.Memories, id)
			}
		case childDropPolaroid:
			if _, ok := state.Polaroids[id]; !ok {
				state.Polaroids[id] = state.Time
			}
		case childDropBuff:
			if buff, ok := educateBuffs[id]; ok {
				state.Buffs[id] = state.Time.afterWeeks(buff.During)
			}
		}
	}
}

// childConfigDrops are a {type, id, number} drop_display as CHILD_DROPs.
func childConfigDrops(drop []int64) []*protobuf.CHILD_DROP {
	if len(drop) < 3 || drop[1] <= 0 {
		return nil
	}
	return []*protobuf.CHILD_DROP{childDrop(uint32(drop[0]), uint32(drop[1]), drop[2])}
}

// educateTaskChanges are the task pushes a change queues: SC_27021 (new),
// SC_27022 (gone) and SC_27025 (progress).
type educateTaskChanges struct {
	added, removed, updated []uint32
}

func (changes *educateTaskChanges) update(id uint32) {
	if changes != nil && !slices.Contains(changes.updated, id) && !slices.Contains(changes.added, id) {
		changes.updated = append(changes.updated, id)
	}
}

// addTask hands a task out.
func (state *educateState) addTask(id uint32, changes *educateTaskChanges) {
	if _, ok := state.Tasks[id]; ok {
		return
	}
	state.Tasks[id] = 0
	if !slices.Contains(state.Issued, id) {
		state.Issued = append(state.Issued, id)
	}
	if changes != nil {
		changes.added = append(changes.added, id)
	}
	state.measure(id, changes)
}

func (state *educateState) removeTask(id uint32, changes *educateTaskChanges) {
	if _, ok := state.Tasks[id]; !ok {
		return
	}
	delete(state.Tasks, id)
	if changes != nil {
		changes.removed = append(changes.removed, id)
		changes.added = slices.DeleteFunc(changes.added, func(other uint32) bool { return other == id })
		changes.updated = slices.DeleteFunc(changes.updated, func(other uint32) bool { return other == id })
	}
}

// issueTasks is the week's task bookkeeping: tasks past their time go, the
// main tasks whose time came are handed out once (the captured start has
// the three open on day one), and from the first wish (gameset
// child_intimacy_task_first) child_task_number_weekly[stage] wishes of the
// week's child_task_group, the groups following each other week by week
// from the start of the game.
func (state *educateState) issueTasks(changes *educateTaskChanges) {
	for _, id := range sortedKeys(state.Tasks) {
		if task := educateTasks[id]; task.Time.set && state.Time.key() > task.Time.end.key() {
			state.removeTask(id, changes)
		}
	}
	for _, id := range educateTaskIDs {
		task := educateTasks[id]
		if task.System == childTaskMain && !slices.Contains(state.Issued, id) && task.Time.open(state.Time) {
			state.addTask(id, changes)
		}
	}
	if educateMindStart.Month == 0 || state.Time.key() < educateMindStart.key() {
		return
	}
	group := int(state.Time.index()) - int(educateStart.index())
	if group < 0 || group >= len(educateTaskGroups) {
		return
	}
	var open []uint32
	for _, id := range educateTaskGroups[group] {
		task, ok := educateTasks[id]
		if !ok || task.System != childTaskMind || !task.Time.open(state.Time) || slices.Contains(state.Issued, id) || slices.Contains(open, id) {
			continue
		}
		open = append(open, id)
	}
	weekly := 1
	if stage := int(state.stage()); stage <= len(educateTasksWeekly) {
		weekly = int(educateTasksWeekly[stage-1])
	}
	rand.Shuffle(len(open), func(i, j int) { open[i], open[j] = open[j], open[i] })
	for _, id := range open[:min(weekly, len(open))] {
		state.addTask(id, changes)
	}
}

// measure sets the progress of the tasks counting a level (attributes,
// items owned) instead of events.
func (state *educateState) measure(id uint32, changes *educateTaskChanges) {
	task := educateTasks[id]
	ids := gamedata.IDs(task.SubType)
	var progress uint32
	switch task.Type {
	case childTaskAttr:
		for _, attr := range ids {
			progress = max(progress, state.Attrs[attr])
		}
	case childTaskItem:
		for _, item := range ids {
			progress = max(progress, state.Items[item])
		}
	default:
		return
	}
	if state.Tasks[id] != progress {
		state.Tasks[id] = progress
		changes.update(id)
	}
}

// measureAll refreshes every level task.
func (state *educateState) measureAll(changes *educateTaskChanges) {
	for _, id := range sortedKeys(state.Tasks) {
		state.measure(id, changes)
	}
}

// count adds n to the open tasks of a type counting sub.
func (state *educateState) count(kind, sub, n uint32, changes *educateTaskChanges) {
	for _, id := range sortedKeys(state.Tasks) {
		task := educateTasks[id]
		if task.Type != kind || !task.matches(sub) || state.Tasks[id] >= task.Arg {
			continue
		}
		state.Tasks[id] = min(state.Tasks[id]+n, task.Arg)
		changes.update(id)
	}
}

// push queues the task changes on tx.
func (changes educateTaskChanges) push(tx *Tx, state *educateState) {
	if len(changes.removed) > 0 {
		tx.push(&protobuf.SC_27022{Ids: changes.removed})
	}
	if len(changes.added) > 0 {
		msg := &protobuf.SC_27021{}
		for _, id := range changes.added {
			msg.Tasks = append(msg.Tasks, childTask(id, state.Tasks[id]))
		}
		tx.push(msg)
	}
	if len(changes.updated) > 0 {
		msg := &protobuf.SC_27025{}
		for _, id := range changes.updated {
			msg.Tasks = append(msg.Tasks, childTask(id, state.Tasks[id]))
		}
		tx.push(msg)
	}
}

func childTask(id, progress uint32) *protobuf.CHILD_TASK {
	return &protobuf.CHILD_TASK{Id: proto.Uint32(id), Progress: proto.Uint32(progress)}
}

// info is the game as SC_27001.child.
func (state *educateState) info() *protobuf.CHILD_INFO {
	child := &protobuf.CHILD_INFO{
		Tid:                     proto.Uint32(educateChild),
		CurTime:                 state.Time.proto(),
		Mood:                    proto.Uint32(state.Mood),
		Money:                   proto.Uint32(state.Money),
		SiteNumber:              proto.Uint32(state.Site),
		Favor:                   &protobuf.CHILD_FAVOR{Lv: proto.Uint32(state.FavorLv), Exp: proto.Uint32(state.FavorExp)},
		Memorys:                 slices.Clone(state.Memories),
		Target:                  proto.Uint32(state.Target),
		RealizedWish:            slices.Clone(state.Wishes),
		UserName:                proto.String(state.Name),
		SpecEvents:              slices.Clone(state.Specials),
		CanTriggerHomeEvent:     proto.Uint32(boolFlag(state.HomeDue)),
		HomeEvents:              slices.Clone(state.HomeEvents),
		IsEnding:                proto.Uint32(boolFlag(state.Ending)),
		NewGamePlusCount:        proto.Uint32(state.Games),
		HadTargetStageAward:     proto.Uint32(boolFlag(state.TargetAward)),
		HadAdjustment:           proto.Uint32(boolFlag(state.Adjusted)),
		IsSpecialSecretaryValid: proto.Uint32(boolFlag(state.Secretary)),
		EndingBuyCount:          proto.Uint32(state.EndingBuys),
		MemoryBuyCount:          proto.Uint32(state.MemoryBuys),
		PolaroidBuyCount:        proto.Uint32(state.PolaroidBuys),
	}
	for _, id := range educateAttrs {
		child.Attrs = append(child.Attrs, &protobuf.CHILD_ATTR{Id: proto.Uint32(id), Val: proto.Uint32(state.Attrs[id])})
	}
	for _, id := range sortedKeys(state.Items) {
		if state.Items[id] > 0 {
			child.Items = append(child.Items, &protobuf.CHILD_ITEM{Id: proto.Uint32(id), Num: proto.Uint32(state.Items[id])})
		}
	}
	for _, id := range sortedKeys(state.History) {
		child.PlanHistory = append(child.PlanHistory, &protobuf.CHILD_PLAN_HISTORY{PlanId: proto.Uint32(id), Count: proto.Uint32(state.History[id])})
	}
	child.Plans = planCells(state.Plans)
	for _, id := range sortedKeys(state.Polaroids) {
		child.Polaroids = append(child.Polaroids, &protobuf.CHILD_POLAROID{Id: proto.Uint32(id), Time: state.Polaroids[id].proto()})
	}
	for _, id := range sortedKeys(state.Tasks) {
		child.Tasks = append(child.Tasks, childTask(id, state.Tasks[id]))
	}
	for _, id := range sortedKeys(state.Buffs) {
		child.Buffs = append(child.Buffs, &protobuf.CHILD_BUFF{Id: proto.Uint32(id), Time: state.Buffs[id].proto()})
	}
	for _, id := range sortedKeys(state.Shops) {
		child.Shop = append(child.Shop, shopData(id, state.Shops[id]))
	}
	for _, id := range sortedKeys(state.Options) {
		child.OptionRecords = append(child.OptionRecords, &protobuf.CHILD_OPTION_RECORD{Id: proto.Uint32(id), Count: proto.Uint32(state.Options[id])})
	}
	return child
}

func shopData(id uint32, goods map[uint32]uint32) *protobuf.CHILD_SHOP_DATA {
	data := &protobuf.CHILD_SHOP_DATA{ShopId: proto.Uint32(id)}
	for _, goodsID := range sortedKeys(goods) {
		data.Goods = append(data.Goods, &protobuf.CHILD_SHOP_GOODS{Id: proto.Uint32(goodsID), Num: proto.Uint32(goods[goodsID])})
	}
	return data
}

// planCells are the schedule as CHILD_PLAN_CELLs.
func planCells(plans []educatePlanCell) []*protobuf.CHILD_PLAN_CELL {
	var cells []*protobuf.CHILD_PLAN_CELL
	for _, plan := range plans {
		cells = append(cells, &protobuf.CHILD_PLAN_CELL{
			Day:   proto.Uint32(plan.Day),
			Index: proto.Uint32(plan.Index),
			Value: []*protobuf.CHILD_PLAN_VAL{{PlanId: proto.Uint32(plan.Plan), EventId: proto.Uint32(0), SpecEventId: proto.Uint32(plan.Special)}},
		})
	}
	return cells
}

// loadEducate reads the commander's game, nil when it never asked for one.
func loadEducate(ctx context.Context, queries *db.Queries, commanderID uint32) (*educateState, error) {
	raw, err := queries.GetEducateState(ctx, commanderID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	state := &educateState{}
	if err := json.Unmarshal([]byte(raw), state); err != nil {
		return nil, fmt.Errorf("commander %d educate state: %w", commanderID, err)
	}
	for _, m := range []*map[uint32]uint32{&state.Attrs, &state.Items, &state.History, &state.Tasks, &state.Options} {
		if *m == nil {
			*m = map[uint32]uint32{}
		}
	}
	if state.Buffs == nil {
		state.Buffs = map[uint32]tbTime{}
	}
	if state.Polaroids == nil {
		state.Polaroids = map[uint32]tbTime{}
	}
	if state.Shops == nil {
		state.Shops = map[uint32]map[uint32]uint32{}
	}
	return state, nil
}

func saveEducate(ctx context.Context, queries *db.Queries, commanderID uint32, state *educateState) error {
	raw, err := json.Marshal(state)
	if err != nil {
		return err
	}
	return queries.SaveEducateState(ctx, db.SaveEducateStateParams{CommanderID: commanderID, State: string(raw)})
}

// educateTx runs fn on the commander's running game in a transaction and
// saves it when fn succeeds; a commander without a game is refused (the
// client always asks for it first, EducateHelper.ReqEducateDataCheck).
func (game *Game) educateTx(client *connection.Client, fn func(tx *Tx, state *educateState) error) error {
	ctx := context.Background()
	return game.tx(ctx, client, func(tx *Tx) error {
		state, err := loadEducate(ctx, tx.Queries, client.CommanderID)
		if err != nil {
			return err
		}
		if state == nil {
			return errRefused
		}
		if err := fn(tx, state); err != nil {
			return err
		}
		return saveEducate(ctx, tx.Queries, client.CommanderID, state)
	})
}

// handleEducate answers EducateRequestCommand (sent in the login burst, and
// again after a reset or a refresh): the commander's TB game, a new one the
// first time (as captured for a new commander).
func (game *Game) handleEducate(client *connection.Client, req *protobuf.CS_27000) error {
	ctx := context.Background()
	state, err := loadEducate(ctx, game.DB.Queries, client.CommanderID)
	if err != nil {
		return err
	}
	if state == nil {
		state = newEducateGame(nil)
		if err := saveEducate(ctx, game.DB.Queries, client.CommanderID, state); err != nil {
			return err
		}
	}
	return client.Send(&protobuf.SC_27001{Result: result(nil), Child: state.info()})
}

// handleEducateEndings answers EducateGetEndingsCommand (EducateProxy:
// initData asks right after the game): the endings reached (endings) and
// the ones unlocked, reached or bought (qualifieds; EducateProxy:SetEndings
// reads them the other way round from the field names).
func (game *Game) handleEducateEndings(client *connection.Client, req *protobuf.CS_27010) error {
	state, err := loadEducate(context.Background(), game.DB.Queries, client.CommanderID)
	if err != nil {
		return err
	}
	reply := &protobuf.SC_27011{}
	if state != nil {
		reply.Endings = slices.Clone(state.Completed)
		reply.Qualifieds = slices.Clone(state.Endings)
	}
	return client.Send(reply)
}
