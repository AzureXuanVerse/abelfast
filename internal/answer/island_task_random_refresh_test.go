package answer

import (
	"testing"
	"time"

	"google.golang.org/protobuf/proto"

	"github.com/ggmolly/belfast/internal/orm"
	"github.com/ggmolly/belfast/internal/protobuf"
)

func seedIslandTaskConfig(t *testing.T) {
	t.Helper()
	seedConfigEntry(t, islandTaskCategory, "40101001", `{"id":40101001,"type":4,"unlock_condition":[],"unlock_time":"always","target_id":[50101]}`)
	seedConfigEntry(t, islandTaskCategory, "40102001", `{"id":40102001,"type":4,"unlock_condition":[],"unlock_time":"always","target_id":[50102]}`)
	seedConfigEntry(t, islandTaskCategory, "40103001", `{"id":40103001,"type":4,"unlock_condition":[],"unlock_time":"always","target_id":[50103]}`)
	seedConfigEntry(t, islandTaskTargetCategory, "50101", `{"id":50101,"target_num":2}`)
	seedConfigEntry(t, islandTaskTargetCategory, "50102", `{"id":50102,"target_num":4}`)
	seedConfigEntry(t, islandTaskTargetCategory, "50103", `{"id":50103,"target_num":6}`)
}

func TestIslandRandomTaskRefreshAppliesDeltaAndPersistsState(t *testing.T) {
	client := setupPlayerUpdateTest(t)
	clearTable(t, &orm.IslandTaskProgress{})
	clearTable(t, &orm.ConfigEntry{})
	seedIslandTaskConfig(t)

	now := uint32(time.Now().UTC().Unix())
	err := orm.WithIslandTaskProgressTx(client.Commander.CommanderID, time.Now().UTC(), func(state *orm.IslandTaskProgress) error {
		state.ActiveTasks = []orm.IslandTaskEntry{{TaskID: 99999999, Timestamp: now - 20}, {TaskID: 40101001, Timestamp: now - 10}}
		state.FinishedTaskIDs = []uint32{88888888}
		state.RandomTaskWindows = []orm.IslandTaskEntry{{TaskID: 40102001, Timestamp: now - 1}}
		state.FutureTaskWindows = append([]orm.IslandTaskEntry{}, state.RandomTaskWindows...)
		state.WeekDailyTaskNum = 1
		return nil
	})
	if err != nil {
		t.Fatalf("seed island state: %v", err)
	}

	payload := protobuf.CS_21030{Type: proto.Uint32(0)}
	buffer, err := proto.Marshal(&payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	if _, _, err := IslandRandomTaskRefresh(&buffer, client); err != nil {
		t.Fatalf("refresh failed: %v", err)
	}

	var response protobuf.SC_21031
	decodeResponse(t, client, &response)
	if len(response.GetRemoveTaskList()) != 1 || response.GetRemoveTaskList()[0] != 99999999 {
		t.Fatalf("expected invalid active task removal, got %v", response.GetRemoveTaskList())
	}
	if len(response.GetRemoveTaskFinish()) != 1 || response.GetRemoveTaskFinish()[0] != 88888888 {
		t.Fatalf("expected invalid finished task removal, got %v", response.GetRemoveTaskFinish())
	}
	if len(response.GetTaskList()) == 0 {
		t.Fatalf("expected promoted/added tasks in response")
	}
	for _, task := range response.GetTaskList() {
		if task.GetId() == 0 || task.GetTimestamp() == 0 {
			t.Fatalf("expected task id and timestamp populated: %+v", task)
		}
		if len(task.GetProcessList()) == 0 || task.GetProcessList()[0].GetTargetCount() == 0 {
			t.Fatalf("expected populated process list for task: %+v", task)
		}
	}

	state, err := orm.LoadIslandTaskProgress(client.Commander.CommanderID, time.Now().UTC())
	if err != nil {
		t.Fatalf("load island state: %v", err)
	}
	for _, active := range state.ActiveTasks {
		if active.TaskID == 99999999 {
			t.Fatalf("expected removed task to be absent from persisted state")
		}
	}
	for _, finishedID := range state.FinishedTaskIDs {
		if finishedID == 88888888 {
			t.Fatalf("expected removed finished id to be absent from persisted state")
		}
	}
	if state.WeekDailyTaskNum != 2 {
		t.Fatalf("expected daily refresh count incremented to 2, got %d", state.WeekDailyTaskNum)
	}
	if state.TraceTaskID == 0 || state.TraceDailyTaskID == 0 {
		t.Fatalf("expected trace ids to be set, got trace=%d daily=%d", state.TraceTaskID, state.TraceDailyTaskID)
	}
}

func TestIslandRandomTaskRefreshReturnsErrorOnMalformedPayload(t *testing.T) {
	client := setupPlayerUpdateTest(t)
	clearTable(t, &orm.IslandTaskProgress{})

	badPayload := []byte{0xff, 0x01, 0x00}
	_, packetID, err := IslandRandomTaskRefresh(&badPayload, client)
	if err == nil {
		t.Fatalf("expected malformed payload to fail")
	}
	if packetID != 21031 {
		t.Fatalf("expected response packet id 21031, got %d", packetID)
	}

	count := queryAnswerTestInt64(t, "SELECT COUNT(*) FROM island_task_progresses WHERE commander_id = $1", int64(client.Commander.CommanderID))
	if count != 0 {
		t.Fatalf("expected no state write on decode failure")
	}
}

func TestIslandRandomTaskRefreshReturnsNoopForNonZeroType(t *testing.T) {
	client := setupPlayerUpdateTest(t)
	clearTable(t, &orm.IslandTaskProgress{})
	clearTable(t, &orm.ConfigEntry{})
	seedIslandTaskConfig(t)

	payload := protobuf.CS_21030{Type: proto.Uint32(3)}
	buffer, err := proto.Marshal(&payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	if _, _, err := IslandRandomTaskRefresh(&buffer, client); err != nil {
		t.Fatalf("refresh failed: %v", err)
	}

	var response protobuf.SC_21031
	decodeResponse(t, client, &response)
	if len(response.GetRemoveTaskList()) != 0 || len(response.GetRemoveTaskFinish()) != 0 || len(response.GetTaskList()) != 0 || len(response.GetTaskListRandom()) != 0 {
		t.Fatalf("expected empty delta for non-zero type, got %+v", response)
	}
}

func TestLoadIslandTaskRefreshConfigFallsBackToLowercaseCategory(t *testing.T) {
	taskEntry := orm.ConfigEntry{Category: islandTaskCategoryLC, Key: "40101001", Data: []byte(`{"id":40101001,"type":4,"unlock_condition":[],"unlock_time":"always","target_id":[50101]}`)}
	targetEntry := orm.ConfigEntry{Category: islandTaskTargetCategoryLC, Key: "50101", Data: []byte(`{"id":50101,"target_num":9}`)}

	entriesByCategory := map[string][]orm.ConfigEntry{
		islandTaskCategory:       {},
		islandTaskCategoryLC:     {taskEntry},
		islandTaskTargetCategory: {},
		islandTaskTargetCategoryLC: {
			targetEntry,
		},
	}

	taskEntries, err := listConfigEntriesWithFallback(islandTaskCategory, islandTaskCategoryLC, func(category string) ([]orm.ConfigEntry, error) {
		return entriesByCategory[category], nil
	})
	if err != nil {
		t.Fatalf("load task config entries: %v", err)
	}
	if len(taskEntries) != 1 || taskEntries[0].Category != islandTaskCategoryLC {
		t.Fatalf("expected lowercase task category fallback, got %+v", taskEntries)
	}

	targetEntries, err := listConfigEntriesWithFallback(islandTaskTargetCategory, islandTaskTargetCategoryLC, func(category string) ([]orm.ConfigEntry, error) {
		return entriesByCategory[category], nil
	})
	if err != nil {
		t.Fatalf("load target config entries: %v", err)
	}
	if len(targetEntries) != 1 || targetEntries[0].Category != islandTaskTargetCategoryLC {
		t.Fatalf("expected lowercase target category fallback, got %+v", targetEntries)
	}
}

func TestRefreshIslandRandomTasksRespectsActiveCapWhenPromotingExpiredWindows(t *testing.T) {
	now := uint32(1000)
	config := &islandTaskRefreshConfig{
		tasksByID: map[uint32]islandTaskTemplate{
			40101001: {ID: 40101001, Type: islandRandomTaskType, UnlockTime: "always", TargetID: []uint32{50101}},
			40102001: {ID: 40102001, Type: islandRandomTaskType, UnlockTime: "always", TargetID: []uint32{50102}},
			40103001: {ID: 40103001, Type: islandRandomTaskType, UnlockTime: "always", TargetID: []uint32{50103}},
			40104001: {ID: 40104001, Type: islandRandomTaskType, UnlockTime: "always", TargetID: []uint32{50104}},
			40105001: {ID: 40105001, Type: islandRandomTaskType, UnlockTime: "always", TargetID: []uint32{50105}},
		},
		randomTasks: []uint32{40101001, 40102001, 40103001, 40104001, 40105001},
		targets: map[uint32]uint32{
			50101: 1,
			50102: 1,
			50103: 1,
			50104: 1,
			50105: 1,
		},
	}
	state := &orm.IslandTaskProgress{
		ActiveTasks: []orm.IslandTaskEntry{
			{TaskID: 40101001, Timestamp: now - 100},
			{TaskID: 40102001, Timestamp: now - 100},
		},
		RandomTaskWindows: []orm.IslandTaskEntry{
			{TaskID: 40103001, Timestamp: now - 10},
			{TaskID: 40104001, Timestamp: now - 5},
			{TaskID: 40105001, Timestamp: now - 1},
		},
		FutureTaskWindows: []orm.IslandTaskEntry{},
	}

	delta := refreshIslandRandomTasks(state, config, now)
	if len(delta.GetTaskList()) != 1 || delta.GetTaskList()[0].GetId() != 40103001 {
		t.Fatalf("expected only one expired window to promote, got %+v", delta.GetTaskList())
	}
	if len(state.ActiveTasks) != 3 {
		t.Fatalf("expected active task cap of 3, got %d", len(state.ActiveTasks))
	}
	if len(state.RandomTaskWindows) != 2 {
		t.Fatalf("expected remaining expired windows to stay queued, got %d", len(state.RandomTaskWindows))
	}
	if state.RandomTaskWindows[0].TaskID != 40104001 || state.RandomTaskWindows[1].TaskID != 40105001 {
		t.Fatalf("unexpected queued tasks after capped promotion: %+v", state.RandomTaskWindows)
	}
}
