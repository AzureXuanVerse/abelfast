package game

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"slices"

	"google.golang.org/protobuf/proto"

	"github.com/ggmolly/belfast/internal/db"
	"github.com/ggmolly/belfast/internal/gamedata"
	"github.com/ggmolly/belfast/internal/logger"
	"github.com/ggmolly/belfast/internal/protobuf"
)

// Meowfficers in battle: the cats a sortie's fleets take along
// (GROUPINCHAPTER_P13.commander_list, read by ChapterFleet:updateCommanders)
// and what their talents change in the rolls the server makes (ambush
// evasion).

// commander_ability_template add entries: [kind, nations, ship types, attr,
// value] (CommanderTalent:getAttrsAddition)
const (
	talentAdditionNumber = 1 // CommanderConst.TALENT_ADDITION_NUMBER
	talentAdditionRatio  = 2 // CommanderConst.TALENT_ADDITION_RATIO, 1/10000
	// attr of a talent: the index of AttributeType.Dodge in
	// CommanderConst.PROPERTIES
	talentAttrDodge = 9
)

// talentAddition is one commander_ability_template add entry.
type talentAddition struct {
	kind, attr         uint32
	nations, shipTypes []uint32 // empty: any
	value              float64
}

var (
	meowfficerTalentAdds map[uint32][]talentAddition // by talent id
	// meowfficerDodgeAbility is Commander:getAbilitysAddition's evasion
	// term, in percent: commander_attribute_template has no rate_dodge,
	// so it is (commander_form_a - commander_form_b / commander_form_c) x
	// commander_form_n whatever the cat (0 with the EN gameset)
	meowfficerDodgeAbility float64
)

func loadMeowfficerBattleData() error {
	var talents map[uint32]struct {
		Add [][]json.RawMessage `json:"add"`
	}
	var attributes map[uint32]map[string]any
	err := gamedata.LoadAll(
		gamedata.Load("commander_ability_template", &talents),
		gamedata.Load("commander_attribute_template", &attributes),
	)
	if err != nil {
		return err
	}
	meowfficerTalentAdds = make(map[uint32][]talentAddition, len(talents))
	for id, row := range talents {
		for _, entry := range row.Add {
			if len(entry) < 5 {
				continue
			}
			var add talentAddition
			if json.Unmarshal(entry[0], &add.kind) != nil || json.Unmarshal(entry[3], &add.attr) != nil || json.Unmarshal(entry[4], &add.value) != nil {
				return fmt.Errorf("commander_ability_template %d: add %s", id, entry)
			}
			add.nations, add.shipTypes = gamedata.IDs(entry[1]), gamedata.IDs(entry[2])
			meowfficerTalentAdds[id] = append(meowfficerTalentAdds[id], add)
		}
	}
	for id, row := range attributes {
		if rate, _ := row["rate_dodge"].(float64); rate != 0 {
			logger.WithFields("Meowfficer", logger.FieldValue("attribute", id)).Warn("commander_attribute_template has a rate_dodge, the ambush evasion leaves it out")
		}
	}
	gameset, err := gamedata.Gameset()
	if err != nil {
		return err
	}
	if gameset["commander_form_c"] == 0 {
		return gamedata.Missing("gameset", "commander_form_c")
	}
	ability := float64(gameset["commander_form_a"]) - float64(gameset["commander_form_b"])/float64(gameset["commander_form_c"])
	meowfficerDodgeAbility = math.Round(ability*float64(gameset["commander_form_n"])*1000) / 1000
	return nil
}

// teamMeowfficers is a TEAM_INFO's cats as a fleet's commander list: the
// command cat at pos 1, the support cat at 2.
func teamMeowfficers(team *protobuf.TEAM_INFO) []*protobuf.COMMANDERSINFO {
	var list []*protobuf.COMMANDERSINFO
	for pos, id := range []uint32{team.GetCommanderMain(), team.GetCommanderSub()} {
		if id != 0 {
			list = append(list, &protobuf.COMMANDERSINFO{Pos: proto.Uint32(uint32(pos + 1)), Id: proto.Uint32(id)})
		}
	}
	return list
}

// checkTeamMeowfficers checks the cats of a sortie's or an elite fleet's
// teams as the client picks them (Commander.canEquipToFleetList): owned, in
// one team at most, never two of the same group in a team. With dropUnowned
// a cat no longer owned is taken out (Chapter:EliteCommanderFilter, the
// saved elite fleets may still name a cat used as material) instead of
// refused.
func checkTeamMeowfficers(ctx context.Context, queries *db.Queries, commanderID uint32, teams []*protobuf.TEAM_INFO, dropUnowned bool) error {
	cats, err := ownedMeowfficers(ctx, queries, commanderID)
	if err != nil {
		return err
	}
	seen := map[uint32]bool{}
	for _, team := range teams {
		ids := []*uint32{team.CommanderMain, team.CommanderSub}
		groups := map[uint32]bool{}
		for i, id := range ids {
			if id == nil || *id == 0 {
				continue
			}
			cat, ok := cats[*id]
			if !ok && dropUnowned {
				ids[i] = proto.Uint32(0)
				continue
			}
			group := meowfficerTemplates[cat.TemplateID].GroupType
			if !ok || seen[*id] || groups[group] {
				return errRefused
			}
			seen[*id], groups[group] = true, true
		}
		team.CommanderMain, team.CommanderSub = ids[0], ids[1]
	}
	return nil
}

func ownedMeowfficers(ctx context.Context, queries *db.Queries, commanderID uint32) (map[uint32]db.Meowfficer, error) {
	list, err := queries.ListMeowfficers(ctx, commanderID)
	if err != nil {
		return nil, err
	}
	cats := make(map[uint32]db.Meowfficer, len(list))
	for _, cat := range list {
		cats[cat.ID] = cat
	}
	return cats, nil
}

// groupMeowfficers are the cats a chapter fleet took along that are still
// owned (ChapterFleet:updateCommanders skips the others), by pos.
func groupMeowfficers(ctx context.Context, queries *db.Queries, commanderID uint32, group *protobuf.GROUPINCHAPTER_P13) ([]db.Meowfficer, error) {
	if len(group.GetCommanderList()) == 0 {
		return nil, nil
	}
	cats, err := ownedMeowfficers(ctx, queries, commanderID)
	if err != nil {
		return nil, err
	}
	var list []db.Meowfficer
	for _, slot := range group.GetCommanderList() {
		if cat, ok := cats[slot.GetId()]; ok {
			list = append(list, cat)
		}
	}
	return list, nil
}

// meowfficersInSortie are the cats in the active sortie's fleets: the client
// won't train one of them (UpgradeCommanderCommand, commander_is_in_battle),
// nor feed it as material.
func meowfficersInSortie(ctx context.Context, queries *db.Queries, commanderID uint32) (map[uint32]bool, error) {
	session, err := loadChapter(ctx, queries, commanderID)
	if err != nil || session == nil {
		return nil, err
	}
	busy := map[uint32]bool{}
	for _, groups := range [][]*protobuf.GROUPINCHAPTER_P13{session.info.MainGroupList, session.info.SubmarineGroupList, session.info.SupportGroupList} {
		for _, group := range groups {
			for _, slot := range group.GetCommanderList() {
				busy[slot.GetId()] = true
			}
		}
	}
	return busy, nil
}

// talentBonus sums an attr's additions over a cat's talents for a ship of a
// nationality and type (Commander:getTalentsAddition): a talent counts its
// last matching entry of the kind (getAttrsAddition keeps one per attr).
func talentBonus(cat db.Meowfficer, kind, attr, nationality, shipType uint32) float64 {
	total := 0.0
	for _, talent := range cat.Talents {
		var found *talentAddition
		for i, add := range meowfficerTalentAdds[talent] {
			if add.kind == kind && add.attr == attr {
				found = &meowfficerTalentAdds[talent][i]
			}
		}
		if found == nil {
			continue
		}
		if len(found.nations) > 0 && !slices.Contains(found.nations, nationality) || len(found.shipTypes) > 0 && !slices.Contains(found.shipTypes, shipType) {
			continue
		}
		total += found.value
	}
	return total
}

// attrBonus is what a fleet's cats add to one attribute of a ship.
type attrBonus struct {
	flat, rate float64
}

// meowfficerDodge is what a fleet's cats add to a ship's evasion in
// Ship:getProperties: getAttrValueAddition flat, and getAttrRatioAddition
// / 100 to its rate (the abilities' term, in percent, plus the ratio
// talents in 1/10000).
func meowfficerDodge(cats []db.Meowfficer, ship db.Ship) attrBonus {
	template := shipTemplates[ship.TemplateID]
	var bonus attrBonus
	for _, cat := range cats {
		bonus.flat += talentBonus(cat, talentAdditionNumber, talentAttrDodge, template.Nationality, template.Type)
		ratio := talentBonus(cat, talentAdditionRatio, talentAttrDodge, template.Nationality, template.Type)
		bonus.rate += (meowfficerDodgeAbility + ratio/100) / 100
	}
	return bonus
}
