package game

import (
	"context"
	"database/sql"
	"errors"
	"slices"

	"github.com/ggmolly/belfast/internal/db"
)

// The back-office's META Showdown: every META ship's Sync Points and their
// reward ladder, the commander's boss and their coordinates.

// MetaStepView is a step of a META ship's Sync Point ladder
// (ship_strengthen_meta target and award_display).
type MetaStepView struct {
	Target  uint32    `json:"target"`
	Award   [3]uint32 `json:"award"`
	Claimed bool      `json:"claimed"`
}

// MetaShipView is a META ship with Sync Points (Meta_Type_Act_PT).
type MetaShipView struct {
	Group uint32 `json:"group"`
	Pt    uint32 `json:"pt"`
	Max   uint32 `json:"max"`
	Owned bool   `json:"owned"`
	// its world_joint_boss_template, 0 when it has none
	Boss  uint32         `json:"boss"`
	Steps []MetaStepView `json:"steps"`
}

// MetaBossChoice is a boss that can be summoned and the levels it can be
// summoned at: the current season's from 1 to worldBossMaxLevel, a Dossier
// Analysis boss at worldBossMaxLevel only (handleActiveWorldBoss).
type MetaBossChoice struct {
	Template uint32 `json:"template"`
	Group    uint32 `json:"group"`
	Current  bool   `json:"current"`
	MinLevel uint32 `json:"min_level"`
	MaxLevel uint32 `json:"max_level"`
}

// MetaAdminView is a commander's META Showdown for the back-office.
type MetaAdminView struct {
	SummonPt       uint32           `json:"summon_pt"`
	SummonPtMax    uint32           `json:"summon_pt_max"`
	SummonPtOld    uint32           `json:"summon_pt_old"`
	SummonPtOldMax uint32           `json:"summon_pt_old_max"`
	SelfBossLv     uint32           `json:"self_boss_lv"`
	ArchivesID     uint32           `json:"archives_id"`
	Boss           *MetaBossView    `json:"boss,omitempty"`
	Bosses         []MetaBossChoice `json:"bosses"`
	Ships          []MetaShipView   `json:"ships"`
}

// MetaAdmin is the commander's META Showdown, read without creating its row.
func (game *Game) MetaAdmin(ctx context.Context, id uint32) (MetaAdminView, error) {
	now := serverNow()
	current := currentWorldBoss(now)
	view := MetaAdminView{
		SummonPtMax: worldBossCurrent.capacity, SummonPtOldMax: worldBossPast.capacity,
		SelfBossLv: 1, Bosses: []MetaBossChoice{}, Ships: []MetaShipView{},
	}
	row, err := game.DB.GetWorldBoss(ctx, id)
	switch {
	case errors.Is(err, sql.ErrNoRows):
	case err != nil:
		return view, err
	default:
		view.SummonPt, view.SummonPtOld, view.ArchivesID = row.SummonPt, row.SummonPtOld, row.ArchivesID
		if row.SelfBossTemplateID == current {
			view.SelfBossLv = row.SelfBossLv
		}
		if row.BossTemplateID != 0 {
			view.Boss = metaBossView(row, now.Unix())
		}
	}

	bossOf := map[uint32]uint32{}
	templates := make([]uint32, 0, len(worldBossTemplates))
	for template, config := range worldBossTemplates {
		bossOf[config.MetaID] = template
		templates = append(templates, template)
	}
	slices.Sort(templates)
	for _, template := range slices.Backward(templates) {
		config := worldBossTemplates[template]
		switch {
		case template == current:
			view.Bosses = append(view.Bosses, MetaBossChoice{Template: template, Group: config.MetaID, Current: true, MinLevel: 1, MaxLevel: worldBossMaxLevel})
		case config.State.Always:
			view.Bosses = append(view.Bosses, MetaBossChoice{Template: template, Group: config.MetaID, MinLevel: worldBossMaxLevel, MaxLevel: worldBossMaxLevel})
		}
	}

	pts, err := game.DB.ListMetaPts(ctx, id)
	if err != nil {
		return view, err
	}
	awards, err := game.DB.ListMetaPtAwards(ctx, id)
	if err != nil {
		return view, err
	}
	owned, err := game.DB.AdminMetaShipTemplates(ctx, id)
	if err != nil {
		return view, err
	}
	groups := make([]uint32, 0, len(metaShips))
	for group, config := range metaShips {
		if config.Type == metaTypePt && len(config.Target) > 0 {
			groups = append(groups, group)
		}
	}
	slices.Sort(groups)
	for _, group := range groups {
		config := metaShips[group]
		ship := MetaShipView{Group: group, Max: config.Target[len(config.Target)-1], Boss: bossOf[group], Steps: make([]MetaStepView, len(config.Target))}
		if i := slices.IndexFunc(pts, func(row db.MetaPt) bool { return row.GroupID == group }); i >= 0 {
			ship.Pt = pts[i].Pt
		}
		ship.Owned = slices.ContainsFunc(owned, func(template uint32) bool { return template/10 == group })
		for i, target := range config.Target {
			step := MetaStepView{Target: target}
			if i < len(config.AwardDisplay) && len(config.AwardDisplay[i]) >= 3 {
				copy(step.Award[:], config.AwardDisplay[i])
			}
			step.Claimed = slices.ContainsFunc(awards, func(row db.MetaPtAward) bool { return row.GroupID == group && row.Target == target })
			ship.Steps[i] = step
		}
		view.Ships = append(view.Ships, ship)
	}
	return view, nil
}

// metaBossView is the commander's boss at now (unix seconds).
func metaBossView(row db.WorldBoss, now int64) *MetaBossView {
	return &MetaBossView{
		TemplateID: row.BossTemplateID, MetaShip: worldBossTemplates[row.BossTemplateID].MetaID,
		Level: row.BossLv, HP: row.BossHp, MaxHP: worldBossLevel(row.BossTemplateID, row.BossLv).HP,
		ExpiresAt: row.BossLastTime, KilledAt: row.BossKillTime, Fights: row.BossFightCount, Damage: row.BossDamage,
		Guild: row.GuildSupport > now, Friend: row.FriendSupport > now, World: row.WorldSupport > now,
	}
}

// SetMetaAward marks a META ship's Sync Point reward step (a target) taken
// or not, without giving its award. MetaPTData:initFromServerData counts
// the steps taken as the leading targets of the sorted fetch_list, and
// MetaPTAwardGetCommand takes them in order, so taking a step takes those
// before it and giving one back gives back those after it. A step is taken
// once its Sync Points are reached (MetaPTData:CanGetAward).
func (game *Game) SetMetaAward(ctx context.Context, id, group, target uint32, claimed bool) (bool, error) {
	config, ok := metaShips[group]
	if !ok || config.Type != metaTypePt {
		return false, adminRefused("no META ship group %d with Sync Points", group)
	}
	step := slices.Index(config.Target, target)
	if step < 0 {
		return false, adminRefused("%d isn't one of its reward targets", target)
	}
	return game.AdminWrite(ctx, id, func(tx *Tx) error {
		if !claimed {
			return tx.AdminMetaUnclaimAwards(tx.ctx, db.AdminMetaUnclaimAwardsParams{CommanderID: id, GroupID: group, Target: target})
		}
		pts, err := tx.ListMetaPts(tx.ctx, id)
		if err != nil {
			return err
		}
		i := slices.IndexFunc(pts, func(row db.MetaPt) bool { return row.GroupID == group })
		if i < 0 || pts[i].Pt < target {
			return adminRefused("it needs %d Sync Points", target)
		}
		for _, earlier := range config.Target[:step+1] {
			if err := tx.AdminMetaClaimAward(tx.ctx, db.AdminMetaClaimAwardParams{CommanderID: id, GroupID: group, Target: earlier}); err != nil {
				return err
			}
		}
		return nil
	})
}

// SetMetaSummonPt sets the META Coordinates (past false) or the Dossier
// Coordinate Data (past true), at most what the gameset's
// curr_boss_ticket / past_joint_boss_ticket store
// (WorldBossProxy:AddSummonPt / AddSummonPtOld).
func (game *Game) SetMetaSummonPt(ctx context.Context, id, value uint32, past bool) (bool, error) {
	capacity := worldBossCurrent.capacity
	if past {
		capacity = worldBossPast.capacity
	}
	if value > capacity {
		return false, adminRefused("at most %d are stored", capacity)
	}
	return game.metaBossTx(ctx, id, false, func(tx *Tx, boss *worldBoss, now int64) error {
		if past {
			boss.SummonPtOld = value
		} else {
			boss.SummonPt = value
		}
		return nil
	})
}

// SummonMetaBoss gives the commander a boss of template at level in place of
// theirs, without spending coordinates: the current season's boss at any
// level, a Dossier Analysis boss at worldBossMaxLevel, which becomes the
// Dossier target (handleActiveWorldBoss).
func (game *Game) SummonMetaBoss(ctx context.Context, id, template, level uint32) (bool, error) {
	now := serverNow()
	config, ok := worldBossTemplates[template]
	switch {
	case !ok:
		return false, adminRefused("no META boss %d", template)
	case template == currentWorldBoss(now):
		if level < 1 || level > worldBossMaxLevel {
			return false, adminRefused("its level goes from 1 to %d", worldBossMaxLevel)
		}
	case config.State.Always:
		if level != worldBossMaxLevel {
			return false, adminRefused("a Dossier boss is summoned at level %d", worldBossMaxLevel)
		}
	default:
		return false, AdminError{Msg: "its season is over"}
	}
	return game.metaBossTx(ctx, id, false, func(tx *Tx, boss *worldBoss, unix int64) error {
		boss.clearSelfBoss()
		if config.State.Always {
			boss.ArchivesID = template
		}
		boss.BossTemplateID = template
		boss.BossLv = level
		boss.BossHp = worldBossLevel(template, level).HP
		boss.BossLastTime = unix + int64(worldBossLifetime)
		return tx.ClearWorldBossShips(tx.ctx, id)
	})
}

// SetMetaBossHP sets the commander's boss's hp, at most its level's
// world_boss_level hp; taking hp off is damage, so a kill raises the
// season's next level as one in battle does (worldBoss.damage).
func (game *Game) SetMetaBossHP(ctx context.Context, id, hp uint32) (bool, error) {
	return game.metaBossTx(ctx, id, true, func(tx *Tx, boss *worldBoss, now int64) error {
		if top := worldBossLevel(boss.BossTemplateID, boss.BossLv).HP; hp > top {
			return adminRefused("its hp goes up to %d", top)
		}
		if hp < boss.BossHp {
			boss.damage(boss.BossHp - hp)
		} else {
			boss.BossHp = hp
		}
		return nil
	})
}

// ResetMetaBossFights sets the battles fought against the commander's boss
// back to 0: the next one is free again (joint_boss_oil_consume).
func (game *Game) ResetMetaBossFights(ctx context.Context, id uint32) (bool, error) {
	return game.metaBossTx(ctx, id, true, func(tx *Tx, boss *worldBoss, now int64) error {
		boss.BossFightCount = 0
		return nil
	})
}

// SetMetaBossTimer gives the commander's boss joint_boss_bonus_time seconds
// from now (extend false), or that much more than it has left (extend true).
func (game *Game) SetMetaBossTimer(ctx context.Context, id uint32, extend bool) (bool, error) {
	return game.metaBossTx(ctx, id, true, func(tx *Tx, boss *worldBoss, now int64) error {
		from := now
		if extend {
			from = max(now, boss.BossLastTime)
		}
		if from+int64(worldBossLifetime) > 1<<32-1 {
			return AdminError{Msg: "its timer can't go further"}
		}
		boss.BossLastTime = from + int64(worldBossLifetime)
		return nil
	})
}

// SetMetaBossSupport turns a support request for the commander's boss on,
// for joint_boss_world_time seconds as CS_34509 does, or off (0, as
// WorldBossProxy's Clear*Supported). kind is WorldBoss.SUPPORT_TYPE_*.
func (game *Game) SetMetaBossSupport(ctx context.Context, id, kind uint32, on bool) (bool, error) {
	if kind != worldSupportFriend && kind != worldSupportGuild && kind != worldSupportWorld {
		return false, adminRefused("no support type %d", kind)
	}
	return game.metaBossTx(ctx, id, true, func(tx *Tx, boss *worldBoss, now int64) error {
		var until int64
		if on {
			until = now + int64(worldBossSupportTime)
		}
		switch kind {
		case worldSupportFriend:
			boss.FriendSupport = until
		case worldSupportGuild:
			boss.GuildSupport = until
		default:
			boss.WorldSupport = until
		}
		return nil
	})
}

// metaBossTx runs fn on the commander's META Showdown state in an AdminWrite;
// with summoned, refused when they have no boss.
func (game *Game) metaBossTx(ctx context.Context, id uint32, summoned bool, fn func(tx *Tx, boss *worldBoss, now int64) error) (bool, error) {
	now := serverNow()
	return game.AdminWrite(ctx, id, func(tx *Tx) error {
		return tx.updateWorldBoss(now, func(boss *worldBoss) error {
			if summoned && !boss.summoned() {
				return AdminError{Msg: "no boss summoned"}
			}
			return fn(tx, boss, now.Unix())
		})
	})
}
