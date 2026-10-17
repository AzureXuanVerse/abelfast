package game

import (
	"context"
	"math"
	"math/rand/v2"

	"google.golang.org/protobuf/proto"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/db"
	"github.com/ggmolly/belfast/internal/logger"
	"github.com/ggmolly/belfast/internal/protobuf"
)

// const.lua SYSTEM_SCENARIO: a battle on a campaign map
const systemScenario = 1

// ys.Battle.BattleConst.BattleScore: above C is a win (Chapter:writeBack)
const (
	scoreC = 1
	scoreA = 3
	scoreS = 4
)

// rankExpRate multiplies ship exp by battle score. The client never computes
// battle exp (BattleGateScenario adds ship_exp_list) and no table has the
// rates; S is the only score the capture shows. A battle with another score
// gives no ship exp rather than an invented rate.
var rankExpRate = map[uint32]float64{scoreS: 1.2}

const (
	// scenarioExpRate: the capture's ship exp is expedition exp x rank x morale
	// x 2 (x flagship / mvp); the client data doesn't say where the 2 comes from.
	scenarioExpRate = 2
	// happyExpRate is the "EXP bonus" of energyHappy morale.
	happyExpRate = 1.2
	maxIntimacy  = 10000
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
			return client.Send(&protobuf.SC_40002{Result: result(errRefused)})
		}
	}
	byID, err := shipsByID(ctx, game.DB.Queries, client.CommanderID, req.GetShipIdList())
	if err != nil {
		return err
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
	return client.Send(&protobuf.SC_40002{Result: result(nil), Key: proto.Uint32(rand.Uint32N(10000))})
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
// SC_12042 right after, then the mailbox counts (SC_30001).
func (game *Game) handleFinishStage(client *connection.Client, req *protobuf.CS_40003) error {
	ctx := context.Background()
	if req.GetSystem() != systemScenario {
		return client.Send(&protobuf.SC_40004{Result: result(nil)})
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
		return client.Send(&protobuf.SC_40004{Result: result(errRefused)})
	}
	group := session.activeGroup()
	score := min(req.GetScore(), scoreS)
	win := score > scoreC
	if _, known := rankExpRate[score]; !known {
		logger.WithFields("Battle", logger.FieldValue("score", score)).Warn("no ship exp rate for this score, ships get no exp")
	}

	statShips := make([]uint32, len(req.GetStatistics()))
	for i, stat := range req.GetStatistics() {
		statShips[i] = stat.GetShipId()
	}
	byID, err := shipsByID(ctx, game.DB.Queries, client.CommanderID, statShips)
	if err != nil {
		return err
	}

	reply := &protobuf.SC_40004{Result: result(nil)}
	var events []TaskEvent
	if win {
		events = append(events, TaskEvent{SubType: TaskSubWin, Count: 1})
		if score >= scoreA {
			events = append(events, TaskEvent{SubType: TaskSubWinA, Count: 1})
		}
		if score >= scoreS {
			events = append(events, TaskEvent{SubType: TaskSubWinS, Count: 1})
		}
		if expedition.isBoss() {
			events = append(events, TaskEvent{SubType: TaskSubDefeatBoss, Count: 1})
		}
		if kills := uint32(len(req.GetKillIdList())); kills > 0 {
			events = append(events, TaskEvent{SubType: TaskSubDefeatEnemy, Count: kills})
		}
	}
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
		events = append(events, TaskEvent{SubType: TaskSubSortieExp, Count: gained})
		affinity := uint32(0)
		if win {
			affinity = battleAffinity(ship.ID == flagship, ship.ID == mvp)
		}
		reply.ShipExpList = append(reply.ShipExpList, &protobuf.SHIP_EXP{
			ShipId:   proto.Uint32(ship.ID),
			Exp:      proto.Uint32(gained),
			Intimacy: proto.Uint32(maxIntimacy + affinity),
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

	// capture: task progress and archive updates (SC_17002) before the
	// result, the new ships (SC_12042) after it
	err = game.tx(ctx, client, func(tx *Tx) error {
		queries := tx.Queries
		tx.newShipsAfterReply = true
		tx.event(events...)
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
			if err := queries.SaveChapterProgress(ctx, db.SaveChapterProgressParams(progress)); err != nil {
				return err
			}
		}
		// reply.PlayerExp isn't added to the commander: their exp and level
		// aren't stored
		if err := tx.grantDrops(reply.DropInfo); err != nil {
			return err
		}
		return session.save(ctx, queries, client.CommanderID)
	})
	if err != nil {
		return err
	}
	// capture: the mailbox counts follow the result (mail arrived meanwhile)
	counts, err := mailCounts(ctx, game.DB.Queries, client.CommanderID)
	if err != nil {
		return err
	}
	if err := client.PushAfterReply(counts); err != nil {
		return err
	}
	return client.Send(reply)
}

// shipBattleExp is the exp a ship gets for a battle: expedition exp x rank x
// scenarioExpRate, x1.2 when happy, x mvp_exp_percent for the MVP and
// x flagship_exp_percent for the flagship.
func shipBattleExp(expedition expeditionTemplate, score uint32, ship db.Ship, flagship, mvp bool) uint32 {
	rate, known := rankExpRate[score]
	if !known {
		return 0
	}
	exp := float64(expedition.Exp) * rate * scenarioExpRate
	if ship.Energy >= happyEnergyFrom {
		exp *= happyExpRate
	}
	if mvp {
		exp = exp * float64(mvpExpPercent) / 100
	}
	if flagship {
		exp = exp * float64(flagshipExpPercent) / 100
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
// ship_level.exp per level, capped at the ship's max_level. UR ships would
// use ship_level.exp_ur, which isn't read.
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
