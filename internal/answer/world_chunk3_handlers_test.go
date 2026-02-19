package answer

import (
	"testing"
	"time"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/orm"
	"github.com/ggmolly/belfast/internal/packets"
	"github.com/ggmolly/belfast/internal/protobuf"
	"google.golang.org/protobuf/proto"
)

func setupWorldChunk3Client(commanderID uint32) *connection.Client {
	return &connection.Client{Commander: &orm.Commander{CommanderID: commanderID}}
}

func decodeChunk3Response(t *testing.T, client *connection.Client, expectedID int, message proto.Message) {
	t.Helper()
	buffer := client.Buffer.Bytes()
	if len(buffer) == 0 {
		t.Fatalf("expected response buffer")
	}

	packetID := packets.GetPacketId(0, &buffer)
	if packetID != expectedID {
		t.Fatalf("expected packet %d, got %d", expectedID, packetID)
	}

	packetSize := packets.GetPacketSize(0, &buffer) + 2
	if len(buffer) < packetSize {
		t.Fatalf("expected packet size %d, got %d", packetSize, len(buffer))
	}

	payloadStart := packets.HEADER_SIZE
	payloadEnd := payloadStart + (packetSize - packets.HEADER_SIZE)
	if err := proto.Unmarshal(buffer[payloadStart:payloadEnd], message); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	client.Buffer.Reset()
}

func TestWorldDailyTaskRefreshAndTriggerFlow(t *testing.T) {
	client := setupWorldChunk3Client(900001)

	refreshPayload := &protobuf.CS_33413{Type: proto.Uint32(0)}
	refreshBuf, err := proto.Marshal(refreshPayload)
	if err != nil {
		t.Fatalf("marshal refresh payload: %v", err)
	}
	if _, _, err := WorldDailyTaskRefresh(&refreshBuf, client); err != nil {
		t.Fatalf("WorldDailyTaskRefresh failed: %v", err)
	}

	refreshResponse := &protobuf.SC_33414{}
	decodeChunk3Response(t, client, 33414, refreshResponse)
	if refreshResponse.GetResult() != 0 {
		t.Fatalf("expected refresh success")
	}
	if refreshResponse.GetNextRefreshTime() <= uint32(time.Now().Unix()) {
		t.Fatalf("expected refresh time in future")
	}
	if len(refreshResponse.GetTaskList()) < 2 {
		t.Fatalf("expected sentinel and tasks")
	}

	triggerPayload := &protobuf.CS_33415{TaskList: []uint32{refreshResponse.GetTaskList()[1], refreshResponse.GetTaskList()[2]}}
	triggerBuf, err := proto.Marshal(triggerPayload)
	if err != nil {
		t.Fatalf("marshal trigger payload: %v", err)
	}
	if _, _, err := WorldTriggerDailyTask(&triggerBuf, client); err != nil {
		t.Fatalf("WorldTriggerDailyTask failed: %v", err)
	}

	triggerResponse := &protobuf.SC_33416{}
	decodeChunk3Response(t, client, 33416, triggerResponse)
	if triggerResponse.GetResult() != 0 {
		t.Fatalf("expected trigger success")
	}
	if len(triggerResponse.GetTaskList()) != 2 {
		t.Fatalf("expected two accepted tasks")
	}
}

func TestWorldTriggerDailyTaskRejectsAlreadyActive(t *testing.T) {
	client := setupWorldChunk3Client(900002)

	refreshPayload := &protobuf.CS_33413{Type: proto.Uint32(0)}
	refreshBuf, err := proto.Marshal(refreshPayload)
	if err != nil {
		t.Fatalf("marshal refresh payload: %v", err)
	}
	if _, _, err := WorldDailyTaskRefresh(&refreshBuf, client); err != nil {
		t.Fatalf("WorldDailyTaskRefresh failed: %v", err)
	}
	refreshResponse := &protobuf.SC_33414{}
	decodeChunk3Response(t, client, 33414, refreshResponse)

	taskID := refreshResponse.GetTaskList()[1]
	payload := &protobuf.CS_33415{TaskList: []uint32{taskID}}
	buf, err := proto.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal trigger payload: %v", err)
	}
	if _, _, err := WorldTriggerDailyTask(&buf, client); err != nil {
		t.Fatalf("first trigger failed: %v", err)
	}
	first := &protobuf.SC_33416{}
	decodeChunk3Response(t, client, 33416, first)
	if first.GetResult() != 0 {
		t.Fatalf("expected first trigger success")
	}

	if _, _, err := WorldTriggerDailyTask(&buf, client); err != nil {
		t.Fatalf("second trigger failed: %v", err)
	}
	second := &protobuf.SC_33416{}
	decodeChunk3Response(t, client, 33416, second)
	if second.GetResult() != worldChunk3ResultSameTask {
		t.Fatalf("expected same-task result %d, got %d", worldChunk3ResultSameTask, second.GetResult())
	}
}

func TestWorldPortRequestAndShopping(t *testing.T) {
	client := setupWorldChunk3Client(900003)

	request := &protobuf.CS_33401{MapId: proto.Uint32(77)}
	requestBuf, err := proto.Marshal(request)
	if err != nil {
		t.Fatalf("marshal request payload: %v", err)
	}
	if _, _, err := WorldPortRequest(&requestBuf, client); err != nil {
		t.Fatalf("WorldPortRequest failed: %v", err)
	}

	requestResponse := &protobuf.SC_33402{}
	decodeChunk3Response(t, client, 33402, requestResponse)
	if requestResponse.GetPort().GetPortId() == 0 {
		t.Fatalf("expected valid port id")
	}
	if len(requestResponse.GetPort().GetGoodsList()) == 0 {
		t.Fatalf("expected goods list")
	}

	goodsID := requestResponse.GetPort().GetGoodsList()[0].GetGoodsId()
	shopping := &protobuf.CS_33403{ShopId: proto.Uint32(goodsID), ShopType: proto.Uint32(1), Count: proto.Uint32(1)}
	shoppingBuf, err := proto.Marshal(shopping)
	if err != nil {
		t.Fatalf("marshal shopping payload: %v", err)
	}
	if _, _, err := WorldPortShopping(&shoppingBuf, client); err != nil {
		t.Fatalf("WorldPortShopping failed: %v", err)
	}

	shoppingResponse := &protobuf.SC_33404{}
	decodeChunk3Response(t, client, 33404, shoppingResponse)
	if shoppingResponse.GetResult() != 0 {
		t.Fatalf("expected shopping success")
	}
	if len(shoppingResponse.GetDropList()) != 1 {
		t.Fatalf("expected one drop")
	}

	shoppingTooMany := &protobuf.CS_33403{ShopId: proto.Uint32(goodsID), ShopType: proto.Uint32(1), Count: proto.Uint32(10)}
	shoppingTooManyBuf, err := proto.Marshal(shoppingTooMany)
	if err != nil {
		t.Fatalf("marshal shopping failure payload: %v", err)
	}
	if _, _, err := WorldPortShopping(&shoppingTooManyBuf, client); err != nil {
		t.Fatalf("WorldPortShopping failure path failed: %v", err)
	}

	failureResponse := &protobuf.SC_33404{}
	decodeChunk3Response(t, client, 33404, failureResponse)
	if failureResponse.GetResult() == 0 {
		t.Fatalf("expected shopping failure for over-limit count")
	}
}

func TestWorldPortShoppingDoesNotRestockSoldOutGoods(t *testing.T) {
	client := setupWorldChunk3Client(900007)

	request := &protobuf.CS_33401{MapId: proto.Uint32(88)}
	requestBuf, err := proto.Marshal(request)
	if err != nil {
		t.Fatalf("marshal request payload: %v", err)
	}
	if _, _, err := WorldPortRequest(&requestBuf, client); err != nil {
		t.Fatalf("WorldPortRequest failed: %v", err)
	}
	requestResponse := &protobuf.SC_33402{}
	decodeChunk3Response(t, client, 33402, requestResponse)

	goodsID := requestResponse.GetPort().GetGoodsList()[0].GetGoodsId()
	buyAll := &protobuf.CS_33403{ShopId: proto.Uint32(goodsID), ShopType: proto.Uint32(1), Count: proto.Uint32(worldChunk3PortGoodsDefault)}
	buyAllBuf, err := proto.Marshal(buyAll)
	if err != nil {
		t.Fatalf("marshal buy-all payload: %v", err)
	}
	if _, _, err := WorldPortShopping(&buyAllBuf, client); err != nil {
		t.Fatalf("WorldPortShopping buy-all failed: %v", err)
	}
	buyAllResp := &protobuf.SC_33404{}
	decodeChunk3Response(t, client, 33404, buyAllResp)
	if buyAllResp.GetResult() != 0 {
		t.Fatalf("expected buy-all success")
	}

	buyAgain := &protobuf.CS_33403{ShopId: proto.Uint32(goodsID), ShopType: proto.Uint32(1), Count: proto.Uint32(1)}
	buyAgainBuf, err := proto.Marshal(buyAgain)
	if err != nil {
		t.Fatalf("marshal buy-again payload: %v", err)
	}
	if _, _, err := WorldPortShopping(&buyAgainBuf, client); err != nil {
		t.Fatalf("WorldPortShopping buy-again failed: %v", err)
	}
	buyAgainResp := &protobuf.SC_33404{}
	decodeChunk3Response(t, client, 33404, buyAgainResp)
	if buyAgainResp.GetResult() == 0 {
		t.Fatalf("expected sold-out purchase to fail")
	}
}

func TestWorldFleetHandlers(t *testing.T) {
	client := setupWorldChunk3Client(900004)

	changePayload := &protobuf.CS_33405{FleetList: []*protobuf.FLEET_CHANGE{{GroupId: proto.Uint32(1), ShipId: []uint32{101, 102}}}}
	changeBuf, err := proto.Marshal(changePayload)
	if err != nil {
		t.Fatalf("marshal change payload: %v", err)
	}
	if _, _, err := WorldFleetChangeCompatibility(&changeBuf, client); err != nil {
		t.Fatalf("WorldFleetChangeCompatibility failed: %v", err)
	}
	changeResponse := &protobuf.SC_33406{}
	decodeChunk3Response(t, client, 33406, changeResponse)
	if changeResponse.GetResult() != 0 {
		t.Fatalf("expected fleet change success")
	}

	changeDuplicate := &protobuf.CS_33405{FleetList: []*protobuf.FLEET_CHANGE{{GroupId: proto.Uint32(2)}, {GroupId: proto.Uint32(2)}}}
	changeDuplicateBuf, err := proto.Marshal(changeDuplicate)
	if err != nil {
		t.Fatalf("marshal duplicate change payload: %v", err)
	}
	if _, _, err := WorldFleetChangeCompatibility(&changeDuplicateBuf, client); err != nil {
		t.Fatalf("WorldFleetChangeCompatibility duplicate path failed: %v", err)
	}
	changeDuplicateResponse := &protobuf.SC_33406{}
	decodeChunk3Response(t, client, 33406, changeDuplicateResponse)
	if changeDuplicateResponse.GetResult() == 0 {
		t.Fatalf("expected duplicate group failure")
	}

	redeployPayload := &protobuf.CS_33409{EliteFleetList: []*protobuf.ELITEFLEETINFO{{
		ShipIdList: []uint32{201, 202},
		Commanders: []*protobuf.COMMANDERSINFO{{Pos: proto.Uint32(1), Id: proto.Uint32(301)}},
	}}}
	redeployBuf, err := proto.Marshal(redeployPayload)
	if err != nil {
		t.Fatalf("marshal redeploy payload: %v", err)
	}
	if _, _, err := WorldFleetRedeploy(&redeployBuf, client); err != nil {
		t.Fatalf("WorldFleetRedeploy failed: %v", err)
	}
	redeployResponse := &protobuf.SC_33410{}
	decodeChunk3Response(t, client, 33410, redeployResponse)
	if redeployResponse.GetResult() != 0 {
		t.Fatalf("expected redeploy success")
	}
	if len(redeployResponse.GetGroupList()) != 1 {
		t.Fatalf("expected one group in redeploy response")
	}
	group := redeployResponse.GetGroupList()[0]
	if group.GetId() == 0 || group.GetPos() == nil || group.GetStartPos() == nil {
		t.Fatalf("expected required group fields to be populated")
	}
}

func TestWorldShipRepair(t *testing.T) {
	client := setupWorldChunk3Client(900005)

	emptyPayload := &protobuf.CS_33407{ShipList: []uint32{}}
	emptyBuf, err := proto.Marshal(emptyPayload)
	if err != nil {
		t.Fatalf("marshal empty payload: %v", err)
	}
	if _, _, err := WorldShipRepair(&emptyBuf, client); err != nil {
		t.Fatalf("WorldShipRepair empty path failed: %v", err)
	}
	emptyResponse := &protobuf.SC_33408{}
	decodeChunk3Response(t, client, 33408, emptyResponse)
	if emptyResponse.GetResult() == 0 {
		t.Fatalf("expected empty ship list to fail")
	}

	validPayload := &protobuf.CS_33407{ShipList: []uint32{11, 11, 12}}
	validBuf, err := proto.Marshal(validPayload)
	if err != nil {
		t.Fatalf("marshal valid payload: %v", err)
	}
	if _, _, err := WorldShipRepair(&validBuf, client); err != nil {
		t.Fatalf("WorldShipRepair valid path failed: %v", err)
	}
	validResponse := &protobuf.SC_33408{}
	decodeChunk3Response(t, client, 33408, validResponse)
	if validResponse.GetResult() != 0 {
		t.Fatalf("expected valid ship repair to succeed")
	}
}

func TestWorldBossSupportAndAchieveClaim(t *testing.T) {
	client := setupWorldChunk3Client(900006)

	supportPayload := &protobuf.CS_33509{Type: proto.Uint32(3)}
	supportBuf, err := proto.Marshal(supportPayload)
	if err != nil {
		t.Fatalf("marshal support payload: %v", err)
	}
	if _, _, err := WorldBossSupport(&supportBuf, client); err != nil {
		t.Fatalf("WorldBossSupport failed: %v", err)
	}
	supportResponse := &protobuf.SC_33510{}
	decodeChunk3Response(t, client, 33510, supportResponse)
	if supportResponse.GetResult() != 0 {
		t.Fatalf("expected supported type to succeed")
	}

	supportInvalidPayload := &protobuf.CS_33509{Type: proto.Uint32(99)}
	supportInvalidBuf, err := proto.Marshal(supportInvalidPayload)
	if err != nil {
		t.Fatalf("marshal invalid support payload: %v", err)
	}
	if _, _, err := WorldBossSupport(&supportInvalidBuf, client); err != nil {
		t.Fatalf("WorldBossSupport invalid path failed: %v", err)
	}
	supportInvalidResponse := &protobuf.SC_33510{}
	decodeChunk3Response(t, client, 33510, supportInvalidResponse)
	if supportInvalidResponse.GetResult() == 0 {
		t.Fatalf("expected unsupported type to fail")
	}

	claimPayload := &protobuf.CS_33602{List: []*protobuf.WORLDTARGET_FETCH{{Id: proto.Uint32(7), StarList: []uint32{1, 2}}}}
	claimBuf, err := proto.Marshal(claimPayload)
	if err != nil {
		t.Fatalf("marshal claim payload: %v", err)
	}
	if _, _, err := WorldAchieveClaim(&claimBuf, client); err != nil {
		t.Fatalf("WorldAchieveClaim failed: %v", err)
	}
	claimResponse := &protobuf.SC_33603{}
	decodeChunk3Response(t, client, 33603, claimResponse)
	if claimResponse.GetResult() != 0 {
		t.Fatalf("expected claim success")
	}
	if len(claimResponse.GetDrops()) != 2 {
		t.Fatalf("expected two drops")
	}

	if _, _, err := WorldAchieveClaim(&claimBuf, client); err != nil {
		t.Fatalf("WorldAchieveClaim duplicate path failed: %v", err)
	}
	claimDuplicateResponse := &protobuf.SC_33603{}
	decodeChunk3Response(t, client, 33603, claimDuplicateResponse)
	if claimDuplicateResponse.GetResult() == 0 {
		t.Fatalf("expected duplicate claim to fail")
	}
}
