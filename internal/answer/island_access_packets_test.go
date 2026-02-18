package answer

import (
	"testing"

	"github.com/ggmolly/belfast/internal/orm"
	"github.com/ggmolly/belfast/internal/protobuf"
	"google.golang.org/protobuf/proto"
)

func TestIslandSetAccessTypeLegacy(t *testing.T) {
	client := setupHandlerCommander(t)
	clearTable(t, &orm.IslandSnapshot{})

	payload := protobuf.CS_21300{OpenFlag: proto.Uint32(1)}
	buffer, _ := proto.Marshal(&payload)
	if _, _, err := IslandSetAccessTypeLegacy(&buffer, client); err != nil {
		t.Fatalf("IslandSetAccessTypeLegacy failed: %v", err)
	}

	var response protobuf.SC_21301
	decodePacketAt(t, client, 0, 21301, &response)
	if response.GetResult() != 0 {
		t.Fatalf("expected success result, got %d", response.GetResult())
	}

	snapshot, err := orm.GetIslandSnapshot(client.Commander.CommanderID)
	if err != nil {
		t.Fatalf("load snapshot: %v", err)
	}
	if snapshot.OpenFlag != 1 {
		t.Fatalf("expected open flag persisted to 1, got %d", snapshot.OpenFlag)
	}

	client.Buffer.Reset()
	invalid := protobuf.CS_21300{OpenFlag: proto.Uint32(9)}
	invalidBuffer, _ := proto.Marshal(&invalid)
	if _, _, err := IslandSetAccessTypeLegacy(&invalidBuffer, client); err != nil {
		t.Fatalf("legacy invalid flag failed unexpectedly: %v", err)
	}
	decodePacketAt(t, client, 0, 21301, &response)
	if response.GetResult() == 0 {
		t.Fatalf("expected invalid flag failure")
	}
}

func TestIslandAccessOpFlowAndLimits(t *testing.T) {
	client := setupHandlerCommander(t)
	clearTable(t, &orm.ConfigEntry{})
	clearTable(t, &orm.CommanderIslandSocialState{})
	seedConfigEntry(t, islandSetCategory, "whit_list_max_cnt", `{"key":"whit_list_max_cnt","key_value_int":2}`)

	setWhite := protobuf.CS_21302{Cmd: proto.Uint32(1), UserIdList: []uint32{1001, 1001, 1002}}
	setWhiteBuffer, _ := proto.Marshal(&setWhite)
	if _, _, err := IslandAccessOp(&setWhiteBuffer, client); err != nil {
		t.Fatalf("set whitelist failed: %v", err)
	}
	var response protobuf.SC_21303
	decodePacketAt(t, client, 0, 21303, &response)
	if response.GetResult() != 0 {
		t.Fatalf("expected whitelist success")
	}

	state, err := orm.GetCommanderIslandSocialState(client.Commander.CommanderID)
	if err != nil {
		t.Fatalf("load social state: %v", err)
	}
	if len(state.WhiteList) != 2 {
		t.Fatalf("expected deduped whitelist, got %v", state.WhiteList)
	}

	setBlack := protobuf.CS_21302{Cmd: proto.Uint32(2), UserIdList: []uint32{2001}}
	setBlackBuffer, _ := proto.Marshal(&setBlack)
	client.Buffer.Reset()
	if _, _, err := IslandAccessOp(&setBlackBuffer, client); err != nil {
		t.Fatalf("set blacklist failed: %v", err)
	}
	decodePacketAt(t, client, 0, 21303, &response)
	if response.GetResult() != 0 {
		t.Fatalf("expected blacklist success")
	}

	kick := protobuf.CS_21302{Cmd: proto.Uint32(3), UserIdList: []uint32{9999}}
	kickBuffer, _ := proto.Marshal(&kick)
	client.Buffer.Reset()
	if _, _, err := IslandAccessOp(&kickBuffer, client); err != nil {
		t.Fatalf("kick op failed: %v", err)
	}
	decodePacketAt(t, client, 0, 21303, &response)
	if response.GetResult() != 0 {
		t.Fatalf("expected kick success")
	}

	stateBefore, _ := orm.GetCommanderIslandSocialState(client.Commander.CommanderID)
	if len(stateBefore.BlackList) != 1 {
		t.Fatalf("expected blacklist unchanged by kick, got %v", stateBefore.BlackList)
	}

	kickBlack := protobuf.CS_21302{Cmd: proto.Uint32(4), UserIdList: []uint32{2001, 3001}}
	kickBlackBuffer, _ := proto.Marshal(&kickBlack)
	client.Buffer.Reset()
	if _, _, err := IslandAccessOp(&kickBlackBuffer, client); err != nil {
		t.Fatalf("kick+black failed: %v", err)
	}
	decodePacketAt(t, client, 0, 21303, &response)
	if response.GetResult() != 0 {
		t.Fatalf("expected kick+black success")
	}

	stateAfter, _ := orm.GetCommanderIslandSocialState(client.Commander.CommanderID)
	if len(stateAfter.BlackList) != 2 {
		t.Fatalf("expected merged blacklist, got %v", stateAfter.BlackList)
	}

	overLimit := protobuf.CS_21302{Cmd: proto.Uint32(1), UserIdList: []uint32{1, 2, 3}}
	overLimitBuffer, _ := proto.Marshal(&overLimit)
	client.Buffer.Reset()
	if _, _, err := IslandAccessOp(&overLimitBuffer, client); err != nil {
		t.Fatalf("over-limit request failed unexpectedly: %v", err)
	}
	decodePacketAt(t, client, 0, 21303, &response)
	if response.GetResult() == 0 {
		t.Fatalf("expected over-limit failure")
	}

	stateFinal, _ := orm.GetCommanderIslandSocialState(client.Commander.CommanderID)
	if len(stateFinal.WhiteList) != 2 {
		t.Fatalf("expected whitelist unchanged after over-limit, got %v", stateFinal.WhiteList)
	}
}

func TestIslandAccessOpSequentialSetConverges(t *testing.T) {
	client := setupHandlerCommander(t)
	clearTable(t, &orm.ConfigEntry{})
	clearTable(t, &orm.CommanderIslandSocialState{})
	seedConfigEntry(t, islandSetCategory, "whit_list_max_cnt", `{"key":"whit_list_max_cnt","key_value_int":10}`)

	requests := []*protobuf.CS_21302{
		{Cmd: proto.Uint32(1), UserIdList: []uint32{10, 20}},
		{Cmd: proto.Uint32(2), UserIdList: []uint32{20}},
		{Cmd: proto.Uint32(1), UserIdList: []uint32{10}},
	}
	for _, req := range requests {
		buffer, _ := proto.Marshal(req)
		client.Buffer.Reset()
		if _, _, err := IslandAccessOp(&buffer, client); err != nil {
			t.Fatalf("IslandAccessOp failed: %v", err)
		}
		var response protobuf.SC_21303
		decodePacketAt(t, client, 0, 21303, &response)
		if response.GetResult() != 0 {
			t.Fatalf("expected sequential set success, got %d", response.GetResult())
		}
	}

	state, err := orm.GetCommanderIslandSocialState(client.Commander.CommanderID)
	if err != nil {
		t.Fatalf("load social state: %v", err)
	}
	if len(state.WhiteList) != 1 || state.WhiteList[0] != 10 {
		t.Fatalf("unexpected final whitelist: %v", state.WhiteList)
	}
	if len(state.BlackList) != 1 || state.BlackList[0] != 20 {
		t.Fatalf("unexpected final blacklist: %v", state.BlackList)
	}
}
