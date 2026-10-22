package game

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"math"
	"slices"
	"time"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/db"
	"github.com/ggmolly/belfast/internal/gamedata"
	"github.com/ggmolly/belfast/internal/protobuf"
)

// Managing meowfficers from the meowfficer scene (CommanderCatScene): lock,
// rename and train (feed other cats to one, "play" in the client).
// Opening several boxes at once (BatchGetCommanderCommand) is a series of
// CS_25004, nothing of its own.

const (
	// RenameCommanderCommand's nameValidityCheck upper bound
	meowfficerNameMaxLength = 20
	// CommanderConst.PLAY_MAX_COUNT: the cats fed at once
	meowfficerMaxMaterials = 10
	// CommanderConst.DESTROY_ATTR_ID: the talent attribute adding to the
	// EXP a cat gives when fed (getDestoryExpValue, getDestoryExpRetio)
	meowfficerDestroyAttr = 202
)

// meowfficerFeedValues are the commander_data_template columns feeding
// reads: the EXP a cat gives (exp), the gold it costs (exp_cost), its
// default name.
type meowfficerFeedValues struct {
	ID      uint32 `json:"id"`
	Name    string `json:"name"`
	Exp     uint32 `json:"exp"`
	ExpCost uint32 `json:"exp_cost"`
}

// meowfficerSkillLevel is a commander_skill_template row: exp is the skill
// EXP from this level to next_id, 0 at the last.
type meowfficerSkillLevel struct {
	ID     uint32 `json:"id"`
	Exp    uint32 `json:"exp"`
	NextID uint32 `json:"next_id"`
	PrevID uint32 `json:"prev_id"`
}

// talentDestroyExp is what a learned talent adds to the EXP its cat gives
// when fed: value flat, ratio in 1/10000.
type talentDestroyExp struct {
	value, ratio float64
}

var (
	meowfficerFeed        map[uint32]meowfficerFeedValues
	meowfficerSkillLevels map[uint32]meowfficerSkillLevel
	meowfficerTalentFeed  map[uint32]talentDestroyExp
	// gameset: commander_rename_coldtime (seconds between renames),
	// commander_exp_a (share of a fed cat's EXP passed on, 1/10000),
	// commander_exp_same_rate (bonus for the same group, 1/10000),
	// commander_skill_exp (skill EXP a same-group cat gives)
	meowfficerRenameCooldown                                         int64
	meowfficerExpShare, meowfficerSameGroupRate, meowfficerSkillFeed float64
)

func loadMeowfficerManageData() error {
	var talents map[uint32]struct {
		Add [][]json.RawMessage `json:"add"`
	}
	err := gamedata.LoadAll(
		gamedata.Load("commander_data_template", &meowfficerFeed),
		gamedata.Load("commander_skill_template", &meowfficerSkillLevels),
		gamedata.Load("commander_ability_template", &talents),
	)
	if err != nil {
		return err
	}
	meowfficerTalentFeed = map[uint32]talentDestroyExp{}
	for id, talent := range talents {
		var feed talentDestroyExp
		for _, add := range talent.Add {
			if len(add) < 5 {
				continue
			}
			var kind, attr uint32
			var value float64
			if json.Unmarshal(add[0], &kind) != nil || json.Unmarshal(add[3], &attr) != nil || json.Unmarshal(add[4], &value) != nil {
				continue
			}
			if attr != meowfficerDestroyAttr {
				continue
			}
			switch kind {
			case talentAdditionNumber:
				feed.value += value
			case talentAdditionRatio:
				feed.ratio += value
			}
		}
		if feed != (talentDestroyExp{}) {
			meowfficerTalentFeed[id] = feed
		}
	}
	gameset, err := gamedata.Gameset()
	if err != nil {
		return err
	}
	for key, value := range map[string]*float64{
		"commander_exp_a":         &meowfficerExpShare,
		"commander_exp_same_rate": &meowfficerSameGroupRate,
		"commander_skill_exp":     &meowfficerSkillFeed,
	} {
		raw, ok := gameset[key]
		if !ok {
			return gamedata.Missing("gameset", "%s", key)
		}
		*value = float64(raw)
	}
	cooldown, ok := gameset["commander_rename_coldtime"]
	if !ok {
		return gamedata.Missing("gameset", "commander_rename_coldtime")
	}
	meowfficerRenameCooldown = int64(cooldown)
	return nil
}

// meowfficerFedExp is the EXP a cat gives the cat of group it is fed to
// (Commander:getDestoryedExp): its template exp plus commander_exp_a of all
// the EXP it earned, commander_exp_same_rate more for the same group, its
// talents' attribute 202 on top. Doubles, as in the Lua; the sum is floored.
func meowfficerFedExp(cat db.Meowfficer, group uint32) float64 {
	rarity := meowfficerTemplates[cat.TemplateID].Rarity
	earned := 0.0
	for level := uint32(1); level < cat.Level; level++ {
		earned += float64(meowfficerLevelExp(level, rarity))
	}
	value, ratio := 0.0, 0.0
	for _, talent := range cat.Talents {
		value += meowfficerTalentFeed[talent].value
		ratio += meowfficerTalentFeed[talent].ratio
	}
	same := 1.0
	if meowfficerTemplates[cat.TemplateID].GroupType == group {
		same = meowfficerSameGroupRate / 10000
	}
	return (float64(meowfficerFeed[cat.TemplateID].Exp)+(earned+float64(cat.Exp))*meowfficerExpShare/10000)*same*(1+ratio/10000) + value
}

// meowfficerFedSkillExp is the skill EXP a cat gives (getDestoryedSkillExp):
// commander_skill_exp from a cat of the same group, nothing otherwise.
func meowfficerFedSkillExp(cat db.Meowfficer, group uint32) float64 {
	if meowfficerTemplates[cat.TemplateID].GroupType == group {
		return meowfficerSkillFeed
	}
	return 0
}

// meowfficerFeedCost is the gold feeding a cat costs
// (Commander:getUpgradeConsume): exp_cost, more with its level.
func meowfficerFeedCost(cat db.Meowfficer) float64 {
	cost := float64(meowfficerFeed[cat.TemplateID].ExpCost)
	level := float64(cat.Level)
	return cost + cost*(level-1)*(0.85+0.15*level)
}

// addMeowfficerSkillExp is a skill's level (id) and EXP after gaining exp,
// as CommanderSkill:addExp: EXP keeps piling up at the last level.
func addMeowfficerSkillExp(id, exp, gained uint32) (uint32, uint32) {
	exp += gained
	for {
		skill, ok := meowfficerSkillLevels[id]
		if !ok || skill.NextID == 0 || exp < skill.Exp {
			return id, exp
		}
		exp -= skill.Exp
		id = skill.NextID
	}
}

// meowfficerSkillChain is the levels of id's skill, from Lv.1: its
// prev_id links back to the first, next_id on to the last.
func meowfficerSkillChain(id uint32) []uint32 {
	for seen := 0; meowfficerSkillLevels[id].PrevID != 0 && seen < 100; seen++ {
		id = meowfficerSkillLevels[id].PrevID
	}
	chain := []uint32{id}
	for next := meowfficerSkillLevels[id].NextID; next != 0 && len(chain) < 100; next = meowfficerSkillLevels[next].NextID {
		chain = append(chain, next)
	}
	return chain
}

// meowfficerSkillMaxed is CommanderSkill:isMaxLevel.
func meowfficerSkillMaxed(id uint32) bool {
	return meowfficerSkillLevels[id].NextID == 0
}

// handleLockMeowfficer locks (flag 1) or unlocks (flag 0) a cat
// (LockCommanderCommande, CS_25016): a locked cat can't be fed to another.
// On result 0 the client sets the flag itself. Built from the Lua alone.
func (game *Game) handleLockMeowfficer(client *connection.Client, req *protobuf.CS_25016) error {
	err := errRefused
	if req.GetFlag() <= 1 {
		err = game.tx(context.Background(), client, func(tx *Tx) error {
			if _, err := tx.GetMeowfficer(tx.ctx, db.GetMeowfficerParams{CommanderID: tx.commanderID, ID: req.GetCommanderid()}); err != nil {
				if errors.Is(err, sql.ErrNoRows) {
					return errRefused
				}
				return err
			}
			return tx.SetMeowfficerLocked(tx.ctx, db.SetMeowfficerLockedParams{
				Locked: req.GetFlag() == 1, CommanderID: tx.commanderID, ID: req.GetCommanderid(),
			})
		})
	}
	if err != nil && !refused(err) {
		return err
	}
	return client.Send(&protobuf.SC_25017{Result: result(err)})
}

// handleRenameMeowfficer renames a cat (RenameCommanderCommand, CS_25020):
// once every commander_rename_coldtime (Commander:canModifyName, counted from
// rename_time, the last rename), a name that isn't its current one (its
// template's when it has none) and passes nameValidityCheck(name, 0, 20):
// not empty, no spaces, at most 20 characters. The client's word filter
// (playerinfo_mask_word) isn't mirrored. On result 0 the client sets the
// name and the cooldown itself. Built from the Lua alone.
func (game *Game) handleRenameMeowfficer(client *connection.Client, req *protobuf.CS_25020) error {
	now := time.Now().Unix()
	name := req.GetName()
	err := errRefused
	if validNameUpTo(name, meowfficerNameMaxLength) {
		err = game.tx(context.Background(), client, func(tx *Tx) error {
			cat, err := tx.GetMeowfficer(tx.ctx, db.GetMeowfficerParams{CommanderID: tx.commanderID, ID: req.GetCommanderid()})
			if errors.Is(err, sql.ErrNoRows) {
				return errRefused
			}
			if err != nil {
				return err
			}
			current := cat.Name
			if current == "" {
				current = meowfficerFeed[cat.TemplateID].Name
			}
			if name == current || cat.RenameTime+meowfficerRenameCooldown > now {
				return errRefused
			}
			return tx.RenameMeowfficer(tx.ctx, db.RenameMeowfficerParams{Name: name, RenameTime: now, CommanderID: tx.commanderID, ID: cat.ID})
		})
	}
	if err != nil && !refused(err) {
		return err
	}
	return client.Send(&protobuf.SC_25021{Result: result(err)})
}

// handleFeedMeowfficers feeds cats to another (UpgradeCommanderCommand,
// CS_25008, the "play" page): targetid gains the materials' EXP
// (meowfficerFedExp) and its skill commander_skill_exp per cat of its group,
// for the materials' gold (meowfficerFeedCost); the materials are gone,
// out of the saved pairs too. As the client: refused when both the cat and
// its skill are at their last level, or a material is the cat itself, a
// duplicate, locked (CollectIgnoredIdsForPlay), in a fleet
// (commander_anyone_is_in_fleet, CheckFormation) or the active sortie
// (commander_is_in_battle: the client tips it but still sends), in the cattery
// (cat_sleep_notplay) or of another group once the ones before it (the
// client's selection order) take the cat to its last level (IsLegalForPlay:
// commander_select_matiral_erro, commander_exp_limit); at most
// PLAY_MAX_COUNT. On result 0 the client adds the EXP, takes the gold and
// removes the materials itself. Built from the Lua alone.
func (game *Game) handleFeedMeowfficers(client *connection.Client, req *protobuf.CS_25008) error {
	materials := req.GetMaterialid()
	err := errRefused
	if len(materials) > 0 && len(materials) <= meowfficerMaxMaterials {
		err = game.tx(context.Background(), client, func(tx *Tx) error {
			return feedMeowfficers(tx, req.GetTargetid(), materials)
		})
	}
	if err != nil && !refused(err) {
		return err
	}
	return client.Send(&protobuf.SC_25009{Result: result(err)})
}

func feedMeowfficers(tx *Tx, targetID uint32, materials []uint32) error {
	cats, err := tx.ListMeowfficers(tx.ctx, tx.commanderID)
	if err != nil {
		return err
	}
	byID := map[uint32]db.Meowfficer{}
	for _, cat := range cats {
		byID[cat.ID] = cat
	}
	target, ok := byID[targetID]
	if !ok {
		return errRefused
	}
	top := meowfficerMaxLevel()
	if target.Level >= top && meowfficerSkillMaxed(target.SkillID) {
		return errRefused
	}
	fleets, err := tx.ListFleetMeowfficers(tx.ctx, tx.commanderID)
	if err != nil {
		return err
	}
	slots, err := tx.ListMeowfficerCatteries(tx.ctx, tx.commanderID)
	if err != nil {
		return err
	}
	sortie, err := meowfficersInSortie(tx.ctx, tx.Queries, tx.commanderID)
	if err != nil {
		return err
	}
	group := meowfficerTemplates[target.TemplateID].GroupType
	exp, skillExp, gold := 0.0, 0.0, 0.0
	for i, id := range materials {
		cat, ok := byID[id]
		if !ok || id == targetID || cat.Locked || slices.Contains(materials[:i], id) || sortie[id] ||
			slices.ContainsFunc(fleets, func(slot db.ListFleetMeowfficersRow) bool { return slot.MeowfficerID == id }) ||
			slices.ContainsFunc(slots, func(slot db.MeowfficerCattery) bool { return slot.MeowfficerID == id }) {
			return errRefused
		}
		if meowfficerTemplates[cat.TemplateID].GroupType != group {
			if level, _ := addMeowfficerExp(target, uint32(math.Floor(exp))); level >= top {
				return errRefused
			}
		}
		exp += meowfficerFedExp(cat, group)
		skillExp += meowfficerFedSkillExp(cat, group)
		gold += meowfficerFeedCost(cat)
	}
	if err := tx.SpendResource(ResourceGold, uint32(math.Floor(gold))); err != nil {
		return err
	}
	level, rest := addMeowfficerExp(target, uint32(math.Floor(exp)))
	if err := tx.SetMeowfficerLevel(tx.ctx, db.SetMeowfficerLevelParams{Level: level, Exp: rest, CommanderID: tx.commanderID, ID: target.ID}); err != nil {
		return err
	}
	skill, skillRest := addMeowfficerSkillExp(target.SkillID, target.SkillExp, uint32(math.Floor(skillExp)))
	if err := tx.SetMeowfficerSkill(tx.ctx, db.SetMeowfficerSkillParams{SkillID: skill, SkillExp: skillRest, CommanderID: tx.commanderID, ID: target.ID}); err != nil {
		return err
	}
	for _, id := range materials {
		if err := tx.DeleteMeowfficerFromPresets(tx.ctx, db.DeleteMeowfficerFromPresetsParams{CommanderID: tx.commanderID, MeowfficerID: id}); err != nil {
			return err
		}
		if err := tx.DeleteMeowfficer(tx.ctx, db.DeleteMeowfficerParams{CommanderID: tx.commanderID, ID: id}); err != nil {
			return err
		}
	}
	return nil
}
