package answer

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"
	"google.golang.org/protobuf/proto"

	"github.com/ggmolly/belfast/internal/consts"
	"github.com/ggmolly/belfast/internal/orm"
	"github.com/ggmolly/belfast/internal/protobuf"
)

const (
	coloringResultSuccess = uint32(0)
	coloringResultFailure = uint32(1)

	coloringTemplateCategory = "ShareCfg/activity_coloring_template.json"
)

var errColoringInsufficientPaint = errors.New("insufficient coloring paint")

type coloringActivityPage struct {
	PageID     uint32
	RewardSpec []uint32
}

type coloringTemplate struct {
	ID          uint32     `json:"id"`
	Blank       uint32     `json:"blank"`
	ColorIDList []uint32   `json:"color_id_list"`
	Cells       [][]uint32 `json:"cells"`
}

type coloringCellTemplate struct {
	Row      uint32
	Column   uint32
	Required uint32
}

func loadColoringActivityPages(actID uint32) ([]coloringActivityPage, error) {
	activity, err := loadActivityTemplate(actID)
	if err != nil {
		return nil, err
	}
	if activity.Type != activityTypeColoringAlpha {
		return nil, fmt.Errorf("unexpected coloring activity type: %d", activity.Type)
	}

	var rawEntries []json.RawMessage
	if err := json.Unmarshal(activity.ConfigData, &rawEntries); err != nil {
		return nil, err
	}

	pages := make([]coloringActivityPage, 0, len(rawEntries))
	for _, raw := range rawEntries {
		var arrayValues []uint32
		if err := json.Unmarshal(raw, &arrayValues); err == nil && len(arrayValues) > 0 {
			entry := coloringActivityPage{PageID: arrayValues[0]}
			if len(arrayValues) > 1 {
				entry.RewardSpec = append([]uint32{}, arrayValues[1:]...)
			}
			pages = append(pages, entry)
			continue
		}

		var single uint32
		if err := json.Unmarshal(raw, &single); err == nil && single > 0 {
			pages = append(pages, coloringActivityPage{PageID: single})
		}
	}
	return pages, nil
}

func loadColoringTemplate(pageID uint32) (coloringTemplate, error) {
	entry, err := orm.GetConfigEntry(coloringTemplateCategory, strconv.FormatUint(uint64(pageID), 10))
	if err != nil {
		return coloringTemplate{}, err
	}
	template := coloringTemplate{}
	if err := json.Unmarshal(entry.Data, &template); err != nil {
		return coloringTemplate{}, err
	}
	return template, nil
}

func buildColoringCellTemplateLookup(template coloringTemplate) map[string]coloringCellTemplate {
	lookup := make(map[string]coloringCellTemplate, len(template.Cells))
	for _, raw := range template.Cells {
		if len(raw) < 2 {
			continue
		}
		cell := coloringCellTemplate{Row: raw[0], Column: raw[1]}
		if len(raw) > 2 {
			cell.Required = raw[2]
		}
		lookup[coloringCellKey(cell.Row, cell.Column)] = cell
	}
	return lookup
}

func coloringCellKey(row uint32, column uint32) string {
	return fmt.Sprintf("%d_%d", row, column)
}

func getOrCreateColoringState(commanderID uint32, actID uint32) (*orm.CommanderColoringState, error) {
	now := uint32(time.Now().Unix())
	return orm.GetOrCreateCommanderColoringState(commanderID, actID, now)
}

func coloringIsPageClaimed(state *orm.CommanderColoringState, pageID uint32) bool {
	for i := range state.Awards {
		if state.Awards[i].PageID == pageID {
			return true
		}
	}
	return false
}

func coloringGetPageFills(state *orm.CommanderColoringState, pageID uint32) map[string]orm.ColoringCellState {
	fills := make(map[string]orm.ColoringCellState)
	for i := range state.Cells {
		cell := state.Cells[i]
		if cell.PageID != pageID {
			continue
		}
		fills[coloringCellKey(cell.Row, cell.Column)] = cell
	}
	return fills
}

func coloringSetCell(state *orm.CommanderColoringState, pageID uint32, row uint32, column uint32, color uint32) {
	key := coloringCellKey(row, column)
	for i := range state.Cells {
		cell := state.Cells[i]
		if cell.PageID != pageID {
			continue
		}
		if coloringCellKey(cell.Row, cell.Column) != key {
			continue
		}
		if color == 0 {
			state.Cells = append(state.Cells[:i], state.Cells[i+1:]...)
			return
		}
		state.Cells[i].Color = color
		return
	}
	if color == 0 {
		return
	}
	state.Cells = append(state.Cells, orm.ColoringCellState{PageID: pageID, Row: row, Column: column, Color: color})
}

func coloringClearPage(state *orm.CommanderColoringState, pageID uint32) {
	if len(state.Cells) == 0 {
		return
	}
	next := make([]orm.ColoringCellState, 0, len(state.Cells))
	for i := range state.Cells {
		if state.Cells[i].PageID == pageID {
			continue
		}
		next = append(next, state.Cells[i])
	}
	state.Cells = next
}

func coloringIsPageComplete(template coloringTemplate, fills map[string]orm.ColoringCellState) bool {
	if template.Blank == 1 {
		return true
	}
	lookup := buildColoringCellTemplateLookup(template)
	for key := range lookup {
		if _, ok := fills[key]; !ok {
			return false
		}
	}
	return true
}

func coloringCurrentPageID(state *orm.CommanderColoringState, pages []coloringActivityPage) uint32 {
	if len(pages) == 0 {
		return 0
	}
	current := pages[0].PageID
	for idx := range pages {
		if coloringIsPageClaimed(state, pages[idx].PageID) {
			if idx+1 < len(pages) {
				current = pages[idx+1].PageID
			} else {
				current = pages[idx].PageID
			}
			continue
		}
		current = pages[idx].PageID
		break
	}
	return current
}

func coloringResolveClaimDrops(spec []uint32) []*protobuf.DROPINFO {
	if len(spec) == 0 {
		return []*protobuf.DROPINFO{}
	}
	if len(spec) == 1 {
		if spec[0] == 0 {
			return []*protobuf.DROPINFO{}
		}
		return []*protobuf.DROPINFO{newDropInfo(consts.DROP_TYPE_ITEM, spec[0], 1)}
	}
	if len(spec) >= 3 {
		if spec[2] == 0 {
			return []*protobuf.DROPINFO{}
		}
		return []*protobuf.DROPINFO{newDropInfo(spec[0], spec[1], spec[2])}
	}
	if spec[0] == 0 || spec[1] == 0 {
		return []*protobuf.DROPINFO{}
	}
	return []*protobuf.DROPINFO{newDropInfo(spec[0], spec[1], 1)}
}

func coloringDropsToState(drops []*protobuf.DROPINFO) []orm.ColoringDropState {
	out := make([]orm.ColoringDropState, 0, len(drops))
	for _, drop := range drops {
		if drop == nil {
			continue
		}
		out = append(out, orm.ColoringDropState{Type: drop.GetType(), ID: drop.GetId(), Number: drop.GetNumber()})
	}
	return out
}

func coloringDropsFromState(drops []orm.ColoringDropState) []*protobuf.DROPINFO {
	out := make([]*protobuf.DROPINFO, 0, len(drops))
	for i := range drops {
		drop := drops[i]
		out = append(out, &protobuf.DROPINFO{Type: proto.Uint32(drop.Type), Id: proto.Uint32(drop.ID), Number: proto.Uint32(drop.Number)})
	}
	return out
}

func coloringBuildCellListForPage(state *orm.CommanderColoringState, pageID uint32) []*protobuf.CELLSINFO {
	list := make([]*protobuf.CELLSINFO, 0)
	for i := range state.Cells {
		cell := state.Cells[i]
		if cell.PageID != pageID {
			continue
		}
		list = append(list, &protobuf.CELLSINFO{Row: proto.Uint32(cell.Row), Column: proto.Uint32(cell.Column), Color: proto.Uint32(cell.Color)})
	}
	sort.Slice(list, func(i int, j int) bool {
		if list[i].GetRow() == list[j].GetRow() {
			return list[i].GetColumn() < list[j].GetColumn()
		}
		return list[i].GetRow() < list[j].GetRow()
	})
	return list
}

func coloringBuildAwardList(state *orm.CommanderColoringState) []*protobuf.AWARDINFO {
	list := make([]*protobuf.AWARDINFO, 0, len(state.Awards))
	for i := range state.Awards {
		award := state.Awards[i]
		drops := coloringDropsFromState(award.Drops)
		sort.Slice(drops, func(i int, j int) bool {
			if drops[i].GetType() == drops[j].GetType() {
				return drops[i].GetId() < drops[j].GetId()
			}
			return drops[i].GetType() < drops[j].GetType()
		})
		list = append(list, &protobuf.AWARDINFO{Id: proto.Uint32(award.PageID), AwardList: drops})
	}
	sort.Slice(list, func(i int, j int) bool {
		return list[i].GetId() < list[j].GetId()
	})
	return list
}

func coloringAddClaim(state *orm.CommanderColoringState, pageID uint32, drops []*protobuf.DROPINFO) {
	for i := range state.Awards {
		if state.Awards[i].PageID == pageID {
			state.Awards[i].Drops = coloringDropsToState(drops)
			return
		}
	}
	state.Awards = append(state.Awards, orm.ColoringAwardState{PageID: pageID, Drops: coloringDropsToState(drops)})
}

func coloringBuildColorList(clientColorCounts map[uint32]uint32) []*protobuf.COLORINFO {
	ids := make([]uint32, 0, len(clientColorCounts))
	for itemID := range clientColorCounts {
		ids = append(ids, itemID)
	}
	sort.Slice(ids, func(i int, j int) bool { return ids[i] < ids[j] })
	list := make([]*protobuf.COLORINFO, 0, len(ids))
	for _, itemID := range ids {
		list = append(list, &protobuf.COLORINFO{Id: proto.Uint32(itemID), Number: proto.Uint32(clientColorCounts[itemID])})
	}
	return list
}

func coloringApplyDropsTx(ctx context.Context, tx pgx.Tx, clientCommanderID uint32, drops []*protobuf.DROPINFO) error {
	dropMap := make(map[string]*protobuf.DROPINFO, len(drops))
	for _, drop := range drops {
		if drop == nil {
			continue
		}
		key := fmt.Sprintf("%d_%d", drop.GetType(), drop.GetId())
		if existing := dropMap[key]; existing != nil {
			existing.Number = proto.Uint32(existing.GetNumber() + drop.GetNumber())
			continue
		}
		dropMap[key] = newDropInfo(drop.GetType(), drop.GetId(), drop.GetNumber())
	}
	for _, drop := range dropMap {
		switch drop.GetType() {
		case consts.DROP_TYPE_RESOURCE:
			if _, err := tx.Exec(ctx, `
INSERT INTO owned_resources (commander_id, resource_id, amount)
VALUES ($1, $2, $3)
ON CONFLICT (commander_id, resource_id)
DO UPDATE SET amount = owned_resources.amount + EXCLUDED.amount
`, int64(clientCommanderID), int64(drop.GetId()), int64(drop.GetNumber())); err != nil {
				return err
			}
		case consts.DROP_TYPE_ITEM:
			if _, err := tx.Exec(ctx, `
INSERT INTO commander_items (commander_id, item_id, count)
VALUES ($1, $2, $3)
ON CONFLICT (commander_id, item_id)
DO UPDATE SET count = commander_items.count + EXCLUDED.count
`, int64(clientCommanderID), int64(drop.GetId()), int64(drop.GetNumber())); err != nil {
				return err
			}
		case consts.DROP_TYPE_VITEM:
			continue
		default:
			return fmt.Errorf("unsupported coloring drop type %d", drop.GetType())
		}
	}
	return nil
}
