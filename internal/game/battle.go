package game

import (
	"context"
	"math"
	"math/rand/v2"

	"google.golang.org/protobuf/proto"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/db"
	"github.com/ggmolly/belfast/internal/protobuf"
)

// const.lua SYSTEM_SCENARIO: a battle on a campaign map
const systemScenario = 1

// ys.Battle.BattleConst.BattleScore: above C is a win (Chapter:writeBack)
const (
	scoreC = 1
	scoreS = 4
)

// rankExpRate multiplies ship exp by battle score (D, C, B, A, S). S is the
// only one the capture shows; the others are the commonly documented rates.
var rankExpRate = [...]float64{0, 0.5, 0.8, 1.0, 1.2}

const (
	// scenarioExpRate: the capture's ship exp is expedition exp x rank x morale
	// x 2 (x flagship / mvp); the client data doesn't say where the 2 comes from.
	scenarioExpRate = 2
	// happyExpRate is the "EXP bonus" of energy_template 4 (121-150 morale).
	happyExpRate    = 1.2
	happyEnergyFrom = 121
	maxIntimacy     = 10000
	// scenarioPlayerExp is the commander exp of a won campaign battle in the
	// capture (both a level 1 and a level 2 stage).
	scenarioPlayerExp = 65
	// scenarioGold / scenarioBossGold are the coins (Coins, 59001 in the
	// award lists) a won normal / boss battle dropped in the capture; the
	// amount isn't in the client data.
	scenarioGold     = 9
	scenarioBossGold = 44
	// a boss battle's draws from the chapter's award pools (capture: one tech
	// pack, three gear parts, one ship)
	bossTechPacks = 1
	bossGearParts = 3
	bossShips     = 1
)

// handleBeginStage starts a battle (BeginStageCommand.SendRequest, CS_40001).
// key is the token the client mixes into its CS_40003 check value; ships
// lose battle_consume_energy morale (the client does the same on result 0).
// On a campaign map the fleet must stand on the enemy being fought.
func (game *Game) handleBeginStage(client *connection.Client, req *protobuf.CS_40001) error {
	ctx := context.Background()
	if req.GetSystem() == systemScenario {
		session, err := loadChapter(ctx, game.DB.Queries, client.CommanderID)
		if err != nil {
			return err
		}
		if session == nil || session.battleCell(req.GetData()) == nil {
			return client.Send(&protobuf.SC_40002{Result: proto.Uint32(1)})
		}
	}
	ships, err := game.DB.ListShipsByCommander(ctx, client.CommanderID)
	if err != nil {
		return err
	}
	byID := make(map[uint32]db.Ship, len(ships))
	for _, ship := range ships {
		byID[ship.ID] = ship
	}
	err = game.DB.InTx(ctx, func(queries *db.Queries) error {
		for _, id := range req.GetShipIdList() {
			ship, ok := byID[id]
			if !ok {
				continue
			}
			ship.Energy -= min(ship.Energy, battleConsumeEnergy)
			if err := saveShipBattleState(ctx, queries, ship); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return err
	}
	return client.Send(&protobuf.SC_40002{Result: proto.Uint32(0), Key: proto.Uint32(rand.Uint32N(10000))})
}

// battleCell is the cell the active fleet fights on, if it holds expedition id.
func (session *chapterSession) battleCell(id uint32) *protobuf.CHAPTERCELLINFO_P13 {
	group := session.activeGroup()
	if group == nil {
		return nil
	}
	cell := session.cell(group.GetPos().GetRow(), group.GetPos().GetColumn())
	if !isEnemyCell(cell) || cell.GetItemId() != id {
		return nil
	}
	return cell
}

// handleFinishStage settles a battle (FinishStageCommand.SendRequest,
// CS_40003). On a campaign map a win (score above C) disables the enemy's
// cell (an ambush's is cleared), the fleet spends one ammo and keeps its hp.
// The reply's ship_exp_list is exp gained (the client levels ships itself),
// intimacy is 10000 + affinity gained, energy extra morale spent; drop_info is
// added by the client through PlayerConst.addTranDrop, new ships come in
// SC_12042 right after.
func (game *Game) handleFinishStage(client *connection.Client, req *protobuf.CS_40003) error {
	ctx := context.Background()
	if req.GetSystem() != systemScenario {
		return client.Send(&protobuf.SC_40004{Result: proto.Uint32(0), PlayerExp: proto.Uint32(0), Mvp: proto.Uint32(0)})
	}
	session, err := loadChapter(ctx, game.DB.Queries, client.CommanderID)
	if err != nil {
		return err
	}
	var cell *protobuf.CHAPTERCELLINFO_P13
	if session != nil {
		cell = session.battleCell(req.GetData())
	}
	expedition, known := expeditionTemplates[req.GetData()]
	if cell == nil || !known {
		return client.Send(&protobuf.SC_40004{Result: proto.Uint32(1), PlayerExp: proto.Uint32(0), Mvp: proto.Uint32(0)})
	}
	group := session.activeGroup()
	score := min(req.GetScore(), scoreS)
	win := score > scoreC

	ships, err := game.DB.ListShipsByCommander(ctx, client.CommanderID)
	if err != nil {
		return err
	}
	byID := make(map[uint32]db.Ship, len(ships))
	for _, ship := range ships {
		byID[ship.ID] = ship
	}

	reply := &protobuf.SC_40004{Result: proto.Uint32(0), PlayerExp: proto.Uint32(0)}
	mvp := uint32(0)
	best := uint32(0)
	for _, stat := range req.GetStatistics() {
		if _, ok := byID[stat.GetShipId()]; ok && (mvp == 0 || stat.GetDamageCause() > best) {
			mvp, best = stat.GetShipId(), stat.GetDamageCause()
		}
	}
	reply.Mvp = proto.Uint32(mvp)
	flagship := uint32(0)
	if len(group.ShipList) > 0 {
		flagship = group.ShipList[0].GetId()
	}

	var updated []db.Ship
	for _, stat := range req.GetStatistics() {
		ship, ok := byID[stat.GetShipId()]
		if !ok {
			continue
		}
		for _, inChapter := range group.ShipList {
			if inChapter.GetId() == ship.ID {
				inChapter.HpRant = proto.Uint32(min(stat.GetHpRest(), fullHP))
			}
		}
		gained := shipBattleExp(expedition, score, ship, ship.ID == flagship, ship.ID == mvp)
		affinity := uint32(0)
		if win {
			affinity = battleAffinity(ship.ID == flagship, ship.ID == mvp)
		}
		reply.ShipExpList = append(reply.ShipExpList, &protobuf.SHIP_EXP{
			ShipId:   proto.Uint32(ship.ID),
			Exp:      proto.Uint32(gained),
			Intimacy: proto.Uint32(maxIntimacy + affinity),
			Energy:   proto.Uint32(0),
		})
		addShipExp(&ship, gained)
		ship.Intimacy = min(ship.Intimacy+affinity, maxIntimacy)
		updated = append(updated, ship)
	}
	group.Bullet = proto.Uint32(group.GetBullet() - min(group.GetBullet(), 1))

	if win {
		reply.PlayerExp = proto.Uint32(scenarioPlayerExp)
		if cell.GetItemType() == attachAmbush {
			setCell(cell, attachNone, 0, 0)
		} else {
			cell.ItemFlag = proto.Uint32(cellFlagDisabled)
		}
		group.KillCount = proto.Uint32(group.GetKillCount() + 1)
		session.info.KillCount = proto.Uint32(session.info.GetKillCount() + 1)
		session.info.ContinuousKillCount = proto.Uint32(session.info.GetContinuousKillCount() + 1)
		reply.DropInfo = scenarioDrops(session.template, expedition)
	} else {
		session.info.ContinuousKillCount = proto.Uint32(0)
	}

	var got granted
	err = game.DB.InTx(ctx, func(queries *db.Queries) error {
		for _, ship := range updated {
			if err := saveShipBattleState(ctx, queries, ship); err != nil {
				return err
			}
		}
		if win {
			progress, err := chapterProgress(ctx, queries, client.CommanderID, session.template.id)
			if err != nil {
				return err
			}
			if expedition.isBoss() {
				progress.KillBossCount++
			} else {
				progress.KillEnemyCount++
			}
			if err := saveChapterProgress(ctx, queries, progress); err != nil {
				return err
			}
		}
		// TODO(player area): add reply.PlayerExp to the commander's exp/level
		got, err = grantDrops(ctx, queries, client.CommanderID, reply.DropInfo)
		if err != nil {
			return err
		}
		return session.save(ctx, queries, client.CommanderID)
	})
	if err != nil {
		return err
	}
	// capture: archive updates (SC_17002) before the result, the new ships
	// (SC_12042) after it
	if err := sendAll(client, got.Pushes); err != nil {
		return err
	}
	if err := client.Send(reply); err != nil {
		return err
	}
	if len(got.Ships) == 0 {
		return nil
	}
	return client.Send(NewShipsPush(got.Ships...))
}

// shipBattleExp is the exp a ship gets for a battle: expedition exp x rank x
// scenarioExpRate, x1.2 when happy, x mvp_exp_percent for the MVP and
// x flagship_exp_percent for the flagship.
func shipBattleExp(expedition expeditionTemplate, score uint32, ship db.Ship, flagship, mvp bool) uint32 {
	exp := float64(expedition.exp) * rankExpRate[score] * scenarioExpRate
	if ship.Energy >= happyEnergyFrom {
		exp *= happyExpRate
	}
	if mvp {
		exp = exp * mvpExpPercent / 100
	}
	if flagship {
		exp = exp * flagshipExpPercent / 100
	}
	return uint32(math.Floor(exp))
}

// battleAffinity is the affinity (intimacy, 1/100) a win gives. It is random
// server-side; the ranges cover what the capture's two battles gave (4-5 for
// any ship, 4-9 for the flagship, 8-14 for the MVP).
func battleAffinity(flagship, mvp bool) uint32 {
	gain := 4 + rand.Uint32N(2)
	if flagship {
		gain += rand.Uint32N(6)
	}
	if mvp {
		gain += 4 + rand.Uint32N(6)
	}
	return gain
}

// addShipExp is Ship:addExp: exp is the progress into the current level,
// ship_level.exp per level, capped at the ship's max_level. UR ships use
// ship_level.exp_ur, which needs ship_data_statistics.rarity (ships area).
func addShipExp(ship *db.Ship, gained uint32) {
	if ship.Level >= ship.MaxLevel {
		return
	}
	ship.Exp += gained
	for ship.Level < ship.MaxLevel && int(ship.Level) <= len(shipLevelExp) && ship.Exp >= shipLevelExp[ship.Level-1] {
		ship.Exp -= shipLevelExp[ship.Level-1]
		ship.Level++
	}
	if ship.Level >= ship.MaxLevel {
		ship.Exp = 0
	}
}

func saveShipBattleState(ctx context.Context, queries *db.Queries, ship db.Ship) error {
	return queries.UpdateShipBattleState(ctx, db.UpdateShipBattleStateParams{
		Level:       ship.Level,
		Exp:         ship.Exp,
		Intimacy:    ship.Intimacy,
		Energy:      ship.Energy,
		ID:          ship.ID,
		CommanderID: ship.CommanderID,
	})
}

// scenarioDrops rolls a won battle's loot. Drop tables live server-side; this
// follows the capture: coins for every battle, and for a boss a Specialized
// Core plus draws from the chapter's award pools (tech pack, gear parts, a
// ship), in the capture's order.
func scenarioDrops(template *chapterTemplate, expedition expeditionTemplate) []*protobuf.DROPINFO {
	if !expedition.isBoss() {
		return []*protobuf.DROPINFO{dropInfo(dropTypeResource, ResourceGold, scenarioGold)}
	}
	drops := []*protobuf.DROPINFO{
		dropInfo(dropTypeVItem, vitemSpecializedCore, 1),
		dropInfo(dropTypeResource, ResourceGold, scenarioBossGold),
	}
	var techPacks, parts, ships []dropRef
	for _, award := range template.awards {
		pool := dropPools[award]
		if len(pool) == 0 {
			continue
		}
		switch {
		case pool[0].kind == dropTypeShip:
			ships = pool
		case pool[0].id >= 30000:
			techPacks = pool
		default:
			parts = pool
		}
	}
	drops = appendDraws(drops, techPacks, bossTechPacks)
	drops = appendDraws(drops, parts, bossGearParts)
	return appendDraws(drops, ships, bossShips)
}

// dropInfo is one DROPINFO of a reward.
func dropInfo(kind, id, number uint32) *protobuf.DROPINFO {
	return &protobuf.DROPINFO{Type: proto.Uint32(kind), Id: proto.Uint32(id), Number: proto.Uint32(number)}
}

// appendDraws draws count times from pool, merging repeats (the capture's
// "17041 x2").
func appendDraws(drops []*protobuf.DROPINFO, pool []dropRef, count int) []*protobuf.DROPINFO {
	if len(pool) == 0 {
		return drops
	}
	start := len(drops)
	for range count {
		ref := pool[rand.IntN(len(pool))]
		merged := false
		for _, existing := range drops[start:] {
			if ref.kind != dropTypeShip && existing.GetType() == ref.kind && existing.GetId() == ref.id {
				existing.Number = proto.Uint32(existing.GetNumber() + 1)
				merged = true
			}
		}
		if !merged {
			drops = append(drops, dropInfo(ref.kind, ref.id, 1))
		}
	}
	return drops
}
