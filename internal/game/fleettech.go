package game

import (
	"context"
	"encoding/json"
	"maps"
	"slices"
	"time"

	"google.golang.org/protobuf/proto"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/db"
	"github.com/ggmolly/belfast/internal/gamedata"
	"github.com/ggmolly/belfast/internal/protobuf"
)

// Fleet Technology (view/technology/technologytreenation*,
// TechnologyNationProxy): each fleet_tech_group (a faction) is researched one
// level (fleet_tech_template) at a time, for its cost in coins and its time,
// once the faction's tech points reach the level's pt. A researched level
// gives its level_award_display once claimed, and its attribute bonuses,
// which the commander may set lower (TechnologyTreeSetAttrLayer). The
// official capture only has an empty SC_64000: everything here is built from
// the Lua alone.

// fleetTechGroup is a fleet_tech_group row: the faction (nation[1]) whose
// points open its levels, and its levels in order.
type fleetTechGroup struct {
	Nation []uint32 `json:"nation"`
	Techs  []uint32 `json:"techs"`
}

// fleetTechBonus is one entry of a fleet_tech_template add: value more of
// attr (a TECH_NATION_ATTRS index) for the ship types.
type fleetTechBonus struct {
	shipTypes   []uint32
	attr, value uint32
}

// fleetTechLevel is a fleet_tech_template row: what researching it costs
// (cost coins, time seconds, pt points of its group's faction), the
// attribute bonuses of the group at that level (add, every level's bonuses
// so far) and its reward (level_award_display).
type fleetTechLevel struct {
	GroupID uint32            `json:"groupid"`
	Cost    uint32            `json:"cost"`
	Time    uint32            `json:"time"`
	Pt      uint32            `json:"pt"`
	Add     []json.RawMessage `json:"add"`
	Award   [][]uint32        `json:"level_award_display"`
	bonuses []fleetTechBonus
}

// fleetTechTypeOneStep is the only type AwardCampTecOneStepCommand sends.
const fleetTechTypeOneStep = 1

// techNationAttrs is #TechnologyConst.TECH_NATION_ATTRS: attr_type is an
// index of it.
const techNationAttrs = 12

var (
	fleetTechGroups map[uint32]fleetTechGroup
	fleetTechLevels map[uint32]fleetTechLevel
)

func loadFleetTechData() error {
	err := gamedata.LoadAll(
		gamedata.Load("fleet_tech_group", &fleetTechGroups),
		gamedata.Load("fleet_tech_template", &fleetTechLevels),
	)
	if err != nil {
		return err
	}
	for id, level := range fleetTechLevels {
		level.bonuses = nil
		for _, raw := range level.Add {
			var entry []json.RawMessage
			var bonus fleetTechBonus
			if json.Unmarshal(raw, &entry) != nil || len(entry) < 3 ||
				json.Unmarshal(entry[0], &bonus.shipTypes) != nil ||
				json.Unmarshal(entry[1], &bonus.attr) != nil ||
				json.Unmarshal(entry[2], &bonus.value) != nil {
				return gamedata.Missing("fleet_tech_template", "add of %d", id)
			}
			level.bonuses = append(level.bonuses, bonus)
		}
		fleetTechLevels[id] = level
	}
	return nil
}

// fleetTechIndex is how many levels of group tech is (table.indexof on its
// techs, 0 for none).
func fleetTechIndex(group fleetTechGroup, tech uint32) int {
	return slices.Index(group.Techs, tech) + 1
}

// fleetTechInfo is a group's state as SC_64000's tech_list sends it.
func fleetTechInfo(tech db.FleetTech) *protobuf.FLEETTECH {
	return &protobuf.FLEETTECH{
		GroupId:         proto.Uint32(tech.GroupID),
		EffectTechId:    proto.Uint32(tech.EffectTechID),
		StudyTechId:     proto.Uint32(tech.StudyTechID),
		StudyFinishTime: proto.Uint32(uint32(tech.StudyFinishAt)),
		RewardedTech:    proto.Uint32(tech.RewardedTechID),
	}
}

// fleetTechState is SC_64000, TechnologyNationProxy's state: the groups
// researched or being researched (a research whose time is up is finished
// by the client itself, TechnologyNationProxy:setTimer sending CS_64003)
// and the attribute bonuses set below their maximum.
func fleetTechState(ctx context.Context, queries *db.Queries, commanderID uint32) (*protobuf.SC_64000, error) {
	techs, err := queries.ListFleetTechs(ctx, commanderID)
	if err != nil {
		return nil, err
	}
	attrs, err := queries.ListFleetTechAttrs(ctx, commanderID)
	if err != nil {
		return nil, err
	}
	msg := &protobuf.SC_64000{
		TechList:    make([]*protobuf.FLEETTECH, len(techs)),
		TechsetList: make([]*protobuf.TECHSET, len(attrs)),
	}
	for i, tech := range techs {
		msg.TechList[i] = fleetTechInfo(tech)
	}
	for i, attr := range attrs {
		msg.TechsetList[i] = &protobuf.TECHSET{
			ShipType: proto.Uint32(attr.ShipType),
			AttrType: proto.Uint32(attr.AttrType),
			SetValue: proto.Uint32(attr.SetValue),
		}
	}
	return msg, nil
}

// fleetTechs are the commander's groups by id; a group never researched is
// its zero row.
func (tx *Tx) fleetTechs() (map[uint32]db.FleetTech, error) {
	rows, err := tx.ListFleetTechs(tx.ctx, tx.commanderID)
	if err != nil {
		return nil, err
	}
	techs := make(map[uint32]db.FleetTech, len(rows))
	for _, row := range rows {
		techs[row.GroupID] = row
	}
	return techs, nil
}

// fleetTech is the commander's state of group.
func (tx *Tx) fleetTech(group uint32) (db.FleetTech, error) {
	techs, err := tx.fleetTechs()
	if err != nil {
		return db.FleetTech{}, err
	}
	tech, ok := techs[group]
	if !ok {
		tech = db.FleetTech{CommanderID: tx.commanderID, GroupID: group}
	}
	return tech, nil
}

func (tx *Tx) saveFleetTech(tech db.FleetTech) error {
	return tx.SaveFleetTech(tx.ctx, db.SaveFleetTechParams(tech))
}

// fleetTechMaxBonuses are the commander's attribute bonuses by ship type and
// attr_type at their maximum, as TechnologyNationProxy:calculateTecBuff sums
// them: every owned group of fleet_tech_ship_template gives its add_get
// bonus, and its add_level one once its maxLV reached 120, then every
// group's researched level its add.
func fleetTechMaxBonuses(owned map[uint32]techShipGroup, techs map[uint32]db.FleetTech) map[uint32]map[uint32]uint32 {
	bonuses := map[uint32]map[uint32]uint32{}
	add := func(shipTypes []uint32, attr, value uint32) {
		for _, shipType := range shipTypes {
			if bonuses[shipType] == nil {
				bonuses[shipType] = map[uint32]uint32{}
			}
			bonuses[shipType][attr] += value
		}
	}
	for id, group := range owned {
		ship, ok := fleetTechShips[id]
		if !ok {
			continue
		}
		add(ship.GetShipTypes, ship.GetAttr, ship.GetValue)
		if group.level >= techShipLevel {
			add(ship.LevelShipTypes, ship.LevelAttr, ship.LevelValue)
		}
	}
	for _, tech := range techs {
		for _, bonus := range fleetTechLevels[tech.EffectTechID].bonuses {
			add(bonus.shipTypes, bonus.attr, bonus.value)
		}
	}
	return bonuses
}

// handleStartFleetTech starts researching the level tech_id of the group
// tech_group_id (StartCampTecCommand, CS_64001): the group's next level,
// while no other research runs and the faction's points reach its pt, for
// its cost in coins. On result 0 the client takes the coins itself and
// counts the level's time down from its server time, then sends CS_64003;
// it tells coloring_cell's error otherwise.
func (game *Game) handleStartFleetTech(client *connection.Client, req *protobuf.CS_64001) error {
	ctx := context.Background()
	err := game.tx(ctx, client, func(tx *Tx) error {
		group, ok := fleetTechGroups[req.GetTechGroupId()]
		level, okLevel := fleetTechLevels[req.GetTechId()]
		if !ok || !okLevel || len(group.Nation) == 0 {
			return errRefused
		}
		techs, err := tx.fleetTechs()
		if err != nil {
			return err
		}
		// one research at a time (TechnologyNationProxy:getStudyingTecItem)
		for _, tech := range techs {
			if tech.StudyTechID != 0 {
				return errRefused
			}
		}
		tech, ok := techs[req.GetTechGroupId()]
		if !ok {
			tech = db.FleetTech{CommanderID: tx.commanderID, GroupID: req.GetTechGroupId()}
		}
		next := fleetTechIndex(group, tech.EffectTechID)
		if next >= len(group.Techs) || group.Techs[next] != req.GetTechId() {
			return errRefused
		}
		points, err := techPoints(ctx, tx.Queries, tx.commanderID)
		if err != nil {
			return err
		}
		if points[group.Nation[0]] < level.Pt {
			return errRefused
		}
		if err := tx.SpendResource(ResourceGold, level.Cost); err != nil {
			return err
		}
		tech.StudyTechID = req.GetTechId()
		tech.StudyFinishAt = time.Now().Unix() + int64(level.Time)
		return tx.saveFleetTech(tech)
	})
	if err != nil && !refused(err) {
		return err
	}
	return client.Send(&protobuf.SC_64002{Result: result(err)})
}

// handleFinishFleetTech ends the research of the group tech_group_id once
// its time is up (FinishCampTecCommand, CS_64003, sent by
// TechnologyNationProxy:setTimer): the level researched becomes the group's.
// On result 0 the client sets the level and its bonuses; it tells
// coloring_cell's error otherwise.
func (game *Game) handleFinishFleetTech(client *connection.Client, req *protobuf.CS_64003) error {
	ctx := context.Background()
	err := game.tx(ctx, client, func(tx *Tx) error {
		tech, err := tx.fleetTech(req.GetTechGroupId())
		if err != nil {
			return err
		}
		if tech.StudyTechID == 0 || tech.StudyFinishAt > time.Now().Unix() {
			return errRefused
		}
		tech.EffectTechID, tech.StudyTechID, tech.StudyFinishAt = tech.StudyTechID, 0, 0
		return tx.saveFleetTech(tech)
	})
	if err != nil && !refused(err) {
		return err
	}
	return client.Send(&protobuf.SC_64004{Result: result(err)})
}

// handleFleetTechAward claims the reward of the level tech_id of the group
// group_id (AwardCampTecCommand, CS_64005): the level after the last one
// claimed, once researched (TechnologyTreeNationScene:updateTecLevelAward),
// its level_award_display. On result 0 the client adds rewards and marks the
// level claimed; it tells "64005 Error Code:" otherwise.
func (game *Game) handleFleetTechAward(client *connection.Client, req *protobuf.CS_64005) error {
	ctx := context.Background()
	var drops []*protobuf.DROPINFO
	err := game.tx(ctx, client, func(tx *Tx) error {
		group, ok := fleetTechGroups[req.GetGroupId()]
		if !ok {
			return errRefused
		}
		tech, err := tx.fleetTech(req.GetGroupId())
		if err != nil {
			return err
		}
		next := fleetTechIndex(group, tech.RewardedTechID)
		if next >= fleetTechIndex(group, tech.EffectTechID) || group.Techs[next] != req.GetTechId() {
			return errRefused
		}
		drops = configDrops(fleetTechLevels[req.GetTechId()].Award)
		tech.RewardedTechID = req.GetTechId()
		if err := tx.saveFleetTech(tech); err != nil {
			return err
		}
		return tx.grantDrops(drops)
	})
	if err != nil && !refused(err) {
		return err
	}
	reply := &protobuf.SC_64006{Result: result(err)}
	if err == nil {
		reply.Rewards = drops
	}
	return client.Send(reply)
}

// handleFleetTechAwardAll claims the rewards of every researched level not
// claimed yet, in every group (AwardCampTecOneStepCommand, CS_64007, type
// always 1). On result 0 the client adds rewards and marks every group's
// level claimed; it tells "64007 Error Code:" otherwise.
func (game *Game) handleFleetTechAwardAll(client *connection.Client, req *protobuf.CS_64007) error {
	ctx := context.Background()
	var drops []*protobuf.DROPINFO
	err := game.tx(ctx, client, func(tx *Tx) error {
		if req.GetType() != fleetTechTypeOneStep {
			return errRefused
		}
		techs, err := tx.fleetTechs()
		if err != nil {
			return err
		}
		for _, id := range slices.Sorted(maps.Keys(techs)) {
			tech, group := techs[id], fleetTechGroups[id]
			done := fleetTechIndex(group, tech.EffectTechID)
			from := fleetTechIndex(group, tech.RewardedTechID)
			if from >= done {
				continue
			}
			for _, level := range group.Techs[from:done] {
				for _, drop := range configDrops(fleetTechLevels[level].Award) {
					drops = mergeDrop(drops, drop.GetType(), drop.GetId(), drop.GetNumber())
				}
			}
			tech.RewardedTechID = tech.EffectTechID
			if err := tx.saveFleetTech(tech); err != nil {
				return err
			}
		}
		if len(drops) == 0 {
			return errRefused
		}
		return tx.grantDrops(drops)
	})
	if err != nil && !refused(err) {
		return err
	}
	reply := &protobuf.SC_64008{Result: result(err)}
	if err == nil {
		reply.Rewards = drops
	}
	return client.Send(reply)
}

// handleSetFleetTechAttrs saves the attribute bonuses set below their
// maximum (SetTecAttrAdditionCommand, CS_64009): techset_list replaces them
// all (TechnologyTreeSetAttrLayer sends every bonus that differs from its
// maximum, none to reset them), each at most the bonus the commander has
// (fleetTechMaxBonuses). On result 0 the client keeps the list and tells
// attrset_save_success; it tells "64009 Error Code:" otherwise.
func (game *Game) handleSetFleetTechAttrs(client *connection.Client, req *protobuf.CS_64009) error {
	ctx := context.Background()
	err := game.tx(ctx, client, func(tx *Tx) error {
		owned, err := techShipGroups(ctx, tx.Queries, tx.commanderID)
		if err != nil {
			return err
		}
		techs, err := tx.fleetTechs()
		if err != nil {
			return err
		}
		bonuses := fleetTechMaxBonuses(owned, techs)
		type key struct{ shipType, attr uint32 }
		seen := map[key]bool{}
		for _, set := range req.GetTechsetList() {
			k := key{set.GetShipType(), set.GetAttrType()}
			top, ok := bonuses[k.shipType][k.attr]
			if seen[k] || !ok || k.attr == 0 || k.attr > techNationAttrs || set.GetSetValue() > top {
				return errRefused
			}
			seen[k] = true
		}
		if err := tx.DeleteFleetTechAttrs(ctx, tx.commanderID); err != nil {
			return err
		}
		for _, set := range req.GetTechsetList() {
			err := tx.AddFleetTechAttr(ctx, db.AddFleetTechAttrParams{
				CommanderID: tx.commanderID,
				ShipType:    set.GetShipType(),
				AttrType:    set.GetAttrType(),
				SetValue:    set.GetSetValue(),
			})
			if err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil && !refused(err) {
		return err
	}
	return client.Send(&protobuf.SC_64010{Result: result(err)})
}
