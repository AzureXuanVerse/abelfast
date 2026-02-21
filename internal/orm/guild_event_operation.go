package orm

import (
	"context"
	"encoding/json"
	"errors"
	"sort"

	"github.com/jackc/pgx/v5"

	"github.com/ggmolly/belfast/internal/db"
)

type GuildOperationState struct {
	GuildID       uint32
	ChapterID     uint32
	StartTime     uint32
	EndTime       uint32
	Events        []GuildOperationEvent
	Perfs         []GuildOperationPerf
	JoinTimes     uint32
	IsParticipant uint32
}

type GuildOperationEvent struct {
	EventTid      uint32
	Position      uint32
	StartTime     uint32
	CompleteTime  uint32
	Efficiency    uint32
	Completed     bool
	ShipInEvent   json.RawMessage
	AttrAccList   json.RawMessage
	AttrCountList json.RawMessage
	EventNodes    json.RawMessage
	PersonShip    json.RawMessage
	FormationTime uint32
}

type GuildOperationPerf struct {
	EventTid uint32
	Index    uint32
}

type GuildReport struct {
	ID        uint32
	GuildID   uint32
	EventID   uint32
	EventType uint32
	Score     uint32
	Status    uint32
	Claimed   bool
	DropType  uint32
	DropID    uint32
	DropCount uint32
	Nodes     []GuildReportNode
}

type GuildReportNode struct {
	NodeID uint32
	Status uint32
}

type GuildReportRank struct {
	UserID uint32
	Damage uint32
}

func GetGuildOperationStateForCommander(commanderID uint32) (*GuildOperationState, error) {
	guild, _, err := GetGuildForCommander(commanderID)
	if err != nil {
		return nil, err
	}
	state, err := GetGuildOperationState(guild.ID)
	if err != nil {
		return nil, err
	}
	joinTimes, isParticipant, err := getGuildOperationParticipant(guild.ID, commanderID)
	if err != nil {
		return nil, err
	}
	state.JoinTimes = joinTimes
	state.IsParticipant = isParticipant
	return state, nil
}

func GetGuildOperationState(guildID uint32) (*GuildOperationState, error) {
	ctx := context.Background()
	row := db.DefaultStore.Pool.QueryRow(ctx, `
SELECT guild_id, chapter_id, start_time, end_time
FROM guild_operation_states
WHERE guild_id = $1
`, int64(guildID))
	state := &GuildOperationState{}
	if err := row.Scan(&state.GuildID, &state.ChapterID, &state.StartTime, &state.EndTime); err != nil {
		return nil, db.MapNotFound(err)
	}
	events, err := listGuildOperationEvents(ctx, db.DefaultStore.Pool, guildID, state.ChapterID)
	if err != nil {
		return nil, err
	}
	perfs, err := listGuildOperationPerfs(ctx, db.DefaultStore.Pool, guildID)
	if err != nil {
		return nil, err
	}
	state.Events = events
	state.Perfs = perfs
	return state, nil
}

func ActivateGuildOperation(commanderID uint32, chapterID uint32, consume uint32, durationSeconds uint32, now uint32) error {
	ctx := context.Background()
	return WithPGXTx(ctx, func(tx pgx.Tx) error {
		guild, member, err := getGuildForCommanderTx(ctx, tx, commanderID)
		if err != nil {
			return err
		}
		if member.Duty != GuildDutyCommander && member.Duty != GuildDutyDeputy {
			return ErrGuildPermission
		}
		if err := ensureNoActiveOperationTx(ctx, tx, guild.ID, now); err != nil {
			return err
		}
		res, err := tx.Exec(ctx, `
UPDATE guilds
SET capital = capital - $2,
    updated_at = CURRENT_TIMESTAMP
WHERE id = $1
  AND capital >= $2
		`, int64(guild.ID), int64(consume))
		if err != nil {
			return err
		}
		if res.RowsAffected() == 0 {
			return ErrGuildInsufficientCap
		}
		if _, err := tx.Exec(ctx, `DELETE FROM guild_operation_participants WHERE guild_id = $1`, int64(guild.ID)); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `
INSERT INTO guild_operation_states (guild_id, chapter_id, start_time, end_time, updated_at)
VALUES ($1, $2, $3, $4, CURRENT_TIMESTAMP)
ON CONFLICT (guild_id)
DO UPDATE SET chapter_id = EXCLUDED.chapter_id,
	start_time = EXCLUDED.start_time,
	end_time = EXCLUDED.end_time,
	updated_at = CURRENT_TIMESTAMP
`, int64(guild.ID), int64(chapterID), int64(now), int64(now+durationSeconds)); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `
UPDATE guilds
SET capital = capital,
    updated_at = CURRENT_TIMESTAMP
WHERE id = $1
`, int64(guild.ID)); err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `
INSERT INTO guild_operation_events (
    guild_id,
    event_tid,
    position,
    start_time,
    complete_time,
    efficiency,
    completed,
    shipinevent,
    attr_acc_list,
    attr_count_list,
    eventnodes,
    personship,
    formation_time
)
VALUES ($1, $2, 1, $3, 0, 0, false, '[]'::jsonb, '[]'::jsonb, '[]'::jsonb, '[]'::jsonb, '[]'::jsonb, 0)
ON CONFLICT (guild_id, event_tid)
DO UPDATE SET
    position = EXCLUDED.position,
    start_time = EXCLUDED.start_time,
    complete_time = EXCLUDED.complete_time,
    efficiency = EXCLUDED.efficiency,
    completed = EXCLUDED.completed,
    shipinevent = EXCLUDED.shipinevent,
    attr_acc_list = EXCLUDED.attr_acc_list,
    attr_count_list = EXCLUDED.attr_count_list,
    eventnodes = EXCLUDED.eventnodes,
    personship = EXCLUDED.personship,
    formation_time = EXCLUDED.formation_time
`, int64(guild.ID), int64(chapterID), int64(now))
		if err != nil {
			return err
		}
		return nil
	})
}

func ensureNoActiveOperationTx(ctx context.Context, tx pgx.Tx, guildID uint32, now uint32) error {
	row := tx.QueryRow(ctx, `
SELECT end_time
FROM guild_operation_states
WHERE guild_id = $1
`, int64(guildID))
	var endTime uint32
	err := row.Scan(&endTime)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	if endTime > now {
		return ErrGuildPermission
	}
	return nil
}

func getGuildForCommanderTx(ctx context.Context, tx pgx.Tx, commanderID uint32) (*Guild, *GuildMember, error) {
	member, err := commanderGuildMembershipTx(ctx, tx, commanderID)
	if err != nil {
		return nil, nil, err
	}
	row := tx.QueryRow(ctx, `
SELECT id, policy, faction, name, level, announce, manifesto, exp, member_count, change_faction_cd, kick_leader_cd, capital, tech_id
FROM guilds
WHERE id = $1
  AND deleted_at IS NULL
`, int64(member.GuildID))
	var guild Guild
	err = row.Scan(&guild.ID, &guild.Policy, &guild.Faction, &guild.Name, &guild.Level, &guild.Announce, &guild.Manifesto, &guild.Exp, &guild.MemberCount, &guild.ChangeFactionCD, &guild.KickLeaderCD, &guild.Capital, &guild.TechID)
	if err != nil {
		return nil, nil, db.MapNotFound(err)
	}
	return &guild, member, nil
}

func listGuildOperationEvents(ctx context.Context, q pgxQuerier, guildID uint32, chapterID uint32) ([]GuildOperationEvent, error) {
	rows, err := q.Query(ctx, `
SELECT event_tid, position, start_time, complete_time, efficiency, completed, shipinevent, attr_acc_list, attr_count_list, eventnodes, personship, formation_time
FROM guild_operation_events
WHERE guild_id = $1
  AND event_tid = $2
ORDER BY event_tid ASC
`, int64(guildID), int64(chapterID))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	events := make([]GuildOperationEvent, 0)
	for rows.Next() {
		var event GuildOperationEvent
		if err := rows.Scan(
			&event.EventTid,
			&event.Position,
			&event.StartTime,
			&event.CompleteTime,
			&event.Efficiency,
			&event.Completed,
			&event.ShipInEvent,
			&event.AttrAccList,
			&event.AttrCountList,
			&event.EventNodes,
			&event.PersonShip,
			&event.FormationTime,
		); err != nil {
			return nil, err
		}
		events = append(events, event)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return events, nil
}

func listGuildOperationPerfs(ctx context.Context, q pgxQuerier, guildID uint32) ([]GuildOperationPerf, error) {
	rows, err := q.Query(ctx, `
SELECT event_tid, perf_index
FROM guild_operation_perfs
WHERE guild_id = $1
ORDER BY event_tid ASC
`, int64(guildID))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	perfs := make([]GuildOperationPerf, 0)
	for rows.Next() {
		var perf GuildOperationPerf
		if err := rows.Scan(&perf.EventTid, &perf.Index); err != nil {
			return nil, err
		}
		perfs = append(perfs, perf)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return perfs, nil
}

type pgxQuerier interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
}

func getGuildOperationParticipant(guildID uint32, commanderID uint32) (uint32, uint32, error) {
	ctx := context.Background()
	row := db.DefaultStore.Pool.QueryRow(ctx, `
SELECT join_times, is_participant
FROM guild_operation_participants
WHERE guild_id = $1 AND commander_id = $2
`, int64(guildID), int64(commanderID))
	var joinTimes uint32
	var isParticipant uint32
	err := row.Scan(&joinTimes, &isParticipant)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, 0, nil
	}
	if err != nil {
		return 0, 0, err
	}
	return joinTimes, isParticipant, nil
}

func UpsertGuildOperationPerf(guildID uint32, eventTid uint32, index uint32) error {
	ctx := context.Background()
	_, err := db.DefaultStore.Pool.Exec(ctx, `
INSERT INTO guild_operation_perfs (guild_id, event_tid, perf_index)
VALUES ($1, $2, $3)
ON CONFLICT (guild_id, event_tid)
DO UPDATE SET perf_index = EXCLUDED.perf_index
`, int64(guildID), int64(eventTid), int64(index))
	return err
}

func GetGuildOperationEvent(guildID uint32, eventTid uint32) (*GuildOperationEvent, error) {
	ctx := context.Background()
	row := db.DefaultStore.Pool.QueryRow(ctx, `
SELECT event_tid, position, start_time, complete_time, efficiency, completed, shipinevent, attr_acc_list, attr_count_list, eventnodes, personship, formation_time
FROM guild_operation_events
WHERE guild_id = $1
  AND event_tid = $2
`, int64(guildID), int64(eventTid))
	var event GuildOperationEvent
	err := row.Scan(&event.EventTid, &event.Position, &event.StartTime, &event.CompleteTime, &event.Efficiency, &event.Completed, &event.ShipInEvent, &event.AttrAccList, &event.AttrCountList, &event.EventNodes, &event.PersonShip, &event.FormationTime)
	if err != nil {
		return nil, db.MapNotFound(err)
	}
	return &event, nil
}

func UpdateGuildOperationEventFormation(guildID uint32, eventTid uint32, personShip json.RawMessage, formationTime uint32) error {
	ctx := context.Background()
	_, err := db.DefaultStore.Pool.Exec(ctx, `
UPDATE guild_operation_events
SET personship = $3,
    formation_time = $4
WHERE guild_id = $1
  AND event_tid = $2
`, int64(guildID), int64(eventTid), personShip, int64(formationTime))
	return err
}

func UpdateGuildOperationEventRefresh(guildID uint32, eventTid uint32, formationTime uint32) error {
	ctx := context.Background()
	_, err := db.DefaultStore.Pool.Exec(ctx, `
UPDATE guild_operation_events
SET formation_time = $3
WHERE guild_id = $1
  AND event_tid = $2
`, int64(guildID), int64(eventTid), int64(formationTime))
	return err
}

func ListGuildReportsSince(guildID uint32, index uint32) ([]GuildReport, error) {
	ctx := context.Background()
	rows, err := db.DefaultStore.Pool.Query(ctx, `
SELECT id, guild_id, event_id, event_type, score, status, claimed, drop_type, drop_id, drop_count
FROM guild_reports
WHERE guild_id = $1
  AND id > $2
ORDER BY id ASC
`, int64(guildID), int64(index))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	reports := make([]GuildReport, 0)
	for rows.Next() {
		var report GuildReport
		if err := rows.Scan(&report.ID, &report.GuildID, &report.EventID, &report.EventType, &report.Score, &report.Status, &report.Claimed, &report.DropType, &report.DropID, &report.DropCount); err != nil {
			return nil, err
		}
		reports = append(reports, report)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for i := range reports {
		nodes, err := listGuildReportNodes(ctx, reports[i].GuildID, reports[i].ID)
		if err != nil {
			return nil, err
		}
		reports[i].Nodes = nodes
	}
	return reports, nil
}

func listGuildReportNodes(ctx context.Context, guildID uint32, reportID uint32) ([]GuildReportNode, error) {
	rows, err := db.DefaultStore.Pool.Query(ctx, `
SELECT node_id, status
FROM guild_report_nodes
WHERE guild_id = $1
  AND report_id = $2
ORDER BY node_id ASC
`, int64(guildID), int64(reportID))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	nodes := make([]GuildReportNode, 0)
	for rows.Next() {
		var node GuildReportNode
		if err := rows.Scan(&node.NodeID, &node.Status); err != nil {
			return nil, err
		}
		nodes = append(nodes, node)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return nodes, nil
}

func ClaimGuildReports(guildID uint32, reportIDs []uint32) ([]GuildReport, error) {
	ctx := context.Background()
	var reports []GuildReport
	err := WithPGXTx(ctx, func(tx pgx.Tx) error {
		var err error
		reports, err = ClaimGuildReportsTx(ctx, tx, guildID, reportIDs)
		return err
	})
	if err != nil {
		return nil, err
	}
	return reports, nil
}

func ClaimGuildReportsTx(ctx context.Context, tx pgx.Tx, guildID uint32, reportIDs []uint32) ([]GuildReport, error) {
	reports := make([]GuildReport, 0, len(reportIDs))
	for _, reportID := range reportIDs {
		row := tx.QueryRow(ctx, `
SELECT id, guild_id, event_id, event_type, score, status, claimed, drop_type, drop_id, drop_count
FROM guild_reports
WHERE guild_id = $1
  AND id = $2
FOR UPDATE
`, int64(guildID), int64(reportID))
		var report GuildReport
		if err := row.Scan(&report.ID, &report.GuildID, &report.EventID, &report.EventType, &report.Score, &report.Status, &report.Claimed, &report.DropType, &report.DropID, &report.DropCount); err != nil {
			return nil, db.MapNotFound(err)
		}
		if report.Claimed {
			return nil, ErrGuildPermission
		}
		if _, err := tx.Exec(ctx, `
UPDATE guild_reports
SET claimed = true,
    status = 2
WHERE guild_id = $1
  AND id = $2
`, int64(guildID), int64(reportID)); err != nil {
			return nil, err
		}
		reports = append(reports, report)
	}
	return reports, nil
}

func ListGuildReportRanks(guildID uint32, reportID uint32) ([]GuildReportRank, error) {
	ctx := context.Background()
	rows, err := db.DefaultStore.Pool.Query(ctx, `
SELECT user_id, damage
FROM guild_report_ranks
WHERE guild_id = $1
  AND report_id = $2
`, int64(guildID), int64(reportID))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	ranks := make([]GuildReportRank, 0)
	for rows.Next() {
		var entry GuildReportRank
		if err := rows.Scan(&entry.UserID, &entry.Damage); err != nil {
			return nil, err
		}
		ranks = append(ranks, entry)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	sort.Slice(ranks, func(i, j int) bool {
		if ranks[i].Damage == ranks[j].Damage {
			return ranks[i].UserID < ranks[j].UserID
		}
		return ranks[i].Damage > ranks[j].Damage
	})
	return ranks, nil
}

func UpdateGuildOperationParticipation(commanderID uint32, now uint32, maxJoinTimes uint32, livenessGain uint32) error {
	ctx := context.Background()
	return WithPGXTx(ctx, func(tx pgx.Tx) error {
		guild, _, err := getGuildForCommanderTx(ctx, tx, commanderID)
		if err != nil {
			return err
		}
		state, err := getGuildOperationStateTx(ctx, tx, guild.ID)
		if err != nil {
			return err
		}
		if state.EndTime <= now {
			return ErrGuildPermission
		}
		joinTimes, _, err := getGuildOperationParticipantTx(ctx, tx, guild.ID, commanderID)
		if err != nil {
			return err
		}
		if joinTimes >= maxJoinTimes {
			res, err := tx.Exec(ctx, `
UPDATE guild_user_infos
SET extra_operation = extra_operation - 1
WHERE commander_id = $1
  AND extra_operation > 0
`, int64(commanderID))
			if err != nil {
				return err
			}
			if res.RowsAffected() == 0 {
				return ErrGuildPermission
			}
		} else {
			joinTimes++
		}
		_, err = tx.Exec(ctx, `
INSERT INTO guild_operation_participants (guild_id, commander_id, join_times, is_participant)
VALUES ($1, $2, $3, 1)
ON CONFLICT (guild_id, commander_id)
DO UPDATE SET join_times = EXCLUDED.join_times,
	is_participant = 1
`, int64(guild.ID), int64(commanderID), int64(joinTimes))
		if err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `
UPDATE guild_members
SET liveness = liveness + $3
WHERE guild_id = $1
  AND commander_id = $2
`, int64(guild.ID), int64(commanderID), int64(livenessGain))
		if err != nil {
			return err
		}
		return nil
	})
}

func getGuildOperationParticipantTx(ctx context.Context, tx pgx.Tx, guildID uint32, commanderID uint32) (uint32, uint32, error) {
	row := tx.QueryRow(ctx, `
SELECT join_times, is_participant
FROM guild_operation_participants
WHERE guild_id = $1
  AND commander_id = $2
`, int64(guildID), int64(commanderID))
	var joinTimes uint32
	var isParticipant uint32
	err := row.Scan(&joinTimes, &isParticipant)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, 0, nil
	}
	if err != nil {
		return 0, 0, err
	}
	return joinTimes, isParticipant, nil
}

func getGuildOperationStateTx(ctx context.Context, tx pgx.Tx, guildID uint32) (*GuildOperationState, error) {
	row := tx.QueryRow(ctx, `
SELECT guild_id, chapter_id, start_time, end_time
FROM guild_operation_states
WHERE guild_id = $1
`, int64(guildID))
	state := &GuildOperationState{}
	err := row.Scan(&state.GuildID, &state.ChapterID, &state.StartTime, &state.EndTime)
	if err != nil {
		return nil, db.MapNotFound(err)
	}
	return state, nil
}
