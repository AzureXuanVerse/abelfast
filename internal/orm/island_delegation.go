package orm

import (
	"context"
	"encoding/json"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/ggmolly/belfast/internal/db"
)

type IslandDelegation struct {
	CommanderID   uint32 `gorm:"primaryKey;column:commander_id"`
	BuildID       uint32 `gorm:"primaryKey;column:build_id"`
	AreaID        uint32 `gorm:"primaryKey;column:area_id"`
	ShipID        uint32 `gorm:"column:ship_id"`
	HasRole       bool   `gorm:"column:has_role"`
	RewardReady   bool   `gorm:"column:reward_ready"`
	FormulaID     uint32 `gorm:"column:formula_id"`
	MaxTimes      uint32 `gorm:"column:max_times"`
	MainNum       uint32 `gorm:"column:main_num"`
	OtherNum      uint32 `gorm:"column:other_num"`
	ExtraMainNum  uint32 `gorm:"column:extra_main_num"`
	ExtraOtherNum uint32 `gorm:"column:extra_other_num"`
	GetTimes      uint32 `gorm:"column:get_times"`
	StartTime     uint32 `gorm:"column:start_time"`
	CostTimeList  []uint32
	SpeedTime     uint32 `gorm:"column:speed_time"`
	TimesExtra    []uint32
	RecoverTime   uint32 `gorm:"column:recover_time"`
	AddExp        uint32 `gorm:"column:add_exp"`
	ReturnNum     uint32 `gorm:"column:return_num"`
	PTAward       uint32 `gorm:"column:pt_award"`
}

func (IslandDelegation) TableName() string {
	return "island_delegations"
}

func UpsertIslandDelegation(slot *IslandDelegation) error {
	ctx := context.Background()
	return upsertIslandDelegationWithQueryer(ctx, db.DefaultStore.Pool, slot)
}

func UpsertIslandDelegationTx(ctx context.Context, tx pgx.Tx, slot *IslandDelegation) error {
	return upsertIslandDelegationWithQueryer(ctx, tx, slot)
}

type islandDelegationExecer interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

func upsertIslandDelegationWithQueryer(ctx context.Context, execer islandDelegationExecer, slot *IslandDelegation) error {
	_, err := execer.Exec(ctx, `
INSERT INTO island_delegations
	(commander_id, build_id, area_id, ship_id, has_role, reward_ready, formula_id, max_times, main_num, other_num, extra_main_num, extra_other_num, get_times, start_time, cost_time_list, speed_time, times_extra, recover_time, add_exp, return_num, pt_award)
VALUES
	($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21)
ON CONFLICT (commander_id, build_id, area_id)
DO UPDATE SET
	ship_id = EXCLUDED.ship_id,
	has_role = EXCLUDED.has_role,
	reward_ready = EXCLUDED.reward_ready,
	formula_id = EXCLUDED.formula_id,
	max_times = EXCLUDED.max_times,
	main_num = EXCLUDED.main_num,
	other_num = EXCLUDED.other_num,
	extra_main_num = EXCLUDED.extra_main_num,
	extra_other_num = EXCLUDED.extra_other_num,
	get_times = EXCLUDED.get_times,
	start_time = EXCLUDED.start_time,
	cost_time_list = EXCLUDED.cost_time_list,
	speed_time = EXCLUDED.speed_time,
	times_extra = EXCLUDED.times_extra,
	recover_time = EXCLUDED.recover_time,
	add_exp = EXCLUDED.add_exp,
	return_num = EXCLUDED.return_num,
	pt_award = EXCLUDED.pt_award
`,
		int64(slot.CommanderID),
		int64(slot.BuildID),
		int64(slot.AreaID),
		int64(slot.ShipID),
		slot.HasRole,
		slot.RewardReady,
		int64(slot.FormulaID),
		int64(slot.MaxTimes),
		int64(slot.MainNum),
		int64(slot.OtherNum),
		int64(slot.ExtraMainNum),
		int64(slot.ExtraOtherNum),
		int64(slot.GetTimes),
		int64(slot.StartTime),
		mustJSON(slot.CostTimeList),
		int64(slot.SpeedTime),
		mustJSON(slot.TimesExtra),
		int64(slot.RecoverTime),
		int64(slot.AddExp),
		int64(slot.ReturnNum),
		int64(slot.PTAward),
	)
	return err
}

func GetIslandDelegation(commanderID uint32, buildID uint32, areaID uint32) (*IslandDelegation, error) {
	ctx := context.Background()
	return queryIslandDelegation(ctx, db.DefaultStore.Pool, commanderID, buildID, areaID, false)
}

func GetIslandDelegationForUpdateTx(ctx context.Context, tx pgx.Tx, commanderID uint32, buildID uint32, areaID uint32) (*IslandDelegation, error) {
	return queryIslandDelegation(ctx, tx, commanderID, buildID, areaID, true)
}

func GetIslandDelegationByAreaForUpdateTx(ctx context.Context, tx pgx.Tx, commanderID uint32, areaID uint32) (*IslandDelegation, error) {
	query := `
SELECT commander_id, build_id, area_id, ship_id, has_role, reward_ready, formula_id, max_times, main_num, other_num, extra_main_num, extra_other_num, get_times, start_time, cost_time_list, speed_time, times_extra, recover_time, add_exp, return_num, pt_award
FROM island_delegations
WHERE commander_id = $1 AND area_id = $2
ORDER BY build_id
LIMIT 1
FOR UPDATE
`
	return scanIslandDelegation(queryerRowFunc(func(ctx context.Context, sql string, args ...any) pgx.Row {
		return tx.QueryRow(ctx, sql, args...)
	}), ctx, query, int64(commanderID), int64(areaID))
}

type islandDelegationQueryer interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

func queryIslandDelegation(ctx context.Context, queryer islandDelegationQueryer, commanderID uint32, buildID uint32, areaID uint32, forUpdate bool) (*IslandDelegation, error) {
	query := `
SELECT commander_id, build_id, area_id, ship_id, has_role, reward_ready, formula_id, max_times, main_num, other_num, extra_main_num, extra_other_num, get_times, start_time, cost_time_list, speed_time, times_extra, recover_time, add_exp, return_num, pt_award
FROM island_delegations
WHERE commander_id = $1 AND build_id = $2 AND area_id = $3
`
	if forUpdate {
		query += " FOR UPDATE"
	}
	return scanIslandDelegation(queryer, ctx, query, int64(commanderID), int64(buildID), int64(areaID))
}

type queryerRowFunc func(ctx context.Context, sql string, args ...any) pgx.Row

func (f queryerRowFunc) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	return f(ctx, sql, args...)
}

func scanIslandDelegation(queryer islandDelegationQueryer, ctx context.Context, query string, args ...any) (*IslandDelegation, error) {
	var (
		commanderIDRaw   int64
		buildIDRaw       int64
		areaIDRaw        int64
		shipIDRaw        int64
		formulaIDRaw     int64
		maxTimesRaw      int64
		mainNumRaw       int64
		otherNumRaw      int64
		extraMainNumRaw  int64
		extraOtherNumRaw int64
		getTimesRaw      int64
		startTimeRaw     int64
		costTimeListRaw  []byte
		speedTimeRaw     int64
		timesExtraRaw    []byte
		recoverTimeRaw   int64
		addExpRaw        int64
		returnNumRaw     int64
		ptAwardRaw       int64
		slot             IslandDelegation
	)
	err := queryer.QueryRow(ctx, query, args...).Scan(
		&commanderIDRaw,
		&buildIDRaw,
		&areaIDRaw,
		&shipIDRaw,
		&slot.HasRole,
		&slot.RewardReady,
		&formulaIDRaw,
		&maxTimesRaw,
		&mainNumRaw,
		&otherNumRaw,
		&extraMainNumRaw,
		&extraOtherNumRaw,
		&getTimesRaw,
		&startTimeRaw,
		&costTimeListRaw,
		&speedTimeRaw,
		&timesExtraRaw,
		&recoverTimeRaw,
		&addExpRaw,
		&returnNumRaw,
		&ptAwardRaw,
	)
	err = db.MapNotFound(err)
	if err != nil {
		return nil, err
	}
	slot.CommanderID = uint32(commanderIDRaw)
	slot.BuildID = uint32(buildIDRaw)
	slot.AreaID = uint32(areaIDRaw)
	slot.ShipID = uint32(shipIDRaw)
	slot.FormulaID = uint32(formulaIDRaw)
	slot.MaxTimes = uint32(maxTimesRaw)
	slot.MainNum = uint32(mainNumRaw)
	slot.OtherNum = uint32(otherNumRaw)
	slot.ExtraMainNum = uint32(extraMainNumRaw)
	slot.ExtraOtherNum = uint32(extraOtherNumRaw)
	slot.GetTimes = uint32(getTimesRaw)
	slot.StartTime = uint32(startTimeRaw)
	if len(costTimeListRaw) > 0 {
		if err := json.Unmarshal(costTimeListRaw, &slot.CostTimeList); err != nil {
			return nil, err
		}
	}
	slot.SpeedTime = uint32(speedTimeRaw)
	if len(timesExtraRaw) > 0 {
		if err := json.Unmarshal(timesExtraRaw, &slot.TimesExtra); err != nil {
			return nil, err
		}
	}
	slot.RecoverTime = uint32(recoverTimeRaw)
	slot.AddExp = uint32(addExpRaw)
	slot.ReturnNum = uint32(returnNumRaw)
	slot.PTAward = uint32(ptAwardRaw)
	return &slot, nil
}

func mustJSON(values []uint32) []byte {
	if len(values) == 0 {
		return []byte("[]")
	}
	data, err := json.Marshal(values)
	if err != nil {
		return []byte("[]")
	}
	return data
}

func ApplyIslandDelegationClaimTx(ctx context.Context, tx pgx.Tx, commanderID uint32, buildID uint32, areaID uint32, claimType uint32) (uint32, error) {
	row := tx.QueryRow(ctx, `
UPDATE island_delegations
SET
	reward_ready = false,
	main_num = 0,
	other_num = 0,
	extra_main_num = 0,
	extra_other_num = 0,
	get_times = CASE
		WHEN $4 = 1 THEN get_times + 1
		WHEN $4 = 2 THEN 0
		ELSE get_times
	END
WHERE commander_id = $1 AND build_id = $2 AND area_id = $3
RETURNING get_times
`, int64(commanderID), int64(buildID), int64(areaID), int64(claimType))
	var getTimesRaw int64
	err := row.Scan(&getTimesRaw)
	err = db.MapNotFound(err)
	if err != nil {
		return 0, err
	}
	return uint32(getTimesRaw), nil
}
