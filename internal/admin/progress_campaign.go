package admin

import (
	"database/sql"
	"errors"
	"fmt"
	"net/http"

	"google.golang.org/protobuf/proto"

	"github.com/ggmolly/belfast/internal/db"
	"github.com/ggmolly/belfast/internal/game"
	"github.com/ggmolly/belfast/internal/protobuf"
)

type progStageStar struct {
	Label string `json:"label"`
	Done  bool   `json:"done"`
}

type progStageRow struct {
	ID          uint32          `json:"id"`
	Name        string          `json:"name"`  // "7–3"
	Title       string          `json:"title"` // the stage's own name
	Progress    uint32          `json:"progress"`
	Stars       []progStageStar `json:"stars"`
	PassCount   uint32          `json:"pass_count"`
	DefeatCount uint32          `json:"defeat_count"`
	UnlockLevel uint32          `json:"unlock_level"`
}

type progCampaignMap struct {
	ID         uint32         `json:"id"`
	Title      string         `json:"title"` // "7"
	Name       string         `json:"name"`
	LevelLimit uint32         `json:"level_limit"`
	Normal     []progStageRow `json:"normal"`
	Hard       []progStageRow `json:"hard"`
}

type progSortieInfo struct {
	Stage     uint32   `json:"stage"`
	Name      string   `json:"name"`
	Title     string   `json:"title"`
	Round     uint32   `json:"round"`
	Fleets    []uint32 `json:"fleets"`
	Ammo      []uint32 `json:"ammo"`
	AmmoTotal uint32   `json:"ammo_total"`
	Start     uint32   `json:"start"`
	Kills     uint32   `json:"kills"`
}

type progMissionRow struct {
	Kind     string `json:"kind"`
	Act      uint32 `json:"act,omitempty"`
	ID       uint32 `json:"id"`
	Type     uint32 `json:"type"` // task_data_template type (Task.TYPE_*), 0 for weekly
	Name     string `json:"name"`
	Desc     string `json:"desc"`
	Progress uint32 `json:"progress"`
	Target   uint32 `json:"target"`
}

type progressView struct {
	Level      uint32            `json:"level"`
	Maps       []progCampaignMap `json:"maps"`
	Sortie     *progSortieInfo   `json:"sortie"`
	Missions   []progMissionRow  `json:"missions"`
	Guide      uint32            `json:"guide"`
	NewGuide   uint32            `json:"new_guide"`
	Collection struct {
		ShipGroups      int64 `json:"ship_groups"`
		ShipGroupsTotal int   `json:"ship_groups_total"`
		Stories         int64 `json:"stories"`
		Trophies        int64 `json:"trophies"`
	} `json:"collection"`
}

// starLabel is ChapterConst.GetAchieveDesc of a star requirement.
func (t *progressTables) starLabel(kind, num, count uint32) string {
	switch kind {
	case 1:
		return "Defeat flagship"
	case 2:
		return fmt.Sprintf("Defeat escort fleet (%d/%d)", min(count, num), num)
	case 3:
		return "Defeat all enemies"
	case 4:
		return fmt.Sprintf("Deployed ships ≤ %d", num)
	case 5:
		return t.shipTypes[num].TypeName + " not deployed"
	case 6:
		return "Clear with a Full Combo"
	}
	return fmt.Sprintf("Requirement %d", kind)
}

// progStarDone is ChapterConst.IsAchieved.
func progStarDone(kind, num, count uint32) bool {
	if kind == 4 || kind == 5 {
		return count >= 1
	}
	return num <= count
}

func (t *progressTables) stageRows(mapID uint32, progress map[uint32]db.ChapterProgress) []progStageRow {
	rows := []progStageRow{}
	for _, id := range t.stages[mapID] {
		config := t.chapters[id]
		row := progress[id]
		counts := [3]uint32{row.KillBossCount, row.KillEnemyCount, row.TakeBoxCount}
		stage := progStageRow{ID: id, Name: config.ChapterName, Title: config.Name, Progress: row.Progress, PassCount: row.PassCount, DefeatCount: row.DefeatCount, UnlockLevel: config.UnlockLevel, Stars: []progStageStar{}}
		for i, star := range [][2]uint32{{config.StarRequire1, config.Num1}, {config.StarRequire2, config.Num2}, {config.StarRequire3, config.Num3}} {
			if star[0] > 0 {
				stage.Stars = append(stage.Stars, progStageStar{Label: t.starLabel(star[0], star[1], counts[i]), Done: progStarDone(star[0], star[1], counts[i])})
			}
		}
		rows = append(rows, stage)
	}
	return rows
}

// handleProgress is the commander's campaign, sortie, missions, tutorial
// and collection.
func (server *Server) handleProgress(r *http.Request) (any, error) {
	commander, err := server.commander(r)
	if err != nil {
		return nil, err
	}
	tables, err := loadProgressTables()
	if err != nil {
		return nil, err
	}
	ctx, id := r.Context(), commander.ID
	view := progressView{Level: commander.Level, Guide: commander.GuideIndex, NewGuide: commander.NewGuideIndex, Maps: []progCampaignMap{}, Missions: []progMissionRow{}}

	rows, err := server.Store.ListChapterProgress(ctx, id)
	if err != nil {
		return nil, err
	}
	progress := make(map[uint32]db.ChapterProgress, len(rows))
	for _, row := range rows {
		progress[row.ChapterID] = row
	}
	for _, mapID := range tables.normalMaps {
		config := tables.maps[mapID]
		view.Maps = append(view.Maps, progCampaignMap{
			ID: mapID, Title: config.Title, Name: config.Name, LevelLimit: config.LevelLimit,
			Normal: tables.stageRows(mapID, progress), Hard: tables.stageRows(config.BindMap, progress),
		})
	}

	state, err := server.Store.GetChapterState(ctx, id)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}
	if err == nil {
		var info protobuf.CURRENTCHAPTERINFO
		if err := (proto.UnmarshalOptions{AllowPartial: true}).Unmarshal(state.State, &info); err != nil {
			return nil, err
		}
		config := tables.chapters[info.GetId()]
		sortie := &progSortieInfo{
			Stage: info.GetId(), Name: config.ChapterName, Title: config.Name, Round: info.GetRound(), AmmoTotal: config.AmmoTotal,
			Start: info.GetStartTime(), Kills: info.GetKillCount(), Fleets: []uint32{}, Ammo: []uint32{},
		}
		for _, group := range info.GetMainGroupList() {
			sortie.Fleets = append(sortie.Fleets, group.GetFleetId())
			sortie.Ammo = append(sortie.Ammo, group.GetBullet())
		}
		view.Sortie = sortie
	}

	tasks, err := server.Store.ListTasks(ctx, id)
	if err != nil {
		return nil, err
	}
	for _, task := range tasks {
		config, ok := tables.tasks[task.TaskID]
		if !ok || config.Type == 5 { // Task.TYPE_HIDDEN: research conditions and the like
			continue
		}
		view.Missions = append(view.Missions, progMissionRow{Kind: game.MissionTask, ID: task.TaskID, Type: config.Type, Name: config.Name, Desc: config.Desc, Progress: task.Progress, Target: config.TargetNum})
	}
	weekly, err := server.Store.ListWeeklyTasks(ctx, id)
	if err != nil {
		return nil, err
	}
	for _, task := range weekly {
		config := tables.weekly[task.TaskID]
		view.Missions = append(view.Missions, progMissionRow{Kind: game.MissionWeekly, ID: task.TaskID, Desc: config.Desc, Progress: task.Progress, Target: config.TargetNum})
	}
	activity, err := server.Store.ListActivityTasks(ctx, id)
	if err != nil {
		return nil, err
	}
	for _, task := range activity {
		config, ok := tables.tasks[task.TaskID]
		if !ok || task.Finished {
			continue
		}
		view.Missions = append(view.Missions, progMissionRow{Kind: game.MissionActivity, Act: task.ActID, ID: task.TaskID, Type: config.Type, Name: config.Name, Desc: config.Desc, Progress: task.Progress, Target: config.TargetNum})
	}

	collection, err := server.Store.ProgressCollection(ctx, id)
	if err != nil {
		return nil, err
	}
	view.Collection.ShipGroups, view.Collection.Stories, view.Collection.Trophies = collection.ShipGroups, collection.Stories, collection.Trophies
	view.Collection.ShipGroupsTotal = tables.shipGroups
	return view, nil
}

// handleSetStage marks a stage cleared, sets its stars and the times it was
// won: {stage, cleared?, stars?, wins?}, stars one flag per star requirement
// in order. A star
// given sets its count to what the requirement asks, one taken sets it to 0.
func (server *Server) handleSetStage(r *http.Request) (any, error) {
	commander, err := server.commander(r)
	if err != nil {
		return nil, err
	}
	tables, err := loadProgressTables()
	if err != nil {
		return nil, err
	}
	var body struct {
		Stage   uint32 `json:"stage"`
		Cleared bool   `json:"cleared"`
		Stars   []bool `json:"stars"`
		// the times it was won
		Wins *uint32 `json:"wins"`
	}
	if err := readJSON(r, &body); err != nil {
		return nil, err
	}
	config, ok := tables.chapters[body.Stage]
	if !ok {
		return nil, badRequest(fmt.Sprintf("no stage %d", body.Stage))
	}
	set := game.ChapterSet{ID: body.Stage, Wins: body.Wins}
	if body.Cleared {
		set.Progress = 100
	}
	if body.Stars != nil {
		counts, n := starCounts(config, body.Stars)
		if len(body.Stars) != n {
			return nil, badRequest(fmt.Sprintf("stage %d has %d stars", body.Stage, n))
		}
		set.Counts = &counts
	}
	return kicked(server.Game.SetChapters(r.Context(), commander.ID, []game.ChapterSet{set}))
}

// starCounts are the kill boss / kill enemy / take box counts that make a
// stage's stars read as done (stars[n] for its n-th star, the ones its
// star_require_* set), and how many stars it has.
func starCounts(config progChapterRow, stars []bool) ([3]uint32, int) {
	var counts [3]uint32
	requires := [][2]uint32{{config.StarRequire1, config.Num1}, {config.StarRequire2, config.Num2}, {config.StarRequire3, config.Num3}}
	n := 0
	for i, star := range requires {
		if star[0] == 0 {
			continue
		}
		if n < len(stars) && stars[n] {
			counts[i] = max(star[1], 1)
		}
		n++
	}
	return counts, n
}

// handleUnlockChapters clears every stage of the campaign's chapters from
// to chapter (1 and on when from is left out), in normal mode (unless normal
// is false), hard mode or both, all their stars too when asked:
// {from?, chapter, normal?, hard, stars}. Otherwise what a stage's stars
// read is kept.
func (server *Server) handleUnlockChapters(r *http.Request) (any, error) {
	commander, err := server.commander(r)
	if err != nil {
		return nil, err
	}
	tables, err := loadProgressTables()
	if err != nil {
		return nil, err
	}
	var body struct {
		From    int   `json:"from"`
		Chapter int   `json:"chapter"`
		Normal  *bool `json:"normal"`
		Hard    bool  `json:"hard"`
		Stars   bool  `json:"stars"`
	}
	if err := readJSON(r, &body); err != nil {
		return nil, err
	}
	if body.Chapter < 1 || body.Chapter > len(tables.normalMaps) {
		return nil, badRequest(fmt.Sprintf("the chapter goes from 1 to %d", len(tables.normalMaps)))
	}
	if body.From == 0 {
		body.From = 1
	}
	if body.From < 1 || body.From > body.Chapter {
		return nil, badRequest(fmt.Sprintf("from goes from 1 to %d", body.Chapter))
	}
	normal := body.Normal == nil || *body.Normal
	if !normal && !body.Hard {
		return nil, badRequest("clear normal mode, hard mode or both")
	}
	var sets []game.ChapterSet
	for _, mapID := range tables.normalMaps[body.From-1 : body.Chapter] {
		var maps []uint32
		if normal {
			maps = append(maps, mapID)
		}
		if body.Hard && tables.maps[mapID].BindMap != 0 {
			maps = append(maps, tables.maps[mapID].BindMap)
		}
		for _, m := range maps {
			for _, stage := range tables.stages[m] {
				set := game.ChapterSet{ID: stage, Progress: 100}
				if body.Stars {
					counts, _ := starCounts(tables.chapters[stage], []bool{true, true, true})
					set.Counts = &counts
				}
				sets = append(sets, set)
			}
		}
	}
	return kicked(server.Game.SetChapters(r.Context(), commander.ID, sets))
}

// handleEndSortie ends the commander's sortie, as a retreat does.
func (server *Server) handleEndSortie(r *http.Request) (any, error) {
	commander, err := server.commander(r)
	if err != nil {
		return nil, err
	}
	return kicked(server.Game.EndSortie(r.Context(), commander.ID))
}

// handleMissionProgress sets a mission's progress: {kind, act?, id, progress}.
func (server *Server) handleMissionProgress(r *http.Request) (any, error) {
	commander, err := server.commander(r)
	if err != nil {
		return nil, err
	}
	var body struct {
		Kind     string `json:"kind"`
		Act      uint32 `json:"act"`
		ID       uint32 `json:"id"`
		Progress uint32 `json:"progress"`
	}
	if err := readJSON(r, &body); err != nil {
		return nil, err
	}
	return kicked(server.Game.SetMissionProgress(r.Context(), commander.ID, body.Kind, body.Act, body.ID, body.Progress))
}

// handleGuide sets the tutorial progress: {guide, new_guide}.
func (server *Server) handleGuide(r *http.Request) (any, error) {
	commander, err := server.commander(r)
	if err != nil {
		return nil, err
	}
	var body struct {
		Guide    uint32 `json:"guide"`
		NewGuide uint32 `json:"new_guide"`
	}
	if err := readJSON(r, &body); err != nil {
		return nil, err
	}
	if body.Guide == 0 || body.NewGuide == 0 {
		return nil, badRequest("the guides start at step 1")
	}
	return kicked(server.Game.SetGuide(r.Context(), commander.ID, body.Guide, body.NewGuide))
}
