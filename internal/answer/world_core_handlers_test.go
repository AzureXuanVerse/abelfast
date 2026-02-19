package answer

import (
	"testing"
	"time"

	"github.com/ggmolly/belfast/internal/orm"
	"github.com/ggmolly/belfast/internal/protobuf"
	"google.golang.org/protobuf/proto"
)

func TestWorldActivateAndMapRequestFlow(t *testing.T) {
	client := setupPlayerUpdateTest(t)
	clearTable(t, &orm.CommanderTask{})

	seedConfigEntry(t, worldChapterRandomCategory, "4001", `{"id":4001,"map":31}`)

	activatePayload := protobuf.CS_33101{
		Id:         proto.Uint32(4001),
		EnterMapId: proto.Uint32(5001),
		Camp:       proto.Uint32(2),
		EliteFleetList: []*protobuf.ELITEFLEETINFO{{
			ShipIdList: []uint32{101, 102},
			Commanders: []*protobuf.COMMANDERSINFO{{Pos: proto.Uint32(1), Id: proto.Uint32(701)}},
		}},
	}
	activateBuf, err := proto.Marshal(&activatePayload)
	if err != nil {
		t.Fatalf("marshal activate payload: %v", err)
	}
	client.Buffer.Reset()
	if _, _, err := WorldActivate(&activateBuf, client); err != nil {
		t.Fatalf("WorldActivate failed: %v", err)
	}

	var activateResponse protobuf.SC_33102
	decodePacketAt(t, client, 0, 33102, &activateResponse)
	if activateResponse.GetResult() != 0 {
		t.Fatalf("expected world activation success")
	}
	if activateResponse.GetWorld() == nil || activateResponse.GetWorld().GetMapId() != 4001 {
		t.Fatalf("expected world payload with active map")
	}
	if activateResponse.GetWorld().GetEnterMapId() != 5001 {
		t.Fatalf("expected enter map id to be persisted")
	}
	if len(activateResponse.GetWorld().GetGroupList()) != 1 {
		t.Fatalf("expected one world group in response")
	}

	requestPayload := protobuf.CS_33106{Id: proto.Uint32(4001)}
	requestBuf, err := proto.Marshal(&requestPayload)
	if err != nil {
		t.Fatalf("marshal request payload: %v", err)
	}
	client.Buffer.Reset()
	if _, _, err := WorldMapRequest(&requestBuf, client); err != nil {
		t.Fatalf("WorldMapRequest failed: %v", err)
	}

	var requestResponse protobuf.SC_33107
	decodePacketAt(t, client, 0, 33107, &requestResponse)
	if requestResponse.GetResult() != 0 {
		t.Fatalf("expected map request success")
	}
	if requestResponse.GetMap() == nil || requestResponse.GetMap().GetId() == nil {
		t.Fatalf("expected map payload")
	}
	if requestResponse.GetMap().GetId().GetRandomId() != 4001 || requestResponse.GetMap().GetId().GetTemplateId() != 31 {
		t.Fatalf("unexpected world map id payload")
	}
}

func TestWorldMapOperationMoveAndInsufficientPower(t *testing.T) {
	client := setupPlayerUpdateTest(t)

	runtime, err := orm.LoadOrCreateWorldRuntime(client.Commander.CommanderID)
	if err != nil {
		t.Fatalf("load runtime: %v", err)
	}
	runtime.ActionPower = 1
	runtime.MapID = 4101
	runtime.EnterMapID = 4101
	runtime.SetMapTemplate(4101, 66)
	if err := orm.SaveWorldRuntime(runtime); err != nil {
		t.Fatalf("save runtime: %v", err)
	}

	movePayload := protobuf.CS_33103{
		Act:     proto.Uint32(1),
		GroupId: proto.Uint32(1),
		PosList: []*protobuf.CHAPTERCELLPOS_P33{{Row: proto.Uint32(1), Column: proto.Uint32(1)}, {Row: proto.Uint32(1), Column: proto.Uint32(2)}},
	}
	moveBuf, err := proto.Marshal(&movePayload)
	if err != nil {
		t.Fatalf("marshal move payload: %v", err)
	}
	client.Buffer.Reset()
	if _, _, err := WorldMapOperation(&moveBuf, client); err != nil {
		t.Fatalf("WorldMapOperation failed: %v", err)
	}
	if _, _, err := WorldMapOperation(&moveBuf, client); err != nil {
		t.Fatalf("WorldMapOperation second call failed: %v", err)
	}

	var first protobuf.SC_33104
	offset := decodePacketAt(t, client, 0, 33104, &first)
	if first.GetResult() != 0 || len(first.GetMovePath()) != 2 {
		t.Fatalf("expected successful move path response")
	}
	if first.GetActionPower() != 0 {
		t.Fatalf("expected action power to be consumed")
	}

	var second protobuf.SC_33104
	decodePacketAt(t, client, offset, 33104, &second)
	if second.GetResult() != worldResultActionPowerInsufficient {
		t.Fatalf("expected insufficient action power result")
	}
}

func TestWorldStaminaExchangeUsesTierCosts(t *testing.T) {
	client := setupPlayerUpdateTest(t)
	if err := client.Commander.SetResource(2, 100); err != nil {
		t.Fatalf("seed oil: %v", err)
	}

	seedConfigEntry(t, worldGamesetCategory, "world_supply_value", `{"description":[[5],[7]],"key_value":0}`)
	seedConfigEntry(t, worldGamesetCategory, "world_supply_price", `{"description":[[0,0,10],[0,0,20]],"key_value":0}`)

	payload := protobuf.CS_33108{Type: proto.Uint32(1)}
	buf, err := proto.Marshal(&payload)
	if err != nil {
		t.Fatalf("marshal stamina payload: %v", err)
	}

	client.Buffer.Reset()
	if _, _, err := WorldStaminaExchange(&buf, client); err != nil {
		t.Fatalf("WorldStaminaExchange first call failed: %v", err)
	}
	if _, _, err := WorldStaminaExchange(&buf, client); err != nil {
		t.Fatalf("WorldStaminaExchange second call failed: %v", err)
	}
	if _, _, err := WorldStaminaExchange(&buf, client); err != nil {
		t.Fatalf("WorldStaminaExchange third call failed: %v", err)
	}

	var first protobuf.SC_33109
	offset := decodePacketAt(t, client, 0, 33109, &first)
	if first.GetResult() != 0 {
		t.Fatalf("expected first exchange success")
	}
	var second protobuf.SC_33109
	offset = decodePacketAt(t, client, offset, 33109, &second)
	if second.GetResult() != 0 {
		t.Fatalf("expected second exchange success")
	}
	var third protobuf.SC_33109
	decodePacketAt(t, client, offset, 33109, &third)
	if third.GetResult() == 0 {
		t.Fatalf("expected exchange limit failure")
	}

	if got := client.Commander.GetResourceCount(2); got != 70 {
		t.Fatalf("expected oil to be reduced by tier costs, got %d", got)
	}
	runtime, err := orm.LoadWorldRuntime(client.Commander.CommanderID)
	if err != nil {
		t.Fatalf("load runtime: %v", err)
	}
	if runtime.ActionPower != 212 || runtime.StaminaExchangeTimes != 2 {
		t.Fatalf("unexpected stamina exchange state: %+v", runtime)
	}
}

func TestWorldTypedDataAndResetResponses(t *testing.T) {
	client := setupPlayerUpdateTest(t)
	runtime, err := orm.LoadOrCreateWorldRuntime(client.Commander.CommanderID)
	if err != nil {
		t.Fatalf("load runtime: %v", err)
	}
	runtime.ResetAvailableAtTimestamp = uint32(time.Now().Unix()) + 120
	runtime.SairenChapter = []uint32{7, 8}
	if err := orm.SaveWorldRuntime(runtime); err != nil {
		t.Fatalf("save runtime: %v", err)
	}

	typedSuccess := protobuf.CS_33110{Type: proto.Uint32(1), Data: proto.Uint32(9)}
	typedFail := protobuf.CS_33110{Type: proto.Uint32(9), Data: proto.Uint32(9)}
	resetDeferred := protobuf.CS_33112{Type: proto.Uint32(1)}
	resetSairen := protobuf.CS_33112{Type: proto.Uint32(2)}

	typedSuccessBuf, _ := proto.Marshal(&typedSuccess)
	typedFailBuf, _ := proto.Marshal(&typedFail)
	resetDeferredBuf, _ := proto.Marshal(&resetDeferred)
	resetSairenBuf, _ := proto.Marshal(&resetSairen)

	client.Buffer.Reset()
	if _, _, err := WorldTypedDataOperation(&typedSuccessBuf, client); err != nil {
		t.Fatalf("typed success call failed: %v", err)
	}
	if _, _, err := WorldTypedDataOperation(&typedFailBuf, client); err != nil {
		t.Fatalf("typed failure call failed: %v", err)
	}
	if _, _, err := WorldResetOrKill(&resetDeferredBuf, client); err != nil {
		t.Fatalf("reset deferred call failed: %v", err)
	}
	if _, _, err := WorldResetOrKill(&resetSairenBuf, client); err != nil {
		t.Fatalf("reset sairen call failed: %v", err)
	}

	var typedSuccessResp protobuf.SC_33111
	offset := decodePacketAt(t, client, 0, 33111, &typedSuccessResp)
	if typedSuccessResp.GetResult() != 0 {
		t.Fatalf("expected typed operation success")
	}
	var typedFailResp protobuf.SC_33111
	offset = decodePacketAt(t, client, offset, 33111, &typedFailResp)
	if typedFailResp.GetResult() == 0 {
		t.Fatalf("expected typed operation non-zero failure")
	}
	var resetDeferredResp protobuf.SC_33113
	offset = decodePacketAt(t, client, offset, 33113, &resetDeferredResp)
	if resetDeferredResp.GetResult() != 0 || resetDeferredResp.GetTime() == 0 {
		t.Fatalf("expected deferred reset with positive time")
	}
	var resetSairenResp protobuf.SC_33113
	decodePacketAt(t, client, offset, 33113, &resetSairenResp)
	if len(resetSairenResp.GetSairenChapter()) != 2 {
		t.Fatalf("expected sairen chapter list in reset response")
	}
}

func TestWorldTaskTriggerAndSubmitFlow(t *testing.T) {
	client := setupPlayerUpdateTest(t)
	clearTable(t, &orm.CommanderTask{})

	seedConfigEntry(t, worldTaskDataCategory, "9901", `{
		"id":9901,
		"need_level":10,
		"need_task_complete":0,
		"complete_condition":1,
		"complete_parameter_num":2,
		"show":[[2,1001,3]],
		"exp":4,
		"intimacy":2,
		"complete_stage":9,
		"event_map_id":0
	}`)

	triggerPayload := protobuf.CS_33205{TaskId: proto.Uint32(9901)}
	triggerBuf, _ := proto.Marshal(&triggerPayload)
	client.Buffer.Reset()
	if _, _, err := WorldTriggerTask(&triggerBuf, client); err != nil {
		t.Fatalf("WorldTriggerTask failed: %v", err)
	}

	var triggerResponse protobuf.SC_33206
	decodePacketAt(t, client, 0, 33206, &triggerResponse)
	if triggerResponse.GetResult() != 0 || triggerResponse.GetTask() == nil {
		t.Fatalf("expected world task trigger success")
	}

	execAnswerTestSQLT(t, "UPDATE commander_tasks SET progress = 2 WHERE commander_id = $1 AND task_id = $2", int64(client.Commander.CommanderID), int64(9901))

	submitPayload := protobuf.CS_33207{TaskId: proto.Uint32(9901)}
	submitBuf, _ := proto.Marshal(&submitPayload)
	client.Buffer.Reset()
	if _, _, err := WorldSubmitTask(&submitBuf, client); err != nil {
		t.Fatalf("WorldSubmitTask failed: %v", err)
	}
	if _, _, err := WorldSubmitTask(&submitBuf, client); err != nil {
		t.Fatalf("WorldSubmitTask duplicate failed: %v", err)
	}

	var first protobuf.SC_33208
	offset := decodePacketAt(t, client, 0, 33208, &first)
	if first.GetResult() != 0 {
		t.Fatalf("expected world submit success")
	}
	if len(first.GetDrops()) != 1 || first.GetDrops()[0].GetId() != 1001 || first.GetDrops()[0].GetNumber() != 3 {
		t.Fatalf("unexpected drop payload")
	}
	if first.GetExp() != 4 || first.GetIntimacy() != 2 {
		t.Fatalf("expected exp and intimacy from config")
	}
	var second protobuf.SC_33208
	decodePacketAt(t, client, offset, 33208, &second)
	if second.GetResult() == 0 {
		t.Fatalf("expected duplicate submit to fail")
	}

	client.Buffer.Reset()
	if _, _, err := WorldTriggerTask(&triggerBuf, client); err != nil {
		t.Fatalf("WorldTriggerTask retrigger failed: %v", err)
	}
	var retriggerResponse protobuf.SC_33206
	decodePacketAt(t, client, 0, 33206, &retriggerResponse)
	if retriggerResponse.GetResult() != worldResultTaskRefused {
		t.Fatalf("expected retrigger of submitted task to be refused")
	}

	if count := client.Commander.GetItemCount(1001); count != 3 {
		t.Fatalf("expected granted world task item reward")
	}
	runtime, err := orm.LoadWorldRuntime(client.Commander.CommanderID)
	if err != nil {
		t.Fatalf("load runtime: %v", err)
	}
	if runtime.TaskFinishCount != 1 || runtime.Progress != 9 {
		t.Fatalf("expected task finish count and progress update")
	}
}

func TestWorldTaskTriggerRespectsRefusalGates(t *testing.T) {
	client := setupPlayerUpdateTest(t)
	clearTable(t, &orm.CommanderTask{})

	seedConfigEntry(t, worldTaskDataCategory, "9910", `{"id":9910,"need_level":99,"need_task_complete":0,"complete_parameter_num":1}`)
	payload := protobuf.CS_33205{TaskId: proto.Uint32(9910)}
	buf, _ := proto.Marshal(&payload)
	client.Buffer.Reset()
	if _, _, err := WorldTriggerTask(&buf, client); err != nil {
		t.Fatalf("WorldTriggerTask failed: %v", err)
	}

	var response protobuf.SC_33206
	decodePacketAt(t, client, 0, 33206, &response)
	if response.GetResult() != worldResultTaskRefused {
		t.Fatalf("expected refusal result code")
	}
}
