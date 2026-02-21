package island

import (
	"testing"

	"google.golang.org/protobuf/proto"

	"github.com/ggmolly/belfast/internal/orm"
	"github.com/ggmolly/belfast/internal/protobuf"
)

func seedIslandTaskFlowConfig(t *testing.T) {
	t.Helper()
	seedConfigEntry(t, islandTaskCategory, "7001", `{"id":7001,"type":8,"complete_type":3,"trigger_type":2,"unlock_time":"always","unlock_condition":[],"link_task":[],"target_id":[17001],"reward_show":[[41,2000,3]]}`)
	seedConfigEntry(t, islandTaskCategory, "7002", `{"id":7002,"type":8,"complete_type":3,"trigger_type":2,"unlock_time":"always","unlock_condition":[[2,7001]],"link_task":[7001],"target_id":[17002],"reward_show":[[41,2000,2]]}`)
	seedConfigEntry(t, islandTaskCategory, "7003", `{"id":7003,"type":8,"complete_type":3,"trigger_type":1,"unlock_time":"always","unlock_condition":[],"link_task":[],"target_id":[17003],"reward_show":[[41,2000,1]]}`)
	seedConfigEntry(t, islandTaskCategory, "7004", `{"id":7004,"type":4,"complete_type":3,"trigger_type":2,"unlock_time":"always","unlock_condition":[],"link_task":[],"target_id":[17001],"reward_show":[[41,2000,4]]}`)
	seedConfigEntry(t, islandTaskTargetCategory, "17001", `{"id":17001,"target_num":2}`)
	seedConfigEntry(t, islandTaskTargetCategory, "17002", `{"id":17002,"target_num":1}`)
	seedConfigEntry(t, islandTaskTargetCategory, "17003", `{"id":17003,"target_num":1}`)
	seedConfigEntry(t, islandSeasonCategory, "1", `{"id":1,"task_list":[7001,7002]}`)
}

func TestIslandAcceptTaskAcceptsEligibleAndDedupes(t *testing.T) {
	client := setupPlayerUpdateTest(t)
	clearTable(t, &orm.IslandTaskProgress{})
	clearTable(t, &orm.ConfigEntry{})
	seedIslandTaskFlowConfig(t)

	payload := protobuf.CS_21032{TaskIdList: []uint32{7001, 7001, 7003, 7004, 9999}}
	buffer, err := proto.Marshal(&payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	if _, _, err := IslandAcceptTask(&buffer, client); err != nil {
		t.Fatalf("accept task failed: %v", err)
	}

	var response protobuf.SC_21033
	decodeResponse(t, client, &response)
	if len(response.GetTaskList()) != 1 || response.GetTaskList()[0].GetId() != 7001 {
		t.Fatalf("expected only task 7001 accepted, got %+v", response.GetTaskList())
	}
	if len(response.GetTaskList()[0].GetProcessList()) != 1 || response.GetTaskList()[0].GetProcessList()[0].GetTargetCount() != 0 {
		t.Fatalf("expected initialized process list, got %+v", response.GetTaskList()[0].GetProcessList())
	}
}

func TestIslandUpdateTaskProgressRejectsZeroTaskID(t *testing.T) {
	client := setupPlayerUpdateTest(t)
	clearTable(t, &orm.IslandTaskProgress{})
	clearTable(t, &orm.ConfigEntry{})
	seedIslandTaskFlowConfig(t)

	seedStateErr := orm.WithIslandTaskProgressTx(client.Commander.CommanderID, nowUTC(), func(state *orm.IslandTaskProgress) error {
		state.ActiveTasks = []orm.IslandTaskEntry{
			{TaskID: 7001, Timestamp: 1, ProcessList: []orm.IslandTaskTargetProcess{{TargetID: 17001, TargetCount: 0}}},
			{TaskID: 7002, Timestamp: 2, ProcessList: []orm.IslandTaskTargetProcess{{TargetID: 17002, TargetCount: 0}}},
		}
		return nil
	})
	if seedStateErr != nil {
		t.Fatalf("seed island state: %v", seedStateErr)
	}

	payload := protobuf.CS_21036{TaskId: proto.Uint32(0), TargetId: proto.Uint32(17001), TargetCount: proto.Uint32(1)}
	buffer, err := proto.Marshal(&payload)
	if err != nil {
		t.Fatalf("marshal update payload: %v", err)
	}
	if _, _, err := IslandUpdateTaskProgress(&buffer, client); err != nil {
		t.Fatalf("update task failed unexpectedly: %v", err)
	}

	var response protobuf.SC_21037
	decodeResponse(t, client, &response)
	if response.GetResult() == 0 {
		t.Fatalf("expected failure result for task_id=0")
	}
	if len(response.GetTaskList()) != 0 {
		t.Fatalf("expected no updated tasks for task_id=0, got %+v", response.GetTaskList())
	}

	state, err := orm.LoadIslandTaskProgress(client.Commander.CommanderID, nowUTC())
	if err != nil {
		t.Fatalf("load island state: %v", err)
	}
	for _, entry := range state.ActiveTasks {
		for _, process := range entry.ProcessList {
			if process.TargetCount != 0 {
				t.Fatalf("expected no progress mutation, task %d process %+v", entry.TaskID, process)
			}
		}
	}
}

func TestIslandUpdateTaskProgressAndSubmit(t *testing.T) {
	client := setupPlayerUpdateTest(t)
	clearTable(t, &orm.IslandTaskProgress{})
	clearTable(t, &orm.ConfigEntry{})
	clearTable(t, &orm.IslandInventory{})
	seedIslandTaskFlowConfig(t)

	acceptPayload := protobuf.CS_21032{TaskIdList: []uint32{7001}}
	acceptBuffer, err := proto.Marshal(&acceptPayload)
	if err != nil {
		t.Fatalf("marshal accept payload: %v", err)
	}
	if _, _, err := IslandAcceptTask(&acceptBuffer, client); err != nil {
		t.Fatalf("accept task failed: %v", err)
	}
	client.Buffer.Reset()

	updatePayload := protobuf.CS_21036{TaskId: proto.Uint32(7001), TargetId: proto.Uint32(17001), TargetCount: proto.Uint32(5)}
	updateBuffer, err := proto.Marshal(&updatePayload)
	if err != nil {
		t.Fatalf("marshal update payload: %v", err)
	}
	if _, _, err := IslandUpdateTaskProgress(&updateBuffer, client); err != nil {
		t.Fatalf("update task failed: %v", err)
	}

	var updateResponse protobuf.SC_21037
	decodeResponse(t, client, &updateResponse)
	if updateResponse.GetResult() != 0 {
		t.Fatalf("expected success result, got %d", updateResponse.GetResult())
	}
	if len(updateResponse.GetTaskList()) != 1 || updateResponse.GetTaskList()[0].GetProcessList()[0].GetTargetCount() != 2 {
		t.Fatalf("expected clamped target count 2, got %+v", updateResponse.GetTaskList())
	}
	client.Buffer.Reset()

	submitPayload := protobuf.CS_21038{TaskId: proto.Uint32(7001)}
	submitBuffer, err := proto.Marshal(&submitPayload)
	if err != nil {
		t.Fatalf("marshal submit payload: %v", err)
	}
	if _, _, err := IslandSubmitTask(&submitBuffer, client); err != nil {
		t.Fatalf("submit task failed: %v", err)
	}

	var submitResponse protobuf.SC_21039
	decodeResponse(t, client, &submitResponse)
	if submitResponse.GetResult() != 0 {
		t.Fatalf("expected submit success, got %d", submitResponse.GetResult())
	}
	if len(submitResponse.GetDropList()) != 1 || submitResponse.GetDropList()[0].GetType() != 41 || submitResponse.GetDropList()[0].GetNumber() != 3 {
		t.Fatalf("unexpected submit drops: %+v", submitResponse.GetDropList())
	}

	state, err := orm.LoadIslandTaskProgress(client.Commander.CommanderID, nowUTC())
	if err != nil {
		t.Fatalf("load island state: %v", err)
	}
	if containsIslandTaskID(extractTaskIDs(state.ActiveTasks), 7001) {
		t.Fatalf("expected submitted task removed from active list")
	}
	if !containsIslandTaskID(state.FinishedTaskIDs, 7001) {
		t.Fatalf("expected submitted task in finished list")
	}
}

func TestIslandSubmitTaskOneStepFailsClosedAndMergesDrops(t *testing.T) {
	client := setupPlayerUpdateTest(t)
	clearTable(t, &orm.IslandTaskProgress{})
	clearTable(t, &orm.ConfigEntry{})
	clearTable(t, &orm.IslandInventory{})
	seedIslandTaskFlowConfig(t)

	seedStateErr := orm.WithIslandTaskProgressTx(client.Commander.CommanderID, nowUTC(), func(state *orm.IslandTaskProgress) error {
		state.ActiveTasks = []orm.IslandTaskEntry{
			{TaskID: 7001, Timestamp: 1, ProcessList: []orm.IslandTaskTargetProcess{{TargetID: 17001, TargetCount: 2}}},
			{TaskID: 7002, Timestamp: 2, ProcessList: []orm.IslandTaskTargetProcess{{TargetID: 17002, TargetCount: 1}}},
		}
		return nil
	})
	if seedStateErr != nil {
		t.Fatalf("seed island state: %v", seedStateErr)
	}

	invalidPayload := protobuf.CS_21041{TaskIds: []uint32{7001, 9999}}
	invalidBuffer, err := proto.Marshal(&invalidPayload)
	if err != nil {
		t.Fatalf("marshal invalid one-step payload: %v", err)
	}
	if _, _, err := IslandSubmitTaskOneStep(&invalidBuffer, client); err != nil {
		t.Fatalf("one-step submit failed unexpectedly: %v", err)
	}
	var invalidResponse protobuf.SC_21042
	decodeResponse(t, client, &invalidResponse)
	if invalidResponse.GetResult() == 0 {
		t.Fatalf("expected failure for invalid mixed payload")
	}
	client.Buffer.Reset()

	validPayload := protobuf.CS_21041{TaskIds: []uint32{7001, 7002, 7002}}
	validBuffer, err := proto.Marshal(&validPayload)
	if err != nil {
		t.Fatalf("marshal valid one-step payload: %v", err)
	}
	if _, _, err := IslandSubmitTaskOneStep(&validBuffer, client); err != nil {
		t.Fatalf("one-step submit failed: %v", err)
	}
	var validResponse protobuf.SC_21042
	decodeResponse(t, client, &validResponse)
	if validResponse.GetResult() != 0 {
		t.Fatalf("expected one-step success, got %d", validResponse.GetResult())
	}
	if len(validResponse.GetDropList()) != 1 || validResponse.GetDropList()[0].GetType() != 41 || validResponse.GetDropList()[0].GetNumber() != 5 {
		t.Fatalf("expected merged island item drop count 5, got %+v", validResponse.GetDropList())
	}
}

func extractTaskIDs(entries []orm.IslandTaskEntry) []uint32 {
	ids := make([]uint32, 0, len(entries))
	for _, entry := range entries {
		ids = append(ids, entry.TaskID)
	}
	return ids
}
