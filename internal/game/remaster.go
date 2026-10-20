package game

import (
	"context"
	"database/sql"
	"errors"
	"sync"
	"time"

	"google.golang.org/protobuf/proto"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/db"
	"github.com/ggmolly/belfast/internal/gamedata"
	"github.com/ggmolly/belfast/internal/protobuf"
)

// War Archives (the "remaster" / reactivation chapters, LevelRemasterView):
// past event stages replayed for archive tickets
// (ITEM_ID_REACT_CHAPTER_TICKET), with clear-count awards per stage.

// vitemRemasterTicket is ITEM_ID_REACT_CHAPTER_TICKET (const.lua): the
// archive tickets, a virtual item the client adds to
// ChapterProxy.remasterTickets up to reactivity_ticket_max (Drop, case 7).
const vitemRemasterTicket = 59823

// remasterTicketCost is ChapterProxy:getRemasterTicketCost, a constant in
// the Lua: the tickets one sortie into an archive stage takes
// (TrackingCommand, StartChapterAutoCommand per permit sortie).
const remasterTicketCost = 5

// remaster activity_type values (BossRushChapterRemasterHelper): SEA and SIA
// rows are boss rush archives played through their activity, not maps.
const (
	remasterTypeSEA = 3
	remasterTypeSIA = 4
)

// remasterAwardKey is one re_map_template.drop_gain award: its
// activity_id (0 for the archives), the chapter whose clears count, and
// its 1-based place in drop_gain (ChapterProxy:buildRemasterInfo).
type remasterAwardKey struct {
	act, chapter, pos uint32
}

// remasterAward is what a drop_gain entry gives once its chapter has been
// cleared max times: one of drop type kind and id.
type remasterAward struct {
	kind, id, max uint32
}

var (
	// gameset reactivity_ticket_daily / reactivity_ticket_max
	remasterTicketDaily, remasterTicketMax uint32
	remasterAwards                         map[remasterAwardKey]remasterAward
	// the award positions of each (act, chapter)
	remasterAwardsByChapter map[[2]uint32][]uint32
	// the chapters listed by the non-activity archives (config_data); the
	// client marks their whole maps as archives (ChapterProxy:buildRemasterMaps)
	remasterChapters map[uint32]bool
)

// remasterMaps is the chapter_template.map of the archive stages, built
// once the chapters are loaded (loadChapterData runs alongside).
var remasterMaps = sync.OnceValue(func() map[uint32]bool {
	maps := map[uint32]bool{}
	for id := range remasterChapters {
		if template := chapterTemplates[id]; template != nil {
			maps[template.mapID] = true
		}
	}
	return maps
})

func loadRemasterData() error {
	var rows map[uint32]struct {
		ActivityID   uint32     `json:"activity_id"`
		ActivityType uint32     `json:"activity_type"`
		ConfigData   []uint32   `json:"config_data"`
		DropGain     [][]uint32 `json:"drop_gain"`
	}
	var gameset map[string]uint32
	err := gamedata.LoadAll(
		gamedata.Load("re_map_template", &rows),
		func() (err error) {
			gameset, err = gamedata.Gameset()
			return err
		},
	)
	if err != nil {
		return err
	}
	var ok bool
	if remasterTicketDaily, ok = gameset["reactivity_ticket_daily"]; !ok {
		return gamedata.Missing("gameset", "reactivity_ticket_daily")
	}
	if remasterTicketMax, ok = gameset["reactivity_ticket_max"]; !ok {
		return gamedata.Missing("gameset", "reactivity_ticket_max")
	}
	remasterAwards = map[remasterAwardKey]remasterAward{}
	remasterAwardsByChapter = map[[2]uint32][]uint32{}
	remasterChapters = map[uint32]bool{}
	for _, row := range rows {
		if row.ActivityType != remasterTypeSEA && row.ActivityType != remasterTypeSIA {
			for _, id := range row.ConfigData {
				remasterChapters[id] = true
			}
		}
		for i, gain := range row.DropGain {
			if len(gain) < 4 {
				continue
			}
			key := remasterAwardKey{act: row.ActivityID, chapter: gain[0], pos: uint32(i + 1)}
			remasterAwards[key] = remasterAward{kind: gain[1], id: gain[2], max: gain[3]}
			chapter := [2]uint32{key.act, key.chapter}
			remasterAwardsByChapter[chapter] = append(remasterAwardsByChapter[chapter], key.pos)
		}
	}
	return nil
}

// isRemasterChapter is Map:isRemaster for the chapter's map.
func isRemasterChapter(chapterID uint32) bool {
	template := chapterTemplates[chapterID]
	return template != nil && remasterMaps()[template.mapID]
}

// remasterState is the commander's archive tickets and the day the daily
// ones were last claimed.
func remasterState(ctx context.Context, queries *db.Queries, commanderID uint32) (db.RemasterState, error) {
	state, err := queries.GetRemasterState(ctx, commanderID)
	if errors.Is(err, sql.ErrNoRows) {
		return db.RemasterState{CommanderID: commanderID}, nil
	}
	return state, err
}

// remasterDailyCount is ChapterProxy.remasterDailyCount: the tickets
// claimed today (updateDailyCount), 0 until then and again after midnight
// (DayCall resetDailyCount); 0 shows the War Archives claim button and tip.
func remasterDailyCount(state db.RemasterState, now time.Time) uint32 {
	if state.ClaimedDay == startOfDay(now).Unix() {
		return remasterTicketDaily
	}
	return 0
}

// reactChapter is SC_13001.react_chapter (ChapterProxy remasterTickets /
// remasterDailyCount). active_timestamp and active_id stay 0 as captured.
func reactChapter(ctx context.Context, queries *db.Queries, commanderID uint32, now time.Time) (*protobuf.REACTCHAPTER_INFO, error) {
	state, err := remasterState(ctx, queries, commanderID)
	if err != nil {
		return nil, err
	}
	return &protobuf.REACTCHAPTER_INFO{
		Count:      proto.Uint32(state.Tickets),
		DailyCount: proto.Uint32(remasterDailyCount(state, now)),
	}, nil
}

// addRemasterTickets adds archive tickets up to reactivity_ticket_max, as
// the client does (Drop case 7, EndChapterAutoCommand).
func (tx *Tx) addRemasterTickets(count uint32) error {
	state, err := remasterState(tx.ctx, tx.Queries, tx.commanderID)
	if err != nil {
		return err
	}
	state.Tickets = max(state.Tickets, min(state.Tickets+count, remasterTicketMax))
	return tx.SaveRemasterState(tx.ctx, db.SaveRemasterStateParams(state))
}

// spendRemasterTickets takes the tickets of sorties into chapterID when it
// is an archive stage (remasterTicketCost each), ErrNotEnough when there
// are fewer; nothing for other stages.
func (tx *Tx) spendRemasterTickets(chapterID, sorties uint32) error {
	if sorties == 0 || !isRemasterChapter(chapterID) {
		return nil
	}
	state, err := remasterState(tx.ctx, tx.Queries, tx.commanderID)
	if err != nil {
		return err
	}
	cost := sorties * remasterTicketCost
	if state.Tickets < cost {
		return ErrNotEnough
	}
	state.Tickets -= cost
	return tx.SaveRemasterState(tx.ctx, db.SaveRemasterStateParams(state))
}

// refundRemasterTickets gives back the tickets of sorties into chapterID
// that didn't run (EndChapterAutoCommand), up to reactivity_ticket_max.
func (tx *Tx) refundRemasterTickets(chapterID, sorties uint32) error {
	if sorties == 0 || !isRemasterChapter(chapterID) {
		return nil
	}
	return tx.addRemasterTickets(sorties * remasterTicketCost)
}

// addRemasterPasses is ChapterProxy:addRemasterPassCount for the archives
// (act 0): n more clears of chapterID towards each of its drop_gain awards,
// up to their max. The client counts them itself on a won retreat
// (ChapterOpCommand) and a handover's end.
func (tx *Tx) addRemasterPasses(chapterID, n uint32) error {
	positions := remasterAwardsByChapter[[2]uint32{0, chapterID}]
	if n == 0 || len(positions) == 0 {
		return nil
	}
	rows, err := tx.ListRemasterAwards(tx.ctx, tx.commanderID)
	if err != nil {
		return err
	}
	for _, pos := range positions {
		award := remasterAwards[remasterAwardKey{chapter: chapterID, pos: pos}]
		row := db.RemasterAward{CommanderID: tx.commanderID, ChapterID: chapterID, Pos: pos}
		for _, saved := range rows {
			if saved.ActID == 0 && saved.ChapterID == chapterID && saved.Pos == pos {
				row = saved
			}
		}
		if row.Count >= award.max {
			continue
		}
		row.Count = min(row.Count+n, award.max)
		if err := tx.SaveRemasterAward(tx.ctx, db.SaveRemasterAwardParams(row)); err != nil {
			return err
		}
	}
	return nil
}

// handleRemasterTickets answers GetRemasterCommand (CS_13503, type 0), the
// War Archives' daily ticket claim: result 0 adds
// min(reactivity_ticket_daily, reactivity_ticket_max - tickets) archive
// tickets, which the client works out and adds itself (GAME.ADD_ITEM), and
// marks today's claim (ChapterProxy:updateDailyCount, the button turns to
// claimed). One claim a day, server time; anything else shows "领取失败".
// Nothing was captured: the reply is built from the Lua alone.
func (game *Game) handleRemasterTickets(client *connection.Client, req *protobuf.CS_13503) error {
	ctx := context.Background()
	now := time.Now()
	err := game.tx(ctx, client, func(tx *Tx) error {
		state, err := remasterState(ctx, tx.Queries, client.CommanderID)
		if err != nil {
			return err
		}
		if remasterDailyCount(state, now) > 0 {
			return errRefused
		}
		state.ClaimedDay = startOfDay(now).Unix()
		state.Tickets = max(state.Tickets, min(state.Tickets+remasterTicketDaily, remasterTicketMax))
		return tx.SaveRemasterState(ctx, db.SaveRemasterStateParams(state))
	})
	if refused(err) {
		return client.Send(&protobuf.SC_13504{Result: result(err)})
	}
	if err != nil {
		return err
	}
	return client.Send(&protobuf.SC_13504{Result: result(nil)})
}

// handleRemasterInfo answers RemasterInfoRequestCommand (CS_13505, type 0,
// asked once per session on opening the War Archives): the clears counted
// towards each drop_gain award (count) and whether it was taken (flag 1),
// by act_id, chapter_id and pos; the client only keeps the entries
// re_map_template knows and leaves the others at 0 (as captured for a
// commander who never played the archives: an empty list).
func (game *Game) handleRemasterInfo(client *connection.Client, req *protobuf.CS_13505) error {
	rows, err := game.DB.Queries.ListRemasterAwards(context.Background(), client.CommanderID)
	if err != nil {
		return err
	}
	reply := &protobuf.SC_13506{}
	for _, row := range rows {
		flag := uint32(0)
		if row.Received {
			flag = 1
		}
		reply.RemapCountList = append(reply.RemapCountList, &protobuf.REMAPCOUNT{
			ActId:     proto.Uint32(row.ActID),
			ChapterId: proto.Uint32(row.ChapterID),
			Pos:       proto.Uint32(row.Pos),
			Count:     proto.Uint32(row.Count),
			Flag:      proto.Uint32(flag),
		})
	}
	return client.Send(reply)
}

// handleRemasterAward answers RemasterAwardReceiveCommand (CS_13507): takes
// the drop_gain award at pos of chapter_id (act_id 0 for the archives) once
// the chapter was cleared its max times and the award wasn't taken. Result
// 0 marks it taken (markRemasterPassReceive) and drop_list, the award (one
// of drop_gain's type and id: a ship, the ship pushed in SC_12042 ahead of
// the reply), is shown and added by the client (PlayerConst.addTranDrop).
// Non-zero shows errorTip. Nothing was captured: the reply is built from
// the Lua alone.
func (game *Game) handleRemasterAward(client *connection.Client, req *protobuf.CS_13507) error {
	ctx := context.Background()
	key := remasterAwardKey{act: req.GetActId(), chapter: req.GetChapterId(), pos: req.GetPos()}
	award, ok := remasterAwards[key]
	if !ok {
		return client.Send(&protobuf.SC_13508{Result: result(errRefused)})
	}
	drops := []*protobuf.DROPINFO{dropInfo(award.kind, award.id, 1)}
	err := game.tx(ctx, client, func(tx *Tx) error {
		rows, err := tx.ListRemasterAwards(ctx, client.CommanderID)
		if err != nil {
			return err
		}
		for _, row := range rows {
			if row.ActID != key.act || row.ChapterID != key.chapter || row.Pos != key.pos {
				continue
			}
			if row.Received || row.Count < award.max {
				return errRefused
			}
			row.Received = true
			if err := tx.SaveRemasterAward(ctx, db.SaveRemasterAwardParams(row)); err != nil {
				return err
			}
			return tx.grantDrops(drops)
		}
		return errRefused
	})
	if refused(err) {
		return client.Send(&protobuf.SC_13508{Result: result(err)})
	}
	if err != nil {
		return err
	}
	return client.Send(&protobuf.SC_13508{Result: result(nil), DropList: drops})
}
