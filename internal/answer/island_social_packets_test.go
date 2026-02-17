package answer

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/ggmolly/belfast/internal/orm"
	"github.com/ggmolly/belfast/internal/protobuf"
	"google.golang.org/protobuf/proto"
)

func TestHandleIslandReconnectProbe(t *testing.T) {
	globalIslandRuntimeState.resetForTest()
	client := setupHandlerCommander(t)

	request := protobuf.CS_21230{IslandId: proto.Uint32(3001)}
	buffer, err := proto.Marshal(&request)
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}

	if _, _, err := HandleIslandReconnect(&buffer, client); err != nil {
		t.Fatalf("first reconnect failed: %v", err)
	}
	var first protobuf.SC_21231
	decodePacketAt(t, client, 0, 21231, &first)
	if first.GetResult() == 0 {
		t.Fatalf("expected reconnect failure without session")
	}

	globalIslandRuntimeState.setSessionForTest(client.Commander.CommanderID, 3001)
	client.Buffer.Reset()
	if _, _, err := HandleIslandReconnect(&buffer, client); err != nil {
		t.Fatalf("second reconnect failed: %v", err)
	}
	var second protobuf.SC_21231
	decodePacketAt(t, client, 0, 21231, &second)
	if second.GetResult() != 0 {
		t.Fatalf("expected reconnect success, got %d", second.GetResult())
	}
}

func TestIslandEnterAndQueuePoll(t *testing.T) {
	globalIslandRuntimeState.resetForTest()
	leader := setupHandlerCommander(t)
	follower := setupHandlerCommander(t)

	enter := protobuf.CS_21202{IslandId: proto.Uint32(9001)}
	leaderPayload, _ := proto.Marshal(&enter)
	if _, _, err := IslandEnter(&leaderPayload, leader); err != nil {
		t.Fatalf("leader enter failed: %v", err)
	}
	var leaderResponse protobuf.SC_21203
	decodePacketAt(t, leader, 0, 21203, &leaderResponse)
	if leaderResponse.GetResult() != 0 {
		t.Fatalf("expected leader enter success, got %d", leaderResponse.GetResult())
	}

	followerPayload, _ := proto.Marshal(&enter)
	if _, _, err := IslandEnter(&followerPayload, follower); err != nil {
		t.Fatalf("follower enter failed: %v", err)
	}
	var queuedResponse protobuf.SC_21203
	decodePacketAt(t, follower, 0, 21203, &queuedResponse)
	if queuedResponse.GetResult() != 6 {
		t.Fatalf("expected queue result 6, got %d", queuedResponse.GetResult())
	}

	pollPayload, _ := proto.Marshal(&protobuf.CS_21208{IslandId: proto.Uint32(9001)})
	follower.Buffer.Reset()
	if _, _, err := HandleIslandQueuePoll(&pollPayload, follower); err != nil {
		t.Fatalf("queue poll failed: %v", err)
	}
	var pollQueued protobuf.SC_21203
	decodePacketAt(t, follower, 0, 21203, &pollQueued)
	if pollQueued.GetResult() != 6 {
		t.Fatalf("expected queued poll result 6, got %d", pollQueued.GetResult())
	}

	globalIslandRuntimeState.clearSessionForTest(leader.Commander.CommanderID)
	follower.Buffer.Reset()
	if _, _, err := HandleIslandQueuePoll(&pollPayload, follower); err != nil {
		t.Fatalf("queue poll after release failed: %v", err)
	}
	var pollSuccess protobuf.SC_21203
	decodePacketAt(t, follower, 0, 21203, &pollSuccess)
	if pollSuccess.GetResult() != 0 {
		t.Fatalf("expected poll success result 0, got %d", pollSuccess.GetResult())
	}
}

func TestIslandHeartbeatVisitorFeed(t *testing.T) {
	globalIslandRuntimeState.resetForTest()
	client := setupHandlerCommander(t)
	globalIslandRuntimeState.setSessionForTest(client.Commander.CommanderID, 7001)

	payload, _ := proto.Marshal(&protobuf.CS_21215{IslandId: proto.Uint32(7001)})
	if _, _, err := IslandHeartbeat(&payload, client); err != nil {
		t.Fatalf("heartbeat failed: %v", err)
	}
	var response protobuf.SC_21216
	decodePacketAt(t, client, 0, 21216, &response)
	if len(response.GetVisitorList()) != 0 {
		t.Fatalf("expected empty visitor list")
	}
}

func TestIslandInvitationGiftTagAndSync(t *testing.T) {
	globalIslandRuntimeState.resetForTest()
	inviter := setupHandlerCommander(t)
	target := setupHandlerCommander(t)

	inviter.Server.AddClient(inviter)
	target.Hash = inviter.Hash + 1
	target.Server = inviter.Server
	target.Server.AddClient(target)

	invitePayload, _ := proto.Marshal(&protobuf.CS_21312{FriendList: []uint32{target.Commander.CommanderID, target.Commander.CommanderID, 0}})
	if _, _, err := IslandSignInInvitation(&invitePayload, inviter); err != nil {
		t.Fatalf("invitation handler failed: %v", err)
	}

	var inviteAck protobuf.SC_21313
	decodePacketAt(t, inviter, 0, 21313, &inviteAck)
	if inviteAck.GetResult() != 0 {
		t.Fatalf("expected invitation success, got %d", inviteAck.GetResult())
	}

	var push protobuf.SC_21314
	decodePacketAt(t, target, 0, 21314, &push)
	if push.GetGiftCount() == 0 {
		t.Fatalf("expected non-zero gift count in push")
	}

	queryPayload, _ := proto.Marshal(&protobuf.CS_21315{UserIdList: []uint32{target.Commander.CommanderID, target.Commander.CommanderID}})
	inviter.Buffer.Reset()
	if _, _, err := HandleIslandGetGiftTag(&queryPayload, inviter); err != nil {
		t.Fatalf("gift tag query failed: %v", err)
	}
	var giftResponse protobuf.SC_21316
	decodePacketAt(t, inviter, 0, 21316, &giftResponse)
	if len(giftResponse.GetGiftList()) != 1 {
		t.Fatalf("expected deduped gift list length 1, got %d", len(giftResponse.GetGiftList()))
	}
	if giftResponse.GetGiftList()[0].GetKey() != target.Commander.CommanderID {
		t.Fatalf("unexpected gift key %d", giftResponse.GetGiftList()[0].GetKey())
	}
}

func TestIslandChatSendPush(t *testing.T) {
	globalIslandRuntimeState.resetForTest()
	sender := setupHandlerCommander(t)
	receiver := setupHandlerCommander(t)

	sender.Server.AddClient(sender)
	receiver.Hash = sender.Hash + 1
	receiver.Server = sender.Server
	receiver.Server.AddClient(receiver)

	globalIslandRuntimeState.setSessionForTest(sender.Commander.CommanderID, 8001)
	globalIslandRuntimeState.setSessionForTest(receiver.Commander.CommanderID, 8001)

	payload, _ := proto.Marshal(&protobuf.CS_21323{IslandId: proto.Uint32(8001), Content: proto.String("hello")})
	if _, _, err := IslandSendChat(&payload, sender); err != nil {
		t.Fatalf("chat send failed: %v", err)
	}

	var ack protobuf.SC_21324
	offset := decodePacketAt(t, sender, 0, 21324, &ack)
	if ack.GetResult() != 0 {
		t.Fatalf("expected chat ack success, got %d", ack.GetResult())
	}
	var senderPush protobuf.SC_21325
	decodePacketAt(t, sender, offset, 21325, &senderPush)
	if senderPush.GetContent() != "hello" {
		t.Fatalf("unexpected sender push content %q", senderPush.GetContent())
	}

	var receiverPush protobuf.SC_21325
	decodePacketAt(t, receiver, 0, 21325, &receiverPush)
	if receiverPush.GetUserId() != sender.Commander.CommanderID {
		t.Fatalf("unexpected receiver push user id %d", receiverPush.GetUserId())
	}
}

func TestIslandRefreshInviteCodeDailyLimit(t *testing.T) {
	globalIslandRuntimeState.resetForTest()
	client := setupHandlerCommander(t)

	payload, _ := proto.Marshal(&protobuf.CS_21008{Type: proto.Uint32(0)})
	if _, _, err := IslandRefreshInviteCode(&payload, client); err != nil {
		t.Fatalf("first refresh failed: %v", err)
	}
	var first protobuf.SC_21009
	decodePacketAt(t, client, 0, 21009, &first)
	if first.GetResult() != 0 || first.GetInviteCode() == "" {
		t.Fatalf("expected first refresh success with code, got result=%d code=%q", first.GetResult(), first.GetInviteCode())
	}

	client.Buffer.Reset()
	if _, _, err := IslandRefreshInviteCode(&payload, client); err != nil {
		t.Fatalf("second refresh failed: %v", err)
	}
	var second protobuf.SC_21009
	decodePacketAt(t, client, 0, 21009, &second)
	if second.GetResult() == 0 {
		t.Fatalf("expected second refresh to fail same day")
	}
	if second.GetInviteCode() != first.GetInviteCode() {
		t.Fatalf("expected stable invite code on failure")
	}
}

func TestIslandWildGatherSign(t *testing.T) {
	globalIslandRuntimeState.resetForTest()
	client := setupHandlerCommander(t)

	data, err := json.Marshal(map[string]uint32{"show": 3})
	if err != nil {
		t.Fatalf("marshal config: %v", err)
	}
	if err := orm.UpsertConfigEntry("ShareCfg/island_wild_gather.json", "77", data); err != nil {
		t.Fatalf("seed gather config: %v", err)
	}

	payload, _ := proto.Marshal(&protobuf.CS_21526{IslandId: proto.Uint32(client.Commander.CommanderID + 100), GatherId: proto.Uint32(77)})
	if _, _, err := HandleIslandWildGatherSign(&payload, client); err != nil {
		t.Fatalf("wild gather sign failed: %v", err)
	}

	var ack protobuf.SC_21527
	offset := decodePacketAt(t, client, 0, 21527, &ack)
	if ack.GetResult() != 0 {
		t.Fatalf("expected gather sign success, got %d", ack.GetResult())
	}
	var push protobuf.SC_21528
	decodePacketAt(t, client, offset, 21528, &push)
	if len(push.GetGatherList()) != 1 {
		t.Fatalf("expected one gather push entry")
	}

	stored, err := orm.ListIslandWildGatherSignStates(client.Commander.CommanderID+100, 77)
	if err != nil {
		t.Fatalf("list gather sign states: %v", err)
	}
	if len(stored) != 1 {
		t.Fatalf("expected one stored gather sign, got %d", len(stored))
	}
}

func TestIslandEnterByInviteCode(t *testing.T) {
	globalIslandRuntimeState.resetForTest()
	owner := setupHandlerCommander(t)
	visitor := setupHandlerCommander(t)

	state, err := orm.GetOrCreateCommanderIslandSocialState(owner.Commander.CommanderID)
	if err != nil {
		t.Fatalf("get owner social state: %v", err)
	}
	state.InviteCode = "TESTCODE"
	state.InviteCodeRefreshDay = uint32(time.Now().UTC().Unix() / 86400)
	if err := orm.SaveCommanderIslandSocialState(state); err != nil {
		t.Fatalf("save owner social state: %v", err)
	}

	request := protobuf.CS_21202{IslandId: proto.Uint32(0), Code: proto.String("TESTCODE")}
	payload, _ := proto.Marshal(&request)
	if _, _, err := IslandEnter(&payload, visitor); err != nil {
		t.Fatalf("island enter by code failed: %v", err)
	}

	var response protobuf.SC_21203
	decodePacketAt(t, visitor, 0, 21203, &response)
	if response.GetIslandId() != owner.Commander.CommanderID {
		t.Fatalf("expected resolved island id %d, got %d", owner.Commander.CommanderID, response.GetIslandId())
	}
}
