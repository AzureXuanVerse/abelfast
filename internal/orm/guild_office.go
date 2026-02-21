package orm

import (
	"context"
	"encoding/json"
	"errors"
	"sort"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/ggmolly/belfast/internal/db"
)

const (
	GuildCapitalLogCategoryIncrease = uint32(1)
	GuildCapitalLogCategoryDecrease = uint32(2)
	GuildCapitalLogCategoryOther    = uint32(3)
)

type GuildOfficeState struct {
	BenefitFinishTime     uint32
	LastBenefitFinishTime uint32
	TechCancelCnt         uint32
}

type GuildWeeklyTaskState struct {
	GuildID       uint32
	TaskID        uint32
	Progress      uint32
	Monday0Clock  uint32
	UpdatedAtUnix uint32
}

type GuildCapitalLogEntry struct {
	Category    uint32
	MemberID    uint32
	Name        string
	EventType   uint32
	EventTarget []uint32
	Time        uint32
}

type GuildMemberRankEntry struct {
	UserID uint32
	Count  uint32
}

func ListGuildDirectoryRefresh(limit uint32) ([]GuildDirectoryEntry, error) {
	if limit == 0 {
		limit = 30
	}
	ctx := context.Background()
	rows, err := db.DefaultStore.Pool.Query(ctx, `
SELECT
	g.id,
	g.policy,
	g.faction,
	g.name,
	g.level,
	g.announce,
	g.manifesto,
	g.exp,
	g.member_count,
	g.change_faction_cd,
	g.kick_leader_cd,
	g.capital,
	g.tech_id,
	COALESCE(c.commander_id, 0),
	COALESCE(c.name, ''),
	COALESCE(c.level, 0),
	COALESCE(c.manifesto, ''),
	COALESCE(EXTRACT(EPOCH FROM c.last_login)::bigint, 0),
	COALESCE(c.display_icon_id, 0),
	COALESCE(c.display_skin_id, 0),
	COALESCE(c.selected_icon_frame_id, 0),
	COALESCE(c.selected_chat_frame_id, 0),
	COALESCE(c.display_icon_theme_id, 0)
FROM guilds g
LEFT JOIN guild_members gm
	ON gm.guild_id = g.id
	AND gm.duty = $1
LEFT JOIN commanders c
	ON c.commander_id = gm.commander_id
	AND c.deleted_at IS NULL
WHERE g.deleted_at IS NULL
ORDER BY g.id ASC
LIMIT $2
`, int64(GuildDutyCommander), int64(limit))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	entries := make([]GuildDirectoryEntry, 0)
	for rows.Next() {
		var entry GuildDirectoryEntry
		var lastLoginUnix int64
		if err := rows.Scan(
			&entry.Guild.ID,
			&entry.Guild.Policy,
			&entry.Guild.Faction,
			&entry.Guild.Name,
			&entry.Guild.Level,
			&entry.Guild.Announce,
			&entry.Guild.Manifesto,
			&entry.Guild.Exp,
			&entry.Guild.MemberCount,
			&entry.Guild.ChangeFactionCD,
			&entry.Guild.KickLeaderCD,
			&entry.Guild.Capital,
			&entry.Guild.TechID,
			&entry.Leader.CommanderID,
			&entry.Leader.Name,
			&entry.Leader.Level,
			&entry.Leader.Manifesto,
			&lastLoginUnix,
			&entry.Leader.DisplayIconID,
			&entry.Leader.DisplaySkinID,
			&entry.Leader.SelectedIconFrameID,
			&entry.Leader.SelectedChatFrameID,
			&entry.Leader.DisplayIconThemeID,
		); err != nil {
			return nil, err
		}
		if lastLoginUnix > 0 {
			entry.Leader.LastLoginUnix = uint32(lastLoginUnix)
		}
		entries = append(entries, entry)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return entries, nil
}

func AcceptGuildJoinRequest(actorCommanderID uint32, applicantCommanderID uint32) (uint32, error) {
	ctx := context.Background()
	const (
		resultSuccess = uint32(0)
		resultJoined  = uint32(1)
		resultWait    = uint32(4)
		resultFrozen  = uint32(4305)
		resultFail    = uint32(2)
	)
	err := WithPGXTx(ctx, func(tx pgx.Tx) error {
		actor, err := commanderGuildMembershipTx(ctx, tx, actorCommanderID)
		if err != nil {
			return err
		}
		if actor.Duty != GuildDutyCommander && actor.Duty != GuildDutyDeputy {
			return ErrGuildPermission
		}

		guild, err := GetGuildByID(actor.GuildID)
		if err != nil {
			return db.MapNotFound(err)
		}
		if guild == nil {
			return db.ErrNotFound
		}

		if _, err := commanderGuildMembershipTx(ctx, tx, applicantCommanderID); err == nil {
			return ErrCommanderInGuild
		} else if !errors.Is(db.MapNotFound(err), db.ErrNotFound) {
			return err
		}

		waitTime, err := commanderGuildWaitTimeTx(ctx, tx, applicantCommanderID)
		if err != nil {
			return err
		}
		if waitTime > uint32(time.Now().Unix()) {
			return ErrGuildInvalidArgument
		}

		memberLimit, err := GetGuildDataLevelMemberLimit(guild.Level)
		if err != nil {
			return err
		}
		if guild.MemberCount >= memberLimit {
			return ErrGuildInsufficientCap
		}

		var requestExists bool
		if err := tx.QueryRow(ctx, `
SELECT EXISTS(
	SELECT 1
	FROM guild_join_requests
	WHERE guild_id = $1
	  AND applicant_commander_id = $2
)
`, int64(actor.GuildID), int64(applicantCommanderID)).Scan(&requestExists); err != nil {
			return err
		}
		if !requestExists {
			return ErrGuildInvalidArgument
		}

		now := uint32(time.Now().Unix())
		if _, err := tx.Exec(ctx, `
INSERT INTO guild_members (guild_id, commander_id, duty, liveness, pre_online_time, join_time)
VALUES ($1, $2, $3, 0, $4, $4)
`, int64(actor.GuildID), int64(applicantCommanderID), int64(GuildDutyOrdinary), int64(now)); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `
INSERT INTO guild_user_infos (commander_id, guild_id, donate_count, benefit_time, weekly_task_flag, extra_donate, extra_operation)
VALUES ($1, $2, 0, 0, 0, 0, 0)
ON CONFLICT (commander_id)
DO UPDATE SET guild_id = EXCLUDED.guild_id
`, int64(applicantCommanderID), int64(actor.GuildID)); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `
DELETE FROM guild_join_requests
WHERE guild_id = $1
	AND applicant_commander_id = $2
`, int64(actor.GuildID), int64(applicantCommanderID)); err != nil {
			return err
		}
		return updateGuildMemberCountTx(ctx, tx, actor.GuildID)
	})
	if err == nil {
		return resultSuccess, nil
	}
	if errors.Is(err, ErrCommanderInGuild) {
		return resultJoined, nil
	}
	if errors.Is(err, ErrGuildInvalidArgument) {
		return resultWait, nil
	}
	if errors.Is(err, db.ErrNotFound) {
		return resultFrozen, nil
	}
	if errors.Is(err, ErrGuildPermission) || errors.Is(err, ErrGuildInsufficientCap) {
		return resultFail, nil
	}
	return resultFail, err
}

func GetGuildOfficeState(guildID uint32) (*GuildOfficeState, error) {
	ctx := context.Background()
	row := db.DefaultStore.Pool.QueryRow(ctx, `
SELECT benefit_finish_time, last_benefit_finish_time, tech_cancel_cnt
FROM guilds
WHERE id = $1
  AND deleted_at IS NULL
`, int64(guildID))
	state := &GuildOfficeState{}
	err := row.Scan(&state.BenefitFinishTime, &state.LastBenefitFinishTime, &state.TechCancelCnt)
	if err != nil {
		return nil, db.MapNotFound(err)
	}
	return state, nil
}

func StartGuildSupply(guildID uint32, cost uint32, benefitFinishTime uint32) error {
	ctx := context.Background()
	return WithPGXTx(ctx, func(tx pgx.Tx) error {
		res, err := tx.Exec(ctx, `
UPDATE guilds
SET
	capital = capital - $2,
	last_benefit_finish_time = benefit_finish_time,
	benefit_finish_time = $3,
	updated_at = CURRENT_TIMESTAMP
WHERE id = $1
	AND deleted_at IS NULL
	AND capital >= $2
`, int64(guildID), int64(cost), int64(benefitFinishTime))
		if err != nil {
			return err
		}
		if res.RowsAffected() == 0 {
			return ErrGuildInsufficientCap
		}
		return nil
	})
}

func UpdateGuildUserBenefitTime(commanderID uint32, benefitTime uint32) error {
	ctx := context.Background()
	_, err := db.DefaultStore.Pool.Exec(ctx, `
INSERT INTO guild_user_infos (commander_id, guild_id, benefit_time)
VALUES ($1, 0, $2)
ON CONFLICT (commander_id)
DO UPDATE SET benefit_time = EXCLUDED.benefit_time
`, int64(commanderID), int64(benefitTime))
	return err
}

func GetGuildWeeklyTaskState(guildID uint32) (*GuildWeeklyTaskState, error) {
	ctx := context.Background()
	row := db.DefaultStore.Pool.QueryRow(ctx, `
SELECT guild_id, task_id, progress, monday_0clock, COALESCE(EXTRACT(EPOCH FROM updated_at)::bigint, 0)
FROM guild_weekly_tasks
WHERE guild_id = $1
`, int64(guildID))
	state := &GuildWeeklyTaskState{}
	err := row.Scan(&state.GuildID, &state.TaskID, &state.Progress, &state.Monday0Clock, &state.UpdatedAtUnix)
	if err != nil {
		return nil, db.MapNotFound(err)
	}
	return state, nil
}

func UpsertGuildWeeklyTaskState(guildID uint32, taskID uint32, progress uint32, monday0Clock uint32) error {
	ctx := context.Background()
	_, err := db.DefaultStore.Pool.Exec(ctx, `
INSERT INTO guild_weekly_tasks (guild_id, task_id, progress, monday_0clock, updated_at)
VALUES ($1, $2, $3, $4, CURRENT_TIMESTAMP)
ON CONFLICT (guild_id)
DO UPDATE SET
	task_id = EXCLUDED.task_id,
	progress = EXCLUDED.progress,
	monday_0clock = EXCLUDED.monday_0clock,
	updated_at = CURRENT_TIMESTAMP
`, int64(guildID), int64(taskID), int64(progress), int64(monday0Clock))
	return err
}

func ListGuildCapitalLogsByCategory(guildID uint32, limit uint32) (map[uint32][]GuildCapitalLogEntry, error) {
	if limit == 0 {
		limit = 50
	}
	ctx := context.Background()
	rows, err := db.DefaultStore.Pool.Query(ctx, `
SELECT category, member_id, name, event_type, event_target, event_time
FROM guild_capital_logs
WHERE guild_id = $1
ORDER BY category ASC, event_time DESC, id DESC
LIMIT $2
`, int64(guildID), int64(limit))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	grouped := map[uint32][]GuildCapitalLogEntry{
		GuildCapitalLogCategoryIncrease: {},
		GuildCapitalLogCategoryDecrease: {},
		GuildCapitalLogCategoryOther:    {},
	}
	for rows.Next() {
		var entry GuildCapitalLogEntry
		var rawTargets []byte
		if err := rows.Scan(&entry.Category, &entry.MemberID, &entry.Name, &entry.EventType, &rawTargets, &entry.Time); err != nil {
			return nil, err
		}
		_ = json.Unmarshal(rawTargets, &entry.EventTarget)
		grouped[entry.Category] = append(grouped[entry.Category], entry)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return grouped, nil
}

func CreateGuildCapitalLog(guildID uint32, entry GuildCapitalLogEntry) error {
	targets, _ := json.Marshal(entry.EventTarget)
	ctx := context.Background()
	_, err := db.DefaultStore.Pool.Exec(ctx, `
INSERT INTO guild_capital_logs (guild_id, category, member_id, name, event_type, event_target, event_time)
VALUES ($1, $2, $3, $4, $5, $6, $7)
`, int64(guildID), int64(entry.Category), int64(entry.MemberID), entry.Name, int64(entry.EventType), targets, int64(entry.Time))
	return err
}

func SetGuildUserDonateTasks(commanderID uint32, tasks []uint32) error {
	encoded, _ := json.Marshal(tasks)
	ctx := context.Background()
	_, err := db.DefaultStore.Pool.Exec(ctx, `
INSERT INTO guild_user_infos (commander_id, guild_id, donate_tasks)
VALUES ($1, 0, $2)
ON CONFLICT (commander_id)
DO UPDATE SET donate_tasks = EXCLUDED.donate_tasks
`, int64(commanderID), encoded)
	return err
}

func GetGuildUserDonateTasks(commanderID uint32) ([]uint32, error) {
	ctx := context.Background()
	row := db.DefaultStore.Pool.QueryRow(ctx, `
SELECT donate_tasks
FROM guild_user_infos
WHERE commander_id = $1
`, int64(commanderID))
	var raw []byte
	err := row.Scan(&raw)
	if errors.Is(err, pgx.ErrNoRows) {
		return []uint32{}, nil
	}
	if err != nil {
		return nil, err
	}
	tasks := make([]uint32, 0)
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &tasks)
	}
	return tasks, nil
}

func IncrementGuildMemberRank(guildID uint32, rankType uint32, userID uint32, amount uint32) error {
	if amount == 0 {
		return nil
	}
	ctx := context.Background()
	for _, period := range []uint32{1, 2, 3} {
		if _, err := db.DefaultStore.Pool.Exec(ctx, `
INSERT INTO guild_member_ranks (guild_id, rank_type, period, user_id, count)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (guild_id, rank_type, period, user_id)
DO UPDATE SET count = guild_member_ranks.count + EXCLUDED.count
`, int64(guildID), int64(rankType), int64(period), int64(userID), int64(amount)); err != nil {
			return err
		}
	}
	return nil
}

func ListGuildMemberRanks(guildID uint32, rankType uint32) (map[uint32][]GuildMemberRankEntry, error) {
	ctx := context.Background()
	rows, err := db.DefaultStore.Pool.Query(ctx, `
SELECT period, user_id, count
FROM guild_member_ranks
WHERE guild_id = $1
  AND rank_type = $2
ORDER BY period ASC, count DESC, user_id ASC
`, int64(guildID), int64(rankType))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	ranks := map[uint32][]GuildMemberRankEntry{1: {}, 2: {}, 3: {}}
	for rows.Next() {
		var period uint32
		var entry GuildMemberRankEntry
		if err := rows.Scan(&period, &entry.UserID, &entry.Count); err != nil {
			return nil, err
		}
		ranks[period] = append(ranks[period], entry)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for period := range ranks {
		sort.SliceStable(ranks[period], func(i, j int) bool {
			if ranks[period][i].Count == ranks[period][j].Count {
				return ranks[period][i].UserID < ranks[period][j].UserID
			}
			return ranks[period][i].Count > ranks[period][j].Count
		})
	}
	return ranks, nil
}

func SetGuildTechnologyActive(guildID uint32, techID uint32) error {
	ctx := context.Background()
	_, err := db.DefaultStore.Pool.Exec(ctx, `
UPDATE guilds
SET tech_id = $2,
	tech_cancel_cnt = tech_cancel_cnt + 1,
	updated_at = CURRENT_TIMESTAMP
WHERE id = $1
	AND deleted_at IS NULL
`, int64(guildID), int64(techID))
	return err
}

func UpsertGuildUserTechnologyState(commanderID uint32, techGroup uint32, techID uint32) error {
	ctx := context.Background()
	_, err := db.DefaultStore.Pool.Exec(ctx, `
INSERT INTO guild_user_technology_states (commander_id, tech_group, tech_id, updated_at)
VALUES ($1, $2, $3, CURRENT_TIMESTAMP)
ON CONFLICT (commander_id, tech_group)
DO UPDATE SET tech_id = EXCLUDED.tech_id, updated_at = CURRENT_TIMESTAMP
`, int64(commanderID), int64(techGroup), int64(techID))
	return err
}

func GetGuildUserTechnologyByGroup(commanderID uint32, techGroup uint32) (uint32, error) {
	ctx := context.Background()
	row := db.DefaultStore.Pool.QueryRow(ctx, `
SELECT tech_id
FROM guild_user_technology_states
WHERE commander_id = $1
	AND tech_group = $2
`, int64(commanderID), int64(techGroup))
	var techID uint32
	err := row.Scan(&techID)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	return techID, nil
}

func ListGuildUserTechnologyState(commanderID uint32) ([]uint32, error) {
	ctx := context.Background()
	rows, err := db.DefaultStore.Pool.Query(ctx, `
SELECT tech_id
FROM guild_user_technology_states
WHERE commander_id = $1
ORDER BY tech_group ASC
`, int64(commanderID))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	techIDs := make([]uint32, 0)
	for rows.Next() {
		var techID uint32
		if err := rows.Scan(&techID); err != nil {
			return nil, err
		}
		techIDs = append(techIDs, techID)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return techIDs, nil
}

func AddGuildCapital(guildID uint32, amount uint32) error {
	if amount == 0 {
		return nil
	}
	ctx := context.Background()
	_, err := db.DefaultStore.Pool.Exec(ctx, `
UPDATE guilds
SET capital = capital + $2,
	updated_at = CURRENT_TIMESTAMP
WHERE id = $1
	AND deleted_at IS NULL
`, int64(guildID), int64(amount))
	return err
}

func AddGuildMemberLiveness(guildID uint32, commanderID uint32, amount uint32) error {
	if amount == 0 {
		return nil
	}
	ctx := context.Background()
	_, err := db.DefaultStore.Pool.Exec(ctx, `
UPDATE guild_members
SET liveness = liveness + $3
WHERE guild_id = $1
	AND commander_id = $2
`, int64(guildID), int64(commanderID), int64(amount))
	return err
}
