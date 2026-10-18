package game

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/ggmolly/belfast/internal/gamedata"
)

// Meowfficers are the client's "commanders" (CommanderProxy, protocol p25):
// cats trained out of cat boxes, assigned to fleets and housed in the
// cattery (CommanderHome).

const (
	// CommanderProxy.MAX_WORK_COUNT: boxes training at the same time, the
	// others wait their turn (NG007: "only 4 can be trained at a time")
	meowfficerBoxWorkers = 4
	// CommanderConst.MAX_FORMATION_POS: a fleet's command and support cats
	meowfficerFleetSlots = 2
	// Item.COMMANDER_QUICKLY_TOOL_ID, the Cat Box Quick Finisher
	itemCatBoxQuickFinisher = 20010
	// commander_data_template ability_refresh_type of the regular cats; the
	// others (1) have fixed talents and never come out of a box
	meowfficerRegular = 0
)

// meowfficerTemplate is a commander_data_template row.
type meowfficerTemplate struct {
	ID                 uint32 `json:"id"`
	Rarity             uint32 `json:"rarity"` // 3 R, 4 SR (Elite), 5 SSR
	SkillID            uint32 `json:"skill_id"`
	GroupType          uint32 `json:"group_type"`
	AbilityRefreshType uint32 `json:"ability_refresh_type"`
}

// meowfficerBoxPool is a commander_data_create_material row: training a box
// of the pool spends number_1 use_item (CommanderBuildPool:getConsume); its
// id is its rarity (getRarity), the cats it yields have rarity id + 2
// (getPrint, rarity2Print).
type meowfficerBoxPool struct {
	ID      uint32 `json:"id"`
	UseItem uint32 `json:"use_item"`
	Number  uint32 `json:"number_1"`
}

// meowfficerLevel is a commander_level row: the EXP from the level to the
// next, by cat rarity (Commander:getConfigExp).
type meowfficerLevel struct {
	Level uint32 `json:"level"`
	Exp3  uint32 `json:"exp_3"`
	Exp4  uint32 `json:"exp_4"`
	Exp5  uint32 `json:"exp_5"`
}

// catteryLevel is a commander_home row. home_exp is the EXP to the next
// level; clean_exp, feed_level[3] and teast_level[3] the cattery EXP a
// clean, a feed and a play give per housed cat (bwiki 指挥喵: 30, 10, 20),
// feed_level[2] the EXP a feed gives each cat, teast_level[2] what a play
// gives (catteryPlayAwards), exp_number the EXP a housed cat earns an hour,
// flower[1] the cleanliness a clean adds and flower[2] what it loses a day,
// nest_number the slots open, nest_appearance the slot styles
// (commander_home_style) that can be picked.
type catteryLevel struct {
	Level      uint32          `json:"level"`
	HomeExp    uint32          `json:"home_exp"`
	CleanExp   uint32          `json:"clean_exp"`
	ExpNumber  uint32          `json:"exp_number"`
	FeedLevel  []uint32        `json:"feed_level"`
	TeastLevel []uint32        `json:"teast_level"`
	Flower     json.RawMessage `json:"flower"`
	NestNumber uint32          `json:"nest_number"`
	NestStyles []uint32        `json:"nest_appearance"`
}

// catteryOpAward is one entry of a play's awards: a drop given with a
// chance in percent.
type catteryOpAward struct {
	kind, id, count, chance uint32
}

var (
	meowfficerTemplates map[uint32]meowfficerTemplate
	meowfficerPools     map[uint32]meowfficerBoxPool
	meowfficerLevels    map[uint32]meowfficerLevel
	catteryLevels       map[uint32]catteryLevel
	// the regular cats a box of each pool yields, those with a training time
	meowfficerCandidates map[uint32][]uint32
	// meowfficerTimes are the training times of the cats (template id ->
	// seconds): server content, server/meowfficer_times.json of the data
	// dir ({"<template id>": seconds}, "_" keys are comments), from the
	// bwiki's 指挥喵训练时间 table (每只指挥喵的训练时间固定: the cat is drawn
	// when the box starts training). A cat without a time is never drawn;
	// without the file no box can be trained.
	meowfficerTimes map[uint32]uint32
	// catteryPlayAwards are what playing with one housed cat gives, by
	// commander_home teast_level[2] (a server-side drop id): server content,
	// server/cattery_play_awards.json ({"<id>": [[type, id, count, chance
	// %], ...]}). Without it a play gives nothing, and NG0029, which waits
	// for the award window after the play, never ends.
	catteryPlayAwards map[uint32][]catteryOpAward
	// gameset values: commander_box_count (boxes), commander_home_number
	// (cattery slots)
	meowfficerBoxCount, catterySlotCount uint32
	// gameset commander_get_cost description: [[gold, _, count], ...], the
	// price of the nth cat box bought in a day (CommanderConst.getBoxComsume)
	meowfficerBoxCosts [][3]uint32
	// the cat box items bought from Akashi, by weight (gametip
	// commander_build_rate_tip, the rates the purchase page shows)
	meowfficerBoxRates []meowfficerBoxRate
	// seconds a Cat Box Quick Finisher takes off a box (its usage_arg)
	catBoxQuickFinisherSeconds uint32
)

type meowfficerBoxRate struct {
	item, weight uint32
}

// rateTipEntry is a "Label: N%" of a rate tip.
var rateTipEntry = regexp.MustCompile(`([A-Za-z][A-Za-z ]*): (\d+)%`)

func loadMeowfficerData() error {
	var items map[uint32]struct {
		UsageArg json.RawMessage `json:"usage_arg"`
	}
	err := gamedata.LoadAll(
		gamedata.Load("commander_data_template", &meowfficerTemplates),
		gamedata.Load("commander_data_create_material", &meowfficerPools),
		gamedata.Load("commander_level", &meowfficerLevels),
		gamedata.Load("commander_home", &catteryLevels),
		gamedata.Load("item_data_statistics", &items),
		loadMeowfficerTimes,
		loadCatteryPlayAwards,
	)
	if err != nil {
		return err
	}
	gameset, err := gamedata.Gameset()
	if err != nil {
		return err
	}
	if meowfficerBoxCount = gameset["commander_box_count"]; meowfficerBoxCount == 0 {
		return gamedata.Missing("gameset", "commander_box_count")
	}
	if catterySlotCount = gameset["commander_home_number"]; catterySlotCount == 0 {
		return gamedata.Missing("gameset", "commander_home_number")
	}
	if err := gamedata.GamesetDescription("commander_get_cost", &meowfficerBoxCosts); err != nil {
		return err
	}
	if len(meowfficerBoxCosts) == 0 {
		return gamedata.Missing("gameset", "commander_get_cost description")
	}
	quick := gamedata.IDs(items[itemCatBoxQuickFinisher].UsageArg)
	if len(quick) == 0 || quick[0] == 0 {
		return gamedata.Missing("item_data_statistics", "usage_arg of %d", itemCatBoxQuickFinisher)
	}
	catBoxQuickFinisherSeconds = quick[0]
	if _, ok := meowfficerLevels[1]; !ok {
		return gamedata.Missing("commander_level", "level 1")
	}
	if _, ok := catteryLevels[1]; !ok {
		return gamedata.Missing("commander_home", "level 1")
	}
	meowfficerCandidates = map[uint32][]uint32{}
	for id, template := range meowfficerTemplates {
		if template.AbilityRefreshType != meowfficerRegular || meowfficerTimes[id] == 0 || template.Rarity < 3 {
			continue
		}
		pool := template.Rarity - 2
		meowfficerCandidates[pool] = append(meowfficerCandidates[pool], id)
	}
	return loadMeowfficerBoxRates()
}

// loadMeowfficerBoxRates reads the purchase rates out of
// commander_build_rate_tip ("Rare: 60%; Elite: 35%; Super Rare: 5%."), the
// labels being the cat rarities of a pool's boxes.
func loadMeowfficerBoxRates() error {
	gametip, err := gamedata.Gametip()
	if err != nil {
		return err
	}
	meowfficerBoxRates = nil
	for _, match := range rateTipEntry.FindAllStringSubmatch(gametip["commander_build_rate_tip"], -1) {
		rarity, ok := rarityByRateName[strings.TrimSpace(match[1])]
		weight, _ := strconv.Atoi(match[2])
		pool, found := meowfficerPools[rarity-2]
		if !ok || !found || weight == 0 {
			continue
		}
		meowfficerBoxRates = append(meowfficerBoxRates, meowfficerBoxRate{item: pool.UseItem, weight: uint32(weight)})
	}
	if len(meowfficerBoxRates) == 0 {
		return gamedata.Missing("gametip", "commander_build_rate_tip rates")
	}
	return nil
}

// serverFileByID reads a server file keyed by id, skipping "_" comments.
func serverFileByID(name string) (map[uint32]json.RawMessage, error) {
	raw, err := gamedata.ServerFile(name)
	if raw == nil || err != nil {
		return nil, err
	}
	var rows map[string]json.RawMessage
	if err := json.Unmarshal(raw, &rows); err != nil {
		return nil, fmt.Errorf("%s: %w", name, err)
	}
	byID := map[uint32]json.RawMessage{}
	for key, row := range rows {
		if strings.HasPrefix(key, "_") {
			continue
		}
		id, err := strconv.ParseUint(key, 10, 32)
		if err != nil {
			return nil, fmt.Errorf("%s: key %q", name, key)
		}
		byID[uint32(id)] = row
	}
	return byID, nil
}

func loadMeowfficerTimes() error {
	rows, err := serverFileByID("meowfficer_times.json")
	if err != nil {
		return err
	}
	meowfficerTimes = map[uint32]uint32{}
	for id, raw := range rows {
		var seconds uint32
		if err := json.Unmarshal(raw, &seconds); err != nil {
			return fmt.Errorf("meowfficer_times.json %d: %w", id, err)
		}
		meowfficerTimes[id] = seconds
	}
	return nil
}

func loadCatteryPlayAwards() error {
	rows, err := serverFileByID("cattery_play_awards.json")
	if err != nil {
		return err
	}
	catteryPlayAwards = map[uint32][]catteryOpAward{}
	for id, raw := range rows {
		var entries [][4]uint32
		if err := json.Unmarshal(raw, &entries); err != nil {
			return fmt.Errorf("cattery_play_awards.json %d: %w", id, err)
		}
		for _, entry := range entries {
			catteryPlayAwards[id] = append(catteryPlayAwards[id], catteryOpAward{entry[0], entry[1], entry[2], entry[3]})
		}
	}
	return nil
}

// meowfficerBoxCost is what the nth cat box bought in a day costs (from 0;
// CommanderConst.getBoxComsume): the first entry whose count n is below,
// else the last; the guild's discount is not applied (no guild).
func meowfficerBoxCost(n uint32) uint32 {
	for _, entry := range meowfficerBoxCosts {
		if n < entry[2] {
			return entry[0]
		}
	}
	return meowfficerBoxCosts[len(meowfficerBoxCosts)-1][0]
}

// meowfficerBoxDailyMax is CommanderConst.MAX_GETBOX_CNT: the boxes that can
// be bought in a day.
func meowfficerBoxDailyMax() uint32 {
	total := uint32(0)
	for _, entry := range meowfficerBoxCosts {
		total += entry[2]
	}
	return total
}

// meowfficerLevelExp is the EXP from level to the next of a cat of rarity;
// 0 at the last level.
func meowfficerLevelExp(level, rarity uint32) uint32 {
	row, ok := meowfficerLevels[max(level, 1)]
	if !ok {
		return 0
	}
	switch rarity {
	case 3:
		return row.Exp3
	case 4:
		return row.Exp4
	}
	return row.Exp5
}

// meowfficerMaxLevel is the last commander_level level.
func meowfficerMaxLevel() uint32 {
	top := uint32(1)
	for level := range meowfficerLevels {
		top = max(top, level)
	}
	return top
}

// cattery is the commander_home row of level, the last one past it.
func cattery(level uint32) catteryLevel {
	if row, ok := catteryLevels[level]; ok {
		return row
	}
	return catteryLevels[catteryMaxLevel()]
}

func catteryMaxLevel() uint32 {
	top := uint32(1)
	for level := range catteryLevels {
		top = max(top, level)
	}
	return top
}

// nth is the nth (1-based) value of a config list, 0 when it has none.
func nth(list []uint32, n int) uint32 {
	if n < 1 || n > len(list) {
		return 0
	}
	return list[n-1]
}
