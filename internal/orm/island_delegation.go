package orm

import (
	"context"

	"github.com/jackc/pgx/v5"

	"github.com/ggmolly/belfast/internal/db"
)

type IslandDelegation struct {
	CommanderID   uint32 `gorm:"primaryKey;column:commander_id"`
	BuildID       uint32 `gorm:"primaryKey;column:build_id"`
	AreaID        uint32 `gorm:"primaryKey;column:area_id"`
	HasRole       bool   `gorm:"column:has_role"`
	RewardReady   bool   `gorm:"column:reward_ready"`
	FormulaID     uint32 `gorm:"column:formula_id"`
	MainNum       uint32 `gorm:"column:main_num"`
	OtherNum      uint32 `gorm:"column:other_num"`
	ExtraMainNum  uint32 `gorm:"column:extra_main_num"`
	ExtraOtherNum uint32 `gorm:"column:extra_other_num"`
	GetTimes      uint32 `gorm:"column:get_times"`
	PTAward       uint32 `gorm:"column:pt_award"`
}

func (IslandDelegation) TableName() string {
	return "island_delegations"
}

func UpsertIslandDelegation(slot *IslandDelegation) error {
	ctx := context.Background()
	_, err := db.DefaultStore.Pool.Exec(ctx, `
INSERT INTO island_delegations
	(commander_id, build_id, area_id, has_role, reward_ready, formula_id, main_num, other_num, extra_main_num, extra_other_num, get_times, pt_award)
VALUES
	($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
ON CONFLICT (commander_id, build_id, area_id)
DO UPDATE SET
	has_role = EXCLUDED.has_role,
	reward_ready = EXCLUDED.reward_ready,
	formula_id = EXCLUDED.formula_id,
	main_num = EXCLUDED.main_num,
	other_num = EXCLUDED.other_num,
	extra_main_num = EXCLUDED.extra_main_num,
	extra_other_num = EXCLUDED.extra_other_num,
	get_times = EXCLUDED.get_times,
	pt_award = EXCLUDED.pt_award
`,
		int64(slot.CommanderID),
		int64(slot.BuildID),
		int64(slot.AreaID),
		slot.HasRole,
		slot.RewardReady,
		int64(slot.FormulaID),
		int64(slot.MainNum),
		int64(slot.OtherNum),
		int64(slot.ExtraMainNum),
		int64(slot.ExtraOtherNum),
		int64(slot.GetTimes),
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

type islandDelegationQueryer interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

func queryIslandDelegation(ctx context.Context, queryer islandDelegationQueryer, commanderID uint32, buildID uint32, areaID uint32, forUpdate bool) (*IslandDelegation, error) {
	query := `
SELECT commander_id, build_id, area_id, has_role, reward_ready, formula_id, main_num, other_num, extra_main_num, extra_other_num, get_times, pt_award
FROM island_delegations
WHERE commander_id = $1 AND build_id = $2 AND area_id = $3
`
	if forUpdate {
		query += " FOR UPDATE"
	}
	var (
		commanderIDRaw   int64
		buildIDRaw       int64
		areaIDRaw        int64
		formulaIDRaw     int64
		mainNumRaw       int64
		otherNumRaw      int64
		extraMainNumRaw  int64
		extraOtherNumRaw int64
		getTimesRaw      int64
		ptAwardRaw       int64
		slot             IslandDelegation
	)
	err := queryer.QueryRow(ctx, query, int64(commanderID), int64(buildID), int64(areaID)).Scan(
		&commanderIDRaw,
		&buildIDRaw,
		&areaIDRaw,
		&slot.HasRole,
		&slot.RewardReady,
		&formulaIDRaw,
		&mainNumRaw,
		&otherNumRaw,
		&extraMainNumRaw,
		&extraOtherNumRaw,
		&getTimesRaw,
		&ptAwardRaw,
	)
	err = db.MapNotFound(err)
	if err != nil {
		return nil, err
	}
	slot.CommanderID = uint32(commanderIDRaw)
	slot.BuildID = uint32(buildIDRaw)
	slot.AreaID = uint32(areaIDRaw)
	slot.FormulaID = uint32(formulaIDRaw)
	slot.MainNum = uint32(mainNumRaw)
	slot.OtherNum = uint32(otherNumRaw)
	slot.ExtraMainNum = uint32(extraMainNumRaw)
	slot.ExtraOtherNum = uint32(extraOtherNumRaw)
	slot.GetTimes = uint32(getTimesRaw)
	slot.PTAward = uint32(ptAwardRaw)
	return &slot, nil
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
