package orm

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/ggmolly/belfast/internal/db"
	"github.com/ggmolly/belfast/internal/protobuf"
	"github.com/ggmolly/belfast/internal/rng"
	"google.golang.org/protobuf/proto"
)

const (
	commanderCreateMaterialCategory = "ShareCfg/commander_data_create_material.json"
	commanderDataTemplateCategory   = "ShareCfg/commander_data_template.json"
	gamesetCategory                 = "ShareCfg/gameset.json"
	commanderQuickFinishItemID      = 20010
	commanderQuickFinishUnitSec     = 1200
	defaultCommanderBoxCount        = 3
	maxCommanderCount               = 200
)

var commanderMeowRng = rng.NewLockedRand()

type CommanderMeow struct {
	ID          uint32
	CommanderID uint32
	TemplateID  uint32
	Level       uint32
	Exp         uint32
	IsLocked    uint32
	UsedPt      uint32
	CreatedAt   time.Time
}

func (CommanderMeow) TableName() string {
	return "commander_meows"
}

type CommanderBox struct {
	CommanderID uint32
	BoxID       uint32
	PoolID      uint32
	BeginTime   uint32
	FinishTime  uint32
}

func (CommanderBox) TableName() string {
	return "commander_boxes"
}

type CommanderCreateMaterialConfig struct {
	ID      uint32 `json:"id"`
	UseItem uint32 `json:"use_item"`
	Number1 uint32 `json:"number_1"`
}

type CommanderDataTemplateConfig struct {
	ID        uint32 `json:"id"`
	GroupType uint32 `json:"group_type"`
	Rarity    uint32 `json:"rarity"`
	Exp       uint32 `json:"exp"`
	ExpCost   uint32 `json:"exp_cost"`
}

type gamesetEntry struct {
	KeyValue uint32 `json:"key_value"`
}

type CommanderQuickFinishCounts struct {
	ItemCnt   uint32
	FinishCnt uint32
	AffectCnt uint32
}

func ListCommanderMeows(commanderID uint32) ([]CommanderMeow, error) {
	ctx := context.Background()
	rows, err := db.DefaultStore.Pool.Query(ctx, `
SELECT id, commander_id, template_id, level, exp, is_locked, used_pt, created_at
FROM commander_meows
WHERE commander_id = $1
ORDER BY id ASC
`, int64(commanderID))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make([]CommanderMeow, 0)
	for rows.Next() {
		var meow CommanderMeow
		var id, commander, templateID, level, exp, isLocked, usedPt int64
		if err := rows.Scan(&id, &commander, &templateID, &level, &exp, &isLocked, &usedPt, &meow.CreatedAt); err != nil {
			return nil, err
		}
		meow.ID = uint32(id)
		meow.CommanderID = uint32(commander)
		meow.TemplateID = uint32(templateID)
		meow.Level = uint32(level)
		meow.Exp = uint32(exp)
		meow.IsLocked = uint32(isLocked)
		meow.UsedPt = uint32(usedPt)
		result = append(result, meow)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return result, nil
}

func GetCommanderMeow(commanderID uint32, meowID uint32) (*CommanderMeow, error) {
	ctx := context.Background()
	var meow CommanderMeow
	var id, commander, templateID, level, exp, isLocked, usedPt int64
	err := db.DefaultStore.Pool.QueryRow(ctx, `
SELECT id, commander_id, template_id, level, exp, is_locked, used_pt, created_at
FROM commander_meows
WHERE commander_id = $1 AND id = $2
`, int64(commanderID), int64(meowID)).Scan(&id, &commander, &templateID, &level, &exp, &isLocked, &usedPt, &meow.CreatedAt)
	err = db.MapNotFound(err)
	if err != nil {
		return nil, err
	}
	meow.ID = uint32(id)
	meow.CommanderID = uint32(commander)
	meow.TemplateID = uint32(templateID)
	meow.Level = uint32(level)
	meow.Exp = uint32(exp)
	meow.IsLocked = uint32(isLocked)
	meow.UsedPt = uint32(usedPt)
	return &meow, nil
}

func CreateCommanderMeowTx(ctx context.Context, tx pgx.Tx, commanderID uint32, templateID uint32) (*CommanderMeow, error) {
	var meow CommanderMeow
	var id, commander, level, exp, isLocked, usedPt int64
	err := tx.QueryRow(ctx, `
INSERT INTO commander_meows (commander_id, template_id, level, exp, is_locked, used_pt)
VALUES ($1, $2, 1, 0, 0, 0)
RETURNING id, commander_id, level, exp, is_locked, used_pt, created_at
`, int64(commanderID), int64(templateID)).Scan(&id, &commander, &level, &exp, &isLocked, &usedPt, &meow.CreatedAt)
	if err != nil {
		return nil, err
	}
	meow.ID = uint32(id)
	meow.CommanderID = uint32(commander)
	meow.TemplateID = templateID
	meow.Level = uint32(level)
	meow.Exp = uint32(exp)
	meow.IsLocked = uint32(isLocked)
	meow.UsedPt = uint32(usedPt)
	return &meow, nil
}

func DeleteCommanderMeowsTx(ctx context.Context, tx pgx.Tx, commanderID uint32, meowIDs []uint32) error {
	if len(meowIDs) == 0 {
		return nil
	}
	args := make([]int64, len(meowIDs))
	for i, id := range meowIDs {
		args[i] = int64(id)
	}
	_, err := tx.Exec(ctx, `
DELETE FROM commander_meows
WHERE commander_id = $1 AND id = ANY($2::bigint[])
`, int64(commanderID), args)
	return err
}

func UpdateCommanderMeowExpTx(ctx context.Context, tx pgx.Tx, commanderID uint32, meowID uint32, exp uint32) error {
	_, err := tx.Exec(ctx, `
UPDATE commander_meows
SET exp = $3
WHERE commander_id = $1 AND id = $2
`, int64(commanderID), int64(meowID), int64(exp))
	return err
}

func EnsureCommanderBoxes(commanderID uint32) ([]CommanderBox, error) {
	count := uint32(defaultCommanderBoxCount)
	if cfgCount, err := getGamesetNumber("commander_box_count"); err == nil && cfgCount > 0 && cfgCount <= 10 {
		count = cfgCount
	}
	ctx := context.Background()
	err := db.DefaultStore.WithPGXTx(ctx, func(tx pgx.Tx) error {
		for i := uint32(1); i <= count; i++ {
			if _, err := tx.Exec(ctx, `
INSERT INTO commander_boxes (commander_id, box_id, pool_id, begin_time, finish_time)
VALUES ($1, $2, 0, 0, 0)
ON CONFLICT (commander_id, box_id) DO NOTHING
`, int64(commanderID), int64(i)); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return ListCommanderBoxes(commanderID)
}

func ListCommanderBoxes(commanderID uint32) ([]CommanderBox, error) {
	ctx := context.Background()
	rows, err := db.DefaultStore.Pool.Query(ctx, `
SELECT commander_id, box_id, pool_id, begin_time, finish_time
FROM commander_boxes
WHERE commander_id = $1
ORDER BY box_id ASC
`, int64(commanderID))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	boxes := make([]CommanderBox, 0)
	for rows.Next() {
		var commander, boxID, poolID, beginTime, finishTime int64
		if err := rows.Scan(&commander, &boxID, &poolID, &beginTime, &finishTime); err != nil {
			return nil, err
		}
		boxes = append(boxes, CommanderBox{
			CommanderID: uint32(commander),
			BoxID:       uint32(boxID),
			PoolID:      uint32(poolID),
			BeginTime:   uint32(beginTime),
			FinishTime:  uint32(finishTime),
		})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return boxes, nil
}

func GetCommanderBox(commanderID uint32, boxID uint32) (*CommanderBox, error) {
	ctx := context.Background()
	var commander, box, poolID, beginTime, finishTime int64
	err := db.DefaultStore.Pool.QueryRow(ctx, `
SELECT commander_id, box_id, pool_id, begin_time, finish_time
FROM commander_boxes
WHERE commander_id = $1 AND box_id = $2
`, int64(commanderID), int64(boxID)).Scan(&commander, &box, &poolID, &beginTime, &finishTime)
	err = db.MapNotFound(err)
	if err != nil {
		return nil, err
	}
	return &CommanderBox{CommanderID: uint32(commander), BoxID: uint32(box), PoolID: uint32(poolID), BeginTime: uint32(beginTime), FinishTime: uint32(finishTime)}, nil
}

func UpsertCommanderBoxTx(ctx context.Context, tx pgx.Tx, box CommanderBox) error {
	_, err := tx.Exec(ctx, `
INSERT INTO commander_boxes (commander_id, box_id, pool_id, begin_time, finish_time)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (commander_id, box_id)
DO UPDATE SET pool_id = EXCLUDED.pool_id, begin_time = EXCLUDED.begin_time, finish_time = EXCLUDED.finish_time
`, int64(box.CommanderID), int64(box.BoxID), int64(box.PoolID), int64(box.BeginTime), int64(box.FinishTime))
	return err
}

func ToProtoCommanderBox(box CommanderBox) *protobuf.COMMANDERBOXINFO {
	return &protobuf.COMMANDERBOXINFO{
		Id:         proto.Uint32(box.BoxID),
		PoolId:     proto.Uint32(box.PoolID),
		BeginTime:  proto.Uint32(box.BeginTime),
		FinishTime: proto.Uint32(box.FinishTime),
	}
}

func ToProtoCommanderInfo(meow CommanderMeow) *protobuf.COMMANDERINFO {
	return &protobuf.COMMANDERINFO{
		Id:            proto.Uint32(meow.ID),
		TemplateId:    proto.Uint32(meow.TemplateID),
		Level:         proto.Uint32(meow.Level),
		Exp:           proto.Uint32(meow.Exp),
		IsLocked:      proto.Uint32(meow.IsLocked),
		Ability:       []uint32{},
		AbilityOrigin: []uint32{},
		AbilityTime:   proto.Uint32(0),
		Skill:         []*protobuf.SKILLINFO{},
		UsedPt:        proto.Uint32(meow.UsedPt),
		Name:          proto.String(""),
		RenameTime:    proto.Uint32(0),
	}
}

func GetCommanderCreateMaterialConfig(poolID uint32) (*CommanderCreateMaterialConfig, error) {
	entry, err := GetConfigEntry(commanderCreateMaterialCategory, strconv.FormatUint(uint64(poolID), 10))
	if err != nil {
		return nil, err
	}
	var cfg CommanderCreateMaterialConfig
	if err := json.Unmarshal(entry.Data, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func ListCommanderDataTemplateConfigs() ([]CommanderDataTemplateConfig, error) {
	entries, err := ListConfigEntries(commanderDataTemplateCategory)
	if err != nil {
		return nil, err
	}
	out := make([]CommanderDataTemplateConfig, 0, len(entries))
	for _, entry := range entries {
		var cfg CommanderDataTemplateConfig
		if err := json.Unmarshal(entry.Data, &cfg); err != nil {
			continue
		}
		out = append(out, cfg)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("commander templates unavailable")
	}
	return out, nil
}

func GetCommanderDataTemplateConfig(templateID uint32) (*CommanderDataTemplateConfig, error) {
	entry, err := GetConfigEntry(commanderDataTemplateCategory, strconv.FormatUint(uint64(templateID), 10))
	if err != nil {
		return nil, err
	}
	var cfg CommanderDataTemplateConfig
	if err := json.Unmarshal(entry.Data, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func RollCommanderTemplateForPool(poolID uint32) (uint32, error) {
	templates, err := ListCommanderDataTemplateConfigs()
	if err != nil {
		return 0, err
	}
	targetRarity := uint32(3)
	switch poolID {
	case 1:
		targetRarity = 5
	case 2:
		targetRarity = 4
	case 3:
		targetRarity = 3
	}
	matches := make([]uint32, 0, len(templates))
	for _, tpl := range templates {
		if tpl.Rarity == targetRarity {
			matches = append(matches, tpl.ID)
		}
	}
	if len(matches) > 0 {
		return matches[commanderMeowRng.IntN(len(matches))], nil
	}
	return templates[0].ID, nil
}

func getGamesetNumber(key string) (uint32, error) {
	entry, err := GetConfigEntry(gamesetCategory, key)
	if err != nil {
		return 0, err
	}
	var data gamesetEntry
	if err := json.Unmarshal(entry.Data, &data); err != nil {
		return 0, err
	}
	return data.KeyValue, nil
}

func GetCommanderUpgradeRates() (sameRate uint32, skillExp uint32, err error) {
	sameRate, err = getGamesetNumber("commander_exp_same_rate")
	if err != nil {
		sameRate = 12000
	}
	skillExp, err = getGamesetNumber("commander_skill_exp")
	if err != nil {
		skillExp = 1
	}
	return sameRate, skillExp, nil
}

func ComputeCommanderQuickFinishCounts(boxes []CommanderBox, now uint32, availableItems uint32) CommanderQuickFinishCounts {
	counts := CommanderQuickFinishCounts{}
	remainingItems := availableItems
	for _, box := range boxes {
		if box.PoolID == 0 || box.FinishTime <= now {
			continue
		}
		if remainingItems == 0 {
			break
		}
		remaining := box.FinishTime - now
		needed := uint32(math.Ceil(float64(remaining) / float64(commanderQuickFinishUnitSec)))
		if needed == 0 {
			continue
		}
		if needed <= remainingItems {
			counts.ItemCnt += needed
			counts.FinishCnt++
			counts.AffectCnt++
			remainingItems -= needed
			continue
		}
		counts.ItemCnt += remainingItems
		counts.AffectCnt++
		remainingItems = 0
		break
	}
	return counts
}

func ApplyCommanderQuickFinishTx(ctx context.Context, tx pgx.Tx, boxes []CommanderBox, now uint32, itemCount uint32) ([]CommanderBox, error) {
	remainingItems := itemCount
	updated := make([]CommanderBox, len(boxes))
	copy(updated, boxes)
	for i := range updated {
		if remainingItems == 0 {
			break
		}
		box := updated[i]
		if box.PoolID == 0 || box.FinishTime <= now {
			continue
		}
		remaining := box.FinishTime - now
		needed := uint32(math.Ceil(float64(remaining) / float64(commanderQuickFinishUnitSec)))
		if needed == 0 {
			continue
		}
		spend := needed
		if spend > remainingItems {
			spend = remainingItems
		}
		reduction := spend * commanderQuickFinishUnitSec
		if reduction >= remaining {
			box.FinishTime = now
		} else {
			box.FinishTime = now + (remaining - reduction)
		}
		if err := UpsertCommanderBoxTx(ctx, tx, box); err != nil {
			return nil, err
		}
		updated[i] = box
		remainingItems -= spend
	}
	return updated, nil
}

func IsCommanderInAnyFleet(commander *Commander, meowID uint32) bool {
	for _, fleet := range commander.Fleets {
		for _, value := range fleet.MeowfficerList {
			if uint32(value) == meowID {
				return true
			}
		}
	}
	return false
}

func UpdateFleetMeowfficerSlot(commander *Commander, groupID uint32, pos uint32, meowID uint32) error {
	fleet, ok := commander.FleetsMap[groupID]
	if !ok {
		return db.ErrNotFound
	}
	if pos == 0 || pos > 2 {
		return fmt.Errorf("invalid position")
	}
	for _, f := range commander.Fleets {
		for idx, value := range f.MeowfficerList {
			if uint32(value) != meowID || meowID == 0 {
				continue
			}
			if f.GameID != groupID || uint32(idx+1) != pos {
				return fmt.Errorf("commander already equipped")
			}
		}
	}
	for len(fleet.MeowfficerList) < 2 {
		fleet.MeowfficerList = append(fleet.MeowfficerList, 0)
	}
	fleet.MeowfficerList[pos-1] = int64(meowID)
	jsonList, err := json.Marshal(fleet.MeowfficerList)
	if err != nil {
		return err
	}
	ctx := context.Background()
	_, err = db.DefaultStore.Pool.Exec(ctx, `
UPDATE fleets
SET meowfficer_list = $2
WHERE id = $1
`, int64(fleet.ID), jsonList)
	if err != nil {
		return err
	}
	for i := range commander.Fleets {
		if commander.Fleets[i].ID == fleet.ID {
			commander.Fleets[i].MeowfficerList = fleet.MeowfficerList
			break
		}
	}
	return nil
}
