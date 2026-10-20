package game

import (
	"encoding/json"
	"slices"
	"unicode/utf8"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/protobuf"
)

// TB's goals, tasks and bond in Project Identity: TB: the growth goal of
// each stage (child_target_set, EducateTargetSetLayer) hands its tasks out,
// the main tasks come with time, the wishes (mind tasks) week by week; a
// finished task is claimed for its drop_display. Wishes give favor exp,
// which levels TB's favor up (child_data favor_exp).

// handleEducateSetTarget answers EducateSetTargetCommand (CS_27019): the
// growth goal of the coming stage (EducateTargetSetLayer offers the
// child_target_set rows of stage 1 before the first goal, of TB's stage + 1
// after; stage 4's need her personality), on a goal setting day. Its tasks
// are handed out (the previous goal's still open go), its award can be
// claimed again, and the "set a growth goal" tasks count it.
func (game *Game) handleEducateSetTarget(client *connection.Client, req *protobuf.CS_27019) error {
	err := game.educateTx(client, func(tx *Tx, state *educateState) error {
		target, ok := educateTargets[req.GetId()]
		if !ok || state.Ending || !state.targetDue() {
			return errRefused
		}
		stage := uint32(1)
		if state.Target != 0 {
			stage = state.stage() + 1
		}
		if target.Stage != stage {
			return errRefused
		}
		var condition [][]uint32
		if json.Unmarshal(target.Condition, &condition) == nil && len(condition) >= 2 && len(condition[1]) > 0 && condition[1][0] != state.personality() {
			return errRefused
		}
		changes := &educateTaskChanges{}
		if previous, ok := educateTargets[state.Target]; ok {
			for _, id := range previous.IDs {
				state.removeTask(id, changes)
			}
		}
		state.Target = req.GetId()
		state.TargetAward = false
		for _, id := range target.IDs {
			if _, ok := educateTasks[id]; ok {
				state.addTask(id, changes)
			}
		}
		state.count(childTaskSetTarget, 0, 1, changes)
		changes.push(tx, state)
		return nil
	})
	if err != nil && !refused(err) {
		return err
	}
	return client.Send(&protobuf.SC_27020{Result: result(err)})
}

// handleEducateSubmitTask answers EducateSubmitTaskCommand (CS_27023):
// claims a finished task of a system (child_task.type_1) for its
// drop_display; a wish becomes a realized wish, and the task named by the
// default special secretary (secretary_special_ship unlock_type 1) unlocks
// her (EducateProxy:SetSecretaryUnlock).
func (game *Game) handleEducateSubmitTask(client *connection.Client, req *protobuf.CS_27023) error {
	var awards []*protobuf.CHILD_DROP
	err := game.educateTx(client, func(tx *Tx, state *educateState) error {
		id := req.GetId()
		task, ok := educateTasks[id]
		progress, open := state.Tasks[id]
		if !ok || !open || task.System != req.GetSystem() || progress < task.Arg {
			return errRefused
		}
		delete(state.Tasks, id)
		if task.System == childTaskMind {
			state.Wishes = append(state.Wishes, id)
		}
		if id == educateSecretaryTask {
			state.Secretary = true
		}
		awards = childConfigDrops(task.Drop)
		state.apply(awards)
		changes := &educateTaskChanges{}
		state.measureAll(changes)
		changes.push(tx, state)
		return nil
	})
	if err != nil && !refused(err) {
		return err
	}
	if err != nil {
		awards = nil
	}
	return client.Send(&protobuf.SC_27024{Result: result(err), Awards: awards})
}

// handleEducateTargetAward answers EducateGetTargetAwardCommand (CS_27035):
// the growth goal's award (child_target_set drop_display), once per goal,
// when the claimed goal tasks' task_target_progress reach its
// target_progress (EducateTaskProxy:GetOtherTargetTaskProgress).
func (game *Game) handleEducateTargetAward(client *connection.Client, req *protobuf.CS_27035) error {
	var drops []*protobuf.CHILD_DROP
	err := game.educateTx(client, func(tx *Tx, state *educateState) error {
		target, ok := educateTargets[state.Target]
		if !ok || state.TargetAward {
			return errRefused
		}
		progress := uint32(0)
		for _, id := range target.IDs {
			if _, open := state.Tasks[id]; !open {
				progress += educateTasks[id].TargetProgress
			}
		}
		if progress < target.Progress {
			return errRefused
		}
		state.TargetAward = true
		drops = childConfigDrops(target.Drop)
		state.apply(drops)
		changes := &educateTaskChanges{}
		state.measureAll(changes)
		changes.push(tx, state)
		return nil
	})
	if err != nil && !refused(err) {
		return err
	}
	if err != nil {
		drops = nil
	}
	return client.Send(&protobuf.SC_27036{Result: result(err), Drops: drops})
}

// handleEducateTaskProgress answers EducateAddTaskProgressCommand
// (CS_27037): the tasks the client counts itself, entering a site
// (EducateSiteDetailPanel:addTaskProgress) and watching a performance
// (PerformMgr), of one system (type_1). The new progress comes back in
// SC_27025 (EducateTaskProxy:UpdateTask), the reply carries nothing.
func (game *Game) handleEducateTaskProgress(client *connection.Client, req *protobuf.CS_27037) error {
	err := game.educateTx(client, func(tx *Tx, state *educateState) error {
		changes := &educateTaskChanges{}
		for _, entry := range req.GetProgresses() {
			id := entry.GetTaskId()
			task, ok := educateTasks[id]
			progress, open := state.Tasks[id]
			if !ok || !open || task.System != req.GetType_1() || (task.Type != childTaskSiteEnter && task.Type != childTaskPerform) {
				return errRefused
			}
			if progress >= task.Arg {
				continue
			}
			state.Tasks[id] = min(progress+entry.GetProgress(), task.Arg)
			changes.update(id)
		}
		changes.push(tx, state)
		return nil
	})
	if err != nil && !refused(err) {
		return err
	}
	return client.Send(&protobuf.SC_27038{Result: result(err)})
}

// handleEducateUpgradeFavor answers EducateUpgradeFavorCommand (CS_27006,
// EducateScene's heart bubble): TB's favor level goes up once its exp
// reaches favor_exp[level], below favor_level. The exp left is what
// EducateChar:UpgradeFavor computes (the new level's favor_exp taken off).
// What a level gives (child_data trigger_drop) is server content: belfast
// gives the status of that id when it is one (child_buff), a guess.
func (game *Game) handleEducateUpgradeFavor(client *connection.Client, req *protobuf.CS_27006) error {
	var drops []*protobuf.CHILD_DROP
	err := game.educateTx(client, func(tx *Tx, state *educateState) error {
		level := state.FavorLv
		if level >= educateTB.FavorLevel || int(level) > len(educateTB.FavorExp) || state.FavorExp < educateTB.FavorExp[level-1] {
			return errRefused
		}
		state.FavorLv++
		need := uint32(999999)
		if int(state.FavorLv) <= len(educateTB.FavorExp) {
			need = educateTB.FavorExp[state.FavorLv-1]
		}
		state.FavorExp -= min(need, state.FavorExp)
		if int(level) <= len(educateTB.TriggerDrop) {
			if buff := educateTB.TriggerDrop[level-1]; educateBuffs[buff].During > 0 {
				drops = append(drops, childDrop(childDropBuff, buff, 1))
			}
		}
		state.apply(drops)
		return nil
	})
	if err != nil && !refused(err) {
		return err
	}
	if err != nil {
		drops = nil
	}
	return client.Send(&protobuf.SC_27007{Result: result(err), Drops: drops})
}

// handleEducateExtraAttr answers EducateAddExtraAttrCommand (CS_27039,
// EducateExtraAttrLayer): once a game, on the day of gameset
// child_attr_2_add, attr_2_add points to a personality. This version's
// gameset sets no day, so no client offers it.
func (game *Game) handleEducateExtraAttr(client *connection.Client, req *protobuf.CS_27039) error {
	err := game.educateTx(client, func(tx *Tx, state *educateState) error {
		if state.Adjusted || state.Time != educateAttr2AddTime || !slices.Contains(educateTB.Attrs2, req.GetAttrId()) {
			return errRefused
		}
		state.Adjusted = true
		state.Attrs[req.GetAttrId()] += educateTB.Attr2Add
		state.measureAll(nil)
		return nil
	})
	if err != nil && !refused(err) {
		return err
	}
	return client.Send(&protobuf.SC_27040{Result: result(err)})
}

// handleEducateSetCall answers EducateSetCallCommand (CS_27031,
// EducateNewCharLayer on the first visit): what TB calls the commander
// (the client checks it with nameValidityCheck, 4 to 14 width).
func (game *Game) handleEducateSetCall(client *connection.Client, req *protobuf.CS_27031) error {
	err := game.educateTx(client, func(tx *Tx, state *educateState) error {
		name := req.GetName()
		if name == "" || !utf8.ValidString(name) || utf8.RuneCountInString(name) > 14 {
			return errRefused
		}
		state.Name = name
		return nil
	})
	if err != nil && !refused(err) {
		return err
	}
	return client.Send(&protobuf.SC_27032{Result: result(err)})
}
