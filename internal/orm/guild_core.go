package orm

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/ggmolly/belfast/internal/db"
)

const (
	GuildDutyCommander uint32 = 1
	GuildDutyDeputy    uint32 = 2
	GuildDutyOrdinary  uint32 = 4
	GuildDutyRecruit   uint32 = 5
)

const (
	guildCreateCostResourceID = 4
	guildQuitCooldownSeconds  = 86400
	impeachOfflineSeconds     = 864000
	impeachCooldownSeconds    = 86400
)

var (
	ErrGuildNameExists      = errors.New("guild name already exists")
	ErrCommanderInGuild     = errors.New("commander already in guild")
	ErrCommanderNotInGuild  = errors.New("commander is not in guild")
	ErrGuildPermission      = errors.New("guild permission denied")
	ErrGuildInvalidArgument = errors.New("invalid guild argument")
)

type Guild struct {
	ID              uint32
	Policy          uint32
	Faction         uint32
	Name            string
	Level           uint32
	Announce        string
	Manifesto       string
	Exp             uint32
	MemberCount     uint32
	ChangeFactionCD uint32
	KickLeaderCD    uint32
	Capital         uint32
	TechID          uint32
}

type GuildMember struct {
	GuildID        uint32
	CommanderID    uint32
	Duty           uint32
	Liveness       uint32
	PreOnlineTime  uint32
	JoinTime       uint32
	CommanderName  string
	CommanderLevel uint32
	Manifesto      string
	LastLogin      time.Time
	DisplayIconID  uint32
	DisplaySkinID  uint32
	IconFrameID    uint32
	ChatFrameID    uint32
	IconThemeID    uint32
}

type GuildUserInfo struct {
	CommanderID    uint32
	GuildID        uint32
	DonateCount    uint32
	BenefitTime    uint32
	WeeklyTaskFlag uint32
	ExtraDonate    uint32
	ExtraOperation uint32
}

type guildConfigEntry struct {
	KeyValue any `json:"key_value"`
}

func parseGuildConfigUint(value any) (uint32, bool) {
	switch v := value.(type) {
	case float64:
		if v < 0 {
			return 0, false
		}
		return uint32(v), true
	case int:
		if v < 0 {
			return 0, false
		}
		return uint32(v), true
	case int64:
		if v < 0 {
			return 0, false
		}
		return uint32(v), true
	case uint32:
		return v, true
	case uint64:
		return uint32(v), true
	case string:
		n, err := strconv.ParseUint(v, 10, 32)
		if err != nil {
			return 0, false
		}
		return uint32(n), true
	default:
		return 0, false
	}
}

func GetGuildSetUint(key string) (uint32, error) {
	entry, err := GetConfigEntry("ShareCfg/guildset.json", key)
	if err != nil {
		return 0, err
	}
	var payload guildConfigEntry
	if err := json.Unmarshal(entry.Data, &payload); err != nil {
		return 0, err
	}
	value, ok := parseGuildConfigUint(payload.KeyValue)
	if !ok {
		return 0, fmt.Errorf("invalid key_value for %s", key)
	}
	return value, nil
}

func GetGuildDataLevelDeputyLimit(level uint32) (uint32, error) {
	entry, err := GetConfigEntry("ShareCfg/guild_data_level.json", strconv.FormatUint(uint64(level), 10))
	if err != nil {
		return 0, err
	}
	var payload map[string]any
	if err := json.Unmarshal(entry.Data, &payload); err != nil {
		return 0, err
	}
	limit, ok := parseGuildConfigUint(payload["assistant_commander"])
	if !ok {
		return 0, fmt.Errorf("invalid assistant_commander at level %d", level)
	}
	return limit, nil
}

func guildNameExistsTx(ctx context.Context, tx pgx.Tx, name string, excludeGuildID uint32) (bool, error) {
	var row pgx.Row
	if excludeGuildID == 0 {
		row = tx.QueryRow(ctx, `
SELECT id
FROM guilds
WHERE deleted_at IS NULL
  AND LOWER(name) = LOWER($1)
LIMIT 1
`, name)
	} else {
		row = tx.QueryRow(ctx, `
SELECT id
FROM guilds
WHERE deleted_at IS NULL
  AND LOWER(name) = LOWER($1)
  AND id <> $2
LIMIT 1
`, name, int64(excludeGuildID))
	}
	var id int64
	err := row.Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

func commanderGuildMembershipTx(ctx context.Context, tx pgx.Tx, commanderID uint32) (*GuildMember, error) {
	row := tx.QueryRow(ctx, `
SELECT gm.guild_id, gm.commander_id, gm.duty, gm.liveness, gm.pre_online_time, gm.join_time
FROM guild_members gm
JOIN guilds g ON g.id = gm.guild_id
WHERE gm.commander_id = $1
  AND g.deleted_at IS NULL
LIMIT 1
`, int64(commanderID))
	var member GuildMember
	err := row.Scan(
		&member.GuildID,
		&member.CommanderID,
		&member.Duty,
		&member.Liveness,
		&member.PreOnlineTime,
		&member.JoinTime,
	)
	if err != nil {
		return nil, db.MapNotFound(err)
	}
	return &member, nil
}

func commanderGuildWaitTimeTx(ctx context.Context, tx pgx.Tx, commanderID uint32) (uint32, error) {
	row := tx.QueryRow(ctx, `
SELECT guild_wait_time
FROM commander_guild_states
WHERE commander_id = $1
`, int64(commanderID))
	var waitTime uint32
	err := row.Scan(&waitTime)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	return waitTime, nil
}

func consumeResourceTx(ctx context.Context, tx pgx.Tx, commanderID uint32, resourceID uint32, amount uint32) error {
	if amount == 0 {
		return nil
	}
	res, err := tx.Exec(ctx, `
UPDATE owned_resources
SET amount = amount - $3
WHERE commander_id = $1
  AND resource_id = $2
  AND amount >= $3
`, int64(commanderID), int64(resourceID), int64(amount))
	if err != nil {
		return err
	}
	if res.RowsAffected() == 0 {
		return fmt.Errorf("not enough resources")
	}
	return nil
}

func adjustCommanderResourceCache(commander *Commander, resourceID uint32, amount uint32) {
	if commander == nil || amount == 0 {
		return
	}
	if commander.OwnedResourcesMap == nil {
		return
	}
	if resource, ok := commander.OwnedResourcesMap[resourceID]; ok {
		if resource.Amount >= amount {
			resource.Amount -= amount
		} else {
			resource.Amount = 0
		}
	}
}

func CreateGuild(commander *Commander, faction uint32, policy uint32, name string, manifesto string, createCost uint32, baseCapital uint32, techID uint32) (uint32, error) {
	if commander == nil || commander.CommanderID == 0 {
		return 0, ErrGuildInvalidArgument
	}
	ctx := context.Background()
	var guildID uint32
	err := db.DefaultStore.WithPGXTx(ctx, func(tx pgx.Tx) error {
		if _, err := commanderGuildMembershipTx(ctx, tx, commander.CommanderID); err == nil {
			return ErrCommanderInGuild
		} else if !errors.Is(err, db.ErrNotFound) {
			return err
		}
		nowUnix := uint32(time.Now().Unix())
		waitTime, err := commanderGuildWaitTimeTx(ctx, tx, commander.CommanderID)
		if err != nil {
			return err
		}
		if waitTime > nowUnix {
			return ErrGuildPermission
		}
		exists, err := guildNameExistsTx(ctx, tx, name, 0)
		if err != nil {
			return err
		}
		if exists {
			return ErrGuildNameExists
		}
		row := tx.QueryRow(ctx, `
INSERT INTO guilds (policy, faction, name, level, announce, manifesto, exp, member_count, change_faction_cd, kick_leader_cd, capital, tech_id)
VALUES ($1, $2, $3, 1, '', $4, 0, 1, 0, 0, $5, $6)
RETURNING id
`, int64(policy), int64(faction), name, manifesto, int64(baseCapital), int64(techID))
		if err := row.Scan(&guildID); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `
INSERT INTO guild_members (guild_id, commander_id, duty, liveness, pre_online_time, join_time)
VALUES ($1, $2, $3, 0, $4, $4)
`, int64(guildID), int64(commander.CommanderID), int64(GuildDutyCommander), int64(nowUnix)); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `
INSERT INTO guild_user_infos (commander_id, guild_id, donate_count, benefit_time, weekly_task_flag, extra_donate, extra_operation)
VALUES ($1, $2, 0, 0, 0, 0, 0)
ON CONFLICT (commander_id)
DO UPDATE SET guild_id = EXCLUDED.guild_id
`, int64(commander.CommanderID), int64(guildID)); err != nil {
			return err
		}
		if err := consumeResourceTx(ctx, tx, commander.CommanderID, guildCreateCostResourceID, createCost); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return 0, err
	}
	adjustCommanderResourceCache(commander, guildCreateCostResourceID, createCost)
	return guildID, nil
}

func GetGuildByID(guildID uint32) (*Guild, error) {
	ctx := context.Background()
	row := db.DefaultStore.Pool.QueryRow(ctx, `
SELECT id, policy, faction, name, level, announce, manifesto, exp, member_count, change_faction_cd, kick_leader_cd, capital, tech_id
FROM guilds
WHERE id = $1
  AND deleted_at IS NULL
`, int64(guildID))
	var guild Guild
	err := row.Scan(
		&guild.ID,
		&guild.Policy,
		&guild.Faction,
		&guild.Name,
		&guild.Level,
		&guild.Announce,
		&guild.Manifesto,
		&guild.Exp,
		&guild.MemberCount,
		&guild.ChangeFactionCD,
		&guild.KickLeaderCD,
		&guild.Capital,
		&guild.TechID,
	)
	if err != nil {
		return nil, db.MapNotFound(err)
	}
	return &guild, nil
}

func GetCommanderGuildMembership(commanderID uint32) (*GuildMember, error) {
	ctx := context.Background()
	row := db.DefaultStore.Pool.QueryRow(ctx, `
SELECT gm.guild_id, gm.commander_id, gm.duty, gm.liveness, gm.pre_online_time, gm.join_time
FROM guild_members gm
JOIN guilds g ON g.id = gm.guild_id
WHERE gm.commander_id = $1
  AND g.deleted_at IS NULL
LIMIT 1
`, int64(commanderID))
	var member GuildMember
	err := row.Scan(
		&member.GuildID,
		&member.CommanderID,
		&member.Duty,
		&member.Liveness,
		&member.PreOnlineTime,
		&member.JoinTime,
	)
	if err != nil {
		return nil, db.MapNotFound(err)
	}
	return &member, nil
}

func GetGuildForCommander(commanderID uint32) (*Guild, *GuildMember, error) {
	member, err := GetCommanderGuildMembership(commanderID)
	if err != nil {
		return nil, nil, err
	}
	guild, err := GetGuildByID(member.GuildID)
	if err != nil {
		return nil, nil, err
	}
	return guild, member, nil
}

func ListGuildMembers(guildID uint32) ([]GuildMember, error) {
	ctx := context.Background()
	rows, err := db.DefaultStore.Pool.Query(ctx, `
SELECT
  gm.guild_id,
  gm.commander_id,
  gm.duty,
  gm.liveness,
  gm.pre_online_time,
  gm.join_time,
  c.name,
  c.level,
  c.manifesto,
  c.last_login,
  c.display_icon_id,
  c.display_skin_id,
  c.selected_icon_frame_id,
  c.selected_chat_frame_id,
  c.display_icon_theme_id
FROM guild_members gm
JOIN commanders c ON c.commander_id = gm.commander_id
WHERE gm.guild_id = $1
ORDER BY gm.duty ASC, gm.join_time ASC
`, int64(guildID))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	members := make([]GuildMember, 0)
	for rows.Next() {
		var member GuildMember
		if err := rows.Scan(
			&member.GuildID,
			&member.CommanderID,
			&member.Duty,
			&member.Liveness,
			&member.PreOnlineTime,
			&member.JoinTime,
			&member.CommanderName,
			&member.CommanderLevel,
			&member.Manifesto,
			&member.LastLogin,
			&member.DisplayIconID,
			&member.DisplaySkinID,
			&member.IconFrameID,
			&member.ChatFrameID,
			&member.IconThemeID,
		); err != nil {
			return nil, err
		}
		members = append(members, member)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return members, nil
}

func GetGuildUserInfo(commanderID uint32) (*GuildUserInfo, error) {
	ctx := context.Background()
	row := db.DefaultStore.Pool.QueryRow(ctx, `
SELECT commander_id, guild_id, donate_count, benefit_time, weekly_task_flag, extra_donate, extra_operation
FROM guild_user_infos
WHERE commander_id = $1
`, int64(commanderID))
	var info GuildUserInfo
	err := row.Scan(
		&info.CommanderID,
		&info.GuildID,
		&info.DonateCount,
		&info.BenefitTime,
		&info.WeeklyTaskFlag,
		&info.ExtraDonate,
		&info.ExtraOperation,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return &GuildUserInfo{CommanderID: commanderID}, nil
	}
	if err != nil {
		return nil, err
	}
	return &info, nil
}

func SetCommanderGuildWaitTime(commanderID uint32, waitTime uint32) error {
	ctx := context.Background()
	_, err := db.DefaultStore.Pool.Exec(ctx, `
INSERT INTO commander_guild_states (commander_id, guild_wait_time)
VALUES ($1, $2)
ON CONFLICT (commander_id)
DO UPDATE SET guild_wait_time = EXCLUDED.guild_wait_time
`, int64(commanderID), int64(waitTime))
	return err
}

func GetCommanderGuildWaitTime(commanderID uint32) (uint32, error) {
	ctx := context.Background()
	row := db.DefaultStore.Pool.QueryRow(ctx, `
SELECT guild_wait_time
FROM commander_guild_states
WHERE commander_id = $1
`, int64(commanderID))
	var value uint32
	err := row.Scan(&value)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	return value, nil
}

func dutyRank(duty uint32) int {
	switch duty {
	case GuildDutyCommander:
		return 0
	case GuildDutyDeputy:
		return 1
	case GuildDutyOrdinary:
		return 2
	case GuildDutyRecruit:
		return 3
	default:
		return 100
	}
}

func IsValidGuildDuty(duty uint32) bool {
	switch duty {
	case GuildDutyCommander, GuildDutyDeputy, GuildDutyOrdinary, GuildDutyRecruit:
		return true
	default:
		return false
	}
}

func CountGuildMembersByDuty(guildID uint32, duty uint32) (uint32, error) {
	ctx := context.Background()
	row := db.DefaultStore.Pool.QueryRow(ctx, `
SELECT COUNT(*)
FROM guild_members
WHERE guild_id = $1
  AND duty = $2
`, int64(guildID), int64(duty))
	var count uint32
	if err := row.Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}

func updateGuildMemberCountTx(ctx context.Context, tx pgx.Tx, guildID uint32) error {
	if _, err := tx.Exec(ctx, `
UPDATE guilds g
SET member_count = (
    SELECT COUNT(*)
    FROM guild_members gm
    WHERE gm.guild_id = g.id
),
updated_at = CURRENT_TIMESTAMP
WHERE g.id = $1
`, int64(guildID)); err != nil {
		return err
	}
	return nil
}

func UpdateGuildBase(commander *Commander, guildID uint32, opType uint32, intValue uint32, strValue string, nameChangeCost uint32) error {
	ctx := context.Background()
	cleanText := strings.TrimSpace(strValue)
	return db.DefaultStore.WithPGXTx(ctx, func(tx pgx.Tx) error {
		membership, err := commanderGuildMembershipTx(ctx, tx, commander.CommanderID)
		if err != nil {
			return err
		}
		if membership.GuildID != guildID {
			return ErrGuildPermission
		}
		if membership.Duty != GuildDutyCommander && membership.Duty != GuildDutyDeputy {
			return ErrGuildPermission
		}

		switch opType {
		case 1:
			exists, err := guildNameExistsTx(ctx, tx, cleanText, guildID)
			if err != nil {
				return err
			}
			if exists {
				return ErrGuildNameExists
			}
			if err := consumeResourceTx(ctx, tx, commander.CommanderID, guildCreateCostResourceID, nameChangeCost); err != nil {
				return err
			}
			_, err = tx.Exec(ctx, `UPDATE guilds SET name = $2, updated_at = CURRENT_TIMESTAMP WHERE id = $1`, int64(guildID), cleanText)
			if err != nil {
				return err
			}
		case 2:
			_, err := tx.Exec(ctx, `UPDATE guilds SET faction = $2, updated_at = CURRENT_TIMESTAMP WHERE id = $1`, int64(guildID), int64(intValue))
			if err != nil {
				return err
			}
		case 3:
			_, err := tx.Exec(ctx, `UPDATE guilds SET policy = $2, updated_at = CURRENT_TIMESTAMP WHERE id = $1`, int64(guildID), int64(intValue))
			if err != nil {
				return err
			}
		case 4:
			_, err := tx.Exec(ctx, `UPDATE guilds SET manifesto = $2, updated_at = CURRENT_TIMESTAMP WHERE id = $1`, int64(guildID), cleanText)
			if err != nil {
				return err
			}
		case 5:
			_, err := tx.Exec(ctx, `UPDATE guilds SET announce = $2, updated_at = CURRENT_TIMESTAMP WHERE id = $1`, int64(guildID), cleanText)
			if err != nil {
				return err
			}
		default:
			return ErrGuildInvalidArgument
		}
		return nil
	})
}

func UpdateGuildDuty(commanderID uint32, targetCommanderID uint32, duty uint32) error {
	if !IsValidGuildDuty(duty) {
		return ErrGuildInvalidArgument
	}
	ctx := context.Background()
	return db.DefaultStore.WithPGXTx(ctx, func(tx pgx.Tx) error {
		actor, err := commanderGuildMembershipTx(ctx, tx, commanderID)
		if err != nil {
			return err
		}
		target, err := commanderGuildMembershipTx(ctx, tx, targetCommanderID)
		if err != nil {
			return err
		}
		if actor.GuildID != target.GuildID {
			return ErrGuildPermission
		}
		if commanderID == targetCommanderID {
			return ErrGuildInvalidArgument
		}
		if actor.Duty != GuildDutyCommander {
			return ErrGuildPermission
		}
		if duty == GuildDutyDeputy {
			guild, err := GetGuildByID(actor.GuildID)
			if err != nil {
				return err
			}
			limit, err := GetGuildDataLevelDeputyLimit(guild.Level)
			if err != nil {
				return err
			}
			count, err := CountGuildMembersByDuty(actor.GuildID, GuildDutyDeputy)
			if err != nil {
				return err
			}
			if target.Duty != GuildDutyDeputy && count >= limit {
				return ErrGuildPermission
			}
		}

		if duty == GuildDutyCommander {
			if target.Duty != GuildDutyDeputy {
				return ErrGuildPermission
			}
			if _, err := tx.Exec(ctx, `UPDATE guild_members SET duty = $3 WHERE guild_id = $1 AND commander_id = $2`, int64(actor.GuildID), int64(actor.CommanderID), int64(GuildDutyDeputy)); err != nil {
				return err
			}
		}

		if _, err := tx.Exec(ctx, `UPDATE guild_members SET duty = $3 WHERE guild_id = $1 AND commander_id = $2`, int64(actor.GuildID), int64(target.CommanderID), int64(duty)); err != nil {
			return err
		}
		return nil
	})
}

func FireGuildMember(commanderID uint32, targetCommanderID uint32) error {
	ctx := context.Background()
	return db.DefaultStore.WithPGXTx(ctx, func(tx pgx.Tx) error {
		actor, err := commanderGuildMembershipTx(ctx, tx, commanderID)
		if err != nil {
			return err
		}
		target, err := commanderGuildMembershipTx(ctx, tx, targetCommanderID)
		if err != nil {
			return err
		}
		if actor.GuildID != target.GuildID {
			return ErrGuildPermission
		}
		if commanderID == targetCommanderID {
			return ErrGuildInvalidArgument
		}
		if dutyRank(actor.Duty) >= dutyRank(target.Duty) {
			return ErrGuildPermission
		}
		if _, err := tx.Exec(ctx, `DELETE FROM guild_members WHERE guild_id = $1 AND commander_id = $2`, int64(target.GuildID), int64(targetCommanderID)); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `UPDATE guild_user_infos SET guild_id = 0 WHERE commander_id = $1`, int64(targetCommanderID)); err != nil {
			return err
		}
		if err := updateGuildMemberCountTx(ctx, tx, actor.GuildID); err != nil {
			return err
		}
		return nil
	})
}

func GuildImpeach(commanderID uint32, targetCommanderID uint32, now time.Time) error {
	ctx := context.Background()
	nowUnix := uint32(now.Unix())
	return db.DefaultStore.WithPGXTx(ctx, func(tx pgx.Tx) error {
		actor, err := commanderGuildMembershipTx(ctx, tx, commanderID)
		if err != nil {
			return err
		}
		target, err := commanderGuildMembershipTx(ctx, tx, targetCommanderID)
		if err != nil {
			return err
		}
		if actor.GuildID != target.GuildID {
			return ErrGuildPermission
		}
		if actor.Duty != GuildDutyDeputy || target.Duty != GuildDutyCommander {
			return ErrGuildPermission
		}
		var kickLeaderCD uint32
		var lastLogin time.Time
		if err := tx.QueryRow(ctx, `SELECT kick_leader_cd FROM guilds WHERE id = $1`, int64(actor.GuildID)).Scan(&kickLeaderCD); err != nil {
			return err
		}
		if kickLeaderCD > nowUnix {
			return ErrGuildPermission
		}
		if err := tx.QueryRow(ctx, `SELECT last_login FROM commanders WHERE commander_id = $1`, int64(targetCommanderID)).Scan(&lastLogin); err != nil {
			return err
		}
		if !lastLogin.Before(now) {
			return ErrGuildPermission
		}
		if now.Sub(lastLogin) <= time.Duration(impeachOfflineSeconds)*time.Second {
			return ErrGuildPermission
		}
		if _, err := tx.Exec(ctx, `UPDATE guilds SET kick_leader_cd = $2, updated_at = CURRENT_TIMESTAMP WHERE id = $1`, int64(actor.GuildID), int64(nowUnix+impeachCooldownSeconds)); err != nil {
			return err
		}
		return nil
	})
}

func GuildQuit(commanderID uint32, guildID uint32) error {
	ctx := context.Background()
	return db.DefaultStore.WithPGXTx(ctx, func(tx pgx.Tx) error {
		membership, err := commanderGuildMembershipTx(ctx, tx, commanderID)
		if err != nil {
			return err
		}
		if membership.GuildID != guildID {
			return ErrGuildInvalidArgument
		}
		if membership.Duty == GuildDutyCommander {
			return ErrGuildPermission
		}
		if _, err := tx.Exec(ctx, `DELETE FROM guild_members WHERE guild_id = $1 AND commander_id = $2`, int64(guildID), int64(commanderID)); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `UPDATE guild_user_infos SET guild_id = 0 WHERE commander_id = $1`, int64(commanderID)); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `
INSERT INTO commander_guild_states (commander_id, guild_wait_time)
VALUES ($1, $2)
ON CONFLICT (commander_id)
DO UPDATE SET guild_wait_time = EXCLUDED.guild_wait_time
`, int64(commanderID), int64(uint32(time.Now().Unix())+guildQuitCooldownSeconds)); err != nil {
			return err
		}
		if err := updateGuildMemberCountTx(ctx, tx, guildID); err != nil {
			return err
		}
		return nil
	})
}

func GuildDissolve(commanderID uint32, guildID uint32) error {
	ctx := context.Background()
	return db.DefaultStore.WithPGXTx(ctx, func(tx pgx.Tx) error {
		membership, err := commanderGuildMembershipTx(ctx, tx, commanderID)
		if err != nil {
			return err
		}
		if membership.GuildID != guildID {
			return ErrGuildInvalidArgument
		}
		if membership.Duty != GuildDutyCommander {
			return ErrGuildPermission
		}
		if _, err := tx.Exec(ctx, `UPDATE guild_user_infos SET guild_id = 0 WHERE guild_id = $1`, int64(guildID)); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `DELETE FROM guild_chat_messages WHERE guild_id = $1`, int64(guildID)); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `
DELETE FROM guild_chat_messages gcm
USING guild_members gm
WHERE gm.guild_id = $1
  AND gm.commander_id = gcm.sender_id
  AND gcm.guild_id = 0
`, int64(guildID)); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `DELETE FROM guild_members WHERE guild_id = $1`, int64(guildID)); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `UPDATE guilds SET deleted_at = CURRENT_TIMESTAMP, updated_at = CURRENT_TIMESTAMP WHERE id = $1`, int64(guildID)); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `
INSERT INTO commander_guild_states (commander_id, guild_wait_time)
VALUES ($1, $2)
ON CONFLICT (commander_id)
DO UPDATE SET guild_wait_time = EXCLUDED.guild_wait_time
`, int64(commanderID), int64(uint32(time.Now().Unix())+guildQuitCooldownSeconds)); err != nil {
			return err
		}
		return nil
	})
}
