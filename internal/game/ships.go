package game

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"google.golang.org/protobuf/proto"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/db"
	"github.com/ggmolly/belfast/internal/packets"
	"github.com/ggmolly/belfast/internal/protobuf"
)

const (
	// every ship is obtained at 50.00 affinity (captured SHIPINFO.intimacy)
	newShipIntimacy = 5000
	// a SHIPINFO always lists 5 equipment slots (ship_data_template equip_1..5),
	// empty ones with id 0
	shipEquipSlots = 5
	// Ship.ENERGY_RECOVER_TIME, the morale tick
	energyRecoverTime = 360
	// Long Island, given to every new commander next to the chosen starter:
	// the official server created her with the starter at CS_10024 (same
	// create_time, ship id 2). She is no client data, the server picks her.
	starterGiftShip = 106011
)

func (game *Game) registerShips() {
	packets.Handle(game.handleBuildShip)
	packets.Handle(game.handleBuildQuickFinish)
	packets.Handle(game.handleBuildResults)
	packets.Handle(game.handleCollectBuilds)
	packets.Handle(game.handleConfirmBuilds)
	packets.Handle(game.handleShipLock)
	packets.Handle(game.handleShipEquip)
	packets.Handle(game.handleLoveLetters)
	packets.Handle(game.handleUpgradeShipEquipment)
	packets.Handle(game.handleComposeEquipment)
	packets.HandleLenient(game.handleSellItems)
}

// NewShip returns the row for a freshly obtained ship: level 1, morale and
// level cap from its ship_data_template (energy, max_level).
func NewShip(commanderID uint32, templateID uint32) db.CreateShipParams {
	// every base template has energy 150 and max_level 70
	params := db.CreateShipParams{
		CommanderID: commanderID,
		TemplateID:  templateID,
		Level:       1,
		Energy:      150,
		Intimacy:    newShipIntimacy,
		MaxLevel:    70,
		CreatedAt:   time.Now().Unix(),
	}
	if template, ok := shipTemplates[templateID]; ok {
		params.Energy = template.Energy
		params.MaxLevel = template.MaxLevel
	}
	return params
}

// AddShip gives the commander a new ship of templateID, with the equipment
// (ship_data_template equip_id_1..3) and skills (buff_list, level 1) it comes
// with, and records it in the archives. When that unlocks a new archive entry
// its SC_17002 is pushed (the official server sends it before the reply or
// push carrying the ship).
func (tx *Tx) AddShip(templateID uint32) (*protobuf.SHIPINFO, error) {
	ctx, queries := tx.ctx, tx.Queries
	template, ok := shipTemplates[templateID]
	if !ok {
		return nil, fmt.Errorf("ship template %d: %w", templateID, errRefused)
	}
	row, err := queries.CreateShip(ctx, NewShip(tx.commanderID, templateID))
	if err != nil {
		return nil, err
	}
	equips := make([]db.ShipEquipment, shipEquipSlots)
	for i := range equips {
		equips[i] = db.ShipEquipment{ShipID: row.ID, Pos: uint32(i + 1)}
		if given := template.equips(); i < len(given) {
			equips[i].EquipID = given[i]
		}
		if err := queries.SetShipEquipment(ctx, db.SetShipEquipmentParams{ShipID: row.ID, Pos: equips[i].Pos, EquipID: equips[i].EquipID}); err != nil {
			return nil, err
		}
	}
	skills := make([]db.ShipSkill, len(template.Skills))
	for i, skill := range template.Skills {
		skills[i] = db.ShipSkill{ShipID: row.ID, SkillID: skill, Level: 1}
		if err := queries.AddShipSkill(ctx, db.AddShipSkillParams{ShipID: row.ID, SkillID: skill, Level: 1}); err != nil {
			return nil, err
		}
	}
	push, err := recordShipGroup(ctx, queries, row)
	if err != nil {
		return nil, err
	}
	if push != nil {
		tx.push(push)
	}
	return fullShipInfo(row, equips, skills), nil
}

// NewShipsPush is SC_12042, ships obtained outside of a reply (drops...),
// added by BayProxy.
func NewShipsPush(ships ...*protobuf.SHIPINFO) *protobuf.SC_12042 {
	return &protobuf.SC_12042{ShipList: ships}
}

// shipInfo is the SHIPINFO of a ship row alone, read by BayProxy into a Ship
// (model/vo/ship.lua).
func shipInfo(ship db.Ship) *protobuf.SHIPINFO {
	locked := uint32(0)
	if ship.IsLocked {
		locked = 1
	}
	return &protobuf.SHIPINFO{
		Id:                  proto.Uint32(ship.ID),
		TemplateId:          proto.Uint32(ship.TemplateID),
		Level:               proto.Uint32(ship.Level),
		Exp:                 proto.Uint32(ship.Exp),
		Energy:              proto.Uint32(ship.Energy),
		State:               proto.Uint32(ship.State),
		IsLocked:            proto.Uint32(locked),
		Intimacy:            proto.Uint32(ship.Intimacy),
		Proficiency:         proto.Uint32(ship.Proficiency),
		CreateTime:          proto.Uint32(uint32(ship.CreatedAt)),
		SkinId:              proto.Uint32(ship.SkinID),
		Propose:             proto.Uint32(ship.Propose),
		Name:                proto.String(ship.Name),
		ChangeNameTimestamp: proto.Uint32(ship.ChangeNameTimestamp),
		MaxLevel:            proto.Uint32(ship.MaxLevel),
		CommonFlag:          proto.Uint32(ship.CommonFlag),
	}
}

// fullShipInfo is the SHIPINFO of a ship with its slots and skills.
func fullShipInfo(ship db.Ship, equips []db.ShipEquipment, skills []db.ShipSkill) *protobuf.SHIPINFO {
	info := shipInfo(ship)
	info.EquipInfoList = make([]*protobuf.EQUIPSKIN_INFO, shipEquipSlots)
	for i := range info.EquipInfoList {
		info.EquipInfoList[i] = &protobuf.EQUIPSKIN_INFO{}
	}
	for _, equip := range equips {
		if equip.Pos >= 1 && equip.Pos <= shipEquipSlots {
			info.EquipInfoList[equip.Pos-1] = &protobuf.EQUIPSKIN_INFO{Id: proto.Uint32(equip.EquipID), SkinId: proto.Uint32(equip.SkinID)}
		}
	}
	for _, skill := range skills {
		info.SkillIdList = append(info.SkillIdList, &protobuf.SHIPSKILL{
			SkillId: proto.Uint32(skill.SkillID), SkillLv: proto.Uint32(skill.Level), SkillExp: proto.Uint32(skill.Exp),
		})
	}
	return info
}

// shipList is SC_12001, BayProxy's ships: the SHIPINFO of each of ships
// (the commander's, by id) with their slots and skills.
func shipList(ctx context.Context, queries *db.Queries, commanderID uint32, ships []db.Ship) (*protobuf.SC_12001, error) {
	equips, err := queries.ListShipEquipmentsByCommander(ctx, commanderID)
	if err != nil {
		return nil, err
	}
	skills, err := queries.ListShipSkillsByCommander(ctx, commanderID)
	if err != nil {
		return nil, err
	}
	equipsByShip := map[uint32][]db.ShipEquipment{}
	for _, equip := range equips {
		equipsByShip[equip.ShipID] = append(equipsByShip[equip.ShipID], equip)
	}
	skillsByShip := map[uint32][]db.ShipSkill{}
	for _, skill := range skills {
		skillsByShip[skill.ShipID] = append(skillsByShip[skill.ShipID], skill)
	}
	msg := &protobuf.SC_12001{Shiplist: make([]*protobuf.SHIPINFO, len(ships))}
	for i, ship := range ships {
		msg.Shiplist[i] = fullShipInfo(ship, equipsByShip[ship.ID], skillsByShip[ship.ID])
	}
	return msg, nil
}

// shipEquipments is the 5 slots of one of the commander's ships,
// errRefused if it isn't theirs.
func shipEquipments(ctx context.Context, queries *db.Queries, commanderID, shipID uint32) ([]db.ShipEquipment, error) {
	_, err := queries.GetShip(ctx, db.GetShipParams{ID: shipID, CommanderID: commanderID})
	if errors.Is(err, sql.ErrNoRows) {
		return nil, errRefused
	} else if err != nil {
		return nil, err
	}
	rows, err := queries.ListShipEquipmentsByCommander(ctx, commanderID)
	if err != nil {
		return nil, err
	}
	slots := make([]db.ShipEquipment, shipEquipSlots)
	for i := range slots {
		slots[i] = db.ShipEquipment{ShipID: shipID, Pos: uint32(i + 1)}
	}
	for _, row := range rows {
		if row.ShipID == shipID && row.Pos >= 1 && row.Pos <= shipEquipSlots {
			slots[row.Pos-1] = row
		}
	}
	return slots, nil
}

// grantShipsStarter gives a new commander the starter ship they chose and
// Long Island, equipped, and the starting bag (SC_12001 / SC_15001 of the
// capture). What a new commander starts with is no progress: the task events
// and pushes it records are dropped.
func grantShipsStarter(ctx context.Context, queries *db.Queries, commander db.Commander) error {
	tx := newTx(ctx, queries, commander.ID)
	for _, template := range []uint32{commander.StarterShip, starterGiftShip} {
		if _, err := tx.AddShip(template); err != nil {
			return err
		}
	}
	return grantStarterRest(tx)
}

func grantStarterRest(tx *Tx) error {
	ctx, queries, commanderID := tx.ctx, tx.Queries, tx.commanderID
	if err := grantStarterItems(tx); err != nil {
		return err
	}
	// Renovation Merit Medal (decor tokens spent) starts at 14 (captured)
	if err := queries.SetTrophy(ctx, db.SetTrophyParams{CommanderID: commanderID, TrophyID: 10041, Progress: 14}); err != nil {
		return err
	}
	_, err := ShipTrophyPushes(ctx, queries, commanderID)
	return err
}

// energyTick is SC_12031, the last morale tick: they run every
// Ship.ENERGY_RECOVER_TIME seconds since the commander was created (the
// capture's value is the creation time).
func energyTick(commander db.Commander, now int64) *protobuf.SC_12031 {
	last := commander.RegisteredAt
	if now > last {
		last += (now - last) / energyRecoverTime * energyRecoverTime
	}
	return &protobuf.SC_12031{EnergyAutoIncreaseTime: proto.Uint32(uint32(last))}
}

// handleShipLock locks or unlocks ships (UpdateShipLockCommand, CS_12022).
func (game *Game) handleShipLock(client *connection.Client, req *protobuf.CS_12022) error {
	ctx := context.Background()
	err := game.DB.InTx(ctx, func(queries *db.Queries) error {
		for _, id := range req.GetShipIdList() {
			if err := queries.SetShipLocked(ctx, db.SetShipLockedParams{IsLocked: req.GetIsLocked() == 1, ID: id, CommanderID: client.CommanderID}); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return err
	}
	return client.Send(&protobuf.SC_12023{Result: result(nil)})
}

// handleShipEquip puts equipment from the depot in a ship slot, or empties
// the slot when equip_id is 0 (EquipToShipCommand, UnequipFromShipCommand...,
// CS_12006). What was in the slot goes back to the depot.
func (game *Game) handleShipEquip(client *connection.Client, req *protobuf.CS_12006) error {
	ctx := context.Background()
	err := game.tx(ctx, client, func(tx *Tx) error {
		slots, err := shipEquipments(ctx, tx.Queries, client.CommanderID, req.GetShipId())
		if err != nil {
			return err
		}
		pos := req.GetPos()
		if pos < 1 || pos > shipEquipSlots {
			return errRefused
		}
		slot := slots[pos-1]
		if req.GetEquipId() != 0 {
			if err := tx.SpendEquipment(req.GetEquipId(), 1); err != nil {
				return err
			}
		} else if slot.EquipID == 0 {
			return errRefused
		}
		if slot.EquipID != 0 {
			if err := tx.AddEquipment(slot.EquipID, 1); err != nil {
				return err
			}
		}
		return tx.SetShipEquipment(ctx, db.SetShipEquipmentParams{ShipID: slot.ShipID, Pos: pos, EquipID: req.GetEquipId(), SkinID: slot.SkinID})
	})
	if err != nil && !refused(err) {
		return err
	}
	return client.Send(&protobuf.SC_12007{Result: result(err)})
}

// handleLoveLetters answers GetAllLoveLetterLevelDataCommand (CS_12406):
// love letters and their medals belong to married ships, none yet.
func (game *Game) handleLoveLetters(client *connection.Client, req *protobuf.CS_12406) error {
	return client.Send(&protobuf.SC_12407{})
}

// collectionState is SC_17001, CollectionProxy: the archives (one
// SHIP_STATISTICS_INFO per ship group owned) and the medals' progress.
func collectionState(ctx context.Context, queries *db.Queries, commanderID uint32) (*protobuf.SC_17001, error) {
	groups, err := queries.ListShipGroups(ctx, commanderID)
	if err != nil {
		return nil, err
	}
	trophies, err := queries.ListTrophies(ctx, commanderID)
	if err != nil {
		return nil, err
	}
	stored := map[uint32]db.CommanderTrophy{}
	for _, trophy := range trophies {
		stored[trophy.TrophyID] = trophy
	}
	msg := &protobuf.SC_17001{}
	for _, id := range archiveTrophies {
		trophy := stored[id]
		msg.ProgressList = append(msg.ProgressList, &protobuf.ACHIEVEMENT_INFO{
			Id: proto.Uint32(id), Progress: proto.Uint32(trophy.Progress), Timestamp: proto.Uint32(trophy.Timestamp),
		})
	}
	for _, group := range groups {
		// heart_count is the likes of every player of the server
		msg.ShipInfoList = append(msg.ShipInfoList, &protobuf.SHIP_STATISTICS_INFO{
			Id:          proto.Uint32(group.GroupID),
			Star:        proto.Uint32(group.Star),
			HeartFlag:   proto.Uint32(group.HeartFlag),
			HeartCount:  proto.Uint32(0),
			MarryFlag:   proto.Uint32(group.MarryFlag),
			IntimacyMax: proto.Uint32(group.IntimacyMax),
			LvMax:       proto.Uint32(group.LvMax),
		})
	}
	return msg, nil
}

// recordShipGroup adds a ship to the archives, and returns the SC_17002 of
// the archive count medal when its group is new.
func recordShipGroup(ctx context.Context, queries *db.Queries, ship db.Ship) (*protobuf.SC_17002, error) {
	before, err := queries.CountShipGroups(ctx, ship.CommanderID)
	if err != nil {
		return nil, err
	}
	err = queries.UpsertShipGroup(ctx, db.UpsertShipGroupParams{
		CommanderID: ship.CommanderID,
		GroupID:     ship.TemplateID / 10, // ship_data_template.group_type
		Star:        shipTemplates[ship.TemplateID].Star,
		IntimacyMax: ship.Intimacy,
		LvMax:       ship.Level,
	})
	if err != nil {
		return nil, err
	}
	after, err := queries.CountShipGroups(ctx, ship.CommanderID)
	if err != nil || after == before {
		return nil, err
	}
	return SetTrophy(ctx, queries, ship.CommanderID, archiveCountTrophy, uint32(after))
}

// SetTrophy stores a medal_template progress and returns its SC_17002 push.
func SetTrophy(ctx context.Context, queries *db.Queries, commanderID, trophyID, progress uint32) (*protobuf.SC_17002, error) {
	if err := queries.SetTrophy(ctx, db.SetTrophyParams{CommanderID: commanderID, TrophyID: trophyID, Progress: progress}); err != nil {
		return nil, err
	}
	return &protobuf.SC_17002{ProgressList: []*protobuf.ACHIEVEMENT_INFO{{
		Id: proto.Uint32(trophyID), Progress: proto.Uint32(progress),
	}}}, nil
}

// ShipTrophyPushes recomputes the faction medals from the commander's ships
// and returns the SC_17002 of those that changed. The official server does
// this after a battle (not when a ship is obtained), sending the affinity
// ones one by one then the exp ones together, factions in descending order.
func ShipTrophyPushes(ctx context.Context, queries *db.Queries, commanderID uint32) ([]proto.Message, error) {
	ships, err := queries.ListShipsByCommander(ctx, commanderID)
	if err != nil {
		return nil, err
	}
	trophies, err := queries.ListTrophies(ctx, commanderID)
	if err != nil {
		return nil, err
	}
	stored := map[uint32]uint32{}
	for _, trophy := range trophies {
		stored[trophy.TrophyID] = trophy.Progress
	}
	values := map[uint32]uint32{}
	for _, nation := range nationTrophies {
		values[nation.level120], values[nation.intimacy], values[nation.exp] = 0, 0, 0
	}
	for _, ship := range ships {
		for _, nation := range nationTrophies {
			if shipTemplates[ship.TemplateID].Nationality != nation.nationality {
				continue
			}
			if ship.Level >= nation.level {
				values[nation.level120]++
			}
			values[nation.intimacy] += ship.Intimacy
			values[nation.exp] += ship.Exp
		}
	}
	var pushes []proto.Message
	grouped := map[string]*protobuf.SC_17002{"level120": {}, "exp": {}}
	for _, nation := range nationTrophies {
		for _, kind := range []struct {
			name string
			id   uint32
		}{{"intimacy", nation.intimacy}, {"exp", nation.exp}, {"level120", nation.level120}} {
			if values[kind.id] == stored[kind.id] {
				continue
			}
			push, err := SetTrophy(ctx, queries, commanderID, kind.id, values[kind.id])
			if err != nil {
				return nil, err
			}
			if kind.name == "intimacy" {
				pushes = append(pushes, push)
			} else {
				grouped[kind.name].ProgressList = append(grouped[kind.name].ProgressList, push.ProgressList...)
			}
		}
	}
	for _, name := range []string{"exp", "level120"} {
		if len(grouped[name].ProgressList) > 0 {
			pushes = append(pushes, grouped[name])
		}
	}
	return pushes, nil
}
