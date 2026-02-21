package orm

import (
	"context"
	"sort"

	"github.com/ggmolly/belfast/internal/db"
	"github.com/jackc/pgx/v5"
)

const DefaultMetaTacticsSwitchCount uint32 = 3

type CommanderMetaTacticsState struct {
	CommanderID    uint32
	ShipID         uint32
	CurrentSkillID uint32
	DailyExp       uint32
	DoubleExp      uint32
	SwitchCnt      uint32
}

type CommanderMetaTacticsSkillState struct {
	CommanderID uint32
	ShipID      uint32
	SkillID     uint32
	SkillPos    uint32
	Level       uint32
	Exp         uint32
}

type CommanderMetaTacticsTaskProgress struct {
	CommanderID uint32
	ShipID      uint32
	SkillID     uint32
	TaskID      uint32
	FinishCnt   uint32
}

func GetOrCreateCommanderMetaTacticsStateTx(ctx context.Context, tx pgx.Tx, commanderID uint32, shipID uint32) (*CommanderMetaTacticsState, error) {
	if _, err := tx.Exec(ctx, `
INSERT INTO commander_meta_tactics_states (commander_id, ship_id, current_skill_id, daily_exp, double_exp, switch_cnt)
VALUES ($1, $2, 0, 0, 0, $3)
ON CONFLICT (commander_id, ship_id)
DO NOTHING
`, int64(commanderID), int64(shipID), int64(DefaultMetaTacticsSwitchCount)); err != nil {
		return nil, err
	}
	row := tx.QueryRow(ctx, `
SELECT commander_id, ship_id, current_skill_id, daily_exp, double_exp, switch_cnt
FROM commander_meta_tactics_states
WHERE commander_id = $1 AND ship_id = $2
FOR UPDATE
`, int64(commanderID), int64(shipID))
	state := &CommanderMetaTacticsState{}
	if err := row.Scan(&state.CommanderID, &state.ShipID, &state.CurrentSkillID, &state.DailyExp, &state.DoubleExp, &state.SwitchCnt); err != nil {
		return nil, db.MapNotFound(err)
	}
	return state, nil
}

func GetCommanderMetaTacticsState(commanderID uint32, shipID uint32) (*CommanderMetaTacticsState, error) {
	row := db.DefaultStore.Pool.QueryRow(context.Background(), `
SELECT commander_id, ship_id, current_skill_id, daily_exp, double_exp, switch_cnt
FROM commander_meta_tactics_states
WHERE commander_id = $1 AND ship_id = $2
`, int64(commanderID), int64(shipID))
	state := &CommanderMetaTacticsState{}
	if err := row.Scan(&state.CommanderID, &state.ShipID, &state.CurrentSkillID, &state.DailyExp, &state.DoubleExp, &state.SwitchCnt); err != nil {
		return nil, db.MapNotFound(err)
	}
	return state, nil
}

func SaveCommanderMetaTacticsStateTx(ctx context.Context, tx pgx.Tx, state *CommanderMetaTacticsState) error {
	_, err := tx.Exec(ctx, `
UPDATE commander_meta_tactics_states
SET current_skill_id = $3,
    daily_exp = $4,
    double_exp = $5,
    switch_cnt = $6,
    updated_at = NOW()
WHERE commander_id = $1 AND ship_id = $2
`, int64(state.CommanderID), int64(state.ShipID), int64(state.CurrentSkillID), int64(state.DailyExp), int64(state.DoubleExp), int64(state.SwitchCnt))
	return err
}

func GetOrCreateCommanderMetaTacticsSkillStateTx(ctx context.Context, tx pgx.Tx, commanderID uint32, shipID uint32, skillID uint32, skillPos uint32) (*CommanderMetaTacticsSkillState, error) {
	if _, err := tx.Exec(ctx, `
INSERT INTO commander_meta_tactics_skill_states (commander_id, ship_id, skill_id, skill_pos, level, exp)
VALUES ($1, $2, $3, $4, 0, 0)
ON CONFLICT (commander_id, ship_id, skill_id)
DO NOTHING
`, int64(commanderID), int64(shipID), int64(skillID), int64(skillPos)); err != nil {
		return nil, err
	}
	row := tx.QueryRow(ctx, `
SELECT commander_id, ship_id, skill_id, skill_pos, level, exp
FROM commander_meta_tactics_skill_states
WHERE commander_id = $1 AND ship_id = $2 AND skill_id = $3
FOR UPDATE
`, int64(commanderID), int64(shipID), int64(skillID))
	state := &CommanderMetaTacticsSkillState{}
	if err := row.Scan(&state.CommanderID, &state.ShipID, &state.SkillID, &state.SkillPos, &state.Level, &state.Exp); err != nil {
		return nil, db.MapNotFound(err)
	}
	return state, nil
}

func SaveCommanderMetaTacticsSkillStateTx(ctx context.Context, tx pgx.Tx, state *CommanderMetaTacticsSkillState) error {
	_, err := tx.Exec(ctx, `
INSERT INTO commander_meta_tactics_skill_states (commander_id, ship_id, skill_id, skill_pos, level, exp, updated_at)
VALUES ($1, $2, $3, $4, $5, $6, NOW())
ON CONFLICT (commander_id, ship_id, skill_id)
DO UPDATE SET
  skill_pos = EXCLUDED.skill_pos,
  level = EXCLUDED.level,
  exp = EXCLUDED.exp,
  updated_at = NOW()
`, int64(state.CommanderID), int64(state.ShipID), int64(state.SkillID), int64(state.SkillPos), int64(state.Level), int64(state.Exp))
	return err
}

func ListCommanderMetaTacticsSkillStates(commanderID uint32, shipID uint32) ([]CommanderMetaTacticsSkillState, error) {
	rows, err := db.DefaultStore.Pool.Query(context.Background(), `
SELECT commander_id, ship_id, skill_id, skill_pos, level, exp
FROM commander_meta_tactics_skill_states
WHERE commander_id = $1 AND ship_id = $2
ORDER BY skill_pos ASC, skill_id ASC
`, int64(commanderID), int64(shipID))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]CommanderMetaTacticsSkillState, 0)
	for rows.Next() {
		entry := CommanderMetaTacticsSkillState{}
		if err := rows.Scan(&entry.CommanderID, &entry.ShipID, &entry.SkillID, &entry.SkillPos, &entry.Level, &entry.Exp); err != nil {
			return nil, err
		}
		result = append(result, entry)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return result, nil
}

func UpsertCommanderMetaTacticsTaskProgressTx(ctx context.Context, tx pgx.Tx, entry *CommanderMetaTacticsTaskProgress) error {
	_, err := tx.Exec(ctx, `
INSERT INTO commander_meta_tactics_task_progress (commander_id, ship_id, skill_id, task_id, finish_cnt, updated_at)
VALUES ($1, $2, $3, $4, $5, NOW())
ON CONFLICT (commander_id, ship_id, skill_id, task_id)
DO UPDATE SET
  finish_cnt = EXCLUDED.finish_cnt,
  updated_at = NOW()
`, int64(entry.CommanderID), int64(entry.ShipID), int64(entry.SkillID), int64(entry.TaskID), int64(entry.FinishCnt))
	return err
}

func ListCommanderMetaTacticsTaskProgress(commanderID uint32, shipID uint32) ([]CommanderMetaTacticsTaskProgress, error) {
	rows, err := db.DefaultStore.Pool.Query(context.Background(), `
SELECT commander_id, ship_id, skill_id, task_id, finish_cnt
FROM commander_meta_tactics_task_progress
WHERE commander_id = $1 AND ship_id = $2
`, int64(commanderID), int64(shipID))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]CommanderMetaTacticsTaskProgress, 0)
	for rows.Next() {
		entry := CommanderMetaTacticsTaskProgress{}
		if err := rows.Scan(&entry.CommanderID, &entry.ShipID, &entry.SkillID, &entry.TaskID, &entry.FinishCnt); err != nil {
			return nil, err
		}
		result = append(result, entry)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].SkillID == result[j].SkillID {
			return result[i].TaskID < result[j].TaskID
		}
		return result[i].SkillID < result[j].SkillID
	})
	return result, nil
}
