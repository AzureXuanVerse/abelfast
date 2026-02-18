package answer

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/ggmolly/belfast/internal/db"
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

func TestIslandRuntimeReassignClearsOldIslandVisitors(t *testing.T) {
	state := newIslandRuntimeState()

	result, _, _ := state.enter(1001, "leader", 9101, 0)
	if result != 0 {
		t.Fatalf("expected first island enter success, got %d", result)
	}

	result, _, _ = state.enter(1001, "leader", 9102, 0)
	if result != 0 {
		t.Fatalf("expected reassigned island enter success, got %d", result)
	}

	result, position, _ := state.enter(1002, "follower", 9101, 0)
	if result != 0 {
		t.Fatalf("expected old island to be available after reassignment, got result=%d position=%d", result, position)
	}

	if _, ok := state.visitors[9101][1001]; ok {
		t.Fatalf("expected stale visitor membership removed from previous island")
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

func TestIslandWildGatherCollect(t *testing.T) {
	globalIslandRuntimeState.resetForTest()
	client := setupHandlerCommander(t)
	islandID := client.Commander.CommanderID + 200
	globalIslandRuntimeState.setSessionForTest(client.Commander.CommanderID, islandID)

	data, err := json.Marshal(map[string]any{"show": 1, "drop_display": [][]uint32{{41, 9101, 3}}})
	if err != nil {
		t.Fatalf("marshal config: %v", err)
	}
	if err := orm.UpsertConfigEntry("ShareCfg/island_wild_gather.json", "88", data); err != nil {
		t.Fatalf("seed gather config: %v", err)
	}

	payload, _ := proto.Marshal(&protobuf.CS_21524{IslandId: proto.Uint32(islandID), GatherId: proto.Uint32(88)})
	if _, _, err := HandleIslandWildGatherCollect(&payload, client); err != nil {
		t.Fatalf("wild gather collect failed: %v", err)
	}

	var ack protobuf.SC_21525
	offset := decodePacketAt(t, client, 0, 21525, &ack)
	if ack.GetResult() != 0 || len(ack.GetDropList()) != 1 {
		t.Fatalf("expected gather collect success drop, got %+v", ack)
	}
	var push protobuf.SC_21528
	decodePacketAt(t, client, offset, 21528, &push)
	if len(push.GetGatherList()) != 1 || push.GetGatherList()[0].GetId() != 88 {
		t.Fatalf("expected gather removal push")
	}

	item, err := orm.GetIslandInventoryItem(client.Commander.CommanderID, 9101)
	if err != nil || item.Count != 3 {
		t.Fatalf("expected inventory drop applied, err=%v item=%+v", err, item)
	}

	client.Buffer.Reset()
	if _, _, err := HandleIslandWildGatherCollect(&payload, client); err != nil {
		t.Fatalf("duplicate gather collect failed: %v", err)
	}
	var duplicateAck protobuf.SC_21525
	decodePacketAt(t, client, 0, 21525, &duplicateAck)
	if duplicateAck.GetResult() == 0 {
		t.Fatalf("expected duplicate gather collect to fail")
	}
}

func TestIslandWildGatherCollectFailures(t *testing.T) {
	globalIslandRuntimeState.resetForTest()
	client := setupHandlerCommander(t)

	invalidPayload, _ := proto.Marshal(&protobuf.CS_21524{IslandId: proto.Uint32(0), GatherId: proto.Uint32(0)})
	if _, _, err := HandleIslandWildGatherCollect(&invalidPayload, client); err != nil {
		t.Fatalf("invalid gather collect failed: %v", err)
	}
	var invalidAck protobuf.SC_21525
	decodePacketAt(t, client, 0, 21525, &invalidAck)
	if invalidAck.GetResult() == 0 {
		t.Fatalf("expected invalid payload failure")
	}

	client.Buffer.Reset()
	islandID := client.Commander.CommanderID + 300
	payload, _ := proto.Marshal(&protobuf.CS_21524{IslandId: proto.Uint32(islandID), GatherId: proto.Uint32(99)})
	if _, _, err := HandleIslandWildGatherCollect(&payload, client); err != nil {
		t.Fatalf("session mismatch gather collect failed: %v", err)
	}
	var mismatchAck protobuf.SC_21525
	decodePacketAt(t, client, 0, 21525, &mismatchAck)
	if mismatchAck.GetResult() == 0 {
		t.Fatalf("expected session mismatch failure")
	}

	bad := []byte{0xff, 0x00}
	if _, responseID, err := HandleIslandWildGatherCollect(&bad, client); err == nil || responseID != 21525 {
		t.Fatalf("expected decode error with response id 21525")
	}
}

func TestIslandWildCollectFragmentAndSignFlow(t *testing.T) {
	globalIslandRuntimeState.resetForTest()
	client := setupHandlerCommander(t)
	islandID := client.Commander.CommanderID + 400
	globalIslandRuntimeState.setSessionForTest(client.Commander.CommanderID, islandID)

	if err := orm.UpsertConfigEntry("ShareCfg/island_collect_fragment.json", "501", []byte(`{"id":501,"collection_id":601,"show":3}`)); err != nil {
		t.Fatalf("seed fragment config: %v", err)
	}
	if err := orm.UpsertConfigEntry("ShareCfg/island_collection.json", "601", []byte(`{"id":601,"fragment_list":[501]}`)); err != nil {
		t.Fatalf("seed collection config: %v", err)
	}

	pickPayload, _ := proto.Marshal(&protobuf.CS_21529{IslandId: proto.Uint32(islandID), FragmentId: proto.Uint32(501)})
	if _, _, err := HandleIslandWildCollectFragment(&pickPayload, client); err != nil {
		t.Fatalf("fragment pickup failed: %v", err)
	}
	var pickupAck protobuf.SC_21530
	offset := decodePacketAt(t, client, 0, 21530, &pickupAck)
	if pickupAck.GetResult() != 0 {
		t.Fatalf("expected fragment pickup success, got %d", pickupAck.GetResult())
	}
	var pickupPush protobuf.SC_21535
	decodePacketAt(t, client, offset, 21535, &pickupPush)
	if len(pickupPush.GetFragmentData()) != 1 || pickupPush.GetFragmentData()[0].GetId() != 501 {
		t.Fatalf("expected fragment pickup push")
	}

	client.Buffer.Reset()
	if _, _, err := HandleIslandWildCollectFragment(&pickPayload, client); err != nil {
		t.Fatalf("duplicate fragment pickup failed: %v", err)
	}
	var duplicatePickupAck protobuf.SC_21530
	decodePacketAt(t, client, 0, 21530, &duplicatePickupAck)
	if duplicatePickupAck.GetResult() == 0 {
		t.Fatalf("expected duplicate fragment pickup failure")
	}

	client.Buffer.Reset()
	signPayload, _ := proto.Marshal(&protobuf.CS_21531{IslandId: proto.Uint32(client.Commander.CommanderID), FragmentId: proto.Uint32(501)})
	if _, _, err := HandleIslandWildCollectFragmentSign(&signPayload, client); err != nil {
		t.Fatalf("fragment sign failed: %v", err)
	}
	var signAck protobuf.SC_21532
	decodePacketAt(t, client, 0, 21532, &signAck)
	if signAck.GetResult() == 0 {
		t.Fatalf("expected self-island sign rejection")
	}

	foreignIsland := islandID + 1
	client.Buffer.Reset()
	foreignSignPayload, _ := proto.Marshal(&protobuf.CS_21531{IslandId: proto.Uint32(foreignIsland), FragmentId: proto.Uint32(501)})
	if _, _, err := HandleIslandWildCollectFragmentSign(&foreignSignPayload, client); err != nil {
		t.Fatalf("foreign fragment sign failed: %v", err)
	}
	var foreignSignAck protobuf.SC_21532
	offset = decodePacketAt(t, client, 0, 21532, &foreignSignAck)
	if foreignSignAck.GetResult() != 0 {
		t.Fatalf("expected foreign sign success")
	}
	var signPush protobuf.SC_21535
	decodePacketAt(t, client, offset, 21535, &signPush)
	if len(signPush.GetFragmentData()) != 1 || signPush.GetFragmentData()[0].GetMark() != client.Commander.CommanderID {
		t.Fatalf("expected sign push mark")
	}
}

func TestIslandCollectionComplete(t *testing.T) {
	globalIslandRuntimeState.resetForTest()
	client := setupHandlerCommander(t)

	if err := orm.UpsertConfigEntry("ShareCfg/island_collect_fragment.json", "7001", []byte(`{"id":7001,"collection_id":8001,"show":1}`)); err != nil {
		t.Fatalf("seed fragment config: %v", err)
	}
	if err := orm.UpsertConfigEntry("ShareCfg/island_collect_fragment.json", "7002", []byte(`{"id":7002,"collection_id":8001,"show":1}`)); err != nil {
		t.Fatalf("seed fragment config: %v", err)
	}
	if err := orm.UpsertConfigEntry("ShareCfg/island_collection.json", "8001", []byte(`{"id":8001,"fragment_list":[7001,7002]}`)); err != nil {
		t.Fatalf("seed collection config: %v", err)
	}

	err := db.DefaultStore.WithPGXTx(context.Background(), func(tx pgx.Tx) error {
		if _, err := orm.CreateIslandCollectFragmentStateTx(context.Background(), tx, client.Commander.CommanderID, 7001, client.Commander.CommanderID, client.Commander.CommanderID); err != nil {
			return err
		}
		if _, err := orm.CreateIslandCollectFragmentStateTx(context.Background(), tx, client.Commander.CommanderID, 7002, client.Commander.CommanderID, client.Commander.CommanderID); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		t.Fatalf("seed fragment states: %v", err)
	}

	payload, _ := proto.Marshal(&protobuf.CS_21533{CollectId: proto.Uint32(8001)})
	if _, _, err := HandleIslandCollectionComplete(&payload, client); err != nil {
		t.Fatalf("collection complete failed: %v", err)
	}
	var ack protobuf.SC_21534
	decodePacketAt(t, client, 0, 21534, &ack)
	if ack.GetResult() != 0 {
		t.Fatalf("expected collection complete success")
	}

	client.Buffer.Reset()
	if _, _, err := HandleIslandCollectionComplete(&payload, client); err != nil {
		t.Fatalf("duplicate collection complete failed: %v", err)
	}
	var duplicateAck protobuf.SC_21534
	decodePacketAt(t, client, 0, 21534, &duplicateAck)
	if duplicateAck.GetResult() == 0 {
		t.Fatalf("expected duplicate collection complete failure")
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
