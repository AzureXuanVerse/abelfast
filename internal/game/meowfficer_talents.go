package game

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand/v2"
	"slices"
	"strconv"
	"time"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/db"
	"github.com/ggmolly/belfast/internal/gamedata"
	"github.com/ggmolly/belfast/internal/protobuf"
)

// Meowfficer talents (CommanderTalent, commander_ability_template): every
// CommanderConst.TALENT_POINT_LEVEL levels gives a talent point, spent on
// one of the talents CS_25010 offers (an upgrade of a learned one or a new
// one); a reset (CS_25014) goes back to the talents the cat came with.

const (
	// CommanderConst.TALENT_POINT_LEVEL and TALENT_POINT: the levels a talent
	// point takes, the points they give (Commander:getTotalPoint)
	meowfficerTalentPointLevel = 5
	meowfficerTalentPoint      = 1
	// commander_data_template ability_refresh_type of the cats whose talents
	// follow their ability_show (Commander:IsRegularTalent, the event cats)
	meowfficerFixedTalents = 1
)

// meowfficerTalent is a commander_ability_template row. worth is its level
// in its group (1..3), next the one it upgrades to (0 none); add's
// entries are [type, nations, ship types, attr, value], a non-empty nations
// list keeping the talent to cats of those nations (the faction talents).
type meowfficerTalent struct {
	ID      uint32          `json:"id"`
	GroupID uint32          `json:"group_id"`
	Worth   uint32          `json:"worth"`
	Next    uint32          `json:"next"`
	Cost    uint32          `json:"cost"`
	Add     json.RawMessage `json:"add"`
	nations []uint32
}

// meowfficerTalentGroup is a commander_ability_group row: its talents by
// level.
type meowfficerTalentGroup struct {
	ID          uint32   `json:"id"`
	AbilityList []uint32 `json:"ability_list"`
}

// meowfficerTalentCat is what the talents read of a commander_data_template
// row: ability_show is the fixed cats' talents, the ones they start with
// then the ones each talent point learns in order.
type meowfficerTalentCat struct {
	ID                 uint32   `json:"id"`
	Rarity             uint32   `json:"rarity"`
	Nationality        uint32   `json:"nationality"`
	MaxLevel           uint32   `json:"max_level"`
	AbilityRefreshType uint32   `json:"ability_refresh_type"`
	AbilityShow        []uint32 `json:"ability_show"`
}

var (
	meowfficerTalents      map[uint32]meowfficerTalent
	meowfficerTalentGroups map[uint32]meowfficerTalentGroup
	meowfficerTalentCats   map[uint32]meowfficerTalentCat
	// gameset: commander_ability_limit (talents a cat holds,
	// CommanderConst.MAX_TELENT_COUNT), commander_learn_ability_total (talents
	// offered), commander_learn_ability_own (of which upgrades at most),
	// commander_ability_reset_time (1: a reset a server day) and
	// commander_ability_reset_coldtime (else the seconds between resets)
	meowfficerTalentLimit, meowfficerTalentOffers, meowfficerTalentUpgradeOffers uint32
	meowfficerResetDaily                                                         bool
	meowfficerResetCooldown                                                      int64
	// gameset commander_skill_reset_cost description[1]: the gold a reset
	// costs by talent points spent (Commander:getResetTalentConsume)
	meowfficerResetCosts []uint32
	// meowfficerTalentRules is server content,
	// server/meowfficer_talents.json of the data dir, from the bwiki's
	// 指挥喵天赋 section: what the regular cats start with and which talents
	// are only the fixed cats'. Without it the cats out of a box start with
	// no talents and every talent can be offered.
	meowfficerTalentRules meowfficerTalentRulesFile
)

// meowfficerTalentRulesFile is server/meowfficer_talents.json ("_" keys are
// comments):
//   - starting: by cat rarity, the possible starting sets, one drawn
//     uniformly; a set lists talent levels (worth), 0 a special talent (of a
//     single-level group) (指挥喵天赋: "SSR喵初始：3个1级天赋 / 1个2级天赋+2个1级
//     天赋 / 2个2级天赋 … 特殊天赋算1级天赋。SSR喵若初始带3个1级天赋，则必定带有特殊天赋")
//   - fixed_only: the talents only the fixed cats have (天赋列表's yellow rows)
//   - pools: by commander_data_template id, the talents the cat may start
//     with (初始天赋池)
type meowfficerTalentRulesFile struct {
	Starting  map[string][][]uint32 `json:"starting"`
	FixedOnly []uint32              `json:"fixed_only"`
	Pools     map[string][]uint32   `json:"pools"`
	fixed     map[uint32]bool
	pools     map[uint32][]uint32
}

func loadMeowfficerTalentData() error {
	err := gamedata.LoadAll(
		gamedata.Load("commander_ability_template", &meowfficerTalents),
		gamedata.Load("commander_ability_group", &meowfficerTalentGroups),
		gamedata.Load("commander_data_template", &meowfficerTalentCats),
		loadMeowfficerTalentRules,
	)
	if err != nil {
		return err
	}
	for id, talent := range meowfficerTalents {
		var add []json.RawMessage
		_ = json.Unmarshal(talent.Add, &add)
		for _, entry := range add {
			var fields []json.RawMessage
			if json.Unmarshal(entry, &fields) == nil && len(fields) > 1 {
				talent.nations = append(talent.nations, gamedata.IDs(fields[1])...)
			}
		}
		meowfficerTalents[id] = talent
	}
	gameset, err := gamedata.Gameset()
	if err != nil {
		return err
	}
	for key, value := range map[string]*uint32{
		"commander_ability_limit":       &meowfficerTalentLimit,
		"commander_learn_ability_total": &meowfficerTalentOffers,
		"commander_learn_ability_own":   &meowfficerTalentUpgradeOffers,
	} {
		if *value = gameset[key]; *value == 0 {
			return gamedata.Missing("gameset", "%s", key)
		}
	}
	meowfficerResetDaily = gameset["commander_ability_reset_time"] == 1
	meowfficerResetCooldown = int64(gameset["commander_ability_reset_coldtime"])
	var costs [][]uint32
	if err := gamedata.GamesetDescription("commander_skill_reset_cost", &costs); err != nil {
		return err
	}
	if len(costs) == 0 || len(costs[0]) == 0 {
		return gamedata.Missing("gameset", "commander_skill_reset_cost description")
	}
	meowfficerResetCosts = costs[0]
	return nil
}

func loadMeowfficerTalentRules() error {
	meowfficerTalentRules = meowfficerTalentRulesFile{}
	raw, err := gamedata.ServerFile("meowfficer_talents.json")
	if raw == nil || err != nil {
		return err
	}
	rules := meowfficerTalentRulesFile{fixed: map[uint32]bool{}, pools: map[uint32][]uint32{}}
	if err := json.Unmarshal(raw, &rules); err != nil {
		return fmt.Errorf("meowfficer_talents.json: %w", err)
	}
	for _, id := range rules.FixedOnly {
		rules.fixed[id] = true
	}
	for key, pool := range rules.Pools {
		id, err := strconv.ParseUint(key, 10, 32)
		if err != nil {
			return fmt.Errorf("meowfficer_talents.json: pool %q", key)
		}
		rules.pools[uint32(id)] = pool
	}
	meowfficerTalentRules = rules
	return nil
}

// talentGroup is the group of a talent, 0 for an unknown one.
func talentGroup(id uint32) uint32 {
	return meowfficerTalents[id].GroupID
}

// specialTalent tells a talent of a single-level group (被期待的新星…).
func specialTalent(id uint32) bool {
	return len(meowfficerTalentGroups[talentGroup(id)].AbilityList) == 1
}

// learnableTalent tells whether a regular cat of nation may learn a talent:
// not a fixed cat's, and a faction talent only for its own nation.
func learnableTalent(id, nation uint32) bool {
	talent, ok := meowfficerTalents[id]
	if !ok || meowfficerTalentRules.fixed[id] {
		return false
	}
	return len(talent.nations) == 0 || slices.Contains(talent.nations, nation)
}

// meowfficerStartingTalents draws the talents a new cat comes with: a fixed
// cat's first ability_show entries (those its talent points don't learn),
// a regular cat one of its rarity's starting sets out of its pool.
func meowfficerStartingTalents(templateID uint32, roll func(n int) int) db.IDList {
	cat := meowfficerTalentCats[templateID]
	if cat.AbilityRefreshType == meowfficerFixedTalents {
		points := int(cat.MaxLevel / meowfficerTalentPointLevel * meowfficerTalentPoint)
		return slices.Clone(cat.AbilityShow[:max(len(cat.AbilityShow)-points, 0)])
	}
	sets := meowfficerTalentRules.Starting[strconv.Itoa(int(cat.Rarity))]
	pool := meowfficerTalentRules.pools[templateID]
	if len(sets) == 0 || len(pool) == 0 {
		return db.IDList{}
	}
	set := slices.Clone(sets[roll(len(sets))])
	// the specials first, then the higher levels: the any-level-1 picks go
	// last so a special set aside for a 0 is still there
	rank := func(tier uint32) int {
		if tier == 0 {
			return 0
		}
		return 10 - int(tier)
	}
	slices.SortFunc(set, func(a, b uint32) int { return rank(a) - rank(b) })
	talents := db.IDList{}
	for _, tier := range set {
		var candidates []uint32
		for _, id := range pool {
			if slices.ContainsFunc(talents, func(have uint32) bool { return talentGroup(have) == talentGroup(id) }) {
				continue
			}
			if (tier == 0 && specialTalent(id)) || (tier != 0 && meowfficerTalents[id].Worth == tier) {
				candidates = append(candidates, id)
			}
		}
		if len(candidates) > 0 {
			talents = append(talents, candidates[roll(len(candidates))])
		}
	}
	return talents
}

// meowfficerTalentPoints is Commander:getTalentPoint, the points left.
func meowfficerTalentPoints(cat db.Meowfficer) uint32 {
	total := cat.Level / meowfficerTalentPointLevel * meowfficerTalentPoint
	return total - min(cat.UsedPt, total)
}

// drawMeowfficerTalentChoices draws what a talent point may learn
// (指挥喵天赋: "使用天赋点时，出现4个可选天赋。优先出现当前可升级天赋，但最多出现2个
// （若当前可升级天赋数大于2，随机选取2个）。其余可选天赋位置出现随机的新天赋"): the
// next level of up to commander_learn_ability_own learned talents, then
// level-1 talents of groups it has none of, commander_learn_ability_total
// in all. A fixed cat is offered the next talent of its ability_show.
func drawMeowfficerTalentChoices(cat db.Meowfficer, roll func(n int) int) db.IDList {
	template := meowfficerTalentCats[cat.TemplateID]
	if template.AbilityRefreshType == meowfficerFixedTalents {
		next := len(cat.TalentOrigins) + int(cat.UsedPt)
		if next >= len(template.AbilityShow) {
			return db.IDList{}
		}
		return db.IDList{template.AbilityShow[next]}
	}
	shuffle := func(list []uint32) {
		for i := len(list) - 1; i > 0; i-- {
			j := roll(i + 1)
			list[i], list[j] = list[j], list[i]
		}
	}
	var upgrades []uint32
	for _, id := range cat.Talents {
		if next := meowfficerTalents[id].Next; next != 0 && learnableTalent(next, template.Nationality) {
			upgrades = append(upgrades, next)
		}
	}
	shuffle(upgrades)
	choices := db.IDList(upgrades[:min(len(upgrades), int(meowfficerTalentUpgradeOffers))])
	var fresh []uint32
	for _, group := range meowfficerTalentGroups {
		if len(group.AbilityList) == 0 {
			continue
		}
		first := group.AbilityList[0]
		if !learnableTalent(first, template.Nationality) || slices.ContainsFunc(cat.Talents, func(id uint32) bool { return talentGroup(id) == group.ID }) {
			continue
		}
		fresh = append(fresh, first)
	}
	slices.Sort(fresh)
	shuffle(fresh)
	for _, id := range fresh {
		if len(choices) >= int(meowfficerTalentOffers) {
			break
		}
		choices = append(choices, id)
	}
	return choices
}

// meowfficerNextReset is Commander:GetNextResetAbilityTime: the next server
// midnight after the last reset (or that plus the cooldown).
func meowfficerNextReset(talentTime int64) int64 {
	if meowfficerResetDaily {
		return nextDay(time.Unix(talentTime, 0)).Unix()
	}
	return talentTime + meowfficerResetCooldown
}

// sameTalents is Commander:IsSameTalent: the cat holds what it came with.
func sameTalents(cat db.Meowfficer) bool {
	return len(cat.TalentOrigins) == len(cat.Talents) && !slices.ContainsFunc(cat.TalentOrigins, func(id uint32) bool {
		return !slices.Contains(cat.Talents, id)
	})
}

// txMeowfficer is the commander's cat id, errRefused when there is none.
func txMeowfficer(tx *Tx, id uint32) (db.Meowfficer, error) {
	cat, err := tx.GetMeowfficer(tx.ctx, db.GetMeowfficerParams{CommanderID: tx.commanderID, ID: id})
	if errors.Is(err, sql.ErrNoRows) {
		return cat, errRefused
	}
	return cat, err
}

// handleFetchMeowfficerTalents answers FetchCommanderTalentCommand
// (CS_25010, when the talent page's learn panel opens on a cat with a
// point and no offer yet): abilityid is what the point may learn, kept
// until it is learned or the talents reset so that asking again (or
// relogging) doesn't draw anew. The panel only opens with a point left
// (CommanderCatTalentPage: commander_skill_point_noengough). No official
// capture has it: built from the Lua alone.
func (game *Game) handleFetchMeowfficerTalents(client *connection.Client, req *protobuf.CS_25010) error {
	var choices db.IDList
	err := game.tx(context.Background(), client, func(tx *Tx) error {
		cat, err := txMeowfficer(tx, req.GetCommanderid())
		if err != nil {
			return err
		}
		if meowfficerTalentPoints(cat) == 0 {
			return errRefused
		}
		if choices = cat.TalentChoices; len(choices) > 0 {
			return nil
		}
		if choices = drawMeowfficerTalentChoices(cat, rand.IntN); len(choices) == 0 {
			return errRefused
		}
		return tx.SetMeowfficerTalentChoices(tx.ctx, db.SetMeowfficerTalentChoicesParams{TalentChoices: choices, CommanderID: tx.commanderID, ID: cat.ID})
	})
	if err != nil && !refused(err) {
		return err
	}
	if err != nil {
		choices = nil
	}
	return client.Send(&protobuf.SC_25011{Result: result(err), Abilityid: choices})
}

// handleLearnMeowfficerTalent spends a talent point (CommanderLearnTalentCommand,
// CS_25012): targetid must be one of the offers (commander_talent_not_exist),
// replaceid 0 or a learned talent (commander_replace_talent_not_exist), the
// talent's cost in gold at hand (common_no_resource). On result 0 the
// client takes the gold, drops the learned talent of the same group (an
// upgrade) and replaceid (the replace page's pick when the cat holds
// commander_ability_limit talents of other groups), adds the talent, counts
// the point used and forgets the offers; all done the same here. Built from
// the Lua alone.
func (game *Game) handleLearnMeowfficerTalent(client *connection.Client, req *protobuf.CS_25012) error {
	target, replace := req.GetTargetid(), req.GetReplaceid()
	err := game.tx(context.Background(), client, func(tx *Tx) error {
		cat, err := txMeowfficer(tx, req.GetCommanderid())
		if err != nil {
			return err
		}
		talent, ok := meowfficerTalents[target]
		if !ok || !slices.Contains(cat.TalentChoices, target) || meowfficerTalentPoints(cat) == 0 {
			return errRefused
		}
		if replace != 0 && !slices.Contains(cat.Talents, replace) {
			return errRefused
		}
		upgrade := slices.ContainsFunc(cat.Talents, func(id uint32) bool { return talentGroup(id) == talent.GroupID })
		if !upgrade && replace == 0 && len(cat.Talents) >= int(meowfficerTalentLimit) {
			return errRefused
		}
		if err := tx.SpendResource(ResourceGold, talent.Cost); err != nil {
			return err
		}
		talents := slices.DeleteFunc(slices.Clone(cat.Talents), func(id uint32) bool {
			return talentGroup(id) == talent.GroupID || id == replace
		})
		return tx.SetMeowfficerTalents(tx.ctx, db.SetMeowfficerTalentsParams{
			Talents: append(talents, target), TalentChoices: db.IDList{}, UsedPt: cat.UsedPt + 1,
			TalentTime: cat.TalentTime, CommanderID: tx.commanderID, ID: cat.ID,
		})
	})
	if err != nil && !refused(err) {
		return err
	}
	return client.Send(&protobuf.SC_25013{Result: result(err)})
}

// handleResetMeowfficerTalents goes back to the talents a cat came with
// (ResetCommanderTalentsCommand, CS_25014, the talent page's reset of a
// regular cat; the fixed ones have none): once a server day
// (commander_reset_talent_time_no_rearch), only when its talents changed
// (commander_reset_talent_is_not_need), for commander_skill_reset_cost's
// gold by points used (common_no_resource). On result 0 the client takes
// the gold, restores the talents, gives the points back, forgets the
// offers and sets ability_time to now; all done the same here. Built from
// the Lua alone.
func (game *Game) handleResetMeowfficerTalents(client *connection.Client, req *protobuf.CS_25014) error {
	now := time.Now().Unix()
	err := game.tx(context.Background(), client, func(tx *Tx) error {
		cat, err := txMeowfficer(tx, req.GetCommanderid())
		if err != nil {
			return err
		}
		if meowfficerTalentCats[cat.TemplateID].AbilityRefreshType == meowfficerFixedTalents ||
			now < meowfficerNextReset(cat.TalentTime) || sameTalents(cat) ||
			cat.UsedPt == 0 || int(cat.UsedPt) > len(meowfficerResetCosts) {
			return errRefused
		}
		if err := tx.SpendResource(ResourceGold, meowfficerResetCosts[cat.UsedPt-1]); err != nil {
			return err
		}
		return tx.SetMeowfficerTalents(tx.ctx, db.SetMeowfficerTalentsParams{
			Talents: slices.Clone(cat.TalentOrigins), TalentChoices: db.IDList{}, UsedPt: 0,
			TalentTime: now, CommanderID: tx.commanderID, ID: cat.ID,
		})
	})
	if err != nil && !refused(err) {
		return err
	}
	return client.Send(&protobuf.SC_25015{Result: result(err)})
}
