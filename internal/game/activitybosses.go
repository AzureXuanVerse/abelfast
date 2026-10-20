package game

import (
	"context"
	"time"

	"google.golang.org/protobuf/proto"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/db"
	"github.com/ggmolly/belfast/internal/gamedata"
	"github.com/ggmolly/belfast/internal/protobuf"
)

// Activity bosses every commander of the server fights together: the boss
// battle events (ActivityBossActivity, one boss whose HP the page shows in
// four bars) and the DATE A LANE collab boss rush (CollabrateBossRushActivity,
// one boss per series, a series unlocking once the ones before it died). The
// client only shows the server-wide HP the server reports, in 1/10000 of the
// boss's HP (bossHpMax).
//
// The boss battle's gauge is per server and falls with the contribution
// points all its commanders earn (the client's world_boss_help, and the
// wiki's "The Big Shot's Proclamation": "Each server's progression will be
// calculated separately"). GUESS (no capture, neither the Lua nor the wiki
// gives the gauge's size or the collab's rule): a boss's HP is its
// extraenemy_template hp, and the damage is the sum of what the commanders
// of this server stored for it:
//   - boss battle: the PT the commanders hold in the PT activity linked to it
//     (activity_event_pt link_id, ActivityProxy:GetActBossLinkPTActID), one
//     PT one HP. The PT are the contribution (damage_resource) its battles
//     drop, and its PT rewards wait for the boss HP to fall below reward_pt
//     (ActivityBossPtData:CanGetAward).
//   - collab boss rush: one HP per series win, the wins being the
//     activity's data1_list (CollabrateBossRushActivity:GetPassCounts, a
//     series id added on every win by BossRushSettlementCommand).
//
// The marks are timestamped the first time belfast computes the HP at or
// below them (activity_boss_marks), not when the damage was done.

// ActivityConst.ACTIVITY_TYPE_*
const (
	activityTypeBossBattleMark2 = 52
	activityTypePTBuffMark2     = 174
)

// bossHpMax is the full HP of a boss in SC_26032 boss_hp and BOSS4TH boss_hp
// (ActivityBossSceneTemplate:UpdatePage: four bars of 2500,
// CollabrateBossRushSeriesData:UpdateCollabBossData: hpRate / 10000).
const bossHpMax = 10000

// activityBossMilestones are the HP marks SC_26032 milestones are the
// times of, in order: the page shows milestones[5 - bar] when bar (1 the
// lowest quarter) emptied.
var activityBossMilestones = []uint32{7500, 5000, 2500, 0}

// activity_event_worldboss row (ActivityBossConfig): boss_id[1] is the
// boss's extraenemy_template
type activityWorldBossConfig struct {
	BossID []uint32 `json:"boss_id"`
}

// extraenemy_template row: a server-wide boss and its HP
type extraEnemyConfig struct {
	HP uint32 `json:"hp"`
}

// extraenemy_series_template row (CollabrateBossRushSeriesData): boss_id[1]
// is the series' extraenemy_template (GetCollabBossID)
type extraEnemySeriesConfig struct {
	BossID []uint32 `json:"boss_id"`
}

var (
	activityWorldBossConfigs map[uint32]activityWorldBossConfig
	extraEnemyConfigs        map[uint32]extraEnemyConfig
	extraEnemySeriesConfigs  map[uint32]extraEnemySeriesConfig
)

func loadActivityBossData() error {
	return gamedata.LoadAll(
		gamedata.Load("activity_event_worldboss", &activityWorldBossConfigs),
		gamedata.Load("extraenemy_template", &extraEnemyConfigs),
		gamedata.Load("extraenemy_series_template", &extraEnemySeriesConfigs),
	)
}

// bossHpRate is what is left of a boss of hp HP once damage was done, in
// 1/10000 of its HP.
func bossHpRate(hp, damage uint32) uint32 {
	if hp == 0 {
		return bossHpMax
	}
	return uint32(uint64(hp-min(damage, hp)) * bossHpMax / uint64(hp))
}

// bossPtActivities are the PT activities linked to boss activity id
// (ActivityProxy:GetActBossLinkPTActID).
func bossPtActivities(id uint32) []uint32 {
	var linked []uint32
	for _, config := range activityConfigs {
		if config.Type != activityTypePTBuff && config.Type != activityTypePTBuffMark2 {
			continue
		}
		if pt, ok := eventPtConfigs[config.ConfigID]; ok && pt.LinkID == id {
			linked = append(linked, config.ID)
		}
	}
	return linked
}

// reachBossMarks timestamps the marks a boss's HP (rate) is at or below
// for the first time, and returns when each reached mark was.
func (tx *Tx) reachBossMarks(activityID, bossID, rate uint32, marks []uint32, now time.Time) (map[uint32]int64, error) {
	for _, mark := range marks {
		if rate > mark {
			continue
		}
		err := tx.AddActivityBossMark(tx.ctx, db.AddActivityBossMarkParams{
			ActivityID: activityID, BossID: bossID, Mark: mark, ReachedAt: now.Unix(),
		})
		if err != nil {
			return nil, err
		}
	}
	rows, err := tx.ListActivityBossMarks(tx.ctx, activityID)
	if err != nil {
		return nil, err
	}
	reached := map[uint32]int64{}
	for _, row := range rows {
		if row.BossID == bossID {
			reached[row.Mark] = row.ReachedAt
		}
	}
	return reached, nil
}

// handleActivityBossPage answers ActivityBossPageUpdateCommond (CS_26031,
// sent each time the boss battle page opens): the boss's server-wide HP and
// the times its HP bars emptied (ActivityBossActivity:UpdatePublicData;
// death, when it died, isn't read). See the top of this file for how the HP
// is counted. A boss activity that isn't running gets result 1 (the client
// only asks for a running one).
func (game *Game) handleActivityBossPage(client *connection.Client, req *protobuf.CS_26031) error {
	ctx := context.Background()
	now := time.Now()
	reply := &protobuf.SC_26032{Result: result(nil), BossHp: proto.Uint32(0), Death: proto.Uint32(0)}
	config, open, err := game.commanderActivity(ctx, client.CommanderID, req.GetActId(), now)
	if err != nil {
		return err
	}
	boss := activityWorldBossConfigs[config.ConfigID].BossID
	if !open || config.Type != activityTypeBossBattleMark2 || len(boss) == 0 {
		reply.Result = result(errRefused)
		return client.Send(reply)
	}
	err = game.tx(ctx, client, func(tx *Tx) error {
		var damage uint32
		for _, id := range bossPtActivities(config.ID) {
			pts, err := tx.ListActivityData1(ctx, id)
			if err != nil {
				return err
			}
			for _, pt := range pts {
				damage += pt
			}
		}
		rate := bossHpRate(extraEnemyConfigs[boss[0]].HP, damage)
		reached, err := tx.reachBossMarks(config.ID, boss[0], rate, activityBossMilestones, now)
		if err != nil {
			return err
		}
		reply.BossHp = proto.Uint32(rate)
		for _, mark := range activityBossMilestones {
			at, ok := reached[mark]
			if !ok {
				break
			}
			reply.Milestones = append(reply.Milestones, uint32(at))
		}
		reply.Death = proto.Uint32(uint32(reached[0]))
		return nil
	})
	if err != nil {
		return err
	}
	return client.Send(reply)
}

// handleCollabBossRushData answers CollabrateBossRushRequestDataCommand
// (CS_26081, sent at every main screen entry while the activity is listed,
// MainRequestActDataSequence, and by the collab page): every series boss's
// server-wide HP and when it died (0 alive). The client looks each series'
// boss up in boss_list (UpdateCollabrateBossData), so every series of the
// activity's config_data is listed. hour_traffic (battles in the last hour)
// and hour_off (HP lost in the last hour, 1/10000) are 0: belfast doesn't
// keep when the wins happened (GUESS, the page then shows -0%). See the top
// of this file for how the HP is counted. An activity that isn't running
// gets result 1.
func (game *Game) handleCollabBossRushData(client *connection.Client, req *protobuf.CS_26081) error {
	ctx := context.Background()
	now := time.Now()
	reply := &protobuf.SC_26082{Result: result(nil)}
	config, open, err := game.commanderActivity(ctx, client.CommanderID, req.GetActId(), now)
	if err != nil {
		return err
	}
	if !open || config.Type != activityTypeBossRushDalCollab {
		reply.Result = result(errRefused)
		return client.Send(reply)
	}
	err = game.tx(ctx, client, func(tx *Tx) error {
		lists, err := tx.ListActivityData1Lists(ctx, config.ID)
		if err != nil {
			return err
		}
		wins := map[uint32]uint32{}
		for _, list := range lists {
			for _, series := range list {
				wins[series]++
			}
		}
		for _, series := range gamedata.IDs(config.ConfigData) {
			boss := extraEnemySeriesConfigs[series].BossID
			if len(boss) == 0 {
				continue
			}
			rate := bossHpRate(extraEnemyConfigs[boss[0]].HP, wins[series])
			reached, err := tx.reachBossMarks(config.ID, boss[0], rate, []uint32{0}, now)
			if err != nil {
				return err
			}
			reply.BossList = append(reply.BossList, &protobuf.BOSS4TH{
				Id:          proto.Uint32(boss[0]),
				BossHp:      proto.Uint32(rate),
				Death:       proto.Uint32(uint32(reached[0])),
				HourTraffic: proto.Uint32(0),
				HourOff:     proto.Uint32(0),
			})
		}
		return nil
	})
	if err != nil {
		return err
	}
	return client.Send(reply)
}
