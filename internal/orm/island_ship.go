package orm

import (
	"context"
	"encoding/json"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/ggmolly/belfast/internal/db"
)

type IslandShipBuff struct {
	ID        uint32 `json:"id"`
	StartTime uint32 `json:"start_time"`
}

type IslandShipAttr struct {
	ID    uint32 `json:"id"`
	Value uint32 `json:"value"`
}

type IslandShip struct {
	CommanderID  uint32 `gorm:"primaryKey;column:commander_id"`
	ShipID       uint32 `gorm:"primaryKey;column:ship_id"`
	Level        uint32 `gorm:"column:level"`
	Exp          uint32 `gorm:"column:exp"`
	BreakLv      uint32 `gorm:"column:break_lv"`
	SkillLv      uint32 `gorm:"column:skill_lv"`
	Power        uint32 `gorm:"column:power"`
	RecoverTime  uint32 `gorm:"column:recover_time"`
	UpLimitState uint32 `gorm:"column:up_limit_state"`
	CurSkinID    uint32 `gorm:"column:cur_skin_id"`
	ExtraAttrs   []IslandShipAttr
	Buffs        []IslandShipBuff
	CanFollow    bool `gorm:"column:can_follow"`
}

func (IslandShip) TableName() string {
	return "island_ships"
}

func UpsertIslandShip(ship *IslandShip) error {
	return upsertIslandShip(context.Background(), db.DefaultStore.Pool, ship)
}

func UpsertIslandShipTx(ctx context.Context, tx pgx.Tx, ship *IslandShip) error {
	return upsertIslandShip(ctx, tx, ship)
}

type islandShipExecer interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

func upsertIslandShip(ctx context.Context, execer islandShipExecer, ship *IslandShip) error {
	if ship.Level == 0 {
		ship.Level = 1
	}
	if ship.BreakLv == 0 {
		ship.BreakLv = 1
	}
	if ship.SkillLv == 0 {
		ship.SkillLv = 1
	}

	extraAttrs := ship.ExtraAttrs
	if extraAttrs == nil {
		extraAttrs = []IslandShipAttr{}
	}
	buffs := ship.Buffs
	if buffs == nil {
		buffs = []IslandShipBuff{}
	}
	extraJSON, err := json.Marshal(extraAttrs)
	if err != nil {
		return err
	}
	buffsJSON, err := json.Marshal(buffs)
	if err != nil {
		return err
	}

	_, err = execer.Exec(ctx, `
INSERT INTO island_ships (
	commander_id, ship_id, level, exp, break_lv, skill_lv, power, recover_time, up_limit_state, cur_skin_id, extra_attr, buffs, can_follow
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
ON CONFLICT (commander_id, ship_id)
DO UPDATE SET
	level = EXCLUDED.level,
	exp = EXCLUDED.exp,
	break_lv = EXCLUDED.break_lv,
	skill_lv = EXCLUDED.skill_lv,
	power = EXCLUDED.power,
	recover_time = EXCLUDED.recover_time,
	up_limit_state = EXCLUDED.up_limit_state,
	cur_skin_id = EXCLUDED.cur_skin_id,
	extra_attr = EXCLUDED.extra_attr,
	buffs = EXCLUDED.buffs,
	can_follow = EXCLUDED.can_follow
`,
		int64(ship.CommanderID),
		int64(ship.ShipID),
		int64(ship.Level),
		int64(ship.Exp),
		int64(ship.BreakLv),
		int64(ship.SkillLv),
		int64(ship.Power),
		int64(ship.RecoverTime),
		int64(ship.UpLimitState),
		int64(ship.CurSkinID),
		extraJSON,
		buffsJSON,
		ship.CanFollow,
	)
	return err
}

func GetIslandShip(commanderID uint32, shipID uint32) (*IslandShip, error) {
	ctx := context.Background()
	return queryIslandShip(ctx, db.DefaultStore.Pool, commanderID, shipID, false)
}

func GetIslandShipForUpdateTx(ctx context.Context, tx pgx.Tx, commanderID uint32, shipID uint32) (*IslandShip, error) {
	return queryIslandShip(ctx, tx, commanderID, shipID, true)
}

func ListIslandShips(commanderID uint32) ([]IslandShip, error) {
	rows, err := db.DefaultStore.Pool.Query(context.Background(), `
SELECT commander_id, ship_id, level, exp, break_lv, skill_lv, power, recover_time, up_limit_state, cur_skin_id, extra_attr, buffs, can_follow
FROM island_ships
WHERE commander_id = $1
ORDER BY ship_id
`, int64(commanderID))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	ships := make([]IslandShip, 0)
	for rows.Next() {
		ship, err := scanIslandShip(rows)
		if err != nil {
			return nil, err
		}
		ships = append(ships, *ship)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return ships, nil
}

type islandShipQueryer interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

func queryIslandShip(ctx context.Context, queryer islandShipQueryer, commanderID uint32, shipID uint32, forUpdate bool) (*IslandShip, error) {
	query := `
SELECT commander_id, ship_id, level, exp, break_lv, skill_lv, power, recover_time, up_limit_state, cur_skin_id, extra_attr, buffs, can_follow
FROM island_ships
WHERE commander_id = $1 AND ship_id = $2
`
	if forUpdate {
		query += " FOR UPDATE"
	}

	var row islandShipScanner
	err := queryer.QueryRow(ctx, query, int64(commanderID), int64(shipID)).Scan(
		&row.CommanderIDRaw,
		&row.ShipIDRaw,
		&row.LevelRaw,
		&row.ExpRaw,
		&row.BreakLvRaw,
		&row.SkillLvRaw,
		&row.PowerRaw,
		&row.RecoverTimeRaw,
		&row.UpLimitStateRaw,
		&row.CurSkinIDRaw,
		&row.ExtraJSON,
		&row.BuffsJSON,
		&row.CanFollow,
	)
	err = db.MapNotFound(err)
	if err != nil {
		return nil, err
	}
	return row.intoShip()
}

type islandShipScanner struct {
	CommanderIDRaw  int64
	ShipIDRaw       int64
	LevelRaw        int64
	ExpRaw          int64
	BreakLvRaw      int64
	SkillLvRaw      int64
	PowerRaw        int64
	RecoverTimeRaw  int64
	UpLimitStateRaw int64
	CurSkinIDRaw    int64
	ExtraJSON       []byte
	BuffsJSON       []byte
	CanFollow       bool
}

type islandShipRowScanner interface {
	Scan(dest ...any) error
}

func scanIslandShip(scanner islandShipRowScanner) (*IslandShip, error) {
	var row islandShipScanner
	if err := scanner.Scan(
		&row.CommanderIDRaw,
		&row.ShipIDRaw,
		&row.LevelRaw,
		&row.ExpRaw,
		&row.BreakLvRaw,
		&row.SkillLvRaw,
		&row.PowerRaw,
		&row.RecoverTimeRaw,
		&row.UpLimitStateRaw,
		&row.CurSkinIDRaw,
		&row.ExtraJSON,
		&row.BuffsJSON,
		&row.CanFollow,
	); err != nil {
		return nil, err
	}
	return row.intoShip()
}

func (r *islandShipScanner) intoShip() (*IslandShip, error) {
	ship := &IslandShip{
		CommanderID:  uint32(r.CommanderIDRaw),
		ShipID:       uint32(r.ShipIDRaw),
		Level:        uint32(r.LevelRaw),
		Exp:          uint32(r.ExpRaw),
		BreakLv:      uint32(r.BreakLvRaw),
		SkillLv:      uint32(r.SkillLvRaw),
		Power:        uint32(r.PowerRaw),
		RecoverTime:  uint32(r.RecoverTimeRaw),
		UpLimitState: uint32(r.UpLimitStateRaw),
		CurSkinID:    uint32(r.CurSkinIDRaw),
		CanFollow:    r.CanFollow,
	}
	if err := json.Unmarshal(r.ExtraJSON, &ship.ExtraAttrs); err != nil {
		return nil, err
	}
	if ship.ExtraAttrs == nil {
		ship.ExtraAttrs = []IslandShipAttr{}
	}
	if err := json.Unmarshal(r.BuffsJSON, &ship.Buffs); err != nil {
		return nil, err
	}
	if ship.Buffs == nil {
		ship.Buffs = []IslandShipBuff{}
	}
	return ship, nil
}

func IncrementIslandShipBreakoutTx(ctx context.Context, tx pgx.Tx, commanderID uint32, shipID uint32) error {
	_, err := tx.Exec(ctx, `
UPDATE island_ships
SET break_lv = break_lv + 1
WHERE commander_id = $1 AND ship_id = $2
`, int64(commanderID), int64(shipID))
	return err
}
