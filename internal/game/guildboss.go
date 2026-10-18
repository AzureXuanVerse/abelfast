package game

import (
	"context"
	"database/sql"
	"errors"
	"math/rand/v2"
	"time"

	"google.golang.org/protobuf/proto"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/db"
	"github.com/ggmolly/belfast/internal/protobuf"
)

// GuildBossMission.MAIN_FLEET_ID and SUB_FLEET_ID
const (
	guildBossMainFleet = 1
	guildBossSubFleet  = 11
)

// const.lua SYSTEM_GUILD: a guild boss battle (BattleGateGuild)
const systemGuild = 5

// guildBossBattleLength is how long after CS_40001 a boss battle counts as
// in progress when no result came (a battle quit without CS_40003).
const guildBossBattleLength = 30 * time.Minute

// guildBossFleets are the commander's boss fleets (BOSSEVENTFLEET), as
// GuildBossMission.New reads CURRENT_OPERATION.fleets.
func guildBossFleets(ctx context.Context, queries *db.Queries, operationID, commanderID uint32) ([]*protobuf.BOSSEVENTFLEET, error) {
	ships, err := queries.ListGuildBossFleetShips(ctx, db.ListGuildBossFleetShipsParams{OperationID: operationID, CommanderID: commanderID})
	if err != nil {
		return nil, err
	}
	cats, err := queries.ListGuildBossFleetMeowfficers(ctx, db.ListGuildBossFleetMeowfficersParams{OperationID: operationID, CommanderID: commanderID})
	if err != nil {
		return nil, err
	}
	var fleets []*protobuf.BOSSEVENTFLEET
	fleet := func(id uint32) *protobuf.BOSSEVENTFLEET {
		for _, existing := range fleets {
			if existing.GetFleetId() == id {
				return existing
			}
		}
		created := &protobuf.BOSSEVENTFLEET{FleetId: proto.Uint32(id)}
		fleets = append(fleets, created)
		return created
	}
	for _, ship := range ships {
		entry := fleet(ship.FleetID)
		entry.Ships = append(entry.Ships, &protobuf.TEAM_CELL{UserId: proto.Uint32(ship.UserID), ShipId: proto.Uint32(ship.ShipID)})
	}
	for _, cat := range cats {
		entry := fleet(cat.FleetID)
		entry.Commanders = append(entry.Commanders, &protobuf.COMMANDERSINFO{Pos: proto.Uint32(cat.Pos), Id: proto.Uint32(cat.MeowfficerID)})
	}
	return fleets, nil
}

// handleGuildBossFleets saves the commander's boss fleets
// (GuildUpdateBossMissionFleetCommand, CS_61013): the main (1) and
// submarine (11) fleets, each ship the commander's own or another member's
// support ship (guildSupportShip), and their meowfficers. Built from the Lua
// alone.
func (game *Game) handleGuildBossFleets(client *connection.Client, req *protobuf.CS_61013) error {
	err := game.guildOperationRequest(client, func(tx *Tx, member db.GuildMember, state *guildOperationState, now time.Time) error {
		ctx := tx.ctx
		if !state.op.BossActive {
			return errRefused
		}
		for _, fleet := range req.GetFleet() {
			id := fleet.GetFleetId()
			if id != guildBossMainFleet && id != guildBossSubFleet {
				return errRefused
			}
			var own []uint32
			for _, ship := range fleet.GetShips() {
				if ship.GetUserId() == client.CommanderID {
					own = append(own, ship.GetShipId())
					continue
				}
				if _, ok, err := game.guildSupportShip(ctx, tx.Queries, member.GuildID, ship.GetUserId(), ship.GetShipId()); err != nil {
					return err
				} else if !ok {
					return errRefused
				}
			}
			owned, err := shipsByID(ctx, tx.Queries, client.CommanderID, own)
			if err != nil {
				return err
			}
			if len(owned) != len(own) {
				return errRefused
			}
			key := db.ClearGuildBossFleetParams{OperationID: state.op.ID, CommanderID: client.CommanderID, FleetID: id}
			if err := tx.ClearGuildBossFleet(ctx, key); err != nil {
				return err
			}
			if err := tx.ClearGuildBossFleetMeowfficers(ctx, db.ClearGuildBossFleetMeowfficersParams(key)); err != nil {
				return err
			}
			for i, ship := range fleet.GetShips() {
				if err := tx.AddGuildBossFleetShip(ctx, db.AddGuildBossFleetShipParams{
					OperationID: state.op.ID, CommanderID: client.CommanderID, FleetID: id,
					Slot: uint32(i + 1), UserID: ship.GetUserId(), ShipID: ship.GetShipId(),
				}); err != nil {
					return err
				}
			}
			for _, cat := range fleet.GetCommanders() {
				if err := tx.AddGuildBossFleetMeowfficer(ctx, db.AddGuildBossFleetMeowfficerParams{
					OperationID: state.op.ID, CommanderID: client.CommanderID, FleetID: id,
					Pos: cat.GetPos(), MeowfficerID: cat.GetId(),
				}); err != nil {
					return err
				}
			}
		}
		return nil
	})
	if err != nil && !refused(err) {
		return err
	}
	return client.Send(&protobuf.SC_61014{Result: result(err)})
}

// handleGuildBossInfo answers GetGuildBossInfoCommand (CS_61027), which the
// event page sends once the last main mission is done: the boss, its hp and
// the damage dealt. Without an open operation (expired, or the boss
// defeated) the result is 20 and the client ends its operation.
func (game *Game) handleGuildBossInfo(client *connection.Client, req *protobuf.CS_61027) error {
	reply := &protobuf.SC_61028{}
	err := game.guildOperationRequest(client, func(tx *Tx, member db.GuildMember, state *guildOperationState, now time.Time) error {
		if !state.op.BossActive {
			return errRefused
		}
		reply.BossEvent = state.eventBoss()
		return nil
	})
	if errors.Is(err, errNoGuildOperation) {
		err = errGuildBossOver
	}
	if err != nil && !refused(err) {
		return err
	}
	reply.Result = result(err)
	return client.Send(reply)
}

// guildBossRank is the damage the members dealt to an operation's boss,
// most first.
func guildBossRank(ctx context.Context, queries *db.Queries, operationID uint32) ([]*protobuf.RANK_INFO_P61, error) {
	members, err := queries.ListGuildOperationMembers(ctx, operationID)
	if err != nil {
		return nil, err
	}
	var rank []*protobuf.RANK_INFO_P61
	for _, member := range members {
		if member.BossDamage > 0 {
			rank = append(rank, &protobuf.RANK_INFO_P61{UserId: proto.Uint32(member.CommanderID), Damage: proto.Uint32(member.BossDamage)})
		}
	}
	return rank, nil
}

// handleGuildBossRank answers GetGuildBossRankCommand (CS_61029): the
// damage ranking of the open operation's boss, empty without one.
func (game *Game) handleGuildBossRank(client *connection.Client, req *protobuf.CS_61029) error {
	reply := &protobuf.SC_61030{}
	err := game.guildOperationRequest(client, func(tx *Tx, member db.GuildMember, state *guildOperationState, now time.Time) error {
		var err error
		reply.List, err = guildBossRank(tx.ctx, tx.Queries, state.op.ID)
		return err
	})
	if err != nil && !refused(err) {
		return err
	}
	return client.Send(reply)
}

// beginGuildBoss starts a boss battle (BattleGateGuild.Entrance: CS_40001
// with system SYSTEM_GUILD, data the boss's stage, the commander's ships in
// ship_id_list and the support ships in other_ship_id_list): a participant
// of the open operation whose boss is up and alive, guildset
// operation_daily_boss_count fights a day. The member gains
// operation_boss_guild_active liveness, as the client adds on result 0.
func (game *Game) beginGuildBoss(client *connection.Client, req *protobuf.CS_40001) error {
	var notices *guildNotices
	err := game.guildOperationRequest(client, func(tx *Tx, member db.GuildMember, state *guildOperationState, now time.Time) error {
		ctx := tx.ctx
		if !state.op.BossActive || state.op.BossDamage >= state.op.BossHp || req.GetData() != guildBosses[state.op.BossID].stage {
			return errRefused
		}
		fighter, err := tx.GetGuildOperationMember(ctx, db.GetGuildOperationMemberParams{OperationID: state.op.ID, CommanderID: client.CommanderID})
		if errors.Is(err, sql.ErrNoRows) {
			return errRefused
		}
		if err != nil {
			return err
		}
		if guildBossDaily(fighter, now) >= guildBossDailyCount {
			return errRefused
		}
		owned, err := shipsByID(ctx, tx.Queries, client.CommanderID, req.GetShipIdList())
		if err != nil {
			return err
		}
		if len(owned) == 0 {
			return errRefused
		}
		for _, other := range req.GetOtherShipIdList() {
			if _, ok, err := game.guildSupportShip(ctx, tx.Queries, member.GuildID, other.GetUserId(), other.GetShipId()); err != nil {
				return err
			} else if !ok {
				return errRefused
			}
		}
		if err := tx.StartGuildBossBattle(ctx, db.StartGuildBossBattleParams{
			BossBattleAt: uint32(now.Unix()), OperationID: state.op.ID, CommanderID: client.CommanderID,
		}); err != nil {
			return err
		}
		if notices, err = tx.guildNoticesFor(member.GuildID); err != nil {
			return err
		}
		return tx.addGuildLiveness(game, member, guildBossLiveness, notices)
	})
	if err != nil && !refused(err) {
		return err
	}
	if err == nil {
		game.tellGuildNotices(client, notices)
	}
	reply := &protobuf.SC_40002{Result: result(err)}
	if err == nil {
		reply.Key = proto.Uint32(rand.Uint32N(10000))
	}
	return client.Send(reply)
}

// finishGuildBoss settles a boss battle (BattleGateGuild.Exit, CS_40003 with
// system SYSTEM_GUILD): the damage the enemies took (enemy_info) is dealt to
// the guild's boss, counted for the member's ranking and the day's fights,
// and the battle costs guildset use_oil (GeneralPlayerCosume takes it on
// the client). A defeated boss ends the operation and leaves a boss report
// for every member that damaged it. The reply carries no drops or exp (the
// rewards come with the report); result 20 tells the client the boss was
// already defeated.
func (game *Game) finishGuildBoss(client *connection.Client, req *protobuf.CS_40003) error {
	ctx := context.Background()
	reply := &protobuf.SC_40004{}
	ids := make([]uint32, len(req.GetStatistics()))
	for i, stat := range req.GetStatistics() {
		ids[i] = stat.GetShipId()
	}
	byID, err := shipsByID(ctx, game.DB.Queries, client.CommanderID, ids)
	if err != nil {
		return err
	}
	err = game.guildOperationRequest(client, func(tx *Tx, member db.GuildMember, state *guildOperationState, now time.Time) error {
		if !state.op.BossActive {
			return errRefused
		}
		if state.op.BossDamage >= state.op.BossHp {
			return errGuildBossOver
		}
		fighter, err := tx.GetGuildOperationMember(ctx, db.GetGuildOperationMemberParams{OperationID: state.op.ID, CommanderID: client.CommanderID})
		if errors.Is(err, sql.ErrNoRows) {
			return errRefused
		}
		if err != nil {
			return err
		}
		damage := uint64(0)
		for _, enemy := range req.GetEnemyInfo() {
			damage += uint64(enemy.GetDamageTaken())
		}
		dealt := uint32(min(damage, uint64(state.op.BossHp-state.op.BossDamage)))
		state.op.BossDamage += dealt
		ended := state.op.BossDamage >= state.op.BossHp
		if err := tx.DamageGuildBoss(ctx, db.DamageGuildBossParams{BossDamage: state.op.BossDamage, Ended: ended, ID: state.op.ID}); err != nil {
			return err
		}
		if err := tx.SaveGuildBossFight(ctx, db.SaveGuildBossFightParams{
			BossDay: uint32(startOfDay(now).Unix()), BossCount: guildBossDaily(fighter, now) + 1,
			BossDamage: fighter.BossDamage + dealt, OperationID: state.op.ID, CommanderID: client.CommanderID,
		}); err != nil {
			return err
		}
		// the client takes the oil whatever it has: take what is there
		if err := tx.SpendResource(ResourceOil, guildBossOil); err != nil && !errors.Is(err, ErrNotEnough) {
			return err
		}
		if ended {
			members, err := tx.ListGuildOperationMembers(ctx, state.op.ID)
			if err != nil {
				return err
			}
			var fighters []db.GuildOperationShip
			for _, row := range members {
				if row.BossDamage > 0 {
					fighters = append(fighters, db.GuildOperationShip{CommanderID: row.CommanderID})
				}
			}
			if err := state.addReport(ctx, tx.Queries, guildReportBoss, state.op.BossID, guildScoreS, uint32(now.Unix()), fighters); err != nil {
				return err
			}
		}
		reply.Mvp = proto.Uint32(battleMVP(req.GetStatistics(), byID))
		// the result page shows the commander's ships from ship_exp_list:
		// listed without exp, intimacy unchanged (10000 + 0 gained)
		for _, stat := range req.GetStatistics() {
			if _, own := byID[stat.GetShipId()]; own {
				reply.ShipExpList = append(reply.ShipExpList, &protobuf.SHIP_EXP{
					ShipId: proto.Uint32(stat.GetShipId()), Exp: proto.Uint32(0), Intimacy: proto.Uint32(maxIntimacy),
				})
			}
		}
		return nil
	})
	if errors.Is(err, errNoGuildOperation) {
		err = errGuildBossOver
	}
	if err != nil && !refused(err) {
		return err
	}
	reply.Result = result(err)
	return client.Send(reply)
}

// guildBossBattleOngoing tells whether the commander is in a boss battle of
// their guild's open operation (began less than guildBossBattleLength ago,
// no result yet), for the assault fleet changes to refuse
// (guild_formation_erro_in_boss_battle).
func guildBossBattleOngoing(ctx context.Context, queries *db.Queries, guildID, commanderID uint32) (bool, error) {
	now := serverNow()
	op, err := queries.GetOpenGuildOperation(ctx, db.GetOpenGuildOperationParams{GuildID: guildID, Now: uint32(now.Unix())})
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	member, err := queries.GetGuildOperationMember(ctx, db.GetGuildOperationMemberParams{OperationID: op.ID, CommanderID: commanderID})
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return member.BossBattleAt != 0 && now.Sub(time.Unix(int64(member.BossBattleAt), 0)) < guildBossBattleLength, nil
}
