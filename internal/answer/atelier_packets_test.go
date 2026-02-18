package answer

import (
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/orm"
	"github.com/ggmolly/belfast/internal/protobuf"
	"google.golang.org/protobuf/proto"
)

const (
	atelierTestActID        = uint32(8801)
	atelierTestInvalidActID = uint32(8802)
)

func TestAtelierCompositeSuccessPersistsState(t *testing.T) {
	client := setupAtelierTestClient(t)
	seedAtelierRecipeConfig(t, 5001, 1001, 201, 3, []uint32{7001, 7002})
	seedAtelierCircleConfig(t, 7001, 5001, 101)
	seedAtelierCircleConfig(t, 7002, 5001, 102)

	state, err := orm.GetOrCreateAtelierState(client.Commander.CommanderID, atelierTestActID)
	if err != nil {
		t.Fatalf("get atelier state: %v", err)
	}
	state.Items[101] = 10
	state.Items[102] = 6
	if err := orm.SaveAtelierState(state); err != nil {
		t.Fatalf("save atelier state: %v", err)
	}

	payload := marshalPacketRequest(t, &protobuf.CS_26053{
		ActId:    proto.Uint32(atelierTestActID),
		RecipeId: proto.Uint32(5001),
		Items: []*protobuf.KVDATA{
			{Key: proto.Uint32(101), Value: proto.Uint32(2)},
			{Key: proto.Uint32(102), Value: proto.Uint32(1)},
		},
		Times: proto.Uint32(2),
	})
	if _, _, err := AtelierComposite(&payload, client); err != nil {
		t.Fatalf("AtelierComposite failed: %v", err)
	}

	response := &protobuf.SC_26054{}
	decodeLoveLetterPacketMessage(t, client, 26054, response)
	if response.GetResult() != atelierResultSuccess {
		t.Fatalf("expected result 0, got %d", response.GetResult())
	}
	if len(response.GetAwardList()) != 1 {
		t.Fatalf("expected one reward, got %+v", response.GetAwardList())
	}
	if response.GetAwardList()[0].GetType() != 1001 || response.GetAwardList()[0].GetId() != 201 || response.GetAwardList()[0].GetNumber() != 2 {
		t.Fatalf("unexpected reward %+v", response.GetAwardList()[0])
	}

	state, err = orm.GetOrCreateAtelierState(client.Commander.CommanderID, atelierTestActID)
	if err != nil {
		t.Fatalf("reload atelier state: %v", err)
	}
	if state.Items[101] != 6 || state.Items[102] != 4 {
		t.Fatalf("unexpected consumed items: %+v", state.Items)
	}
	if state.Items[201] != 2 {
		t.Fatalf("expected synthesized ryza item count 2, got %d", state.Items[201])
	}
	if state.RecipeUses[5001] != 2 {
		t.Fatalf("expected recipe use count 2, got %d", state.RecipeUses[5001])
	}
}

func TestAtelierCompositeValidationAndLimits(t *testing.T) {
	client := setupAtelierTestClient(t)
	seedAtelierRecipeConfig(t, 5002, 1001, 202, 2, []uint32{7003})
	seedAtelierCircleConfig(t, 7003, 5002, 103)

	state, err := orm.GetOrCreateAtelierState(client.Commander.CommanderID, atelierTestActID)
	if err != nil {
		t.Fatalf("get atelier state: %v", err)
	}
	state.Items[103] = 1
	state.RecipeUses[5002] = 1
	if err := orm.SaveAtelierState(state); err != nil {
		t.Fatalf("save atelier state: %v", err)
	}

	insufficientPayload := marshalPacketRequest(t, &protobuf.CS_26053{
		ActId:    proto.Uint32(atelierTestActID),
		RecipeId: proto.Uint32(5002),
		Items:    []*protobuf.KVDATA{{Key: proto.Uint32(103), Value: proto.Uint32(2)}},
		Times:    proto.Uint32(1),
	})
	if _, _, err := AtelierComposite(&insufficientPayload, client); err != nil {
		t.Fatalf("AtelierComposite insufficient failed: %v", err)
	}
	insufficientResp := &protobuf.SC_26054{}
	decodeLoveLetterPacketMessage(t, client, 26054, insufficientResp)
	if insufficientResp.GetResult() != atelierResultInsufficientItems {
		t.Fatalf("expected insufficient result, got %d", insufficientResp.GetResult())
	}
	if len(insufficientResp.GetAwardList()) != 0 {
		t.Fatalf("expected no awards on failure")
	}

	limitPayload := marshalPacketRequest(t, &protobuf.CS_26053{
		ActId:    proto.Uint32(atelierTestActID),
		RecipeId: proto.Uint32(5002),
		Items:    []*protobuf.KVDATA{{Key: proto.Uint32(103), Value: proto.Uint32(1)}},
		Times:    proto.Uint32(2),
	})
	if _, _, err := AtelierComposite(&limitPayload, client); err != nil {
		t.Fatalf("AtelierComposite limit failed: %v", err)
	}
	limitResp := &protobuf.SC_26054{}
	decodeLoveLetterPacketMessage(t, client, 26054, limitResp)
	if limitResp.GetResult() != atelierResultRecipeLimitReached {
		t.Fatalf("expected limit result, got %d", limitResp.GetResult())
	}

	invalidItemsPayload := marshalPacketRequest(t, &protobuf.CS_26053{
		ActId:    proto.Uint32(atelierTestActID),
		RecipeId: proto.Uint32(5002),
		Items:    []*protobuf.KVDATA{{Key: proto.Uint32(999), Value: proto.Uint32(1)}},
		Times:    proto.Uint32(1),
	})
	if _, _, err := AtelierComposite(&invalidItemsPayload, client); err != nil {
		t.Fatalf("AtelierComposite invalid items failed: %v", err)
	}
	invalidItemsResp := &protobuf.SC_26054{}
	decodeLoveLetterPacketMessage(t, client, 26054, invalidItemsResp)
	if invalidItemsResp.GetResult() != atelierResultInvalidRecipeOrItem {
		t.Fatalf("expected invalid recipe/item result, got %d", invalidItemsResp.GetResult())
	}

	invalidActivityPayload := marshalPacketRequest(t, &protobuf.CS_26053{
		ActId:    proto.Uint32(atelierTestInvalidActID),
		RecipeId: proto.Uint32(5002),
		Items:    []*protobuf.KVDATA{{Key: proto.Uint32(103), Value: proto.Uint32(1)}},
		Times:    proto.Uint32(1),
	})
	if _, _, err := AtelierComposite(&invalidActivityPayload, client); err != nil {
		t.Fatalf("AtelierComposite invalid activity failed: %v", err)
	}
	invalidActivityResp := &protobuf.SC_26054{}
	decodeLoveLetterPacketMessage(t, client, 26054, invalidActivityResp)
	if invalidActivityResp.GetResult() != atelierResultInvalidActivity {
		t.Fatalf("expected invalid activity result, got %d", invalidActivityResp.GetResult())
	}

	state, err = orm.GetOrCreateAtelierState(client.Commander.CommanderID, atelierTestActID)
	if err != nil {
		t.Fatalf("reload atelier state: %v", err)
	}
	if state.Items[103] != 1 || state.RecipeUses[5002] != 1 {
		t.Fatalf("expected failed requests to preserve state, got %+v", state)
	}
}

func TestAtelierCompositeConcurrencyAllowsSingleSuccess(t *testing.T) {
	client := setupAtelierTestClient(t)
	seedAtelierRecipeConfig(t, 5003, 1001, 203, 10, []uint32{7004})
	seedAtelierCircleConfig(t, 7004, 5003, 104)

	state, err := orm.GetOrCreateAtelierState(client.Commander.CommanderID, atelierTestActID)
	if err != nil {
		t.Fatalf("get atelier state: %v", err)
	}
	state.Items[104] = 2
	if err := orm.SaveAtelierState(state); err != nil {
		t.Fatalf("save atelier state: %v", err)
	}

	payload := marshalPacketRequest(t, &protobuf.CS_26053{
		ActId:    proto.Uint32(atelierTestActID),
		RecipeId: proto.Uint32(5003),
		Items:    []*protobuf.KVDATA{{Key: proto.Uint32(104), Value: proto.Uint32(2)}},
		Times:    proto.Uint32(1),
	})

	clientA := newAtelierCommanderClient(t, client.Commander.CommanderID)
	clientB := newAtelierCommanderClient(t, client.Commander.CommanderID)

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		_, _, _ = AtelierComposite(&payload, clientA)
	}()
	go func() {
		defer wg.Done()
		_, _, _ = AtelierComposite(&payload, clientB)
	}()
	wg.Wait()

	respA := &protobuf.SC_26054{}
	decodeLoveLetterPacketMessage(t, clientA, 26054, respA)
	respB := &protobuf.SC_26054{}
	decodeLoveLetterPacketMessage(t, clientB, 26054, respB)

	successes := 0
	if respA.GetResult() == atelierResultSuccess {
		successes++
	}
	if respB.GetResult() == atelierResultSuccess {
		successes++
	}
	if successes != 1 {
		t.Fatalf("expected exactly one successful synthesis, got %d (a=%d b=%d)", successes, respA.GetResult(), respB.GetResult())
	}

	state, err = orm.GetOrCreateAtelierState(client.Commander.CommanderID, atelierTestActID)
	if err != nil {
		t.Fatalf("reload atelier state: %v", err)
	}
	if state.Items[104] != 0 {
		t.Fatalf("expected materials fully consumed, got %d", state.Items[104])
	}
	if state.RecipeUses[5003] != 1 {
		t.Fatalf("expected recipe use count 1, got %d", state.RecipeUses[5003])
	}
}

func TestAtelierRefreshBuffAndRequestReadback(t *testing.T) {
	client := setupAtelierTestClient(t)
	seedAtelierBuffItemConfig(t, 301, []uint32{11, 12, 13})
	seedAtelierBuffItemConfig(t, 302, []uint32{21})
	state, err := orm.GetOrCreateAtelierState(client.Commander.CommanderID, atelierTestActID)
	if err != nil {
		t.Fatalf("load atelier state: %v", err)
	}
	state.Items[301] = 1
	state.Items[302] = 1
	if err := orm.SaveAtelierState(state); err != nil {
		t.Fatalf("save atelier state: %v", err)
	}

	payload := marshalPacketRequest(t, &protobuf.CS_26055{
		ActId: proto.Uint32(atelierTestActID),
		Slots: []*protobuf.BUFF_SLOT{
			{Pos: proto.Uint32(1), Itemid: proto.Uint32(301), Itemnum: proto.Uint32(2)},
			{Pos: proto.Uint32(3), Itemid: proto.Uint32(302), Itemnum: proto.Uint32(1)},
		},
	})
	if _, _, err := AtelierRefreshBuff(&payload, client); err != nil {
		t.Fatalf("AtelierRefreshBuff failed: %v", err)
	}
	refreshResp := &protobuf.SC_26056{}
	decodeLoveLetterPacketMessage(t, client, 26056, refreshResp)
	if refreshResp.GetResult() != atelierResultSuccess {
		t.Fatalf("expected refresh success, got %d", refreshResp.GetResult())
	}

	requestPayload := marshalPacketRequest(t, &protobuf.CS_26051{ActId: proto.Uint32(atelierTestActID)})
	if _, _, err := AtelierRequest(&requestPayload, client); err != nil {
		t.Fatalf("AtelierRequest failed: %v", err)
	}
	requestResp := &protobuf.SC_26052{}
	decodeLoveLetterPacketMessage(t, client, 26052, requestResp)
	if requestResp.GetResult() != atelierResultSuccess {
		t.Fatalf("expected request success, got %d", requestResp.GetResult())
	}
	if len(requestResp.GetSlots()) != 5 {
		t.Fatalf("expected five slots in readback, got %d", len(requestResp.GetSlots()))
	}
	if requestResp.GetSlots()[0].GetItemid() != 301 || requestResp.GetSlots()[0].GetItemnum() != 2 {
		t.Fatalf("unexpected slot 1 state: %+v", requestResp.GetSlots()[0])
	}
	if requestResp.GetSlots()[2].GetItemid() != 302 || requestResp.GetSlots()[2].GetItemnum() != 1 {
		t.Fatalf("unexpected slot 3 state: %+v", requestResp.GetSlots()[2])
	}
	if requestResp.GetSlots()[1].GetItemid() != 0 || requestResp.GetSlots()[4].GetItemid() != 0 {
		t.Fatalf("expected omitted slots to be cleared, got %+v", requestResp.GetSlots())
	}
}

func TestAtelierRefreshBuffFailuresAndFullReplace(t *testing.T) {
	client := setupAtelierTestClient(t)
	seedAtelierBuffItemConfig(t, 401, []uint32{1, 2})
	seedAtelierBuffItemConfig(t, 402, []uint32{1})
	seedAtelierBuffItemConfig(t, 403, []uint32{1})
	seedAtelierNonBuffItemConfig(t, 499)
	state, err := orm.GetOrCreateAtelierState(client.Commander.CommanderID, atelierTestActID)
	if err != nil {
		t.Fatalf("load atelier state: %v", err)
	}
	state.Items[401] = 1
	state.Items[402] = 1
	if err := orm.SaveAtelierState(state); err != nil {
		t.Fatalf("save atelier state: %v", err)
	}

	seedPayload := marshalPacketRequest(t, &protobuf.CS_26055{
		ActId: proto.Uint32(atelierTestActID),
		Slots: []*protobuf.BUFF_SLOT{{Pos: proto.Uint32(2), Itemid: proto.Uint32(401), Itemnum: proto.Uint32(1)}},
	})
	if _, _, err := AtelierRefreshBuff(&seedPayload, client); err != nil {
		t.Fatalf("seed AtelierRefreshBuff failed: %v", err)
	}
	seedResp := &protobuf.SC_26056{}
	decodeLoveLetterPacketMessage(t, client, 26056, seedResp)
	if seedResp.GetResult() != atelierResultSuccess {
		t.Fatalf("expected seed success")
	}

	cases := []struct {
		name   string
		slots  []*protobuf.BUFF_SLOT
		result uint32
	}{
		{name: "invalid position", slots: []*protobuf.BUFF_SLOT{{Pos: proto.Uint32(6), Itemid: proto.Uint32(401), Itemnum: proto.Uint32(1)}}, result: atelierResultMalformedRequest},
		{name: "invalid item", slots: []*protobuf.BUFF_SLOT{{Pos: proto.Uint32(1), Itemid: proto.Uint32(9999), Itemnum: proto.Uint32(1)}}, result: atelierResultInvalidRecipeOrItem},
		{name: "non buff item", slots: []*protobuf.BUFF_SLOT{{Pos: proto.Uint32(1), Itemid: proto.Uint32(499), Itemnum: proto.Uint32(1)}}, result: atelierResultInvalidRecipeOrItem},
		{name: "missing owned item", slots: []*protobuf.BUFF_SLOT{{Pos: proto.Uint32(1), Itemid: proto.Uint32(403), Itemnum: proto.Uint32(1)}}, result: atelierResultInvalidRecipeOrItem},
		{name: "duplicate item", slots: []*protobuf.BUFF_SLOT{{Pos: proto.Uint32(1), Itemid: proto.Uint32(401), Itemnum: proto.Uint32(1)}, {Pos: proto.Uint32(3), Itemid: proto.Uint32(401), Itemnum: proto.Uint32(1)}}, result: atelierResultMalformedRequest},
		{name: "out of range level", slots: []*protobuf.BUFF_SLOT{{Pos: proto.Uint32(1), Itemid: proto.Uint32(402), Itemnum: proto.Uint32(2)}}, result: atelierResultMalformedRequest},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			payload := marshalPacketRequest(t, &protobuf.CS_26055{ActId: proto.Uint32(atelierTestActID), Slots: tc.slots})
			if _, _, err := AtelierRefreshBuff(&payload, client); err != nil {
				t.Fatalf("AtelierRefreshBuff failed: %v", err)
			}
			resp := &protobuf.SC_26056{}
			decodeLoveLetterPacketMessage(t, client, 26056, resp)
			if resp.GetResult() != tc.result {
				t.Fatalf("expected %d got %d", tc.result, resp.GetResult())
			}
		})
	}

	partialPayload := marshalPacketRequest(t, &protobuf.CS_26055{
		ActId: proto.Uint32(atelierTestActID),
		Slots: []*protobuf.BUFF_SLOT{{Pos: proto.Uint32(1), Itemid: proto.Uint32(402), Itemnum: proto.Uint32(1)}},
	})
	if _, _, err := AtelierRefreshBuff(&partialPayload, client); err != nil {
		t.Fatalf("AtelierRefreshBuff partial failed: %v", err)
	}
	partialResp := &protobuf.SC_26056{}
	decodeLoveLetterPacketMessage(t, client, 26056, partialResp)
	if partialResp.GetResult() != atelierResultSuccess {
		t.Fatalf("expected partial update success")
	}

	state, err = orm.GetOrCreateAtelierState(client.Commander.CommanderID, atelierTestActID)
	if err != nil {
		t.Fatalf("reload atelier state: %v", err)
	}
	if state.Slots[1].ItemID != 402 {
		t.Fatalf("expected slot 1 updated")
	}
	if state.Slots[2].ItemID != 0 {
		t.Fatalf("expected full replace semantics to clear omitted slot 2, got %+v", state.Slots[2])
	}
}

func setupAtelierTestClient(t *testing.T) *connection.Client {
	t.Helper()
	client := setupPlayerUpdateTest(t)
	seedConfigEntry(t, "ShareCfg/activity_template.json", "8801", `{"id":8801,"type":88,"time":"always"}`)
	seedConfigEntry(t, "ShareCfg/activity_template.json", "8802", `{"id":8802,"type":77,"time":"always"}`)
	return client
}

func newAtelierCommanderClient(t *testing.T, commanderID uint32) *connection.Client {
	t.Helper()
	commander, err := orm.GetCommanderCoreByID(commanderID)
	if err != nil {
		t.Fatalf("load commander core: %v", err)
	}
	if err := commander.Load(); err != nil {
		t.Fatalf("load commander details: %v", err)
	}
	return &connection.Client{Commander: commander}
}

func seedAtelierRecipeConfig(t *testing.T, recipeID uint32, dropType uint32, dropID uint32, itemNum uint32, circles []uint32) {
	t.Helper()
	seedConfigEntry(t, atelierRecipeCategory, protoID(recipeID), `{"id":`+protoID(recipeID)+`,"item_id":[`+protoID(dropType)+`,`+protoID(dropID)+`],"item_num":`+protoID(itemNum)+`,"recipe_circle":[`+joinUint32(circles)+`]}`)
}

func seedAtelierCircleConfig(t *testing.T, circleID uint32, recipeID uint32, ryzaItemID uint32) {
	t.Helper()
	seedConfigEntry(t, atelierCircleCategory, protoID(circleID), `{"id":`+protoID(circleID)+`,"recipe_id":`+protoID(recipeID)+`,"ryza_item_id":`+protoID(ryzaItemID)+`}`)
}

func seedAtelierBuffItemConfig(t *testing.T, itemID uint32, buffs []uint32) {
	t.Helper()
	seedConfigEntry(t, atelierItemCategory, protoID(itemID), `{"id":`+protoID(itemID)+`,"benefit_buff":[`+joinUint32(buffs)+`]}`)
}

func seedAtelierNonBuffItemConfig(t *testing.T, itemID uint32) {
	t.Helper()
	seedConfigEntry(t, atelierItemCategory, protoID(itemID), `{"id":`+protoID(itemID)+`,"benefit_buff":""}`)
}

func protoID(value uint32) string {
	return strconv.FormatUint(uint64(value), 10)
}

func joinUint32(values []uint32) string {
	if len(values) == 0 {
		return ""
	}
	out := make([]string, 0, len(values))
	for _, value := range values {
		out = append(out, protoID(value))
	}
	return strings.Join(out, ",")
}
