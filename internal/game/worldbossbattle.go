package game

import (
	"context"
	"math/rand/v2"

	"google.golang.org/protobuf/proto"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/db"
	"github.com/ggmolly/belfast/internal/protobuf"
)

// systemWorldBoss is SYSTEM_WORLD_BOSS (const.lua): a battle against a META
// Showdown boss (BattleGateWorldBoss), data the boss id. battle_cost_template
// 50 costs no morale and gives no exp; the oil is the boss's
// (joint_boss_oil_consume). A practice battle (isSimulate) never reaches
// the server.
const systemWorldBoss = 50

// worldBossClockSlack is how early, in seconds, the client may stop a
// finished Dossier auto-deploy by its own clock.
const worldBossClockSlack = 5

// beginWorldBoss starts a battle against the commander's boss
// (BattleGateWorldBoss.Entrance): it must be up and not in auto-deploy, the
// fleet the commander's ships. From the second battle on it costs
// joint_boss_oil_consume oil (the client takes the same on result 0 and
// counts the battle, WorldBoss:IncreaseFightCnt). The fleet is kept for the
// damage ranking's formation view. Result 1 is the client's "Coordinates
// have expired". Built from the Lua alone (never captured).
func (game *Game) beginWorldBoss(client *connection.Client, req *protobuf.CS_40001) error {
	ctx := context.Background()
	now := serverNow()
	err := game.tx(ctx, client, func(tx *Tx) error {
		owned, err := ownsShips(ctx, tx.Queries, client.CommanderID, req.GetShipIdList())
		if err != nil {
			return err
		}
		if !owned || len(req.GetShipIdList()) == 0 {
			return errRefused
		}
		if err := tx.updateWorldBoss(now, func(boss *worldBoss) error {
			if req.GetData() != client.CommanderID || !boss.fightable(now) {
				return worldBossNone
			}
			if boss.AutoFightFinishTime != 0 {
				return errRefused
			}
			if boss.BossFightCount > 0 {
				if err := tx.SpendResource(ResourceOil, worldBossOilCost(boss.BossFightCount+1)); err != nil {
					return err
				}
			}
			boss.BossFightCount++
			return nil
		}); err != nil {
			return err
		}
		if err := tx.ClearWorldBossShips(ctx, client.CommanderID); err != nil {
			return err
		}
		for pos, id := range req.GetShipIdList() {
			if err := tx.AddWorldBossShip(ctx, db.AddWorldBossShipParams{CommanderID: client.CommanderID, Pos: uint32(pos), ShipID: id}); err != nil {
				return err
			}
		}
		return nil
	})
	if refused(err) {
		return client.Send(&protobuf.SC_40002{Result: result(err)})
	}
	if err != nil {
		return err
	}
	return client.Send(&protobuf.SC_40002{Result: result(nil), Key: proto.Uint32(rand.Uint32N(10000))})
}

// finishWorldBoss settles a battle against the commander's boss
// (BattleGateWorldBoss.Exit): the damage dealt to the boss enemy
// (enemy_info, the highest damage taken when it isn't listed, which is what
// the client keeps as its highest damage for the Dossier auto-deploy) comes
// off its hp. No drops or exp: a killed boss's reward is taken with
// CS_34511. SC_34508 then tells the client the boss's hp (WorldProxy asks
// for the ranking again and updates the boss). Built from the Lua alone.
func (game *Game) finishWorldBoss(client *connection.Client, req *protobuf.CS_40003) error {
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
	var hp uint32
	err = game.tx(ctx, client, func(tx *Tx) error {
		return tx.updateWorldBoss(now, func(boss *worldBoss) error {
			// a battle begun before the boss expired still counts
			if req.GetData() != client.CommanderID || !boss.summoned() || boss.BossHp == 0 {
				return errRefused
			}
			enemy := worldBossLevel(boss.BossTemplateID, boss.BossLv).EnemyID
			var damage, highest uint32
			found := false
			for _, info := range req.GetEnemyInfo() {
				highest = max(highest, info.GetDamageTaken())
				if info.GetEnemyId() == enemy {
					damage, found = info.GetDamageTaken(), true
				}
			}
			if !found {
				damage = highest
			}
			boss.AutoFightMaxDamage = max(boss.AutoFightMaxDamage, highest)
			boss.damage(damage)
			hp = boss.BossHp
			return nil
		})
	})
	if refused(err) {
		return client.Send(&protobuf.SC_40004{Result: result(err)})
	}
	if err != nil {
		return err
	}
	if err := client.Send(&protobuf.SC_40004{Result: result(nil), Mvp: proto.Uint32(battleMVP(req.GetStatistics(), byID))}); err != nil {
		return err
	}
	return client.PushAfterReply(&protobuf.SC_34508{BossId: proto.Uint32(client.CommanderID), Hp: proto.Uint32(hp)})
}

// autoBattles is how many battles the Dossier auto-deploy takes to kill the
// boss, and the oil they cost (WorldBossConst.GetAutoBattleCnt /
// GetAutoBattleOilConsume: the highest damage of one battle each time).
func (boss *worldBoss) autoBattles() (count, oil uint32) {
	highest := max(boss.AutoFightMaxDamage, 1)
	count = (boss.BossHp + highest - 1) / highest
	for n := boss.BossFightCount + 1; n <= boss.BossFightCount+count; n++ {
		oil += worldBossOilCost(n)
	}
	return count, oil
}

// handleWorldBossAutoBattle answers WorldBossArchivesAutoBattleCommand
// (CS_34523): starts the Dossier auto-deploy on the commander's Dossier
// boss, which needs a battle fought against it (and damage dealt) first. It
// costs the oil of the battles it takes (the client takes the same on
// result 0) and ends after past_joint_boss_autofight_time seconds
// (auto_fight_finish_time). Built from the Lua alone.
func (game *Game) handleWorldBossAutoBattle(client *connection.Client, req *protobuf.CS_34523) error {
	ctx := context.Background()
	now := serverNow()
	var finish int64
	err := game.tx(ctx, client, func(tx *Tx) error {
		return tx.updateWorldBoss(now, func(boss *worldBoss) error {
			if req.GetBossId() != client.CommanderID || !boss.fightable(now) || !archivedWorldBoss(boss.BossTemplateID) ||
				boss.AutoFightFinishTime != 0 || boss.BossFightCount == 0 || boss.AutoFightMaxDamage == 0 {
				return errRefused
			}
			_, oil := boss.autoBattles()
			if err := tx.SpendResource(ResourceOil, oil); err != nil {
				return err
			}
			boss.AutoFightFinishTime = now.Unix() + int64(worldBossAutoTime)
			finish = boss.AutoFightFinishTime
			return nil
		})
	})
	if refused(err) {
		return client.Send(&protobuf.SC_34524{Result: result(err)})
	}
	if err != nil {
		return err
	}
	return client.Send(&protobuf.SC_34524{Result: result(nil), AutoFightFinishTime: proto.Uint32(uint32(finish))})
}

// handleWorldBossStopAutoBattle answers
// WorldBossArchivesStopAutoBattleCommand (CS_34525): the end of a Dossier
// auto-deploy. Once its time is up the boss is killed: count battles were
// fought, dealing its hp for oil; stopped before, nothing is dealt and the
// oil is lost (gametip world_boss_archives_stop_auto_battle_tip1), the
// reply all zero. A kill is followed by SC_34508 with the boss's hp (0).
// Built from the Lua alone.
func (game *Game) handleWorldBossStopAutoBattle(client *connection.Client, req *protobuf.CS_34525) error {
	ctx := context.Background()
	now := serverNow()
	reply := &protobuf.SC_34526{Result: result(nil)}
	killed := false
	err := game.tx(ctx, client, func(tx *Tx) error {
		return tx.updateWorldBoss(now, func(boss *worldBoss) error {
			if req.GetBossId() != client.CommanderID || !boss.summoned() || boss.AutoFightFinishTime == 0 {
				return errRefused
			}
			if now.Unix()+worldBossClockSlack >= boss.AutoFightFinishTime && boss.BossHp > 0 {
				count, oil := boss.autoBattles()
				reply.Count, reply.Damage, reply.Oil = proto.Uint32(count), proto.Uint32(boss.BossHp), proto.Uint32(oil)
				boss.BossFightCount += count
				boss.damage(boss.BossHp)
				killed = true
			}
			boss.AutoFightFinishTime = 0
			return nil
		})
	})
	if refused(err) {
		return client.Send(&protobuf.SC_34526{Result: result(err)})
	}
	if err != nil {
		return err
	}
	if err := client.Send(reply); err != nil {
		return err
	}
	if !killed {
		return nil
	}
	return client.PushAfterReply(&protobuf.SC_34508{BossId: proto.Uint32(client.CommanderID), Hp: proto.Uint32(0)})
}
