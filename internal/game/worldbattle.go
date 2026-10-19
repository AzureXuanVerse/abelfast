package game

import (
	"context"
	"encoding/json"
	"math/rand/v2"
	"slices"

	"google.golang.org/protobuf/proto"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/db"
	"github.com/ggmolly/belfast/internal/protobuf"
)

// systemWorld is SYSTEM_WORLD (const.lua): a battle against an enemy of an
// Operation Siren map (BattleGateWorld), data the enemy's expedition id,
// the ships the active fleet's. battle_cost_template 51 costs no oil or
// morale and gives ship, commander and meowfficer exp. Built from the Lua
// alone (never captured).
const systemWorld = 51

// worldBattleFleet is the normal fleet standing on an alive enemy of
// expedition id whose ships include ship, with that enemy.
func worldBattleFleet(state *worldState, m *worldMap, id, ship uint32) (*worldFleet, *worldAttachment) {
	for _, fleet := range state.Fleets {
		if fleet.Submarine || !slices.ContainsFunc(fleet.Ships, func(s worldShip) bool { return s.ID == ship }) {
			continue
		}
		if enemy := m.stageEnemy(fleetCell(fleet)); enemy != nil && enemy.ID == id {
			return fleet, enemy
		}
	}
	return nil, nil
}

// beginWorld starts a battle on the world's active map
// (BattleGateWorld.Entrance, WorldMediator.OnStart): the fleet must stand
// on the alive enemy fought, with a morale damage no higher than its
// morale_limit. Nothing is spent.
func (game *Game) beginWorld(client *connection.Client, req *protobuf.CS_40001) error {
	ctx := context.Background()
	now := serverNow()
	err := func() error {
		state, err := loadWorldState(ctx, game.DB.Queries, client.CommanderID, now)
		if err != nil {
			return err
		}
		if !state.activated() || len(req.GetShipIdList()) == 0 {
			return errRefused
		}
		m, err := loadWorldMap(ctx, game.DB.Queries, client.CommanderID, state.MapID)
		if err != nil {
			return err
		}
		if m == nil {
			return errRefused
		}
		fleet, _ := worldBattleFleet(state, m, req.GetData(), req.GetShipIdList()[0])
		if fleet == nil {
			return errRefused
		}
		if limit, ok := worldExpeditions[req.GetData()]; ok && fleet.Damage > limit.MoraleLimit {
			return errRefused
		}
		return nil
	}()
	if refused(err) {
		return client.Send(&protobuf.SC_40002{Result: result(err)})
	}
	if err != nil {
		return err
	}
	return client.Send(&protobuf.SC_40002{Result: result(nil), Key: proto.Uint32(rand.Uint32N(10000))})
}

// finishWorld settles a battle of the world (BattleGateWorld.Exit, then
// WorldMap:WriteBack, which the client applies itself and the server
// mirrors): the ships fought keep their hp (hp_rest), a sunk one comes back
// at world_death_hpfix with the broken buff ("Tolls of War"); a win (score
// above C) beats the enemy (flag 1), counts a kill for the fleet (and the
// supporting submarine fleet, which spends an ammo), clears every fleet's
// morale damage; a loss adds the enemy's failed_morale to the fleet's
// damage level; the enemy stays (the client takes one off its data, which
// can kill it, WriteBack; the SC_33105 push after the reply puts it back:
// a generated enemy has data 1). Ship exp is the
// enemy's exp_world for the map's difficulty, by rank as in a campaign;
// the drops one of award_display_world's for the difficulty. The cell is
// pushed again (SC_33105) after the reply.
func (game *Game) finishWorld(client *connection.Client, req *protobuf.CS_40003) error {
	ctx := context.Background()
	now := serverNow()
	statShips := make([]uint32, len(req.GetStatistics()))
	for i, stat := range req.GetStatistics() {
		statShips[i] = stat.GetShipId()
	}
	byID, err := shipsByID(ctx, game.DB.Queries, client.CommanderID, statShips)
	if err != nil {
		return err
	}
	commander, err := game.DB.GetCommander(ctx, client.CommanderID)
	if err != nil {
		return err
	}
	reply := &protobuf.SC_40004{Result: result(nil)}
	var push *protobuf.SC_33105
	var targets []*protobuf.WORLDTARGET
	err = game.tx(ctx, client, func(tx *Tx) error {
		state, err := tx.worldState(now)
		if err != nil {
			return err
		}
		if !state.activated() || len(statShips) == 0 {
			return errRefused
		}
		m, err := loadWorldMap(ctx, tx.Queries, client.CommanderID, state.MapID)
		if err != nil {
			return err
		}
		if m == nil || worldTemplates[m.TemplateID] == nil {
			return errRefused
		}
		var fleet *worldFleet
		var enemy *worldAttachment
		for _, id := range statShips {
			if fleet, enemy = worldBattleFleet(state, m, req.GetData(), id); fleet != nil {
				break
			}
		}
		expedition, known := expeditionTemplates[req.GetData()]
		if fleet == nil || !known {
			return errRefused
		}
		score := min(req.GetScore(), scoreS)
		win := score > scoreC
		happened := &worldHappenings{}

		var sub *worldFleet
		for _, f := range state.Fleets {
			if f.Submarine && state.Submarine {
				sub = f
			}
		}
		subAid := false
		for _, stat := range req.GetStatistics() {
			ship := state.worldShip(stat.GetShipId())
			if ship == nil {
				continue
			}
			if sub != nil && slices.ContainsFunc(sub.Ships, func(s worldShip) bool { return s.ID == ship.ID }) {
				subAid = true
			}
			ship.HPRant = min(stat.GetHpRest(), fullHP)
			if ship.HPRant == 0 {
				ship.HPRant = worldOpSettings.deathHPFix
				ship.Buffs = addWorldBuff(ship.Buffs, worldPortSettings.brokenBuff, 1)
			}
		}
		if subAid && sub.Bullet > 0 {
			sub.Bullet--
		}
		if win {
			worldEnemyBeaten(state, fleet, enemy)
			if subAid {
				sub.KillCount++
			}
			happened.Kills = append(happened.Kills, enemy.ID)
		} else {
			failed := max(worldExpeditions[enemy.ID].FailedMorale, 1)
			fleet.Damage = min(fleet.Damage+failed, worldOpSettings.damageLevel)
			happened.Losses = append(happened.Losses, enemy.ID)
		}

		// exp: the enemy's exp_world for the map's difficulty
		difficulty := worldTemplates[m.TemplateID].Difficulty
		world := expedition
		world.Exp = 0
		for _, entry := range worldExpeditions[enemy.ID].ExpWorld {
			if entry[0] == difficulty || world.Exp == 0 {
				world.Exp = entry[1]
			}
		}
		mvp := battleMVP(req.GetStatistics(), byID)
		reply.Mvp = proto.Uint32(mvp)
		flagship := uint32(0)
		if len(fleet.Ships) > 0 {
			flagship = fleet.Ships[0].ID
		}
		ships := settleBattleShips(req.GetStatistics(), byID, func(ship db.Ship) uint32 {
			return shipBattleExp(world, 1, score, ship, ship.ID == flagship, ship.ID == mvp)
		}, win, flagship, mvp)
		reply.ShipExpList = ships.exps
		reply.PlayerExp = proto.Uint32(0)
		if win {
			reply.PlayerExp = proto.Uint32(playerBattleExp(expedition, score, uint32(len(ships.updated)), commander.Level))
		}
		tx.newShipsAfterReply = true
		tx.event(ships.events...)
		if win {
			tx.event(TaskEvent{SubType: TaskSubWin, Count: 1})
			if expedition.isBoss() {
				tx.event(TaskEvent{SubType: TaskSubDefeatBoss, Count: 1})
			}
			reply.DropInfo = worldBattleDrops(enemy.ID, difficulty)
		}
		if ships.techLevel {
			tx.techPointsChanged()
		}
		for _, ship := range ships.updated {
			if err := saveShipBattleState(ctx, tx.Queries, ship); err != nil {
				return err
			}
		}
		if err := addCommanderExp(tx, reply.GetPlayerExp()); err != nil {
			return err
		}
		if err := tx.grantWorldDrops(state, reply.DropInfo); err != nil {
			return err
		}
		if err := tx.addClassProficiency(ships.proficiency, now); err != nil {
			return err
		}
		targets = tx.worldTargetsUpdated(state, happened)
		if err := tx.worldTasksUpdated(state, happened); err != nil {
			return err
		}
		push = &protobuf.SC_33105{PosList: []*protobuf.WORLDPOSINFO{m.posInfo(enemy.Row, enemy.Column, state.Fleets)}}
		if err := tx.saveWorldMap(m); err != nil {
			return err
		}
		return tx.saveWorldState(state)
	})
	if refused(err) {
		return client.Send(&protobuf.SC_40004{Result: result(err)})
	}
	if err != nil {
		return err
	}
	if err := client.Send(reply); err != nil {
		return err
	}
	if len(targets) > 0 {
		if err := client.PushAfterReply(&protobuf.SC_33601{TargetList: targets}); err != nil {
			return err
		}
	}
	return client.PushAfterReply(push)
}

// worldEnemyBeaten is a won battle's WriteBack on the world: the enemy is
// gone (flag 1, hp 0), the fleet counts a kill, every fleet's morale damage
// clears.
func worldEnemyBeaten(state *worldState, fleet *worldFleet, enemy *worldAttachment) {
	enemy.Flag = 1
	enemy.BossHP = 0
	fleet.KillCount++
	for _, f := range state.Fleets {
		f.Damage = 0
	}
}

// worldBattleDrops is a won world battle's loot: one of the enemy's
// award_display_world entries for the map's difficulty (the first listed
// when it has none for it), the drop tables being the server's.
func worldBattleDrops(enemy, difficulty uint32) []*protobuf.DROPINFO {
	var pool [][]uint32
	for _, raw := range worldExpeditions[enemy].AwardDisplayWorld {
		var entry []json.RawMessage
		if json.Unmarshal(raw, &entry) != nil || len(entry) != 2 {
			continue
		}
		var awards [][]uint32
		if json.Unmarshal(entry[1], &awards) != nil {
			continue
		}
		if rawUint(entry[0]) == difficulty || pool == nil {
			pool = awards
		}
	}
	pool = slices.DeleteFunc(pool, func(award []uint32) bool { return len(award) < 2 })
	if len(pool) == 0 {
		return nil
	}
	award := pool[rand.IntN(len(pool))]
	count := uint32(1)
	if len(award) >= 3 && award[2] > 0 {
		count = award[2]
	}
	return []*protobuf.DROPINFO{dropInfo(award[0], award[1], count)}
}
