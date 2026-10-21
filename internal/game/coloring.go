package game

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand/v2"
	"slices"
	"time"

	"google.golang.org/protobuf/proto"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/db"
	"github.com/ggmolly/belfast/internal/gamedata"
	"github.com/ggmolly/belfast/internal/logger"
	"github.com/ggmolly/belfast/internal/protobuf"
)

// The coloring activity (ActivityConst.ACTIVITY_TYPE_COLORING_ALPHA,
// ColoringProxy, ColoringScene: An Shan's Drawing Book on the wiki): a row
// of pictures (activity_coloring_template, the activity's config_data
// pairs {picture, award id}) filled cell by cell with pigments, virtual
// items (virtual_type 4) handed out by the event's tasks. Every cell of a
// picture takes one pigment of its color (color_id_list[color]); a finished
// picture gives its award, and the next one opens the day after the
// previous one at the earliest (ColoringProxy:checkState: picture n opens
// on day n of the activity). A "blank" picture (the last, award 0) is
// painted freely in any of its colors, for nothing.
//
// The award ids are in no table: what each gives is server content,
// server/coloring_awards.json (from the wiki's event pages).

const activityTypeColoring = 43

// activity_coloring_template row
type coloringGroupConfig struct {
	ID          uint32      `json:"id"`
	Blank       uint32      `json:"blank"`
	Cells       [][3]uint32 `json:"cells"` // {row, column, color}
	ColorIDList []uint32    `json:"color_id_list"`
	Colors      [][]float64 `json:"colors"`
	Theme       [2]uint32   `json:"theme"` // the last row and column of a blank picture
}

// customised is ColorGroup:canBeCustomised: a blank picture.
func (group coloringGroupConfig) customised() bool { return group.Blank == 1 }

// coloringAward is a server/coloring_awards.json entry: fixed drops, and
// random ones drawn uniformly from a pool once per unit.
type coloringAward struct {
	Drops  [][3]uint32 `json:"drops"`
	Random []struct {
		Count uint32      `json:"count"`
		Pool  [][2]uint32 `json:"pool"`
	} `json:"random"`
}

var (
	coloringGroups map[uint32]coloringGroupConfig
	coloringAwards map[uint32]coloringAward
)

func loadColoringData() error {
	var err error
	if coloringGroups, err = gamedata.Table[coloringGroupConfig]("activity_coloring_template"); err != nil {
		return err
	}
	rows, err := serverFileByID("coloring_awards.json")
	if err != nil {
		return err
	}
	if rows == nil {
		logger.WithFields("Coloring").Warn("no server/coloring_awards.json: coloring pictures give nothing")
	}
	coloringAwards = make(map[uint32]coloringAward, len(rows))
	for id, raw := range rows {
		var award coloringAward
		if err := json.Unmarshal(raw, &award); err != nil {
			return fmt.Errorf("coloring_awards.json %d: %w", id, err)
		}
		coloringAwards[id] = award
	}
	return nil
}

// draw is what an award gives.
func (award coloringAward) draw() []*protobuf.DROPINFO {
	var drops []*protobuf.DROPINFO
	for _, drop := range award.Drops {
		drops = mergeDrop(drops, drop[0], drop[1], drop[2])
	}
	for _, random := range award.Random {
		if len(random.Pool) == 0 {
			continue
		}
		for range random.Count {
			pick := random.Pool[rand.IntN(len(random.Pool))]
			drops = mergeDrop(drops, pick[0], pick[1], 1)
		}
	}
	return drops
}

// coloringPicture is one of an activity's pictures, in config_data order.
type coloringPicture struct {
	config coloringGroupConfig
	award  uint32 // 0: none
}

// coloringActivity is a running coloring activity's pictures and the day
// it started (0 when it runs at the commander's own time: every picture
// then opens with the previous one).
type coloringActivity struct {
	id       uint32
	pictures []coloringPicture
	start    time.Time
}

// runningColoring is the coloring activity id when it runs for the
// commander at now, errRefused otherwise.
func (game *Game) runningColoring(ctx context.Context, commanderID, id uint32, now time.Time) (coloringActivity, error) {
	config, open, err := game.commanderActivity(ctx, commanderID, id, now)
	if err != nil {
		return coloringActivity{}, err
	}
	if !open || config.Type != activityTypeColoring {
		return coloringActivity{}, errRefused
	}
	var pairs [][]uint32
	if err := json.Unmarshal(config.ConfigData, &pairs); err != nil {
		return coloringActivity{}, fmt.Errorf("activity %d config_data: %w", id, err)
	}
	activity := coloringActivity{id: id}
	if _, timed := openActivity(id, now); timed {
		activity.start = config.Time.Start
	}
	for _, pair := range pairs {
		if len(pair) < 2 {
			return coloringActivity{}, fmt.Errorf("activity %d config_data: %v is no {picture, award}", id, pair)
		}
		group, ok := coloringGroups[pair[0]]
		if !ok {
			return coloringActivity{}, gamedata.Missing("activity_coloring_template", "picture %d of activity %d", pair[0], id)
		}
		activity.pictures = append(activity.pictures, coloringPicture{config: group, award: pair[1]})
	}
	return activity, nil
}

// coloringProgress is how far the commander got: the pictures awarded (by
// id) and the index of the one being painted (len(pictures) when all are
// awarded). Pictures are awarded in order, a blank one never is.
func coloringProgress(ctx context.Context, queries *db.Queries, commanderID uint32, activity coloringActivity) (map[uint32]db.Drops, int, error) {
	rows, err := queries.ListColoringAwards(ctx, db.ListColoringAwardsParams{CommanderID: commanderID, ActivityID: activity.id})
	if err != nil {
		return nil, 0, err
	}
	awarded := make(map[uint32]db.Drops, len(rows))
	for _, row := range rows {
		awarded[row.GroupID] = row.Drops
	}
	current := 0
	for current < len(activity.pictures) {
		if _, ok := awarded[activity.pictures[current].config.ID]; !ok {
			break
		}
		current++
	}
	return awarded, current, nil
}

// paintable is whether the commander can paint picture index at now:
// the one after the last awarded, from day index+1 of the activity
// (ColoringProxy:checkState, the first one at once).
func (activity coloringActivity) paintable(index, current int, now time.Time) bool {
	if index != current || index >= len(activity.pictures) {
		return false
	}
	if index == 0 || activity.start.IsZero() {
		return true
	}
	days := int(startOfDay(now.In(gamedata.Zone())).Sub(startOfDay(activity.start.In(gamedata.Zone()))) / day)
	return index < days+1
}

// picture is the index of picture id in the activity, -1 when it isn't one.
func (activity coloringActivity) picture(id uint32) int {
	return slices.IndexFunc(activity.pictures, func(picture coloringPicture) bool { return picture.config.ID == id })
}

// handleColoringFetch answers ColoringFetchCommand (CS_26008, sent by
// MainRequestActDataSequence while the activity runs) with SC_26001, read by
// ColoringProxy:netUpdateData: id is the picture being painted (the ones
// before it count as finished) and cell_list its cells, award_list the
// drops of each awarded picture, color_list the pigments owned and
// start_time the activity's start, from which the client counts the days
// opening the pictures. Built from the Lua alone.
func (game *Game) handleColoringFetch(client *connection.Client, req *protobuf.CS_26008) error {
	ctx := context.Background()
	activity, err := game.runningColoring(ctx, client.CommanderID, req.GetActId(), time.Now())
	if refused(err) {
		// nothing to show: no pictures for an id the client doesn't run
		return client.Send(&protobuf.SC_26001{Id: proto.Uint32(0), StartTime: proto.Uint32(0)})
	}
	if err != nil {
		return err
	}
	queries := game.DB.Queries
	awarded, current, err := coloringProgress(ctx, queries, client.CommanderID, activity)
	if err != nil {
		return err
	}
	reply := &protobuf.SC_26001{StartTime: proto.Uint32(0)}
	if !activity.start.IsZero() {
		reply.StartTime = proto.Uint32(uint32(activity.start.Unix()))
	}
	if len(activity.pictures) > 0 {
		picture := activity.pictures[min(current, len(activity.pictures)-1)].config.ID
		reply.Id = proto.Uint32(picture)
		cells, err := queries.ListColoringCells(ctx, db.ListColoringCellsParams{CommanderID: client.CommanderID, ActivityID: activity.id, GroupID: picture})
		if err != nil {
			return err
		}
		for _, cell := range cells {
			reply.CellList = append(reply.CellList, &protobuf.CELLSINFO{Row: proto.Uint32(cell.Row), Column: proto.Uint32(cell.Col), Color: proto.Uint32(cell.Color)})
		}
	} else {
		reply.Id = proto.Uint32(0)
	}
	pigments := map[uint32]bool{}
	for _, picture := range activity.pictures {
		if drops, ok := awarded[picture.config.ID]; ok {
			info := &protobuf.AWARDINFO{Id: proto.Uint32(picture.config.ID)}
			for _, drop := range drops {
				info.AwardList = append(info.AwardList, dropInfo(drop[0], drop[1], drop[2]))
			}
			reply.AwardList = append(reply.AwardList, info)
		}
		for _, item := range picture.config.ColorIDList {
			if pigments[item] {
				continue
			}
			pigments[item] = true
			amount, err := virtualItemAmount(ctx, queries, client.CommanderID, item)
			if err != nil {
				return err
			}
			reply.ColorList = append(reply.ColorList, &protobuf.COLORINFO{Id: proto.Uint32(item), Number: proto.Uint32(amount)})
		}
	}
	return client.Send(reply)
}

// virtualItemAmount is how many of a virtual item the commander has.
func virtualItemAmount(ctx context.Context, queries *db.Queries, commanderID, itemID uint32) (uint32, error) {
	amount, err := queries.GetVirtualItem(ctx, db.GetVirtualItemParams{CommanderID: commanderID, ItemID: itemID})
	if errors.Is(err, sql.ErrNoRows) {
		return 0, nil
	}
	return amount, err
}

// handleColoringCell answers ColoringCellCommand (CS_26004): paints cells
// of the picture being painted. In a picture, a cell takes the color the
// template gives it, once, for one pigment color_id_list[color]
// (ColoringScene:SearchValidDiagonalColoringCells, ColoringCellCommand
// takes them off the client's count); in a blank picture any cell within
// its theme takes any of its colors or 0 (erased), for nothing. The whole
// request is refused when a cell can't be painted or the pigments run out
// (coloring_cell error tip). Built from the Lua alone.
func (game *Game) handleColoringCell(client *connection.Client, req *protobuf.CS_26004) error {
	ctx := context.Background()
	now := time.Now()
	activity, err := game.runningColoring(ctx, client.CommanderID, req.GetActId(), now)
	if err == nil {
		err = game.tx(ctx, client, func(tx *Tx) error {
			_, current, err := coloringProgress(ctx, tx.Queries, client.CommanderID, activity)
			if err != nil {
				return err
			}
			index := activity.picture(req.GetId())
			if index < 0 || !activity.paintable(index, current, now) || len(req.GetCellList()) == 0 {
				return errRefused
			}
			group := activity.pictures[index].config
			key := db.ListColoringCellsParams{CommanderID: client.CommanderID, ActivityID: activity.id, GroupID: group.ID}
			filled, err := tx.ListColoringCells(ctx, key)
			if err != nil {
				return err
			}
			painted := make(map[[2]uint32]bool, len(filled))
			for _, cell := range filled {
				painted[[2]uint32{cell.Row, cell.Col}] = true
			}
			colors := make(map[[2]uint32]uint32, len(group.Cells))
			for _, cell := range group.Cells {
				colors[[2]uint32{cell[0], cell[1]}] = cell[2]
			}
			spent := map[uint32]uint32{}
			for _, cell := range req.GetCellList() {
				at, color := [2]uint32{cell.GetRow(), cell.GetColumn()}, cell.GetColor()
				if group.customised() {
					if at[0] > group.Theme[0] || at[1] > group.Theme[1] || int(color) > len(group.Colors) {
						return errRefused
					}
				} else {
					if want, ok := colors[at]; !ok || want != color || color == 0 || painted[at] || int(color) > len(group.ColorIDList) {
						return errRefused
					}
					painted[at] = true
					spent[group.ColorIDList[color-1]]++
				}
				if color == 0 {
					err = tx.DeleteColoringCell(ctx, db.DeleteColoringCellParams{CommanderID: client.CommanderID, ActivityID: activity.id, GroupID: group.ID, Row: at[0], Col: at[1]})
				} else {
					err = tx.SetColoringCell(ctx, db.SetColoringCellParams{CommanderID: client.CommanderID, ActivityID: activity.id, GroupID: group.ID, Row: at[0], Col: at[1], Color: color})
				}
				if err != nil {
					return err
				}
			}
			for item, count := range spent {
				n, err := tx.SpendVirtualItem(ctx, db.SpendVirtualItemParams{CommanderID: client.CommanderID, ItemID: item, Count: count})
				if err != nil {
					return err
				}
				if n == 0 {
					return ErrNotEnough
				}
			}
			return nil
		})
	}
	if err != nil && !refused(err) {
		return err
	}
	return client.Send(&protobuf.SC_26005{Result: result(err)})
}

// handleColoringClear answers ColoringClearCommand (CS_26006): erases a
// blank picture being painted, the only one the client offers it for
// (ColoringScene's erase-all button). Built from the Lua alone.
func (game *Game) handleColoringClear(client *connection.Client, req *protobuf.CS_26006) error {
	ctx := context.Background()
	now := time.Now()
	activity, err := game.runningColoring(ctx, client.CommanderID, req.GetActId(), now)
	if err == nil {
		err = game.tx(ctx, client, func(tx *Tx) error {
			_, current, err := coloringProgress(ctx, tx.Queries, client.CommanderID, activity)
			if err != nil {
				return err
			}
			index := activity.picture(req.GetId())
			if index < 0 || !activity.paintable(index, current, now) || !activity.pictures[index].config.customised() {
				return errRefused
			}
			return tx.ClearColoringCells(ctx, db.ClearColoringCellsParams{CommanderID: client.CommanderID, ActivityID: activity.id, GroupID: req.GetId()})
		})
	}
	if err != nil && !refused(err) {
		return err
	}
	return client.Send(&protobuf.SC_26007{Result: result(err)})
}

// handleColoringAchieve answers ColoringAchieveCommand (CS_26002, sent by
// ColoringMediator:tryColoringAchieve for a finished picture with an
// award): the picture's server/coloring_awards.json drops, once, when every
// one of its cells is painted. Built from the Lua alone; the drops are the
// wiki's.
func (game *Game) handleColoringAchieve(client *connection.Client, req *protobuf.CS_26002) error {
	ctx := context.Background()
	now := time.Now()
	reply := &protobuf.SC_26003{Result: result(nil)}
	activity, err := game.runningColoring(ctx, client.CommanderID, req.GetActId(), now)
	if err == nil {
		err = game.tx(ctx, client, func(tx *Tx) error {
			_, current, err := coloringProgress(ctx, tx.Queries, client.CommanderID, activity)
			if err != nil {
				return err
			}
			index := activity.picture(req.GetId())
			if index < 0 || index != current {
				return errRefused
			}
			picture := activity.pictures[index]
			if picture.award == 0 || picture.config.customised() {
				return errRefused
			}
			filled, err := tx.CountColoringCells(ctx, db.CountColoringCellsParams{CommanderID: client.CommanderID, ActivityID: activity.id, GroupID: picture.config.ID})
			if err != nil {
				return err
			}
			if int(filled) < len(picture.config.Cells) {
				return errRefused
			}
			award, ok := coloringAwards[picture.award]
			if !ok {
				logger.WithFields("Coloring", logger.FieldValue("award", picture.award)).Warn("no coloring_awards.json entry: the picture gives nothing")
			}
			reply.DropList = award.draw()
			stored := make(db.Drops, 0, len(reply.DropList))
			for _, drop := range reply.DropList {
				stored = append(stored, [3]uint32{drop.GetType(), drop.GetId(), drop.GetNumber()})
			}
			added, err := tx.AddColoringAward(ctx, db.AddColoringAwardParams{CommanderID: client.CommanderID, ActivityID: activity.id, GroupID: picture.config.ID, Drops: stored})
			if err != nil {
				return err
			}
			if added == 0 {
				return errRefused
			}
			return tx.grantDrops(reply.DropList)
		})
	}
	if refused(err) {
		return client.Send(&protobuf.SC_26003{Result: result(err)})
	}
	if err != nil {
		return err
	}
	return client.Send(reply)
}
