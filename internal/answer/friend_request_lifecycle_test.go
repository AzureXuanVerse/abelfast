package answer

import (
	"fmt"
	"testing"
	"time"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/orm"
	"github.com/ggmolly/belfast/internal/protobuf"
	"google.golang.org/protobuf/proto"
)

func setupFriendClients(t *testing.T) (*connection.Client, *connection.Client) {
	t.Helper()
	requester := setupHandlerCommander(t)
	targetID := uint32(time.Now().UnixNano())
	if err := orm.CreateCommanderRoot(targetID, targetID, fmt.Sprintf("Target %d", targetID), 0, 0); err != nil {
		t.Fatalf("create target commander: %v", err)
	}
	targetCommander := orm.Commander{CommanderID: targetID}
	if err := targetCommander.Load(); err != nil {
		t.Fatalf("load target commander: %v", err)
	}
	target := &connection.Client{Commander: &targetCommander}
	server := connection.NewServer("127.0.0.1", 0, nil)
	requester.Server = server
	target.Server = server
	target.Hash = requester.Hash + 1
	server.AddClient(requester)
	server.AddClient(target)
	return requester, target
}

func TestSendFriendRequestSuccessAndDuplicate(t *testing.T) {
	requester, target := setupFriendClients(t)

	request := protobuf.CS_50003{Id: proto.Uint32(target.Commander.CommanderID), Content: proto.String("hello")}
	payload, err := proto.Marshal(&request)
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}

	if _, _, err := SendFriendRequest(&payload, requester); err != nil {
		t.Fatalf("send friend request failed: %v", err)
	}
	var response protobuf.SC_50004
	decodePacketAt(t, requester, 0, 50004, &response)
	if response.GetResult() != friendOperationSuccess {
		t.Fatalf("expected result 0, got %d", response.GetResult())
	}

	requests, err := orm.ListFriendRequestsForTarget(target.Commander.CommanderID)
	if err != nil {
		t.Fatalf("list requests: %v", err)
	}
	if len(requests) != 1 {
		t.Fatalf("expected one friend request, got %d", len(requests))
	}
	if requests[0].Content != "hello" {
		t.Fatalf("expected content hello, got %q", requests[0].Content)
	}

	var push protobuf.SC_50005
	decodePacketAt(t, target, 0, 50005, &push)
	if push.GetMsg().GetPlayer().GetId() != requester.Commander.CommanderID {
		t.Fatalf("expected push player %d, got %d", requester.Commander.CommanderID, push.GetMsg().GetPlayer().GetId())
	}

	requester.Buffer.Reset()
	target.Buffer.Reset()
	if _, _, err := SendFriendRequest(&payload, requester); err != nil {
		t.Fatalf("duplicate friend request failed: %v", err)
	}
	var duplicateResponse protobuf.SC_50004
	decodePacketAt(t, requester, 0, 50004, &duplicateResponse)
	if duplicateResponse.GetResult() != friendOperationFailure {
		t.Fatalf("expected duplicate result 1, got %d", duplicateResponse.GetResult())
	}
	if target.Buffer.Len() != 0 {
		t.Fatalf("expected no push on duplicate request")
	}
}

func TestAcceptFriendRequestSuccess(t *testing.T) {
	requester, target := setupFriendClients(t)
	created, err := orm.CreateFriendRequest(requester.Commander.CommanderID, target.Commander.CommanderID, "accept me")
	if err != nil {
		t.Fatalf("seed friend request: %v", err)
	}
	if !created {
		t.Fatalf("expected request to be created")
	}

	request := protobuf.CS_50006{Id: proto.Uint32(requester.Commander.CommanderID)}
	payload, err := proto.Marshal(&request)
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}

	if _, _, err := AcceptFriendRequest(&payload, target); err != nil {
		t.Fatalf("accept friend request failed: %v", err)
	}

	var response protobuf.SC_50007
	offset := decodePacketAt(t, target, 0, 50007, &response)
	if response.GetResult() != friendOperationSuccess {
		t.Fatalf("expected result 0, got %d", response.GetResult())
	}

	var acceptorPush protobuf.SC_50008
	decodePacketAt(t, target, offset, 50008, &acceptorPush)
	if acceptorPush.GetPlayer().GetId() != requester.Commander.CommanderID {
		t.Fatalf("expected accepter push id %d, got %d", requester.Commander.CommanderID, acceptorPush.GetPlayer().GetId())
	}

	var requesterPush protobuf.SC_50008
	decodePacketAt(t, requester, 0, 50008, &requesterPush)
	if requesterPush.GetPlayer().GetId() != target.Commander.CommanderID {
		t.Fatalf("expected requester push id %d, got %d", target.Commander.CommanderID, requesterPush.GetPlayer().GetId())
	}

	areFriends, err := orm.AreFriends(requester.Commander.CommanderID, target.Commander.CommanderID)
	if err != nil {
		t.Fatalf("check friendship: %v", err)
	}
	if !areFriends {
		t.Fatalf("expected accepted users to be friends")
	}

	requests, err := orm.ListFriendRequestsForTarget(target.Commander.CommanderID)
	if err != nil {
		t.Fatalf("list requests: %v", err)
	}
	if len(requests) != 0 {
		t.Fatalf("expected no pending requests after accept, got %d", len(requests))
	}
}

func TestAcceptFriendRequestMaxFriends(t *testing.T) {
	requester, target := setupFriendClients(t)
	created, err := orm.CreateFriendRequest(requester.Commander.CommanderID, target.Commander.CommanderID, "accept me")
	if err != nil {
		t.Fatalf("seed friend request: %v", err)
	}
	if !created {
		t.Fatalf("expected request to be created")
	}

	for i := 0; i < int(maxFriendCount); i++ {
		friendID := uint32(900000 + i)
		if err := orm.CreateCommanderRoot(friendID, friendID, fmt.Sprintf("Friend %d", friendID), 0, 0); err != nil {
			t.Fatalf("create friend commander %d: %v", friendID, err)
		}
		if err := orm.CreateFriendLinkPair(target.Commander.CommanderID, friendID); err != nil {
			t.Fatalf("seed friend link %d: %v", friendID, err)
		}
	}

	request := protobuf.CS_50006{Id: proto.Uint32(requester.Commander.CommanderID)}
	payload, err := proto.Marshal(&request)
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}

	if _, _, err := AcceptFriendRequest(&payload, target); err != nil {
		t.Fatalf("accept friend request failed: %v", err)
	}
	var response protobuf.SC_50007
	decodePacketAt(t, target, 0, 50007, &response)
	if response.GetResult() != friendOperationMaxed {
		t.Fatalf("expected result 6, got %d", response.GetResult())
	}

	requests, err := orm.ListFriendRequestsForTarget(target.Commander.CommanderID)
	if err != nil {
		t.Fatalf("list requests: %v", err)
	}
	if len(requests) != 1 {
		t.Fatalf("expected pending request to remain, got %d", len(requests))
	}
}

func TestRejectFriendRequestSingleAndAll(t *testing.T) {
	_, target := setupFriendClients(t)
	requesterOneID := uint32(time.Now().UnixNano())
	requesterTwoID := requesterOneID + 1
	if err := orm.CreateCommanderRoot(requesterOneID, requesterOneID, fmt.Sprintf("Requester %d", requesterOneID), 0, 0); err != nil {
		t.Fatalf("seed requester one: %v", err)
	}
	if err := orm.CreateCommanderRoot(requesterTwoID, requesterTwoID, fmt.Sprintf("Requester %d", requesterTwoID), 0, 0); err != nil {
		t.Fatalf("seed requester two: %v", err)
	}

	if _, err := orm.CreateFriendRequest(requesterOneID, target.Commander.CommanderID, "one"); err != nil {
		t.Fatalf("seed request one: %v", err)
	}
	if _, err := orm.CreateFriendRequest(requesterTwoID, target.Commander.CommanderID, "two"); err != nil {
		t.Fatalf("seed request two: %v", err)
	}

	single := protobuf.CS_50009{Id: proto.Uint32(requesterOneID)}
	singlePayload, _ := proto.Marshal(&single)
	if _, _, err := RejectFriendRequest(&singlePayload, target); err != nil {
		t.Fatalf("single reject failed: %v", err)
	}
	var singleResponse protobuf.SC_50010
	decodePacketAt(t, target, 0, 50010, &singleResponse)
	if singleResponse.GetResult() != friendOperationSuccess {
		t.Fatalf("expected single reject success")
	}

	remaining, err := orm.ListFriendRequestsForTarget(target.Commander.CommanderID)
	if err != nil {
		t.Fatalf("list requests: %v", err)
	}
	if len(remaining) != 1 || remaining[0].RequesterID != requesterTwoID {
		t.Fatalf("expected requester two to remain, got %+v", remaining)
	}

	target.Buffer.Reset()
	all := protobuf.CS_50009{Id: proto.Uint32(0)}
	allPayload, _ := proto.Marshal(&all)
	if _, _, err := RejectFriendRequest(&allPayload, target); err != nil {
		t.Fatalf("reject all failed: %v", err)
	}
	var allResponse protobuf.SC_50010
	decodePacketAt(t, target, 0, 50010, &allResponse)
	if allResponse.GetResult() != friendOperationSuccess {
		t.Fatalf("expected reject all success")
	}

	remaining, err = orm.ListFriendRequestsForTarget(target.Commander.CommanderID)
	if err != nil {
		t.Fatalf("list requests: %v", err)
	}
	if len(remaining) != 0 {
		t.Fatalf("expected no pending requests after reject all, got %d", len(remaining))
	}
}

func TestFriendHandlersDecodeFailures(t *testing.T) {
	client := setupHandlerCommander(t)
	bad := []byte{0xff, 0x00}

	if _, packetID, err := SendFriendRequest(&bad, client); err == nil || packetID != 50004 {
		t.Fatalf("expected send decode failure with packet 50004")
	}
	if _, packetID, err := AcceptFriendRequest(&bad, client); err == nil || packetID != 50007 {
		t.Fatalf("expected accept decode failure with packet 50007")
	}
	if _, packetID, err := RejectFriendRequest(&bad, client); err == nil || packetID != 50010 {
		t.Fatalf("expected reject decode failure with packet 50010")
	}
}

func TestCommanderFriendListIncludesFriendsAndRequests(t *testing.T) {
	requester, target := setupFriendClients(t)
	if _, err := orm.CreateFriendRequest(requester.Commander.CommanderID, target.Commander.CommanderID, "hello"); err != nil {
		t.Fatalf("seed friend request: %v", err)
	}

	friendID := uint32(time.Now().UnixNano())
	if err := orm.CreateCommanderRoot(friendID, friendID, fmt.Sprintf("Friend %d", friendID), 0, 0); err != nil {
		t.Fatalf("seed friend commander: %v", err)
	}
	if err := orm.CreateFriendLinkPair(target.Commander.CommanderID, friendID); err != nil {
		t.Fatalf("seed friend link: %v", err)
	}

	buffer := []byte{}
	if _, _, err := CommanderFriendList(&buffer, target); err != nil {
		t.Fatalf("commander friend list failed: %v", err)
	}
	var response protobuf.SC_50000
	decodePacketAt(t, target, 0, 50000, &response)

	if len(response.GetFriendList()) != 1 {
		t.Fatalf("expected one friend in list, got %d", len(response.GetFriendList()))
	}
	if response.GetFriendList()[0].GetId() != friendID {
		t.Fatalf("expected friend id %d, got %d", friendID, response.GetFriendList()[0].GetId())
	}
	if len(response.GetRequestList()) != 1 {
		t.Fatalf("expected one request in list, got %d", len(response.GetRequestList()))
	}
	if response.GetRequestList()[0].GetPlayer().GetId() != requester.Commander.CommanderID {
		t.Fatalf("expected requester id %d, got %d", requester.Commander.CommanderID, response.GetRequestList()[0].GetPlayer().GetId())
	}
}
