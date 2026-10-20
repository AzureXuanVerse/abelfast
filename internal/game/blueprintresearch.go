package game

import (
	"cmp"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"slices"
	"time"

	"google.golang.org/protobuf/proto"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/db"
	"github.com/ggmolly/belfast/internal/gamedata"
	"github.com/ggmolly/belfast/internal/protobuf"
)

// blueprintColdTime is how long after a research starts another one may
// start: TechnologyProxy:updateColdTime (the client sets it itself on
// SC_63201), the wiki's "24-hour cooldown for swapping".
const blueprintColdTime = 24 * time.Hour

// blueprintPursueReset is when the catch-up's daily discounted counts start
// again: TechnologyProxy's HourCall at hour 4 (resetPursuingTimes), "4
// hours after the daily reset" says the wiki.
const blueprintPursueReset = 4 * time.Hour

// blueprintTaskDrift is how many seconds ahead of the server's clock the
// client's may open a research task.
const blueprintTaskDrift = 5

// TaskSubShipGroups is task_data_template sub_type 1040: ship groups owned
// of target_id's [nationality, type] pairs (the first series' "Register 20
// or more Royal Navy Vanguard ships in your Library"). The client doesn't
// compute it (Task.getProgress): the server counts the groups, kept or
// retired ("the ships do NOT have to be in the dock", says the wiki).
const TaskSubShipGroups = 1040

// task_data_template sub types whose progress is what the commander owns,
// paid when the task is claimed (Task.getProgress,
// SubmitTaskCommand.CheckTaskSub): the research ships' "Gather" tasks.
const (
	TaskSubGiveItem     = 1000 // TASK_SUB_TYPE_GIVE_ITEM: target_id an item
	TaskSubGiveResource = 1002 // TASK_SUB_TYPE_PLAYER_RES: target_id a player_resource
)

// taskGiveCost is what claiming a "Gather" task takes, as a [type, id,
// count] cost for payDrops; nil for other tasks.
func taskGiveCost(config taskConfig) []uint32 {
	ids := gamedata.IDs(config.TargetID)
	if len(ids) == 0 {
		return nil
	}
	switch config.SubType {
	case TaskSubGiveItem:
		return []uint32{dropTypeItem, ids[0], config.TargetNum}
	case TaskSubGiveResource:
		return []uint32{dropTypeResource, ids[0], config.TargetNum}
	}
	return nil
}

// blueprintPursueDay is the unix start of the catch-up day now falls in.
func blueprintPursueDay(now time.Time) int64 {
	return startOfDay(now.Add(-blueprintPursueReset)).Add(blueprintPursueReset).Unix()
}

// blueprintRow is the commander's state of a research ship, untouched
// (nothing started nor built) when there is no row yet.
func blueprintRow(ctx context.Context, queries *db.Queries, commanderID, id uint32) (db.CommanderBlueprint, error) {
	row, err := queries.GetBlueprint(ctx, db.GetBlueprintParams{CommanderID: commanderID, BlueprintID: id})
	if errors.Is(err, sql.ErrNoRows) {
		return db.CommanderBlueprint{CommanderID: commanderID, BlueprintID: id}, nil
	}
	return row, err
}

func (tx *Tx) saveBlueprint(row db.CommanderBlueprint) error {
	return tx.SaveBlueprint(tx.ctx, db.SaveBlueprintParams{
		CommanderID: tx.commanderID, BlueprintID: row.BlueprintID, ShipID: row.ShipID, StartTime: row.StartTime, Duration: row.Duration,
	})
}

// blueprintState is the commander's cold time and catch-up counts, the
// counts back to 0 once their day is over.
func blueprintState(ctx context.Context, queries *db.Queries, commanderID uint32, now time.Time) (db.CommanderBlueprintState, error) {
	state, err := queries.GetBlueprintState(ctx, commanderID)
	if errors.Is(err, sql.ErrNoRows) {
		state, err = db.CommanderBlueprintState{CommanderID: commanderID}, nil
	}
	if day := blueprintPursueDay(now); state.PursueDay != day {
		state.PursueDay, state.Pursued, state.PursuedUr = day, 0, 0
	}
	return state, err
}

func (tx *Tx) saveBlueprintState(state db.CommanderBlueprintState) error {
	return tx.SaveBlueprintState(tx.ctx, db.SaveBlueprintStateParams{
		CommanderID: tx.commanderID, ColdTime: state.ColdTime, PursueDay: state.PursueDay, Pursued: state.Pursued, PursuedUr: state.PursuedUr,
	})
}

// shipBlueprintProgress is a research ship's development, from its
// strength_list entry (Ship:getBluePrint).
func shipBlueprintProgress(ctx context.Context, queries *db.Queries, config blueprintConfig, shipID uint32) (blueprintProgress, error) {
	rows, err := queries.ListShipStrengths(ctx, shipID)
	if err != nil || len(rows) == 0 {
		return readBlueprintProgress(config, 0, 0), err
	}
	return readBlueprintProgress(config, rows[0].AttrID, rows[0].Exp), nil
}

// blueprintList is SC_63100, TechnologyProxy's research ships: each one
// started or built (the client asserts every id is a ship_data_blueprint
// row), the cold time and the catch-up counts of the day. The official
// capture only has an empty one (a commander with none).
func blueprintList(ctx context.Context, queries *db.Queries, commanderID uint32, now time.Time) (*protobuf.SC_63100, error) {
	rows, err := queries.ListBlueprints(ctx, commanderID)
	if err != nil {
		return nil, err
	}
	state, err := blueprintState(ctx, queries, commanderID, now)
	if err != nil {
		return nil, err
	}
	msg := &protobuf.SC_63100{
		ColdTime:                 proto.Uint32(uint32(state.ColdTime)),
		DailyCatchupStrengthen:   proto.Uint32(state.Pursued),
		DailyCatchupStrengthenUr: proto.Uint32(state.PursuedUr),
	}
	for _, row := range rows {
		config, ok := blueprintConfigs[row.BlueprintID]
		if !ok {
			continue
		}
		var progress blueprintProgress
		if row.ShipID != 0 {
			if progress, err = shipBlueprintProgress(ctx, queries, config, row.ShipID); err != nil {
				return nil, err
			}
		}
		msg.BlueprintList = append(msg.BlueprintList, &protobuf.BLUPRINTINFO{
			Id:             proto.Uint32(row.BlueprintID),
			ShipId:         proto.Uint32(row.ShipID),
			StartTime:      proto.Uint32(uint32(row.StartTime)),
			BluePrintLevel: proto.Uint32(progress.stored()),
			Exp:            proto.Uint32(progress.exp),
			StartDuration:  proto.Uint32(uint32(row.Duration)),
		})
	}
	return msg, nil
}

// tasksClaimed tells whether the commander claimed every task of ids.
func (tx *Tx) tasksClaimed(ids []uint32) (bool, error) {
	for _, id := range ids {
		task, err := tx.GetTask(tx.ctx, db.GetTaskParams{CommanderID: tx.commanderID, TaskID: id})
		if errors.Is(err, sql.ErrNoRows) {
			return false, nil
		} else if err != nil {
			return false, err
		}
		if task.SubmitTime == 0 {
			return false, nil
		}
	}
	return true, nil
}

// handleStartBlueprint answers BuildShipBluePrintCommand
// (controller/command/technology/buildshipblueprintcommand.lua, the Dev.
// Dock's start button): starts the research of blueprint_id once its
// opening tasks are claimed (isFinishPrevTask), the cold time is over and
// no other research runs (the client stops it first with CS_63206). time is
// the start; a research stopped before resumes where it was, its tasks
// opening on the same schedule, so it starts its start_duration back. On
// result 0 the client sets the cold time a day later itself. No official
// capture has it: the reply is built from the Lua alone.
func (game *Game) handleStartBlueprint(client *connection.Client, req *protobuf.CS_63200) error {
	now := time.Now()
	var start int64
	err := game.tx(context.Background(), client, func(tx *Tx) error {
		config, ok := blueprintConfigs[req.GetBlueprintId()]
		if !ok {
			return errRefused
		}
		row, err := blueprintRow(tx.ctx, tx.Queries, tx.commanderID, config.ID)
		if err != nil {
			return err
		}
		if row.ShipID != 0 || row.StartTime != 0 {
			return errRefused
		}
		if opened, err := tx.tasksClaimed(config.OpenCondition); err != nil || !opened {
			return cmp.Or(err, errRefused)
		}
		state, err := blueprintState(tx.ctx, tx.Queries, tx.commanderID, now)
		if err != nil {
			return err
		}
		if now.Unix() < state.ColdTime {
			return errRefused
		}
		rows, err := tx.ListBlueprints(tx.ctx, tx.commanderID)
		if err != nil {
			return err
		}
		for _, other := range rows {
			if other.ShipID == 0 && other.StartTime != 0 {
				return errRefused // one research at a time
			}
		}
		start = now.Unix() - row.Duration
		row.StartTime, row.Duration = start, 0
		if err := tx.saveBlueprint(row); err != nil {
			return err
		}
		state.ColdTime = now.Add(blueprintColdTime).Unix()
		return tx.saveBlueprintState(state)
	})
	if err != nil && !refused(err) {
		return err
	}
	return client.Send(&protobuf.SC_63201{Result: result(err), Time: proto.Uint32(uint32(start))})
}

// handleStopBlueprint answers StopBluePrintCommand
// (controller/command/technology/stopblueprintcommand.lua): pauses the
// running research of blueprint_id, its tasks kept. On result 0 the client
// keeps how long it ran (start_duration) and shows it locked again. No
// official capture has it: the reply is built from the Lua alone.
func (game *Game) handleStopBlueprint(client *connection.Client, req *protobuf.CS_63206) error {
	now := time.Now()
	err := game.tx(context.Background(), client, func(tx *Tx) error {
		if _, ok := blueprintConfigs[req.GetBlueprintId()]; !ok {
			return errRefused
		}
		row, err := blueprintRow(tx.ctx, tx.Queries, tx.commanderID, req.GetBlueprintId())
		if err != nil {
			return err
		}
		if row.ShipID != 0 || row.StartTime == 0 {
			return errRefused
		}
		row.Duration, row.StartTime = max(now.Unix()-row.StartTime, 0), 0
		return tx.saveBlueprint(row)
	})
	if err != nil && !refused(err) {
		return err
	}
	return client.Send(&protobuf.SC_63207{Result: result(err)})
}

// buildBlueprintShip gives the commander the research ship of row and
// records it built.
func (tx *Tx) buildBlueprintShip(config blueprintConfig, row db.CommanderBlueprint) (*protobuf.SHIPINFO, error) {
	ship, err := tx.AddShip(config.shipTemplate())
	if err != nil {
		return nil, err
	}
	row.ShipID, row.Duration = ship.GetId(), 0
	return ship, tx.saveBlueprint(row)
}

// handleFinishBlueprint answers FinishBluePrintCommand
// (controller/command/technology/finishblueprintcommand.lua, the Dev.
// Dock's build button once every task of the research is claimed): builds
// the research ship. On result 0 the client adds ship and marks the
// research done. No official capture has it: the reply is built from the
// Lua alone.
func (game *Game) handleFinishBlueprint(client *connection.Client, req *protobuf.CS_63202) error {
	var ship *protobuf.SHIPINFO
	err := game.tx(context.Background(), client, func(tx *Tx) error {
		config, ok := blueprintConfigs[req.GetBlueprintId()]
		if !ok {
			return errRefused
		}
		row, err := blueprintRow(tx.ctx, tx.Queries, tx.commanderID, config.ID)
		if err != nil {
			return err
		}
		if row.ShipID != 0 || row.StartTime == 0 {
			return errRefused
		}
		if done, err := tx.tasksClaimed(config.taskIDs()); err != nil || !done {
			return cmp.Or(err, errRefused)
		}
		ship, err = tx.buildBlueprintShip(config, row)
		return err
	})
	if err != nil && !refused(err) {
		return err
	}
	return client.Send(&protobuf.SC_63203{Result: result(err), Ship: ship})
}

// handleItemUnlockBlueprint answers ItemUnlockBluePrintCommand
// (controller/command/technology/itemunlockblueprintcommand.lua): a PR
// voucher of the research's gain_item_id builds its ship at once, whatever
// the research's state. On result 0 the client takes one voucher and adds
// ship. No official capture has it: the reply is built from the Lua alone.
func (game *Game) handleItemUnlockBlueprint(client *connection.Client, req *protobuf.CS_63214) error {
	var ship *protobuf.SHIPINFO
	err := game.tx(context.Background(), client, func(tx *Tx) error {
		config, ok := blueprintConfigs[req.GetGroup()]
		if !ok || !slices.Contains(config.GainItemID, req.GetItemid()) {
			return errRefused
		}
		row, err := blueprintRow(tx.ctx, tx.Queries, tx.commanderID, config.ID)
		if err != nil {
			return err
		}
		if row.ShipID != 0 {
			return errRefused
		}
		if err := tx.SpendItem(req.GetItemid(), 1); err != nil {
			return err
		}
		row.StartTime = 0
		ship, err = tx.buildBlueprintShip(config, row)
		return err
	})
	if err != nil && !refused(err) {
		return err
	}
	return client.Send(&protobuf.SC_63215{Result: result(err), Ship: ship})
}

// developBlueprint gives a built research ship count blueprints' worth of
// development exp: the ship must be the commander's research ship, not
// fully developed, and at the ship level the result needs (fateNeed: the
// fate phase's need_lv counts too, as ModBluePrintCommand checks and
// PursuingBluePrintCommand doesn't). pay takes the price once the request
// is known valid. The levels and phases reached take effect as the client
// applies them (ModBluePrintCommand): skills learnt, a skin given, a limit
// break (upgradeStar), a skill upgraded; the development is stored as the
// ship's strength_list entry, which the client reads it from
// (Ship:getBluePrint).
func (tx *Tx) developBlueprint(shipID, count uint32, fateNeed bool, pay func(config blueprintConfig) error) error {
	ship, err := tx.GetShip(tx.ctx, db.GetShipParams{ID: shipID, CommanderID: tx.commanderID})
	if errors.Is(err, sql.ErrNoRows) {
		return errRefused
	} else if err != nil {
		return err
	}
	config, ok := blueprintConfigs[shipGroup(ship.TemplateID)]
	if !ok || count == 0 || config.itemExp() == 0 {
		return errRefused
	}
	row, err := blueprintRow(tx.ctx, tx.Queries, tx.commanderID, config.ID)
	if err != nil {
		return err
	}
	if row.ShipID != ship.ID {
		return errRefused
	}
	before, err := shipBlueprintProgress(tx.ctx, tx.Queries, config, ship.ID)
	if err != nil {
		return err
	}
	if before.maxed(config) {
		return errRefused
	}
	after := before.addExp(config, count*config.itemExp())
	if ship.Level < after.needLevel(config, fateNeed) {
		return errRefused
	}
	if err := pay(config); err != nil {
		return err
	}
	var effects []blueprintLevel
	if after.level > before.level {
		effects = append(effects, config.levels[before.level:after.level]...)
	}
	if after.fate > max(before.fate, 0) {
		effects = append(effects, config.fates[max(before.fate, 0):after.fate]...)
	}
	for _, level := range effects {
		if level.Special != 1 {
			continue
		}
		for _, skill := range level.skills {
			if err := tx.AddShipSkill(tx.ctx, db.AddShipSkillParams{ShipID: ship.ID, SkillID: skill, Level: 1}); err != nil {
				return err
			}
		}
		if level.EffectSkin != 0 {
			if err := tx.AddSkin(level.EffectSkin); err != nil {
				return err
			}
		}
		if level.EffectBreakout != 0 {
			// upgradeStar: the ship's own ship_data_breakout step
			if next := breakouts[ship.TemplateID].BreakoutID; next != 0 {
				if err := tx.limitBreak(ship, next); err != nil {
					return err
				}
				if ship, err = tx.GetShip(tx.ctx, db.GetShipParams{ID: ship.ID, CommanderID: tx.commanderID}); err != nil {
					return err
				}
			}
		}
		if len(level.changeSkill) == 2 {
			if err := tx.renameTemplateSkills(ship.ID, level.changeSkill[:1], level.changeSkill[1:]); err != nil {
				return err
			}
		}
	}
	if err := tx.DeleteShipStrengths(tx.ctx, ship.ID); err != nil {
		return err
	}
	return tx.SetShipStrength(tx.ctx, db.SetShipStrengthParams{ShipID: ship.ID, AttrID: after.stored(), Exp: after.exp})
}

// handleModBlueprint answers ModBluePrintCommand
// (controller/command/technology/modblueprintcommand.lua, the research
// ship page's enhance and fate simulation): count of the ship's own
// blueprints develop it. On result 0 the client takes the blueprints and
// applies the development itself (developBlueprint). No official capture
// has it: the reply is built from the Lua alone.
func (game *Game) handleModBlueprint(client *connection.Client, req *protobuf.CS_63204) error {
	err := game.tx(context.Background(), client, func(tx *Tx) error {
		return tx.developBlueprint(req.GetShipId(), req.GetCount(), true, func(config blueprintConfig) error {
			return tx.SpendItem(config.StrengthenItem, req.GetCount())
		})
	})
	if err != nil && !refused(err) {
		return err
	}
	return client.Send(&protobuf.SC_63205{Result: result(err)})
}

// handlePursueBlueprint answers PursuingBluePrintCommand
// (controller/command/technology/pursuingblueprintcommand.lua, the coin
// catch-up of older series' ships, is_pursuing 1): count blueprints' worth
// of development bought at the ship's price, the first ones of the day
// discounted (gameset blueprint_pursue_discount_ssr / _ur,
// TechnologyProxy:calcPursuingCost, Decisive ships counted apart). On
// result 0 the client takes the coins, adds count to the day's count and
// applies the development itself (developBlueprint). No official capture
// has it: the reply is built from the Lua alone.
func (game *Game) handlePursueBlueprint(client *connection.Client, req *protobuf.CS_63212) error {
	now := time.Now()
	err := game.tx(context.Background(), client, func(tx *Tx) error {
		return tx.developBlueprint(req.GetShipId(), req.GetCount(), false, func(config blueprintConfig) error {
			if config.IsPursuing != 1 {
				return errRefused
			}
			state, err := blueprintState(tx.ctx, tx.Queries, tx.commanderID, now)
			if err != nil {
				return err
			}
			pursued := &state.Pursued
			if config.ur() {
				pursued = &state.PursuedUr
			}
			if err := tx.SpendResource(ResourceGold, config.pursueCost(*pursued, req.GetCount())); err != nil {
				return err
			}
			*pursued += req.GetCount()
			return tx.saveBlueprintState(state)
		})
	})
	if err != nil && !refused(err) {
		return err
	}
	return client.Send(&protobuf.SC_63213{Result: result(err)})
}

// handleBlueprintSpeedup answers UseTecSpeedUpItemCommand
// (controller/command/technology/usetecspeedupitemcommand.lua,
// TecSpeedUpLayer): number of the series' combat data packs (gameset
// technology_catchup_itemid, [item, exp] by blueprint_version) add their
// exp to task_id, the first or fourth task of the running research (its
// "Combat Data Collection"), up to its target. On result 0 the client takes
// the packs; the progress is pushed (SC_20002) before the reply. No
// official capture has it: the reply is built from the Lua alone.
func (game *Game) handleBlueprintSpeedup(client *connection.Client, req *protobuf.CS_63210) error {
	err := game.tx(context.Background(), client, func(tx *Tx) error {
		config, ok := blueprintConfigs[req.GetBlueprintid()]
		if !ok || req.GetNumber() == 0 || config.Version < 1 || int(config.Version) > len(blueprintSpeedups) {
			return errRefused
		}
		speedup := blueprintSpeedups[config.Version-1]
		tasks := config.taskIDs()
		if len(speedup) < 2 || speedup[0] != req.GetItemid() || len(tasks) < 4 ||
			(req.GetTaskId() != tasks[0] && req.GetTaskId() != tasks[3]) {
			return errRefused
		}
		row, err := blueprintRow(tx.ctx, tx.Queries, tx.commanderID, config.ID)
		if err != nil {
			return err
		}
		if row.ShipID != 0 || row.StartTime == 0 {
			return errRefused
		}
		task, err := tx.GetTask(tx.ctx, db.GetTaskParams{CommanderID: tx.commanderID, TaskID: req.GetTaskId()})
		if errors.Is(err, sql.ErrNoRows) {
			return errRefused
		} else if err != nil {
			return err
		}
		target := taskConfigs[task.TaskID].TargetNum
		if task.SubmitTime != 0 || task.Progress >= target {
			return errRefused
		}
		if err := tx.SpendItem(req.GetItemid(), req.GetNumber()); err != nil {
			return err
		}
		progress := min(task.Progress+req.GetNumber()*speedup[1], target)
		if err := tx.SetTaskProgress(tx.ctx, db.SetTaskProgressParams{Progress: progress, CommanderID: tx.commanderID, TaskID: task.TaskID}); err != nil {
			return err
		}
		tx.push(&protobuf.SC_20002{Info: []*protobuf.TASK_PROGRESS{{Id: proto.Uint32(task.TaskID), Progress: proto.Uint32(progress)}}})
		return nil
	})
	if err != nil && !refused(err) {
		return err
	}
	return client.Send(&protobuf.SC_63211{Result: result(err)})
}

// triggerBlueprintTask accepts a research ship's task for TriggerTaskCommand
// (CS_20007), which the Dev. Dock sends once the task opens
// (ShipBluePrintMediator.ON_TASK_OPEN): the research must be running, the
// task's time come (getTaskOpenTimeStamp, blueprintTaskDrift allowed) and
// its open_need tasks claimed (TaskProxy:isFinishPrevTasks). A task
// already accepted (a research resumed) is sent back as it is.
func (tx *Tx) triggerBlueprintTask(id uint32, now time.Time) (*protobuf.TASK_ADD, error) {
	config := blueprintConfigs[blueprintOfTask[id]]
	row, err := blueprintRow(tx.ctx, tx.Queries, tx.commanderID, config.ID)
	if err != nil {
		return nil, err
	}
	opens, ok := config.taskOpensAt(id, row.StartTime)
	if !ok || row.ShipID != 0 || row.StartTime == 0 || now.Unix()+blueprintTaskDrift < opens {
		return nil, errRefused
	}
	task, err := tx.GetTask(tx.ctx, db.GetTaskParams{CommanderID: tx.commanderID, TaskID: id})
	if err == nil {
		return &protobuf.TASK_ADD{Id: proto.Uint32(task.TaskID), Progress: proto.Uint32(task.Progress), AcceptTime: proto.Uint32(task.AcceptTime), SubmitTime: proto.Uint32(task.SubmitTime)}, nil
	} else if !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}
	if done, err := tx.tasksClaimed(gamedata.IDs(taskConfigs[id].OpenNeed)); err != nil || !done {
		return nil, cmp.Or(err, errRefused)
	}
	added, err := acceptTasks(tx.ctx, tx.Queries, tx.commanderID, []uint32{id}, uint32(now.Unix()))
	if err != nil {
		return nil, err
	}
	if added == nil {
		return nil, errRefused
	}
	return added.Info[0], nil
}

// blueprintOpenTasksAt are the research ships' opening tasks
// (unlock_task_open_condition) a commander at level gets: the Dev. Dock
// opens at level 30 (open_systems_limited, the tasks' level) and reads them
// from TaskProxy for every research ship (ShipBluePrint:isFinishPrevTask,
// ShipBluePrintScene's lock panel, which fails on a missing one).
func blueprintOpenTasksAt(level uint32) []uint32 {
	var ids []uint32
	for _, id := range blueprintOpenTasks {
		if config, ok := taskConfigs[id]; ok && config.Level <= level {
			ids = append(ids, id)
		}
	}
	return ids
}

// syncBlueprintTasks gives the commander the research ships' opening tasks
// of their level at login, before SC_20001 lists them, then brings their
// TaskSubShipGroups tasks to the ship groups owned.
func (game *Game) syncBlueprintTasks(ctx context.Context, commanderID, level uint32) error {
	return game.DB.InTx(ctx, func(queries *db.Queries) error {
		if _, err := acceptTasks(ctx, queries, commanderID, blueprintOpenTasksAt(level), uint32(time.Now().Unix())); err != nil {
			return err
		}
		tasks, err := queries.ListTasks(ctx, commanderID)
		if err != nil {
			return err
		}
		var groups []db.CommanderShipGroup
		for _, task := range tasks {
			config := taskConfigs[task.TaskID]
			if config.SubType != TaskSubShipGroups {
				continue
			}
			if groups == nil {
				if groups, err = queries.ListShipGroups(ctx, commanderID); err != nil {
					return err
				}
			}
			var pairs [][]uint32
			_ = json.Unmarshal(config.TargetID, &pairs)
			var owned uint32
			for _, group := range groups {
				template := shipTemplates[group.GroupID*10+1]
				if slices.ContainsFunc(pairs, func(pair []uint32) bool {
					return len(pair) == 2 && pair[0] == template.Nationality && pair[1] == template.Type
				}) {
					owned++
				}
			}
			if err := queries.SetTaskProgress(ctx, db.SetTaskProgressParams{Progress: min(owned, config.TargetNum), CommanderID: commanderID, TaskID: task.TaskID}); err != nil {
				return err
			}
		}
		return nil
	})
}
