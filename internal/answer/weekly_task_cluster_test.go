package answer

import (
	"testing"

	"github.com/ggmolly/belfast/internal/orm"
	"github.com/ggmolly/belfast/internal/protobuf"
	"google.golang.org/protobuf/proto"
)

func seedWeeklyTaskConfig(t *testing.T) {
	t.Helper()
	seedConfigEntry(t, weeklyTaskTemplateCategory, "10001", `{"id":10001,"sub_type":20,"target_num":5,"award_display":[8,59012,10]}`)
	seedConfigEntry(t, weeklyTaskTemplateCategory, "10002", `{"id":10002,"sub_type":20,"target_num":10,"award_display":[8,59012,20]}`)
	seedConfigEntry(t, weeklyTaskTemplateCategory, "20001", `{"id":20001,"sub_type":11,"target_num":3,"award_display":[8,59012,15]}`)
	seedConfigEntry(t, weeklyTaskTemplateCategory, "20002", `{"id":20002,"sub_type":11,"target_num":6,"award_display":[8,59012,25]}`)
	seedConfigEntry(t, gamesetCategory, "weekly_target", `{"description":[10,20,40],"key_value":0}`)
	seedConfigEntry(t, gamesetCategory, "weekly_drop_client", `{"description":[[[1,1,100]],[[2,20001,2]],[[1,2,200]]],"key_value":0}`)
}

func seedWeeklyTaskState(t *testing.T, commanderID uint32, pt uint32, rewardLv uint32, tasks []orm.WeeklyTaskEntry) {
	t.Helper()
	err := orm.WithWeeklyTaskProgressTx(commanderID, func(state *orm.WeeklyTaskProgress) error {
		state.Pt = pt
		state.RewardLv = rewardLv
		state.Tasks = tasks
		return nil
	})
	if err != nil {
		t.Fatalf("seed weekly state: %v", err)
	}
}

func TestWeeklyMissionsReadsPersistedState(t *testing.T) {
	client := setupPlayerUpdateTest(t)
	clearTable(t, &orm.WeeklyTaskProgress{})
	clearTable(t, &orm.ConfigEntry{})
	seedWeeklyTaskConfig(t)
	seedWeeklyTaskState(t, client.Commander.CommanderID, 77, 2, []orm.WeeklyTaskEntry{{ID: 10002, Progress: 8}})

	buffer := []byte{}
	if _, _, err := WeeklyMissions(&buffer, client); err != nil {
		t.Fatalf("weekly missions failed: %v", err)
	}

	var response protobuf.SC_20101
	decodeResponse(t, client, &response)
	if response.GetInfo().GetPt() != 77 {
		t.Fatalf("expected pt 77, got %d", response.GetInfo().GetPt())
	}
	if response.GetInfo().GetRewardLv() != 2 {
		t.Fatalf("expected reward lv 2, got %d", response.GetInfo().GetRewardLv())
	}
	if len(response.GetInfo().GetTask()) != 1 || response.GetInfo().GetTask()[0].GetId() != 10002 {
		t.Fatalf("unexpected task list: %+v", response.GetInfo().GetTask())
	}
}

func TestWeeklyMissionsSeedsInitialTasksWhenEmpty(t *testing.T) {
	client := setupPlayerUpdateTest(t)
	clearTable(t, &orm.WeeklyTaskProgress{})
	clearTable(t, &orm.ConfigEntry{})
	seedWeeklyTaskConfig(t)
	seedWeeklyTaskState(t, client.Commander.CommanderID, 0, 0, []orm.WeeklyTaskEntry{})

	buffer := []byte{}
	if _, _, err := WeeklyMissions(&buffer, client); err != nil {
		t.Fatalf("weekly missions failed: %v", err)
	}

	var response protobuf.SC_20101
	decodeResponse(t, client, &response)
	if len(response.GetInfo().GetTask()) == 0 {
		t.Fatalf("expected seeded weekly tasks in response")
	}

	state, err := orm.LoadWeeklyTaskProgress(client.Commander.CommanderID, nowUTC())
	if err != nil {
		t.Fatalf("load weekly state: %v", err)
	}
	if len(state.Tasks) == 0 {
		t.Fatalf("expected seeded weekly tasks to be persisted")
	}
}

func TestSubmitWeeklyTaskSuccess(t *testing.T) {
	client := setupPlayerUpdateTest(t)
	clearTable(t, &orm.WeeklyTaskProgress{})
	clearTable(t, &orm.ConfigEntry{})
	seedWeeklyTaskConfig(t)
	seedWeeklyTaskState(t, client.Commander.CommanderID, 0, 0, []orm.WeeklyTaskEntry{{ID: 10001, Progress: 5}})

	payload := protobuf.CS_20106{Id: proto.Uint32(10001)}
	buffer, err := proto.Marshal(&payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	if _, _, err := SubmitWeeklyTask(&buffer, client); err != nil {
		t.Fatalf("submit weekly task failed: %v", err)
	}

	var response protobuf.SC_20107
	decodeResponse(t, client, &response)
	if response.GetResult() != weeklyTaskSuccessResult {
		t.Fatalf("expected success result, got %d", response.GetResult())
	}
	if response.GetNext() == nil || response.GetNext().GetId() != 10002 || response.GetNext().GetProgress() != 5 {
		t.Fatalf("unexpected next task: %+v", response.GetNext())
	}

	state, err := orm.LoadWeeklyTaskProgress(client.Commander.CommanderID, nowUTC())
	if err != nil {
		t.Fatalf("load weekly state: %v", err)
	}
	if state.Pt != 10 {
		t.Fatalf("expected pt 10, got %d", state.Pt)
	}
	if len(state.Tasks) != 1 || state.Tasks[0].ID != 10002 || state.Tasks[0].Progress != 5 {
		t.Fatalf("unexpected persisted tasks: %+v", state.Tasks)
	}
}

func TestSubmitWeeklyTaskBatchRejectsDuplicateIDs(t *testing.T) {
	client := setupPlayerUpdateTest(t)
	clearTable(t, &orm.WeeklyTaskProgress{})
	clearTable(t, &orm.ConfigEntry{})
	seedWeeklyTaskConfig(t)
	seedWeeklyTaskState(t, client.Commander.CommanderID, 3, 0, []orm.WeeklyTaskEntry{{ID: 10001, Progress: 5}})

	payload := protobuf.CS_20108{Id: []uint32{10001, 10001}}
	buffer, err := proto.Marshal(&payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	if _, _, err := SubmitWeeklyTaskBatch(&buffer, client); err != nil {
		t.Fatalf("submit batch failed: %v", err)
	}

	var response protobuf.SC_20109
	decodeResponse(t, client, &response)
	if response.GetResult() == weeklyTaskSuccessResult {
		t.Fatalf("expected duplicate batch to fail")
	}
	if response.GetPt() != 3 {
		t.Fatalf("expected unchanged pt 3, got %d", response.GetPt())
	}

	state, err := orm.LoadWeeklyTaskProgress(client.Commander.CommanderID, nowUTC())
	if err != nil {
		t.Fatalf("load weekly state: %v", err)
	}
	if state.Pt != 3 || len(state.Tasks) != 1 || state.Tasks[0].ID != 10001 {
		t.Fatalf("expected unchanged state, got %+v", state)
	}
}

func TestSubmitWeeklyTaskBatchSuccess(t *testing.T) {
	client := setupPlayerUpdateTest(t)
	clearTable(t, &orm.WeeklyTaskProgress{})
	clearTable(t, &orm.ConfigEntry{})
	seedWeeklyTaskConfig(t)
	seedWeeklyTaskState(t, client.Commander.CommanderID, 0, 0, []orm.WeeklyTaskEntry{{ID: 10001, Progress: 5}, {ID: 20001, Progress: 3}})

	payload := protobuf.CS_20108{Id: []uint32{10001, 20001}}
	buffer, err := proto.Marshal(&payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	if _, _, err := SubmitWeeklyTaskBatch(&buffer, client); err != nil {
		t.Fatalf("submit batch failed: %v", err)
	}

	var response protobuf.SC_20109
	decodeResponse(t, client, &response)
	if response.GetResult() != weeklyTaskSuccessResult {
		t.Fatalf("expected success result, got %d", response.GetResult())
	}
	if response.GetPt() != 25 {
		t.Fatalf("expected pt 25, got %d", response.GetPt())
	}
	if len(response.GetNext()) != 2 {
		t.Fatalf("expected 2 replacement tasks, got %d", len(response.GetNext()))
	}

	state, err := orm.LoadWeeklyTaskProgress(client.Commander.CommanderID, nowUTC())
	if err != nil {
		t.Fatalf("load weekly state: %v", err)
	}
	if state.Pt != 25 {
		t.Fatalf("expected pt 25, got %d", state.Pt)
	}
	if len(state.Tasks) != 2 {
		t.Fatalf("expected 2 persisted tasks, got %d", len(state.Tasks))
	}
}

func TestClaimWeeklyTaskProgressRewardSuccess(t *testing.T) {
	client := setupPlayerUpdateTest(t)
	initCommanderMaps(client)
	clearTable(t, &orm.WeeklyTaskProgress{})
	clearTable(t, &orm.ConfigEntry{})
	seedWeeklyTaskConfig(t)
	seedWeeklyTaskState(t, client.Commander.CommanderID, 15, 0, []orm.WeeklyTaskEntry{})

	payload := protobuf.CS_20110{Id: proto.Uint32(0)}
	buffer, err := proto.Marshal(&payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	if _, _, err := ClaimWeeklyTaskProgressReward(&buffer, client); err != nil {
		t.Fatalf("claim progress reward failed: %v", err)
	}

	var response protobuf.SC_20111
	decodeResponse(t, client, &response)
	if response.GetResult() != weeklyTaskSuccessResult {
		t.Fatalf("expected success result, got %d", response.GetResult())
	}
	if len(response.GetAwardList()) != 1 || response.GetAwardList()[0].GetType() != 1 || response.GetAwardList()[0].GetId() != 1 || response.GetAwardList()[0].GetNumber() != 100 {
		t.Fatalf("unexpected award list: %+v", response.GetAwardList())
	}

	state, err := orm.LoadWeeklyTaskProgress(client.Commander.CommanderID, nowUTC())
	if err != nil {
		t.Fatalf("load weekly state: %v", err)
	}
	if state.RewardLv != 1 {
		t.Fatalf("expected reward level 1, got %d", state.RewardLv)
	}

	coins := queryAnswerTestInt64(t, "SELECT amount FROM owned_resources WHERE commander_id = $1 AND resource_id = 1", int64(client.Commander.CommanderID))
	if coins <= 0 {
		t.Fatalf("expected resource reward to be applied, got %d", coins)
	}
}

func TestClaimWeeklyTaskProgressRewardRejectsMismatchedID(t *testing.T) {
	client := setupPlayerUpdateTest(t)
	initCommanderMaps(client)
	clearTable(t, &orm.WeeklyTaskProgress{})
	clearTable(t, &orm.ConfigEntry{})
	seedWeeklyTaskConfig(t)
	seedWeeklyTaskState(t, client.Commander.CommanderID, 15, 0, []orm.WeeklyTaskEntry{{ID: 10001, Progress: 5}})

	payload := protobuf.CS_20110{Id: proto.Uint32(1)}
	buffer, err := proto.Marshal(&payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	if _, _, err := ClaimWeeklyTaskProgressReward(&buffer, client); err != nil {
		t.Fatalf("claim progress reward failed: %v", err)
	}

	var response protobuf.SC_20111
	decodeResponse(t, client, &response)
	if response.GetResult() == weeklyTaskSuccessResult {
		t.Fatalf("expected mismatched id to fail")
	}

	state, err := orm.LoadWeeklyTaskProgress(client.Commander.CommanderID, nowUTC())
	if err != nil {
		t.Fatalf("load weekly state: %v", err)
	}
	if state.RewardLv != 0 {
		t.Fatalf("expected reward level unchanged, got %d", state.RewardLv)
	}
}
