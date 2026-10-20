package game

import (
	"context"
	"database/sql"
	"errors"
	"slices"

	"google.golang.org/protobuf/proto"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/db"
	"github.com/ggmolly/belfast/internal/gamedata"
	"github.com/ggmolly/belfast/internal/protobuf"
)

// transform_data_template row: a retrofit node. Each level (up to
// max_level) costs use_gold, use_item[level] and use_ship material ships,
// and adds effect[level] to the ship's stats. It needs the condition_id
// nodes finished, the ship at level_limit and star_limit; doing it removes
// the edit_trans nodes, moves a ship of template ship_id[i][0] to
// ship_id[i][1], teaches skill_id and puts on skin_id.
type transformConfig struct {
	ConditionID []uint32             `json:"condition_id"`
	EditTrans   []uint32             `json:"edit_trans"`
	Effect      []map[string]float64 `json:"effect"`
	LevelLimit  uint32               `json:"level_limit"`
	MaxLevel    uint32               `json:"max_level"`
	ShipID      [][]uint32           `json:"ship_id"`
	SkillID     uint32               `json:"skill_id"`
	SkinID      uint32               `json:"skin_id"`
	StarLimit   uint32               `json:"star_limit"`
	UseGold     uint32               `json:"use_gold"`
	UseItem     [][][]uint32         `json:"use_item"`
	UseShip     uint32               `json:"use_ship"`
}

// ship_data_trans row: a ship group's retrofit tree, transform_list rows of
// [position, transform_data_template id]
type shipTransConfig struct {
	TransformList [][][]uint32 `json:"transform_list"`
}

var (
	transforms map[uint32]transformConfig
	// transformGroups are the retrofit nodes of each ship group
	// (ship_data_trans transform_list), by group_type
	transformGroups map[uint32][]uint32
)

func loadTransformData() error {
	var trees map[uint32]shipTransConfig
	if err := gamedata.LoadAll(
		gamedata.Load("transform_data_template", &transforms),
		gamedata.Load("ship_data_trans", &trees),
	); err != nil {
		return err
	}
	transformGroups = make(map[uint32][]uint32, len(trees))
	for group, tree := range trees {
		for _, row := range tree.TransformList {
			for _, node := range row {
				if len(node) >= 2 {
					transformGroups[group] = append(transformGroups[group], node[1])
				}
			}
		}
	}
	return nil
}

// transformTarget is the template a node moves a ship of templateID to
// (its ship_id pair starting with it), false when it moves none.
func (config transformConfig) transformTarget(templateID uint32) (uint32, bool) {
	for _, pair := range config.ShipID {
		if len(pair) >= 2 && pair[0] == templateID {
			return pair[1], true
		}
	}
	return 0, false
}

// transformInfo is SHIPINFO.transform_list from a ship's ship_transforms.
func transformInfo(rows []db.ShipTransform) []*protobuf.TRANSFORM_INFO {
	list := make([]*protobuf.TRANSFORM_INFO, len(rows))
	for i, row := range rows {
		list[i] = &protobuf.TRANSFORM_INFO{Id: proto.Uint32(row.TransformID), Level: proto.Uint32(row.Level)}
	}
	return list
}

// transformLevels are a ship's retrofit nodes, their level by id.
func transformLevels(rows []db.ShipTransform) map[uint32]uint32 {
	levels := make(map[uint32]uint32, len(rows))
	for _, row := range rows {
		levels[row.TransformID] = row.Level
	}
	return levels
}

// transformBonus is what a ship's retrofits add to attribute (an
// AttributeType name) in Ship:getShipProperties: effect[1..level] of every
// node it has.
func transformBonus(rows []db.ShipTransform, attribute string) float64 {
	bonus := 0.0
	for _, row := range rows {
		effect := transforms[row.TransformID].Effect
		for level := range min(int(row.Level), len(effect)) {
			bonus += effect[level][attribute]
		}
	}
	return bonus
}

// remoulded is Ship:isRemoulded: a node of the ship group's tree that gives
// a skin is at its max level.
func remoulded(group uint32, levels map[uint32]uint32) bool {
	for _, id := range transformGroups[group] {
		config := transforms[id]
		if config.SkinID != 0 && levels[id] > 0 && levels[id] == config.MaxLevel {
			return true
		}
	}
	return false
}

// handleRemouldShip answers RemouldShipCommand (CS_12011,
// controller/command/ship/remouldshipcommand.lua, ShipRemouldLayer): one
// level of the retrofit node remould_id on ship_id. ShipRemouldLayer:
// canRemould offers it for a node of the ship's group tree
// (ship_data_trans) whose condition_id nodes are at their max level, to a
// ship at its level_limit and star_limit, until the node is finished
// (max_level, the command's #effect); the ship must pass onModify (not in
// the active chapter). It costs use_gold, use_item of the level and
// use_ship material ships of the ship's group or Bulins of its rarity that
// the dock lets go (ShipRemouldMediator onShip, as a limit break). A node
// moving the ship to a template of another team (ship_id) needs the ship
// out of its fleet, the client takes it out first.
//
// On result 0 the client raises the node's level, drops the edit_trans
// nodes, moves the ship to its new template (the buff_list skills that
// differ renamed in place, levels kept, the academy lesson following),
// teaches skill_id, takes the costs, puts on skin_id (not added to the
// skins owned: ShipFashionView lets a remoulded ship wear it) and flags the
// group's archive entry (ShipGroup.trans), takes off an augment the new
// template can't wear and removes the material ships, their gear and
// augments to the depot. Gear the new template can't take is taken off by
// the client with its own 12006s. No official capture has it: built from
// the Lua alone.
func (game *Game) handleRemouldShip(client *connection.Client, req *protobuf.CS_12011) error {
	err := game.tx(context.Background(), client, func(tx *Tx) error {
		return tx.remouldShip(req.GetShipId(), req.GetRemouldId(), req.GetMaterialId())
	})
	if err != nil && !refused(err) {
		return err
	}
	return client.Send(&protobuf.SC_12012{Result: result(err)})
}

func (tx *Tx) remouldShip(shipID, remouldID uint32, materialIDs []uint32) error {
	ctx := tx.ctx
	ship, err := tx.modifiableShip(shipID)
	if err != nil {
		return err
	}
	config, ok := transforms[remouldID]
	group := shipGroup(ship.TemplateID)
	template, known := ownableShips[ship.TemplateID]
	if !ok || !known || !slices.Contains(transformGroups[group], remouldID) {
		return errRefused
	}
	rows, err := tx.ListShipTransforms(ctx, ship.ID)
	if err != nil {
		return err
	}
	levels := transformLevels(rows)
	for _, id := range config.ConditionID {
		if levels[id] == 0 || levels[id] != transforms[id].MaxLevel {
			return errRefused
		}
	}
	level := levels[remouldID]
	if ship.Level < config.LevelLimit || template.Star < config.StarLimit || level >= config.MaxLevel ||
		(len(config.Effect) > 0 && int(level) >= len(config.Effect)) {
		return errRefused
	}
	if uint32(len(materialIDs)) != config.UseShip {
		return errRefused
	}
	var materials []db.Ship
	if config.UseShip > 0 {
		if materials, err = tx.materialShips(ship.ID, materialIDs); err != nil {
			return err
		}
		for _, material := range materials {
			if !upgradeMaterial(ship, material) {
				return errRefused
			}
		}
	}
	target, moves := config.transformTarget(ship.TemplateID)
	next, nextKnown := ownableShips[target]
	if moves {
		if !nextKnown {
			return errRefused
		}
		if shipTeams[template.Type] != shipTeams[next.Type] {
			// ShipStatus "onTeamChange": the client takes it out of its fleet first
			fleets, err := tx.ListFleetShips(ctx, tx.commanderID)
			if err != nil {
				return err
			}
			for _, row := range fleets {
				if row.ShipID == ship.ID {
					return errRefused
				}
			}
		}
	}

	if err := tx.SpendResource(ResourceGold, config.UseGold); err != nil {
		return err
	}
	if int(level) < len(config.UseItem) {
		for _, item := range config.UseItem[level] {
			if len(item) < 2 {
				return errRefused
			}
			if err := tx.SpendItem(item[0], item[1]); err != nil {
				return err
			}
		}
	}
	if err := tx.consumeShips(materials); err != nil {
		return err
	}
	if err := tx.SetShipTransform(ctx, db.SetShipTransformParams{ShipID: ship.ID, TransformID: remouldID, Level: level + 1}); err != nil {
		return err
	}
	for _, id := range config.EditTrans {
		if err := tx.DeleteShipTransform(ctx, db.DeleteShipTransformParams{ShipID: ship.ID, TransformID: id}); err != nil {
			return err
		}
	}
	if moves {
		if err := tx.renameTemplateSkills(ship.ID, template.Skills, next.Skills); err != nil {
			return err
		}
		ship.TemplateID = target
		if err := tx.saveShipGrowth(ship); err != nil {
			return err
		}
		if err := tx.dropUnfitSpWeapon(ship); err != nil {
			return err
		}
	}
	if config.SkillID != 0 {
		if err := tx.AddShipSkill(ctx, db.AddShipSkillParams{ShipID: ship.ID, SkillID: config.SkillID, Level: 1}); err != nil {
			return err
		}
	}
	if config.SkinID != 0 {
		if err := tx.SetShipSkin(ctx, db.SetShipSkinParams{SkinID: config.SkinID, ID: ship.ID, CommanderID: tx.commanderID}); err != nil {
			return err
		}
		if err := tx.SetShipGroupTrans(ctx, db.SetShipGroupTransParams{CommanderID: tx.commanderID, GroupID: group}); err != nil {
			return err
		}
	}
	return nil
}

// renameTemplateSkills is RemouldShipCommand's buff_list loop: each skill of
// the old template's list that differs from the new one's at the same place
// becomes it, level and exp kept (a lesson on it too), or the new one is
// learnt at level 1 when the ship hasn't the old one. One already learnt
// stays and the old one goes.
func (tx *Tx) renameTemplateSkills(shipID uint32, before, after []uint32) error {
	ctx := tx.ctx
	has := func(skill uint32) (bool, error) {
		_, err := tx.GetShipSkill(ctx, db.GetShipSkillParams{CommanderID: tx.commanderID, ShipID: shipID, SkillID: skill})
		if errors.Is(err, sql.ErrNoRows) {
			return false, nil
		}
		return err == nil, err
	}
	for i := range max(len(before), len(after)) {
		var old, skill uint32
		if i < len(before) {
			old = before[i]
		}
		if i < len(after) {
			skill = after[i]
		}
		if old == skill || skill == 0 {
			continue
		}
		hadOld, err := has(old)
		if err != nil {
			return err
		}
		hasNew, err := has(skill)
		if err != nil {
			return err
		}
		switch {
		case hadOld && !hasNew:
			if err := tx.RenameShipSkill(ctx, db.RenameShipSkillParams{NewSkillID: skill, ShipID: shipID, SkillID: old}); err != nil {
				return err
			}
			if err := tx.RenameSkillClassSkill(ctx, db.RenameSkillClassSkillParams{NewSkillID: skill, CommanderID: tx.commanderID, ShipID: shipID, SkillID: old}); err != nil {
				return err
			}
		case hadOld:
			// the new one already learnt takes its place
			if err := tx.DeleteShipSkill(ctx, db.DeleteShipSkillParams{ShipID: shipID, SkillID: old}); err != nil {
				return err
			}
		case !hasNew:
			if err := tx.AddShipSkill(ctx, db.AddShipSkillParams{ShipID: shipID, SkillID: skill, Level: 1}); err != nil {
				return err
			}
		}
	}
	return nil
}

// dropUnfitSpWeapon takes off the augment a ship wears when its template
// can't wear it any more (Ship:CanEquipSpWeapon), back to the bag.
func (tx *Tx) dropUnfitSpWeapon(ship db.Ship) error {
	uid, err := tx.GetShipSpWeapon(tx.ctx, ship.ID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	} else if err != nil {
		return err
	}
	row, err := tx.GetSpWeapon(tx.ctx, db.GetSpWeaponParams{CommanderID: tx.commanderID, ID: uid})
	if err != nil {
		return err
	}
	if spweapons[row.TemplateID].fits(ship) {
		return nil
	}
	return tx.UnequipShipSpWeapon(tx.ctx, ship.ID)
}
