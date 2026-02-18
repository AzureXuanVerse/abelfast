package answer

import (
	"testing"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/orm"
	"github.com/ggmolly/belfast/internal/protobuf"
	"google.golang.org/protobuf/proto"
)

func TestCityRebuildGetDataSuccessAndFailure(t *testing.T) {
	client := setupCityRebuildTestClient(t)
	state, err := orm.GetOrCreateCityRebuildState(client.Commander.CommanderID, 9001)
	if err != nil {
		t.Fatalf("get state: %v", err)
	}
	state.Pt = 250
	state.Builds = []uint32{1}
	state.Roles = []uint32{2001}
	state.Recruits = []orm.CityRebuildRecruit{{ID: 2002, StartTime: 100}}
	state.Buffs = map[uint32]uint32{1: 2}
	state.MaxLevel = 3
	state.CurLevel = 2
	state.MaxDisplay = 3
	state.SummaryPt = 12
	if err := orm.SaveCityRebuildState(state); err != nil {
		t.Fatalf("save state: %v", err)
	}

	payload := marshalPacketRequest(t, &protobuf.CS_26060{ActId: proto.Uint32(9001)})
	if _, _, err := CityRebuildGetData(&payload, client); err != nil {
		t.Fatalf("CityRebuildGetData failed: %v", err)
	}
	response := &protobuf.SC_26061{}
	decodeLoveLetterPacketMessage(t, client, 26061, response)
	if response.GetResult() != 0 {
		t.Fatalf("expected result 0, got %d", response.GetResult())
	}
	if response.GetInfo() == nil {
		t.Fatalf("expected info payload")
	}
	if response.GetInfo().GetPt() == nil || response.GetInfo().GetAdjust() == nil || response.GetInfo().GetSummaryPt() == nil {
		t.Fatalf("expected stable fields in info")
	}

	payload = marshalPacketRequest(t, &protobuf.CS_26060{ActId: proto.Uint32(9999)})
	if _, _, err := CityRebuildGetData(&payload, client); err != nil {
		t.Fatalf("CityRebuildGetData invalid failed: %v", err)
	}
	response = &protobuf.SC_26061{}
	decodeLoveLetterPacketMessage(t, client, 26061, response)
	if response.GetResult() == 0 {
		t.Fatalf("expected failure for invalid activity")
	}
}

func TestCityRebuildBuildingRecruitAndEndRecruit(t *testing.T) {
	client := setupCityRebuildTestClient(t)
	state, err := orm.GetOrCreateCityRebuildState(client.Commander.CommanderID, 9001)
	if err != nil {
		t.Fatalf("get state: %v", err)
	}
	state.Pt = 500
	if err := orm.SaveCityRebuildState(state); err != nil {
		t.Fatalf("save state: %v", err)
	}

	payload := marshalPacketRequest(t, &protobuf.CS_26064{ActId: proto.Uint32(9001), BuildingId: proto.Uint32(1)})
	if _, _, err := CityRebuildBuildingAction(&payload, client); err != nil {
		t.Fatalf("CityRebuildBuildingAction rebuild failed: %v", err)
	}
	buildResponse := &protobuf.SC_26065{}
	decodeLoveLetterPacketMessage(t, client, 26065, buildResponse)
	if buildResponse.GetResult() != 0 || buildResponse.GetAdjust() == nil {
		t.Fatalf("expected rebuild success with adjust")
	}

	payload = marshalPacketRequest(t, &protobuf.CS_26064{ActId: proto.Uint32(9001), BuildingId: proto.Uint32(2)})
	if _, _, err := CityRebuildBuildingAction(&payload, client); err != nil {
		t.Fatalf("CityRebuildBuildingAction recruit failed: %v", err)
	}
	buildResponse = &protobuf.SC_26065{}
	decodeLoveLetterPacketMessage(t, client, 26065, buildResponse)
	if buildResponse.GetResult() != 0 {
		t.Fatalf("expected recruit-start success")
	}

	seedConfigEntry(t, cityRebuildBuildingCategory, "3", `{"id":3,"type":2,"pt_cost":[8,65103,20],"role_id":2002,"need_level":0}`)
	payload = marshalPacketRequest(t, &protobuf.CS_26064{ActId: proto.Uint32(9001), BuildingId: proto.Uint32(3)})
	if _, _, err := CityRebuildBuildingAction(&payload, client); err != nil {
		t.Fatalf("CityRebuildBuildingAction second recruit failed: %v", err)
	}
	buildResponse = &protobuf.SC_26065{}
	decodeLoveLetterPacketMessage(t, client, 26065, buildResponse)
	if buildResponse.GetResult() != 0 {
		t.Fatalf("expected second recruit-start success")
	}

	payload = marshalPacketRequest(t, &protobuf.CS_26062{ActId: proto.Uint32(9001), Roles: []uint32{2001, 2001}})
	if _, _, err := CityRebuildEndRecruit(&payload, client); err != nil {
		t.Fatalf("CityRebuildEndRecruit failed: %v", err)
	}
	recruitResponse := &protobuf.SC_26063{}
	decodeLoveLetterPacketMessage(t, client, 26063, recruitResponse)
	if recruitResponse.GetResult() != 0 || recruitResponse.GetAdjust() == nil {
		t.Fatalf("expected end recruit success with adjust")
	}

	state, err = orm.GetOrCreateCityRebuildState(client.Commander.CommanderID, 9001)
	if err != nil {
		t.Fatalf("reload state: %v", err)
	}
	if !cityRebuildContainsRecruit(state.Recruits, 2002) {
		t.Fatalf("expected unrelated recruit to remain active")
	}

	payload = marshalPacketRequest(t, &protobuf.CS_26062{ActId: proto.Uint32(9001), Roles: []uint32{2001}})
	if _, _, err := CityRebuildEndRecruit(&payload, client); err != nil {
		t.Fatalf("CityRebuildEndRecruit duplicate failed: %v", err)
	}
	recruitResponse = &protobuf.SC_26063{}
	decodeLoveLetterPacketMessage(t, client, 26063, recruitResponse)
	if recruitResponse.GetResult() == 0 {
		t.Fatalf("expected duplicate end recruit failure")
	}
}

func TestCityRebuildUpgradeBuff(t *testing.T) {
	client := setupCityRebuildTestClient(t)
	state, err := orm.GetOrCreateCityRebuildState(client.Commander.CommanderID, 9001)
	if err != nil {
		t.Fatalf("get state: %v", err)
	}
	state.Pt = 200
	if err := orm.SaveCityRebuildState(state); err != nil {
		t.Fatalf("save state: %v", err)
	}

	payload := marshalPacketRequest(t, &protobuf.CS_26066{ActId: proto.Uint32(9001), Group: proto.Uint32(1), Count: proto.Uint32(2)})
	if _, _, err := CityRebuildUpgradeBuff(&payload, client); err != nil {
		t.Fatalf("CityRebuildUpgradeBuff failed: %v", err)
	}
	response := &protobuf.SC_26067{}
	decodeLoveLetterPacketMessage(t, client, 26067, response)
	if response.GetResult() != 0 || response.GetAdjust() == nil {
		t.Fatalf("expected buff upgrade success")
	}

	payload = marshalPacketRequest(t, &protobuf.CS_26066{ActId: proto.Uint32(9001), Group: proto.Uint32(1), Count: proto.Uint32(2)})
	if _, _, err := CityRebuildUpgradeBuff(&payload, client); err != nil {
		t.Fatalf("CityRebuildUpgradeBuff boundary failed: %v", err)
	}
	response = &protobuf.SC_26067{}
	decodeLoveLetterPacketMessage(t, client, 26067, response)
	if response.GetResult() == 0 {
		t.Fatalf("expected over-cap buff upgrade failure")
	}
}

func TestCityRebuildChooseLevelPersists(t *testing.T) {
	client := setupCityRebuildTestClient(t)
	state, err := orm.GetOrCreateCityRebuildState(client.Commander.CommanderID, 9001)
	if err != nil {
		t.Fatalf("get state: %v", err)
	}
	state.MaxLevel = 3
	state.CurLevel = 1
	if err := orm.SaveCityRebuildState(state); err != nil {
		t.Fatalf("save state: %v", err)
	}

	payload := marshalPacketRequest(t, &protobuf.CS_26070{ActId: proto.Uint32(9001), Level: proto.Uint32(2)})
	if _, _, err := CityRebuildChooseLevel(&payload, client); err != nil {
		t.Fatalf("CityRebuildChooseLevel failed: %v", err)
	}
	chooseResponse := &protobuf.SC_26071{}
	decodeLoveLetterPacketMessage(t, client, 26071, chooseResponse)
	if chooseResponse.GetResult() != 0 {
		t.Fatalf("expected choose level success")
	}

	payload = marshalPacketRequest(t, &protobuf.CS_26060{ActId: proto.Uint32(9001)})
	if _, _, err := CityRebuildGetData(&payload, client); err != nil {
		t.Fatalf("CityRebuildGetData failed: %v", err)
	}
	getResponse := &protobuf.SC_26061{}
	decodeLoveLetterPacketMessage(t, client, 26061, getResponse)
	if getResponse.GetInfo().GetCurLevel() != 2 {
		t.Fatalf("expected persisted level 2, got %d", getResponse.GetInfo().GetCurLevel())
	}

	payload = marshalPacketRequest(t, &protobuf.CS_26070{ActId: proto.Uint32(9001), Level: proto.Uint32(9)})
	if _, _, err := CityRebuildChooseLevel(&payload, client); err != nil {
		t.Fatalf("CityRebuildChooseLevel invalid failed: %v", err)
	}
	chooseResponse = &protobuf.SC_26071{}
	decodeLoveLetterPacketMessage(t, client, 26071, chooseResponse)
	if chooseResponse.GetResult() == 0 {
		t.Fatalf("expected invalid level failure")
	}
}

func TestCityRebuildSummaryClaimAndInitTime(t *testing.T) {
	client := setupCityRebuildTestClient(t)
	state, err := orm.GetOrCreateCityRebuildState(client.Commander.CommanderID, 9001)
	if err != nil {
		t.Fatalf("get state: %v", err)
	}
	state.SummaryReady = true
	state.SummaryPt = 33
	if err := orm.SaveCityRebuildState(state); err != nil {
		t.Fatalf("save state: %v", err)
	}

	payload := marshalPacketRequest(t, &protobuf.CS_26068{ActId: proto.Uint32(9001)})
	if _, _, err := CityRebuildResultSummary(&payload, client); err != nil {
		t.Fatalf("CityRebuildResultSummary failed: %v", err)
	}
	summaryResponse := &protobuf.SC_26069{}
	decodeLoveLetterPacketMessage(t, client, 26069, summaryResponse)
	if summaryResponse.GetResult() != 0 || summaryResponse.GetSummary() == nil {
		t.Fatalf("expected summary success")
	}
	if summaryResponse.GetSummary().GetSummaryPt() == nil || summaryResponse.GetSummary().GetAdjust() == nil {
		t.Fatalf("expected summary payload shape")
	}

	payload = marshalPacketRequest(t, &protobuf.CS_26068{ActId: proto.Uint32(9001)})
	if _, _, err := CityRebuildResultSummary(&payload, client); err != nil {
		t.Fatalf("CityRebuildResultSummary duplicate failed: %v", err)
	}
	summaryResponse = &protobuf.SC_26069{}
	decodeLoveLetterPacketMessage(t, client, 26069, summaryResponse)
	if summaryResponse.GetResult() == 0 {
		t.Fatalf("expected duplicate summary failure")
	}

	payload = marshalPacketRequest(t, &protobuf.CS_26072{ActId: proto.Uint32(9001)})
	if _, _, err := CityRebuildInitTime(&payload, client); err != nil {
		t.Fatalf("CityRebuildInitTime failed: %v", err)
	}
	initResponse := &protobuf.SC_26073{}
	decodeLoveLetterPacketMessage(t, client, 26073, initResponse)
	if initResponse.GetResult() != 0 || initResponse.GetAdjust() == nil || initResponse.GetAdjust().GetTime() == 0 {
		t.Fatalf("expected init time adjust response")
	}
}

func setupCityRebuildTestClient(t *testing.T) *connection.Client {
	t.Helper()
	client := setupPlayerUpdateTest(t)
	seedConfigEntry(t, "ShareCfg/activity_template.json", "9001", `{"id":9001,"type":777,"time":"always"}`)
	seedConfigEntry(t, cityRebuildBuildingCategory, "1", `{"id":1,"type":1,"pt_cost":[8,65103,50],"need_level":0}`)
	seedConfigEntry(t, cityRebuildBuildingCategory, "2", `{"id":2,"type":2,"pt_cost":[8,65103,30],"role_id":2001,"need_level":0}`)
	seedConfigEntry(t, cityRebuildBuffCategory, "1001", `{"group":1,"level":1,"basic_cost":10}`)
	seedConfigEntry(t, cityRebuildBuffCategory, "1002", `{"group":1,"level":2,"basic_cost":10}`)
	seedConfigEntry(t, cityRebuildBuffCategory, "1003", `{"group":1,"level":3,"basic_cost":10}`)
	return client
}
