package answer

import (
	"testing"

	"github.com/ggmolly/belfast/internal/orm"
	"github.com/ggmolly/belfast/internal/protobuf"
	"google.golang.org/protobuf/proto"
)

func TestEducateSetCallAndRequestRoundTrip(t *testing.T) {
	client := setupConfigTest(t)
	setCall := protobuf.CS_27031{Name: proto.String("Commander")}
	data, err := proto.Marshal(&setCall)
	if err != nil {
		t.Fatalf("marshal set call: %v", err)
	}
	if _, _, err := EducateSetCall(&data, client); err != nil {
		t.Fatalf("set call failed: %v", err)
	}
	var setCallResp protobuf.SC_27032
	decodeResponse(t, client, &setCallResp)
	if setCallResp.GetResult() != 0 {
		t.Fatalf("expected set call success")
	}

	requestData := []byte{}
	client.Buffer.Reset()
	if _, _, err := EducateRequest(&requestData, client); err != nil {
		t.Fatalf("educate request failed: %v", err)
	}
	var requestResp protobuf.SC_27001
	decodeResponse(t, client, &requestResp)
	if requestResp.GetChild().GetUserName() != "Commander" {
		t.Fatalf("expected call name Commander, got %q", requestResp.GetChild().GetUserName())
	}

	invalid := protobuf.CS_27031{Name: proto.String("abc")}
	invalidData, _ := proto.Marshal(&invalid)
	client.Buffer.Reset()
	if _, _, err := EducateSetCall(&invalidData, client); err != nil {
		t.Fatalf("set call invalid failed: %v", err)
	}
	decodeResponse(t, client, &setCallResp)
	if setCallResp.GetResult() == 0 {
		t.Fatalf("expected invalid call name to fail")
	}
}

func TestEducateSetTargetAndRoundTrip(t *testing.T) {
	client := setupConfigTest(t)
	seedConfigEntry(t, childTargetSetCategory, "7", `{"id":7}`)

	payload := protobuf.CS_27019{Id: proto.Uint32(7)}
	data, _ := proto.Marshal(&payload)
	if _, _, err := EducateSetTarget(&data, client); err != nil {
		t.Fatalf("set target failed: %v", err)
	}
	var resp protobuf.SC_27020
	decodeResponse(t, client, &resp)
	if resp.GetResult() != 0 {
		t.Fatalf("expected target set success")
	}

	client.Buffer.Reset()
	requestData := []byte{}
	if _, _, err := EducateRequest(&requestData, client); err != nil {
		t.Fatalf("educate request failed: %v", err)
	}
	var requestResp protobuf.SC_27001
	decodeResponse(t, client, &requestResp)
	if requestResp.GetChild().GetTarget() != 7 {
		t.Fatalf("expected target 7, got %d", requestResp.GetChild().GetTarget())
	}

	bad := protobuf.CS_27019{Id: proto.Uint32(99)}
	badData, _ := proto.Marshal(&bad)
	client.Buffer.Reset()
	if _, _, err := EducateSetTarget(&badData, client); err != nil {
		t.Fatalf("set target invalid failed: %v", err)
	}
	decodeResponse(t, client, &resp)
	if resp.GetResult() == 0 {
		t.Fatalf("expected invalid target to fail")
	}
}

func TestEducateTriggerEndAndGetEndings(t *testing.T) {
	client := setupConfigTest(t)
	seedConfigEntry(t, childEndingCategory, "11", `{"id":11}`)

	payload := protobuf.CS_27008{EndingId: proto.Uint32(11), QualifiedId: []uint32{21, 22}}
	data, _ := proto.Marshal(&payload)
	if _, _, err := EducateTriggerEnd(&data, client); err != nil {
		t.Fatalf("trigger end failed: %v", err)
	}
	var triggerResp protobuf.SC_27009
	decodeResponse(t, client, &triggerResp)
	if triggerResp.GetResult() != 0 {
		t.Fatalf("expected ending trigger success")
	}

	client.Buffer.Reset()
	if _, _, err := EducateTriggerEnd(&data, client); err != nil {
		t.Fatalf("trigger end retry failed: %v", err)
	}
	decodeResponse(t, client, &triggerResp)
	if triggerResp.GetResult() != 0 {
		t.Fatalf("expected ending trigger retry success")
	}

	getPayload := protobuf.CS_27010{Type: proto.Uint32(0)}
	getData, _ := proto.Marshal(&getPayload)
	client.Buffer.Reset()
	if _, _, err := EducateGetEndings(&getData, client); err != nil {
		t.Fatalf("get endings failed: %v", err)
	}
	var getResp protobuf.SC_27011
	decodeResponse(t, client, &getResp)
	if len(getResp.GetEndings()) != 1 || getResp.GetEndings()[0] != 11 {
		t.Fatalf("expected endings [11], got %v", getResp.GetEndings())
	}
	if len(getResp.GetQualifieds()) != 2 || getResp.GetQualifieds()[0] != 21 || getResp.GetQualifieds()[1] != 22 {
		t.Fatalf("expected qualifieds [21 22], got %v", getResp.GetQualifieds())
	}
}

func TestEducateMapSiteOperateBehavior(t *testing.T) {
	client := setupConfigTest(t)
	seedConfigEntry(t, childSiteCategory, "1", `{"id":1,"option":[101]}`)
	seedConfigEntry(t, childSiteOptionCategory, "101", `{"id":101,"type":2,"result":[201],"cost":[[2,3,1]],"count_limit":[1,100]}`)
	seedConfigEntry(t, childSiteOptionBranchCategory, "201", `{"id":201}`)

	payload := protobuf.CS_27004{Siteid: proto.Uint32(1), Optionid: proto.Uint32(101)}
	data, _ := proto.Marshal(&payload)
	if _, _, err := EducateMapSiteOperate(&data, client); err != nil {
		t.Fatalf("map site operate failed: %v", err)
	}
	var resp protobuf.SC_27005
	decodeResponse(t, client, &resp)
	if resp.GetResult() != 0 || resp.GetBranchId() == 0 {
		t.Fatalf("expected success with branch id, got result=%d branch=%d", resp.GetResult(), resp.GetBranchId())
	}

	client.Buffer.Reset()
	if _, _, err := EducateMapSiteOperate(&data, client); err != nil {
		t.Fatalf("map site operate retry failed: %v", err)
	}
	decodeResponse(t, client, &resp)
	if resp.GetResult() == 0 {
		t.Fatalf("expected count-limited option to fail on second request")
	}
}

func TestEducateExtraAttrAndTaskFlow(t *testing.T) {
	client := setupConfigTest(t)
	seedConfigEntry(t, childDataCategory, "1", `{"id":1,"attr_2_list":[201,202,203],"attr_2_add":5,"favor_level":3}`)
	seedConfigEntry(t, childTaskCategory, "501", `{"id":501,"type_1":2,"arg":3,"drop_display":[3,301,5]}`)

	extra := protobuf.CS_27039{AttrId: proto.Uint32(201)}
	extraData, _ := proto.Marshal(&extra)
	if _, _, err := EducateAddExtraAttr(&extraData, client); err != nil {
		t.Fatalf("extra attr failed: %v", err)
	}
	var extraResp protobuf.SC_27040
	decodeResponse(t, client, &extraResp)
	if extraResp.GetResult() != 0 {
		t.Fatalf("expected extra attr success")
	}

	client.Buffer.Reset()
	if _, _, err := EducateAddExtraAttr(&extraData, client); err != nil {
		t.Fatalf("extra attr retry failed: %v", err)
	}
	decodeResponse(t, client, &extraResp)
	if extraResp.GetResult() == 0 {
		t.Fatalf("expected repeated extra attr to fail")
	}

	progress := protobuf.CS_27037{
		Type_1: proto.Uint32(2),
		Progresses: []*protobuf.CHILD_PROGRESS{{
			TaskId:   proto.Uint32(501),
			Progress: proto.Uint32(2),
		}},
	}
	progressData, _ := proto.Marshal(&progress)
	client.Buffer.Reset()
	if _, _, err := EducateAddTaskProgress(&progressData, client); err != nil {
		t.Fatalf("add progress failed: %v", err)
	}
	packetIDs := decodePacketIDs(t, client.Buffer.Bytes())
	if len(packetIDs) != 2 || packetIDs[0] != 27038 || packetIDs[1] != 27025 {
		t.Fatalf("expected packets [27038 27025], got %v", packetIDs)
	}

	submit := protobuf.CS_27023{Id: proto.Uint32(501), System: proto.Uint32(2)}
	submitData, _ := proto.Marshal(&submit)
	client.Buffer.Reset()
	if _, _, err := EducateSubmitTask(&submitData, client); err != nil {
		t.Fatalf("submit task failed: %v", err)
	}
	var submitResp protobuf.SC_27024
	decodeResponse(t, client, &submitResp)
	if submitResp.GetResult() == 0 {
		t.Fatalf("expected incomplete task submit to fail")
	}

	progress.Progresses[0].Progress = proto.Uint32(1)
	progressData, _ = proto.Marshal(&progress)
	client.Buffer.Reset()
	if _, _, err := EducateAddTaskProgress(&progressData, client); err != nil {
		t.Fatalf("add progress second step failed: %v", err)
	}

	client.Buffer.Reset()
	if _, _, err := EducateSubmitTask(&submitData, client); err != nil {
		t.Fatalf("submit task second attempt failed: %v", err)
	}
	decodeResponse(t, client, &submitResp)
	if submitResp.GetResult() != 0 || len(submitResp.GetAwards()) != 1 {
		t.Fatalf("expected submit success with one award, got result=%d awards=%d", submitResp.GetResult(), len(submitResp.GetAwards()))
	}
}

func TestEducateUpgradeFavorAndChangeCharacter(t *testing.T) {
	client := setupConfigTest(t)
	seedConfigEntry(t, childDataCategory, "1", `{"id":1,"attr_2_list":[201,202,203],"attr_2_add":5,"favor_level":3}`)
	seedConfigEntry(t, childEndingCategory, "777", `{"id":777}`)
	seedConfigEntry(t, secretarySpecialShipCategory, "777", `{"id":777}`)

	upgrade := protobuf.CS_27006{Type: proto.Uint32(0)}
	upgradeData, _ := proto.Marshal(&upgrade)
	for i := 0; i < 2; i++ {
		client.Buffer.Reset()
		if _, _, err := EducateUpgradeFavor(&upgradeData, client); err != nil {
			t.Fatalf("upgrade favor failed: %v", err)
		}
	}

	client.Buffer.Reset()
	requestData := []byte{}
	if _, _, err := EducateRequest(&requestData, client); err != nil {
		t.Fatalf("educate request failed: %v", err)
	}
	var requestResp protobuf.SC_27001
	decodeResponse(t, client, &requestResp)
	if requestResp.GetChild().GetFavor().GetLv() != 3 {
		t.Fatalf("expected favor level 3, got %d", requestResp.GetChild().GetFavor().GetLv())
	}

	triggerEnd := protobuf.CS_27008{EndingId: proto.Uint32(777)}
	triggerData, _ := proto.Marshal(&triggerEnd)
	client.Buffer.Reset()
	if _, _, err := EducateTriggerEnd(&triggerData, client); err != nil {
		t.Fatalf("trigger educate ending failed: %v", err)
	}

	change := protobuf.CS_27041{EndingId: proto.Uint32(777)}
	changeData, _ := proto.Marshal(&change)
	client.Buffer.Reset()
	if _, _, err := ChangeEducateCharacter(&changeData, client); err != nil {
		t.Fatalf("change educate character failed: %v", err)
	}
	var changeResp protobuf.SC_27042
	decodeResponse(t, client, &changeResp)
	if changeResp.GetResult() != 0 {
		t.Fatalf("expected change educate character success")
	}

	got := queryAnswerTestInt64(t, "SELECT child_display FROM commanders WHERE commander_id = $1", int64(client.Commander.CommanderID))
	if got != 777 {
		t.Fatalf("expected persisted child display 777, got %d", got)
	}
}

func TestPlayerInfoUsesChangedEducateCharacter(t *testing.T) {
	client := setupPlayerUpdateTest(t)
	seedConfigEntry(t, childEndingCategory, "555", `{"id":555}`)
	seedConfigEntry(t, secretarySpecialShipCategory, "555", `{"id":555}`)

	triggerEnd := protobuf.CS_27008{EndingId: proto.Uint32(555)}
	triggerData, _ := proto.Marshal(&triggerEnd)
	if _, _, err := EducateTriggerEnd(&triggerData, client); err != nil {
		t.Fatalf("trigger educate ending failed: %v", err)
	}

	change := protobuf.CS_27041{EndingId: proto.Uint32(555)}
	changeData, _ := proto.Marshal(&change)
	if _, _, err := ChangeEducateCharacter(&changeData, client); err != nil {
		t.Fatalf("change educate character failed: %v", err)
	}

	client.Commander.Ships = []orm.OwnedShip{{
		OwnerID:           client.Commander.CommanderID,
		ShipID:            202124,
		IsSecretary:       true,
		SecretaryPosition: proto.Uint32(0),
	}}

	client.Buffer.Reset()
	buffer := []byte{}
	if _, _, err := PlayerInfo(&buffer, client); err != nil {
		t.Fatalf("player info failed: %v", err)
	}
	payload := decodeFirstPacketPayload(t, client.Buffer.Bytes())
	var response protobuf.SC_11003
	if err := proto.Unmarshal(payload, &response); err != nil {
		t.Fatalf("unmarshal player info: %v", err)
	}
	if response.GetChildDisplay() != 555 {
		t.Fatalf("expected child display 555, got %d", response.GetChildDisplay())
	}
}
