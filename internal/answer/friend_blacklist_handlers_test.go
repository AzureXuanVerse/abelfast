package answer

import (
	"fmt"
	"testing"
	"time"

	"github.com/ggmolly/belfast/internal/orm"
	"github.com/ggmolly/belfast/internal/protobuf"
	"google.golang.org/protobuf/proto"
)

func TestFriendBlacklistFlowAddFetchRemove(t *testing.T) {
	client := setupHandlerCommander(t)
	clearTable(t, &orm.CommanderIslandSocialState{})

	targetID := uint32(time.Now().UnixNano())
	if err := orm.CreateCommanderRoot(targetID, targetID, fmt.Sprintf("Blacklist Target %d", targetID), 0, 0); err != nil {
		t.Fatalf("create target commander: %v", err)
	}

	addPayload := protobuf.CS_50109{Id: proto.Uint32(targetID)}
	addBuffer, err := proto.Marshal(&addPayload)
	if err != nil {
		t.Fatalf("marshal add payload: %v", err)
	}

	if _, _, err := AddFriendBlacklist(&addBuffer, client); err != nil {
		t.Fatalf("add friend blacklist: %v", err)
	}

	var addResponse protobuf.SC_50110
	decodePacketAt(t, client, 0, 50110, &addResponse)
	if addResponse.GetResult() != friendBlacklistResultSuccess {
		t.Fatalf("expected add success, got %d", addResponse.GetResult())
	}

	client.Buffer.Reset()
	fetchPayload := protobuf.CS_50016{Type: proto.Uint32(0)}
	fetchBuffer, err := proto.Marshal(&fetchPayload)
	if err != nil {
		t.Fatalf("marshal fetch payload: %v", err)
	}
	if _, _, err := GetFriendBlacklist(&fetchBuffer, client); err != nil {
		t.Fatalf("fetch friend blacklist: %v", err)
	}

	var fetchResponse protobuf.SC_50017
	decodePacketAt(t, client, 0, 50017, &fetchResponse)
	if len(fetchResponse.GetBlackList()) != 1 {
		t.Fatalf("expected 1 blacklisted entry, got %d", len(fetchResponse.GetBlackList()))
	}
	entry := fetchResponse.GetBlackList()[0]
	if entry.GetId() != targetID {
		t.Fatalf("expected blacklisted id %d, got %d", targetID, entry.GetId())
	}

	client.Buffer.Reset()
	removePayload := protobuf.CS_50107{Id: proto.Uint32(targetID)}
	removeBuffer, err := proto.Marshal(&removePayload)
	if err != nil {
		t.Fatalf("marshal remove payload: %v", err)
	}
	if _, _, err := RelieveFriendBlacklist(&removeBuffer, client); err != nil {
		t.Fatalf("remove friend blacklist: %v", err)
	}

	var removeResponse protobuf.SC_50108
	decodePacketAt(t, client, 0, 50108, &removeResponse)
	if removeResponse.GetResult() != friendBlacklistResultSuccess {
		t.Fatalf("expected remove success, got %d", removeResponse.GetResult())
	}

	blacklist, err := orm.GetFriendBlacklist(client.Commander.CommanderID)
	if err != nil {
		t.Fatalf("reload blacklist: %v", err)
	}
	if len(blacklist) != 0 {
		t.Fatalf("expected blacklist to be empty after remove, got %v", blacklist)
	}
}

func TestFriendBlacklistAddValidationAndIdempotency(t *testing.T) {
	client := setupHandlerCommander(t)
	clearTable(t, &orm.CommanderIslandSocialState{})

	zeroPayload := protobuf.CS_50109{Id: proto.Uint32(0)}
	zeroBuffer, _ := proto.Marshal(&zeroPayload)
	if _, _, err := AddFriendBlacklist(&zeroBuffer, client); err != nil {
		t.Fatalf("zero target should return result, got error: %v", err)
	}
	var zeroResponse protobuf.SC_50110
	decodePacketAt(t, client, 0, 50110, &zeroResponse)
	if zeroResponse.GetResult() != friendBlacklistResultInvalidTarget {
		t.Fatalf("expected invalid target result for id=0, got %d", zeroResponse.GetResult())
	}

	client.Buffer.Reset()
	selfPayload := protobuf.CS_50109{Id: proto.Uint32(client.Commander.CommanderID)}
	selfBuffer, _ := proto.Marshal(&selfPayload)
	if _, _, err := AddFriendBlacklist(&selfBuffer, client); err != nil {
		t.Fatalf("self target should return result, got error: %v", err)
	}
	var selfResponse protobuf.SC_50110
	decodePacketAt(t, client, 0, 50110, &selfResponse)
	if selfResponse.GetResult() != friendBlacklistResultInvalidTarget {
		t.Fatalf("expected invalid target result for self target, got %d", selfResponse.GetResult())
	}

	client.Buffer.Reset()
	missingID := uint32(time.Now().UnixNano())
	missingPayload := protobuf.CS_50109{Id: proto.Uint32(missingID)}
	missingBuffer, _ := proto.Marshal(&missingPayload)
	if _, _, err := AddFriendBlacklist(&missingBuffer, client); err != nil {
		t.Fatalf("missing target should return result, got error: %v", err)
	}
	var missingResponse protobuf.SC_50110
	decodePacketAt(t, client, 0, 50110, &missingResponse)
	if missingResponse.GetResult() != friendBlacklistResultInvalidTarget {
		t.Fatalf("expected invalid target result for missing target, got %d", missingResponse.GetResult())
	}

	targetID := missingID + 1
	if err := orm.CreateCommanderRoot(targetID, targetID, fmt.Sprintf("Blacklist Add Duplicate %d", targetID), 0, 0); err != nil {
		t.Fatalf("create target commander: %v", err)
	}

	client.Buffer.Reset()
	payload := protobuf.CS_50109{Id: proto.Uint32(targetID)}
	buffer, _ := proto.Marshal(&payload)
	if _, _, err := AddFriendBlacklist(&buffer, client); err != nil {
		t.Fatalf("first add failed: %v", err)
	}
	var firstResponse protobuf.SC_50110
	decodePacketAt(t, client, 0, 50110, &firstResponse)
	if firstResponse.GetResult() != friendBlacklistResultSuccess {
		t.Fatalf("expected first add success, got %d", firstResponse.GetResult())
	}

	client.Buffer.Reset()
	if _, _, err := AddFriendBlacklist(&buffer, client); err != nil {
		t.Fatalf("duplicate add failed: %v", err)
	}
	var secondResponse protobuf.SC_50110
	decodePacketAt(t, client, 0, 50110, &secondResponse)
	if secondResponse.GetResult() != friendBlacklistResultSuccess {
		t.Fatalf("expected duplicate add success, got %d", secondResponse.GetResult())
	}

	blacklist, err := orm.GetFriendBlacklist(client.Commander.CommanderID)
	if err != nil {
		t.Fatalf("get blacklist: %v", err)
	}
	if len(blacklist) != 1 || blacklist[0] != targetID {
		t.Fatalf("expected one deduped blacklist entry, got %v", blacklist)
	}
}

func TestFriendBlacklistRemoveFailurePaths(t *testing.T) {
	client := setupHandlerCommander(t)
	clearTable(t, &orm.CommanderIslandSocialState{})

	zeroPayload := protobuf.CS_50107{Id: proto.Uint32(0)}
	zeroBuffer, _ := proto.Marshal(&zeroPayload)
	if _, _, err := RelieveFriendBlacklist(&zeroBuffer, client); err != nil {
		t.Fatalf("zero target should return result, got error: %v", err)
	}
	var zeroResponse protobuf.SC_50108
	decodePacketAt(t, client, 0, 50108, &zeroResponse)
	if zeroResponse.GetResult() != friendBlacklistResultInvalidTarget {
		t.Fatalf("expected invalid target result for id=0, got %d", zeroResponse.GetResult())
	}

	client.Buffer.Reset()
	selfPayload := protobuf.CS_50107{Id: proto.Uint32(client.Commander.CommanderID)}
	selfBuffer, _ := proto.Marshal(&selfPayload)
	if _, _, err := RelieveFriendBlacklist(&selfBuffer, client); err != nil {
		t.Fatalf("self target should return result, got error: %v", err)
	}
	var selfResponse protobuf.SC_50108
	decodePacketAt(t, client, 0, 50108, &selfResponse)
	if selfResponse.GetResult() != friendBlacklistResultInvalidTarget {
		t.Fatalf("expected invalid target result for self target, got %d", selfResponse.GetResult())
	}

	client.Buffer.Reset()
	missingPayload := protobuf.CS_50107{Id: proto.Uint32(12345)}
	missingBuffer, _ := proto.Marshal(&missingPayload)
	if _, _, err := RelieveFriendBlacklist(&missingBuffer, client); err != nil {
		t.Fatalf("missing target should return result, got error: %v", err)
	}
	var missingResponse protobuf.SC_50108
	decodePacketAt(t, client, 0, 50108, &missingResponse)
	if missingResponse.GetResult() != friendBlacklistResultNotFound {
		t.Fatalf("expected not found result, got %d", missingResponse.GetResult())
	}
}

func TestFriendBlacklistFetchSkipsMissingCommanders(t *testing.T) {
	client := setupHandlerCommander(t)
	clearTable(t, &orm.CommanderIslandSocialState{})

	existingID := uint32(time.Now().UnixNano())
	if err := orm.CreateCommanderRoot(existingID, existingID, fmt.Sprintf("Blacklist Existing %d", existingID), 0, 0); err != nil {
		t.Fatalf("create existing commander: %v", err)
	}

	state, err := orm.GetOrCreateCommanderIslandSocialState(client.Commander.CommanderID)
	if err != nil {
		t.Fatalf("get social state: %v", err)
	}
	state.BlackList = []uint32{existingID, existingID + 99}
	if err := orm.SaveCommanderIslandSocialState(state); err != nil {
		t.Fatalf("save social state: %v", err)
	}

	payload := protobuf.CS_50016{Type: proto.Uint32(0)}
	buffer, _ := proto.Marshal(&payload)
	if _, _, err := GetFriendBlacklist(&buffer, client); err != nil {
		t.Fatalf("fetch friend blacklist: %v", err)
	}

	var response protobuf.SC_50017
	decodePacketAt(t, client, 0, 50017, &response)
	if len(response.GetBlackList()) != 1 {
		t.Fatalf("expected only existing commander in response, got %d", len(response.GetBlackList()))
	}
	if response.GetBlackList()[0].GetId() != existingID {
		t.Fatalf("expected existing commander id %d, got %d", existingID, response.GetBlackList()[0].GetId())
	}
}

func TestFriendBlacklistDecodeFailures(t *testing.T) {
	client := setupHandlerCommander(t)

	invalid := []byte{0xff, 0x00}
	if _, outID, err := GetFriendBlacklist(&invalid, client); err == nil || outID != 50017 {
		t.Fatalf("expected fetch decode error with outgoing id 50017, got err=%v out=%d", err, outID)
	}
	if _, outID, err := AddFriendBlacklist(&invalid, client); err == nil || outID != 50110 {
		t.Fatalf("expected add decode error with outgoing id 50110, got err=%v out=%d", err, outID)
	}
	if _, outID, err := RelieveFriendBlacklist(&invalid, client); err == nil || outID != 50108 {
		t.Fatalf("expected remove decode error with outgoing id 50108, got err=%v out=%d", err, outID)
	}
}
