package game

import (
	"context"
	"encoding/json"
	"math"
	"slices"
	"strconv"
	"strings"
	"time"

	"google.golang.org/protobuf/proto"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/gamedata"
	"github.com/ggmolly/belfast/internal/packets"
	"github.com/ggmolly/belfast/internal/protobuf"
	"github.com/ggmolly/belfast/internal/protocol"
)

// The island's characters, outfits and NPC actions: see island_core.go.
//
// A character (IslandShip, island_chara_template, PB_ISLAND_SHIP in
// ship_sys.ship_list) is invited from an invitation (ship_sys.invite_list,
// DROP_TYPE_ISLAND_INVITATION). Its level goes up with EXP textbooks
// (island_chara_level) up to its limit-break stage times
// upgrade_level[1]; a limit break (upgrade_material) raises that, its
// stamina cap (upgrade_power) and unlocks its skill at skill_unlock
// (island_chara_skill, upgraded with its material). Its six stats get extra
// points from textbooks (att_item) up to extra_max. Its stamina is power
// as of recover_time, recovering one point every power_recover seconds
// (IslandShip:GetCurrentEnergy); gifts add stamina and statuses
// (island_buff_template). The client applies every change itself from the
// reply, so each handler makes the same change to the stored island.

// IslandCharacterAgency.NPC_CONFIG_ID: the Workerjuu, a character every
// island has without it being in ship_list.
const islandNpcShipID = 1

// IslandItem.TYPE_SHIP_EXP_BOOK
const islandItemTypeExpBook = 5

// island_item_data_template usage of the character gifts
const islandUsageGift = "usage_island_gift"

// IslandConst.GIFT_INDEX_*: which island_item usage_arg entry a gift gives
const (
	islandGiftCommon   = 0 // GIFT_INDEX_COMMON (1 in Lua)
	islandGiftFavorite = 1 // GIFT_INDEX_FAVORITE
)

// IslandBuffType
const (
	islandBuffPowerRecover           = 2
	islandBuffGlobalManageSellPrice  = 603
	islandBuffGlobalManageSellNum    = 604
	islandBuffPowerRecoverByGreeting = 701
	islandBuffAwardByGreeting        = 702
)

// IslandTaskTargetType and IslandAchievementType the server counts here
const (
	islandTaskGiveGift       = 25
	islandTaskShipExpBook    = 32
	islandTaskSkillUpgrade   = 33
	islandTaskActionHello    = 37
	islandTaskFollow         = 38
	islandAchieveGiveGift    = 23
	islandAchieveNpcInteract = 32
)

// islandShipConfig is an island_chara_template row.
type islandShipConfig struct {
	AttItem         [][]uint32          `json:"att_item"`
	ExtraMax        [][]json.RawMessage `json:"extra_max"` // [attr, [cap, unlocked cap]]
	GiftID          []uint32            `json:"gift_id"`
	Power           uint32              `json:"power"`
	PowerRecover    uint32              `json:"power_recover"`
	SkillID         uint32              `json:"skill_id"`
	SkillUnlock     uint32              `json:"skill_unlock"`
	UpgradeLevel    []uint32            `json:"upgrade_level"` // [levels per stage, limit breaks]
	UpgradeMaterial [][][2]uint32       `json:"upgrade_material"`
	UpgradePower    []uint32            `json:"upgrade_power"`
}

// islandCharaLevelConfig is an island_chara_level row.
type islandCharaLevelConfig struct {
	LevelUpExp uint32 `json:"level_up_exp"`
}

// islandSkillConfig is an island_chara_skill row: each level's effects
// (island_buff_template) and the cost of the next level.
type islandSkillConfig struct {
	Material    [][][2]uint32 `json:"material"`
	SkillEffect [][]uint32    `json:"skill_effect"`
}

// islandBuffConfig is an island_buff_template row: a status or a skill
// effect.
type islandBuffConfig struct {
	BuffGroup uint32          `json:"buff_group"`
	BuffTime  uint32          `json:"buff_time"`
	BuffType  uint32          `json:"buff_type"`
	TypeDuel  []uint32        `json:"type_duel"`
	BuffDuel  []uint32        `json:"buff_duel"`
	TypeUse   json.RawMessage `json:"type_use"`
}

// numbers is a buff's type_use as a flat list of numbers (nil when it is
// another shape).
func (config islandBuffConfig) numbers() []float64 {
	var values []float64
	if json.Unmarshal(config.TypeUse, &values) != nil {
		return nil
	}
	return values
}

// islandShipItemConfig is what this file reads of island_item_data_template.
type islandShipItemConfig struct {
	Type     uint32          `json:"type"`
	Usage    string          `json:"usage"`
	UsageArg json.RawMessage `json:"usage_arg"`
}

// value is a textbook's usage_arg: the EXP or stat points one gives.
func (config islandShipItemConfig) value() uint32 {
	var text string
	if json.Unmarshal(config.UsageArg, &text) != nil {
		return 0
	}
	value, _ := strconv.ParseUint(strings.TrimSpace(text), 10, 32)
	return uint32(value)
}

// gift is a gift's usage_arg entry: the stamina it gives and the statuses
// it adds.
func (config islandShipItemConfig) gift(index int) (uint32, []uint32, bool) {
	var entries [][2]json.RawMessage
	if config.Usage != islandUsageGift || json.Unmarshal(config.UsageArg, &entries) != nil || index >= len(entries) {
		return 0, nil, false
	}
	var energy uint32
	if json.Unmarshal(entries[index][0], &energy) != nil {
		return 0, nil, false
	}
	return energy, gamedata.IDs(entries[index][1]), true
}

// islandCostConfig is a row with a cost in island items: an
// island_skin_colordiff_template (skin_group) or
// island_dress_colordiff_template (belongto_dress) color.
type islandCostConfig struct {
	Cost          [][2]uint32 `json:"cost"`
	SkinGroup     uint32      `json:"skin_group"`
	BelongToDress uint32      `json:"belongto_dress"`
}

var (
	islandShipConfigs  map[uint32]islandShipConfig
	islandCharaLevels  map[uint32]islandCharaLevelConfig
	islandSkills       map[uint32]islandSkillConfig
	islandBuffs        map[uint32]islandBuffConfig
	islandShipItems    map[uint32]islandShipItemConfig
	islandSkinColors   map[uint32]islandCostConfig
	islandDressCosts   map[uint32]islandCostConfig
	islandMaxFollowers uint32
)

func (game *Game) registerIslandShips() {
	packets.Handle(game.handleIslandShipBreakout)
	packets.Handle(game.handleIslandShipAttrLimit)
	packets.Handle(game.handleIslandShipAttrUpgrade)
	packets.Handle(game.handleIslandShipExpBook)
	packets.Handle(game.handleIslandShipInvite)
	packets.Handle(game.handleIslandShipSkillUpgrade)
	packets.Handle(game.handleIslandShipGift)
	packets.Handle(game.handleIslandShipDress)
	packets.Handle(game.handleIslandShipSkinColor)
	packets.Handle(game.handleIslandCommanderDressRead)
	packets.Handle(game.handleIslandShipDressRead)
	packets.Handle(game.handleIslandCommanderDress)
	packets.Handle(game.handleIslandDressColor)
	packets.Handle(game.handleIslandFollower)
	packets.Handle(game.handleIslandAnimationOp)
	packets.Handle(game.handleIslandNpcActionAward)
}

func loadIslandShipData() error {
	err := gamedata.LoadAll(
		gamedata.Load("island_chara_template", &islandShipConfigs),
		gamedata.Load("island_chara_level", &islandCharaLevels),
		gamedata.Load("island_chara_skill", &islandSkills),
		gamedata.Load("island_buff_template", &islandBuffs),
		gamedata.Load("island_item_data_template", &islandShipItems),
		gamedata.Load("island_skin_colordiff_template", &islandSkinColors),
		gamedata.Load("island_dress_colordiff_template", &islandDressCosts),
		loadIslandNpcData,
	)
	if err != nil {
		return err
	}
	for id, config := range islandShipConfigs {
		if len(config.UpgradeLevel) < 2 || config.UpgradeLevel[0] == 0 || config.PowerRecover == 0 {
			return gamedata.Missing("island_chara_template", "%d upgrade_level/power_recover", id)
		}
	}
	islandMaxFollowers = islandSetInt("max_follower_cnt")
	return nil
}

// islandShipTx runs fn on the commander's island; a refusal leaves the
// island as it was. It returns the reply's result.
func (game *Game) islandShipTx(client *connection.Client, fn func(tx *Tx, state *islandState, now time.Time) error) (*uint32, error) {
	err := game.tx(context.Background(), client, func(tx *Tx) error {
		state, err := tx.island()
		if err != nil {
			return err
		}
		now := serverNow()
		islandShipsDaily(state, now)
		return fn(tx, state, now)
	})
	if err != nil && !refused(err) {
		return nil, err
	}
	return result(err), nil
}

// islandShipsExtra is the island extra key of this file: the server day the
// characters' daily state was last reset at.
const islandShipsExtra = "ships"

type islandShipsExtraState struct {
	Day int64 `json:"day"`
}

// islandShipsDaily starts a new day on the island if the server day
// changed since the last one, as the client does at midnight
// (Island:UpdatePerDay): the characters' once-a-day greeting skills can be
// used again (ResetShipSkillUsed) and the strolling characters give their
// greeting rewards again (IslandNpcFeedbackAgency:UpdatePerDay).
func islandShipsDaily(state *islandState, now time.Time) {
	var extra islandShipsExtraState
	_, _ = state.getExtra(islandShipsExtra, &extra)
	day := startOfDay(now).Unix()
	if extra.Day == day {
		return
	}
	for _, ship := range state.public().GetShipSys().GetShipList() {
		ship.SkillUseState = proto.Uint32(0)
	}
	state.private().ActionFeedbackNpcList = nil
	_ = state.setExtra(islandShipsExtra, islandShipsExtraState{Day: day})
}

// islandShip is one of the island's characters, nil if it has none of that
// id.
func (state *islandState) islandShip(id uint32) *protobuf.PB_ISLAND_SHIP {
	for _, ship := range state.public().GetShipSys().GetShipList() {
		if ship.GetId() == id {
			return ship
		}
	}
	return nil
}

// takeIslandItems takes every cost from the warehouse, or nothing when one
// is short (ErrNotEnough).
func (state *islandState) takeIslandItems(costs [][2]uint32) error {
	need := map[uint32]uint32{}
	for _, cost := range costs {
		need[cost[0]] += cost[1]
	}
	for id, count := range need {
		if state.islandItemCount(id) < count {
			return ErrNotEnough
		}
	}
	for _, cost := range costs {
		if err := state.takeIslandItem(cost[0], cost[1]); err != nil {
			return err
		}
	}
	return nil
}

// newIslandShip is a character as it joins the island: level 1 at its
// first limit-break stage (IslandShip:InitMaxLevel, stage x
// upgrade_level[1]), full stamina, its skill locked unless unlocked from
// the start. GUESS: no capture shows one; the stage and stamina follow
// from the tables (upgrade_material has an entry per stage from 1).
func newIslandShip(id uint32, config islandShipConfig, now time.Time) *protobuf.PB_ISLAND_SHIP {
	ship := &protobuf.PB_ISLAND_SHIP{
		Id: proto.Uint32(id), Lv: proto.Uint32(1), BreakLv: proto.Uint32(1),
		Power: proto.Uint32(config.maxEnergy(1)), RecoverTime: proto.Uint32(uint32(now.Unix())),
		WorkPlace: &protobuf.PB_SHIP_WORK_PLACE{},
	}
	if config.SkillUnlock <= 1 {
		ship.SkillLv = proto.Uint32(1)
	}
	return protocol.FillRequired(ship)
}

// maxLevel is the level cap at a limit-break stage (IslandShip:InitMaxLevel).
func (config islandShipConfig) maxLevel(breakLv uint32) uint32 {
	return breakLv * config.UpgradeLevel[0]
}

// maxBreak is the last limit-break stage (IslandShip:GetBreakMaxLevel).
func (config islandShipConfig) maxBreak() uint32 {
	return config.UpgradeLevel[1] + 1
}

// maxEnergy is the stamina cap at a limit-break stage
// (IslandShip:InitMaxEnergy).
func (config islandShipConfig) maxEnergy(breakLv uint32) uint32 {
	energy := config.Power
	for i := uint32(0); i < breakLv && int(i) < len(config.UpgradePower); i++ {
		energy += config.UpgradePower[i]
	}
	return energy
}

// extraMax is the cap of a stat's extra points, before or after its limit
// is unlocked (IslandShip:InitMaxExtraAttrs).
func (config islandShipConfig) extraMax(attr uint32, unlocked bool) uint32 {
	for _, entry := range config.ExtraMax {
		var id uint32
		var caps [2]uint32
		if len(entry) != 2 || json.Unmarshal(entry[0], &id) != nil || json.Unmarshal(entry[1], &caps) != nil || id != attr {
			continue
		}
		if unlocked {
			return caps[1]
		}
		return caps[0]
	}
	return 0
}

// skillUnlocked tells whether a character's skill is unlocked
// (IslandShip:InitSkill).
func (config islandShipConfig) skillUnlocked(ship *protobuf.PB_ISLAND_SHIP) bool {
	return config.SkillUnlock <= ship.GetBreakLv()
}

// skillEffects are the effects (island_buff_template) of a character's
// skill at its level, none while it is locked.
func (config islandShipConfig) skillEffects(ship *protobuf.PB_ISLAND_SHIP) []uint32 {
	level := ship.GetSkillLv()
	if !config.skillUnlocked(ship) || level == 0 {
		return nil
	}
	effects := islandSkills[config.SkillID].SkillEffect
	if int(level) > len(effects) {
		return nil
	}
	return effects[level-1]
}

// globalEffects tells whether some of effects are global (IslandBuffType
// GetGlobalTypes): the characters whose skill has one are listed in
// global_buff.forever_list (IslandGlobalBuffAgency).
func globalEffects(effects []uint32) bool {
	return slices.ContainsFunc(effects, func(id uint32) bool {
		kind := islandBuffs[id].BuffType
		return kind == islandBuffGlobalManageSellPrice || kind == islandBuffGlobalManageSellNum
	})
}

// skillChanged updates global_buff.forever_list after a character's skill
// was unlocked or upgraded (IslandGlobalBuffAgency OnShipSkillUnlock,
// OnShipSkillUpgrade).
func (state *islandState) skillChanged(ship *protobuf.PB_ISLAND_SHIP, config islandShipConfig) {
	global := state.private().GlobalBuff
	if global == nil {
		global = protocol.FillRequired(&protobuf.PB_ISLAND_GLOBAL_BUFF{})
		state.private().GlobalBuff = global
	}
	id := ship.GetId()
	global.ForeverList = slices.DeleteFunc(global.ForeverList, func(other uint32) bool { return other == id })
	if globalEffects(config.skillEffects(ship)) {
		global.ForeverList = append(global.ForeverList, id)
	}
}

// addExp adds EXP to a character the way the client does
// (IslandShip:AddExp): levels up while it reaches level_up_exp, up to its
// cap, where its EXP is 0.
func (config islandShipConfig) addExp(ship *protobuf.PB_ISLAND_SHIP, exp uint32) {
	max := config.maxLevel(ship.GetBreakLv())
	level := ship.GetLv()
	if level >= max {
		return
	}
	total := ship.GetExp() + exp
	for level < max {
		need := islandCharaLevels[level].LevelUpExp
		if need == 0 || total < need {
			break
		}
		total -= need
		level++
	}
	if level >= max {
		total = 0
	}
	ship.Lv, ship.Exp = proto.Uint32(level), proto.Uint32(total)
}

// islandShipWorking tells whether a character is busy (delegated, in a lab
// or restaurant) rather than idle (IslandShip:GetState).
func islandShipWorking(ship *protobuf.PB_ISLAND_SHIP, now time.Time) bool {
	return int64(ship.GetRecoverTime()) > now.Unix() && ship.GetWorkPlace().GetType() != 0
}

// statusEnd is when a character's status ends, 0 for never
// (IslandShipStatus:InitEndTime).
func statusEnd(status *protobuf.PB_ISLAND_BUFF) int64 {
	duration := islandBuffs[status.GetId()].BuffTime
	if duration == 0 {
		return 0
	}
	return int64(status.GetStartTime()) + int64(duration)
}

// statusValid tells whether a character's status still runs.
func statusValid(status *protobuf.PB_ISLAND_BUFF, now time.Time) bool {
	end := statusEnd(status)
	return end == 0 || end > now.Unix()
}

// islandShipEnergy is a character's stamina now (IslandShip:GetCurrentEnergy):
// power plus a point every power_recover seconds since recover_time, faster
// with its skill's and statuses' stamina recovery, up to its cap.
func islandShipEnergy(ship *protobuf.PB_ISLAND_SHIP, config islandShipConfig, now time.Time) uint32 {
	max := float64(config.maxEnergy(ship.GetBreakLv()))
	energy := float64(ship.GetPower())
	if islandShipWorking(ship, now) {
		return uint32(math.Min(max, energy))
	}
	speed := float64(config.PowerRecover)
	elapsed := float64(now.Unix() - int64(ship.GetRecoverTime()))
	current := math.Floor(energy + elapsed/speed)
	skillBonus := 0.0
	for _, id := range config.skillEffects(ship) {
		if buff := islandBuffs[id]; buff.BuffType == islandBuffPowerRecover {
			if values := buff.numbers(); len(values) > 0 {
				skillBonus += values[0]
			}
		}
	}
	if skillBonus > 0 {
		current += math.Floor(math.Floor(elapsed/speed) * skillBonus * 0.01)
	}
	for _, status := range ship.GetBuffList() {
		buff := islandBuffs[status.GetId()]
		values := buff.numbers()
		if buff.BuffType != islandBuffPowerRecover || !statusValid(status, now) || len(values) == 0 {
			continue
		}
		from := math.Max(float64(ship.GetRecoverTime()), float64(status.GetStartTime()))
		to := math.Min(float64(now.Unix()), float64(statusEnd(status)))
		if overlap := to - from; overlap > 0 {
			current += math.Floor(overlap / speed * values[0] * 0.01)
		}
		break
	}
	return uint32(math.Max(0, math.Min(max, current)))
}

// addStatus gives a character a status from now (IslandShip:AddStatus): it
// replaces the same status, a running one of a group it cancels
// (type_duel) and one it cancels by id (buff_duel). The ended ones are
// dropped.
func addStatus(ship *protobuf.PB_ISLAND_SHIP, id uint32, now time.Time) {
	config := islandBuffs[id]
	list := slices.DeleteFunc(ship.BuffList, func(status *protobuf.PB_ISLAND_BUFF) bool {
		return status.GetId() == id || !statusValid(status, now)
	})
	if i := slices.IndexFunc(list, func(status *protobuf.PB_ISLAND_BUFF) bool {
		return slices.Contains(config.TypeDuel, islandBuffs[status.GetId()].BuffGroup)
	}); i >= 0 {
		list = slices.Delete(list, i, i+1)
	}
	if i := slices.IndexFunc(list, func(status *protobuf.PB_ISLAND_BUFF) bool {
		return slices.Contains(config.BuffDuel, status.GetId())
	}); i >= 0 {
		list = slices.Delete(list, i, i+1)
	}
	ship.BuffList = append(list, &protobuf.PB_ISLAND_BUFF{Id: proto.Uint32(id), StartTime: proto.Uint32(uint32(now.Unix()))})
}

// handleIslandShipBreakout answers IslandShipBreakOutCommand (CS_21601): a
// character at its level cap goes to its next limit-break stage for
// upgrade_material[stage]. Its stamina cap rises (and its stamina with it,
// IslandShip:UpgradeBreakOut) and its skill unlocks at skill_unlock (level
// 1, IslandShipSkill:Unlock). GUESS: refused below the level cap (the
// breakout page only opens there).
func (game *Game) handleIslandShipBreakout(client *connection.Client, req *protobuf.CS_21601) error {
	ret, err := game.islandShipTx(client, func(tx *Tx, state *islandState, now time.Time) error {
		ship := state.islandShip(req.GetShipId())
		config, ok := islandShipConfigs[req.GetShipId()]
		if ship == nil || !ok {
			return errRefused
		}
		stage := ship.GetBreakLv()
		if stage >= config.maxBreak() || ship.GetLv() < config.maxLevel(stage) {
			return errRefused
		}
		var costs [][2]uint32
		if stage >= 1 && int(stage) <= len(config.UpgradeMaterial) {
			costs = config.UpgradeMaterial[stage-1]
		}
		if err := state.takeIslandItems(costs); err != nil {
			return err
		}
		wasUnlocked := config.skillUnlocked(ship)
		ship.BreakLv = proto.Uint32(stage + 1)
		ship.Power = proto.Uint32(ship.GetPower() + config.maxEnergy(stage+1) - config.maxEnergy(stage))
		if !wasUnlocked && config.skillUnlocked(ship) {
			if ship.GetSkillLv() == 0 {
				ship.SkillLv = proto.Uint32(1)
			}
			state.skillChanged(ship, config)
		}
		return nil
	})
	if err != nil {
		return err
	}
	return client.Send(&protobuf.SC_21602{Result: ret})
}

// islandAttrLimitItem is the island item IslandShip:GetExtraAttrLimitUnlockConsume
// asks for to raise a character's stat caps (hard-coded in the client; no
// island_item_data_template row has it yet).
const islandAttrLimitItem = 100000

// handleIslandShipAttrLimit answers IslandShipAttrLimitUnlockCommand
// (CS_21603): a character's extra stat caps go to their second value
// (extra_max, up_limit_state 1). GUESS: it costs the one item the page
// shows (islandAttrLimitItem), which the client doesn't take off itself.
func (game *Game) handleIslandShipAttrLimit(client *connection.Client, req *protobuf.CS_21603) error {
	ret, err := game.islandShipTx(client, func(tx *Tx, state *islandState, now time.Time) error {
		ship := state.islandShip(req.GetShipId())
		if ship == nil || ship.GetUpLimitState() == 1 {
			return errRefused
		}
		if err := state.takeIslandItems([][2]uint32{{islandAttrLimitItem, 1}}); err != nil {
			return err
		}
		ship.UpLimitState = proto.Uint32(1)
		return nil
	})
	if err != nil {
		return err
	}
	return client.Send(&protobuf.SC_21604{Result: ret})
}

// handleIslandShipAttrUpgrade answers IslandShipAttrUpgradeCommand
// (CS_21605): textbooks of a stat (type, IslandShipAttr index; att_item)
// add their usage_arg in extra points to it, up to its cap
// (IslandShip:AddExtraAttr). Refused when the stat is already at its cap
// (IslandShipAttrUpgradePage:CanAddItemForAttrValue).
func (game *Game) handleIslandShipAttrUpgrade(client *connection.Client, req *protobuf.CS_21605) error {
	ret, err := game.islandShipTx(client, func(tx *Tx, state *islandState, now time.Time) error {
		ship := state.islandShip(req.GetShipId())
		config, ok := islandShipConfigs[req.GetShipId()]
		attr := req.GetType()
		if ship == nil || !ok || attr == 0 || int(attr) > len(config.AttItem) || len(req.GetItemList()) == 0 {
			return errRefused
		}
		var costs [][2]uint32
		added := uint32(0)
		for _, item := range req.GetItemList() {
			if item.GetNum() == 0 || !slices.Contains(config.AttItem[attr-1], item.GetId()) {
				return errRefused
			}
			costs = append(costs, [2]uint32{item.GetId(), item.GetNum()})
			added += islandShipItems[item.GetId()].value() * item.GetNum()
		}
		limit := config.extraMax(attr, ship.GetUpLimitState() == 1)
		var entry *protobuf.PB_SHIP_ATTR
		for _, extra := range ship.ExtraAttrList {
			if extra.GetId() == attr {
				entry = extra
			}
		}
		if entry == nil {
			entry = &protobuf.PB_SHIP_ATTR{Id: proto.Uint32(attr), Value: proto.Uint32(0)}
		}
		if entry.GetValue() >= limit {
			return errRefused
		}
		if err := state.takeIslandItems(costs); err != nil {
			return err
		}
		if !slices.Contains(ship.ExtraAttrList, entry) {
			ship.ExtraAttrList = append(ship.ExtraAttrList, entry)
		}
		entry.Value = proto.Uint32(min(entry.GetValue()+added, limit))
		return nil
	})
	if err != nil {
		return err
	}
	return client.Send(&protobuf.SC_21606{Result: ret})
}

// handleIslandShipExpBook answers IslandUseShipExpBookCommand (CS_21607):
// EXP textbooks (island item type 5) give a character their usage_arg in
// EXP each, add_exp in the reply, which the client adds itself
// (IslandShip:AddExp). GUESS: refused at the level cap (the page doesn't
// let one pick a book there).
func (game *Game) handleIslandShipExpBook(client *connection.Client, req *protobuf.CS_21607) error {
	added := uint32(0)
	ret, err := game.islandShipTx(client, func(tx *Tx, state *islandState, now time.Time) error {
		ship := state.islandShip(req.GetShipId())
		config, ok := islandShipConfigs[req.GetShipId()]
		if ship == nil || !ok || len(req.GetItemList()) == 0 || ship.GetLv() >= config.maxLevel(ship.GetBreakLv()) {
			return errRefused
		}
		var costs [][2]uint32
		exp := uint32(0)
		for _, item := range req.GetItemList() {
			book, ok := islandShipItems[item.GetId()]
			if item.GetNum() == 0 || !ok || book.Type != islandItemTypeExpBook {
				return errRefused
			}
			costs = append(costs, [2]uint32{item.GetId(), item.GetNum()})
			exp += book.value() * item.GetNum()
		}
		if err := state.takeIslandItems(costs); err != nil {
			return err
		}
		config.addExp(ship, exp)
		added = exp
		for _, item := range req.GetItemList() {
			if err := tx.islandTaskEvent(islandTaskShipExpBook, item.GetNum(), item.GetId()); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return err
	}
	return client.Send(&protobuf.SC_21608{Result: ret, AddExp: proto.Uint32(added)})
}

// handleIslandShipInvite answers IslandInviteShipCommand (CS_21609): an
// invitation (ship_sys.invite_list) becomes the character, sent whole in
// the reply (newIslandShip).
func (game *Game) handleIslandShipInvite(client *connection.Client, req *protobuf.CS_21609) error {
	var ship *protobuf.PB_ISLAND_SHIP
	id := req.GetShipId()
	ret, err := game.islandShipTx(client, func(tx *Tx, state *islandState, now time.Time) error {
		ships := state.public().ShipSys
		config, ok := islandShipConfigs[id]
		if !ok || id == islandNpcShipID || !slices.Contains(ships.InviteList, id) || state.islandShip(id) != nil {
			return errRefused
		}
		ships.InviteList = slices.DeleteFunc(ships.InviteList, func(other uint32) bool { return other == id })
		ship = newIslandShip(id, config, now)
		ships.ShipList = append(ships.ShipList, ship)
		if config.skillUnlocked(ship) {
			state.skillChanged(ship, config)
		}
		return nil
	})
	if err != nil {
		return err
	}
	if ship == nil {
		ship = protocol.FillRequired(&protobuf.PB_ISLAND_SHIP{})
	}
	return client.Send(&protobuf.SC_21610{Result: ret, Ship: ship})
}

// handleIslandShipSkillUpgrade answers IslandShipSkillUpgradeCommand
// (CS_21611): an unlocked skill goes up a level for material[level]
// (IslandShip:CanUpgradeSkill). The client takes material[level + 1] off
// its own count (it reads the cost after Upgrade); belfast takes the cost
// the page showed, the next island data corrects the client.
func (game *Game) handleIslandShipSkillUpgrade(client *connection.Client, req *protobuf.CS_21611) error {
	ret, err := game.islandShipTx(client, func(tx *Tx, state *islandState, now time.Time) error {
		ship := state.islandShip(req.GetShipId())
		config, ok := islandShipConfigs[req.GetShipId()]
		if ship == nil || !ok || !config.skillUnlocked(ship) {
			return errRefused
		}
		skill := islandSkills[config.SkillID]
		level := max(ship.GetSkillLv(), 1)
		if int(level) >= len(skill.SkillEffect) || int(level) > len(skill.Material) {
			return errRefused
		}
		if err := state.takeIslandItems(skill.Material[level-1]); err != nil {
			return err
		}
		ship.SkillLv = proto.Uint32(level + 1)
		state.skillChanged(ship, config)
		return tx.islandTaskEvent(islandTaskSkillUpgrade, 1)
	})
	if err != nil {
		return err
	}
	return client.Send(&protobuf.SC_21612{Result: ret})
}

// handleIslandShipGift answers IslandGiveGiftCommand (CS_21613): a gift
// (usage_island_gift) gives a character the stamina (up to its cap,
// IslandShip:AddEnergy) and statuses of its usage_arg entry, the favorite
// one for the character's gift_id. One gift is used.
func (game *Game) handleIslandShipGift(client *connection.Client, req *protobuf.CS_21613) error {
	ret, err := game.islandShipTx(client, func(tx *Tx, state *islandState, now time.Time) error {
		ship := state.islandShip(req.GetShipId())
		config, ok := islandShipConfigs[req.GetShipId()]
		gift := req.GetGiftId()
		index := islandGiftCommon
		if slices.Contains(config.GiftID, gift) {
			index = islandGiftFavorite
		}
		energy, statuses, valid := islandShipItems[gift].gift(index)
		if ship == nil || !ok || !valid {
			return errRefused
		}
		if err := state.takeIslandItems([][2]uint32{{gift, 1}}); err != nil {
			return err
		}
		for _, id := range statuses {
			addStatus(ship, id, now)
		}
		ship.Power = proto.Uint32(min(ship.GetPower()+energy, config.maxEnergy(ship.GetBreakLv())))
		if err := tx.islandTaskEvent(islandTaskGiveGift, 1, gift); err != nil {
			return err
		}
		return tx.islandAchievementEvent(islandAchieveGiveGift, 0, 1)
	})
	if err != nil {
		return err
	}
	return client.Send(&protobuf.SC_21614{Result: ret})
}
