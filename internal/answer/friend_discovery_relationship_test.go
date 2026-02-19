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

func decodeSinglePacket[T proto.Message](t *testing.T, client *connection.Client, packetID int, msg T) T {
	t.Helper()
	buffer := client.Buffer.Bytes()
	if len(buffer) == 0 {
		t.Fatalf("expected response packet %d", packetID)
	}
	actualPacketID := packets.GetPacketId(0, &buffer)
	if actualPacketID != packetID {
		t.Fatalf("expected packet %d, got %d", packetID, actualPacketID)
	}
	packetSize := packets.GetPacketSize(0, &buffer) + 2
	payloadStart := packets.HEADER_SIZE
	payloadEnd := payloadStart + (packetSize - packets.HEADER_SIZE)
	if err := proto.Unmarshal(buffer[payloadStart:payloadEnd], msg); err != nil {
		t.Fatalf("unmarshal packet %d: %v", packetID, err)
	}
	client.Buffer.Next(packetSize)
	return msg
}

func seedSocialCommander(t *testing.T, commanderID uint32, name string) orm.CommanderSocialProfile {
	t.Helper()
	if err := orm.CreateCommanderRoot(commanderID, commanderID, name, 0, 0); err != nil {
		t.Fatalf("create commander: %v", err)
	}
	lastLogin := time.Now().UTC().Add(-5 * time.Minute)
	execAnswerTestSQLT(t, `
UPDATE commanders
SET level = $2,
    manifesto = $3,
    last_login = $4,
    display_icon_id = $5,
    display_skin_id = $6,
    selected_icon_frame_id = $7,
    selected_chat_frame_id = $8,
    display_icon_theme_id = $9,
    collect_attack_count = $10
WHERE commander_id = $1
`, int64(commanderID), int64(35), "hello commander", lastLogin, int64(101), int64(202), int64(303), int64(404), int64(505), int64(13))

	profile, err := orm.GetCommanderSocialProfileByID(commanderID)
	if err != nil {
		t.Fatalf("get social profile: %v", err)
	}
	return *profile
}

func TestSearchFriendByIDSuccess(t *testing.T) {
	client := setupHandlerCommander(t)
	targetProfile := seedSocialCommander(t, 910001, "Searchable")

	request := &protobuf.CS_50001{Type: proto.Uint32(0), Keyword: proto.String("910001")}
	buffer, err := proto.Marshal(request)
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}

	if _, _, err := SearchFriend(&buffer, client); err != nil {
		t.Fatalf("SearchFriend failed: %v", err)
	}
	response := decodeSinglePacket(t, client, 50002, &protobuf.SC_50002{})
	if response.GetResult() != 0 {
		t.Fatalf("expected result 0, got %d", response.GetResult())
	}
	if response.GetPlayer() == nil {
		t.Fatalf("expected player payload")
	}
	if response.GetPlayer().GetId() != targetProfile.CommanderID {
		t.Fatalf("expected player id %d, got %d", targetProfile.CommanderID, response.GetPlayer().GetId())
	}
	if response.GetPlayer().GetName() != targetProfile.Name {
		t.Fatalf("expected player name %q, got %q", targetProfile.Name, response.GetPlayer().GetName())
	}
	if response.GetPlayer().GetDisplay() == nil {
		t.Fatalf("expected display info")
	}
	if response.GetPlayer().GetDisplay().GetIcon() != targetProfile.DisplayIconID {
		t.Fatalf("expected icon %d, got %d", targetProfile.DisplayIconID, response.GetPlayer().GetDisplay().GetIcon())
	}
}

func TestSearchFriendInvalidNumericKeyword(t *testing.T) {
	client := setupHandlerCommander(t)
	request := &protobuf.CS_50001{Type: proto.Uint32(0), Keyword: proto.String("abc")}
	buffer, err := proto.Marshal(request)
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}

	if _, _, err := SearchFriend(&buffer, client); err != nil {
		t.Fatalf("SearchFriend failed: %v", err)
	}
	response := decodeSinglePacket(t, client, 50002, &protobuf.SC_50002{})
	if response.GetResult() == 0 {
		t.Fatalf("expected non-zero result")
	}
	if response.GetPlayer() != nil {
		t.Fatalf("expected no player on invalid numeric keyword")
	}
}

func TestFriendSearchListAndBatchLookup(t *testing.T) {
	client := setupHandlerCommander(t)
	firstProfile := seedSocialCommander(t, 910101, "List One")
	secondProfile := seedSocialCommander(t, 910102, "List Two")
	if err := orm.CreateCommanderFriendRelationPair(client.Commander.CommanderID, firstProfile.CommanderID); err != nil {
		t.Fatalf("seed existing friend relation: %v", err)
	}

	listRequest := &protobuf.CS_50014{Type: proto.Uint32(0)}
	listBuffer, err := proto.Marshal(listRequest)
	if err != nil {
		t.Fatalf("marshal list request: %v", err)
	}

	if _, _, err := FriendSearchList(&listBuffer, client); err != nil {
		t.Fatalf("FriendSearchList failed: %v", err)
	}
	listResponse := decodeSinglePacket(t, client, 50015, &protobuf.SC_50015{})
	if len(listResponse.GetPlayerList()) == 0 {
		t.Fatalf("expected at least one recommendation entry")
	}
	for _, player := range listResponse.GetPlayerList() {
		if player.GetId() == firstProfile.CommanderID {
			t.Fatalf("did not expect existing friend %d in recommendation list", firstProfile.CommanderID)
		}
	}

	batchRequest := &protobuf.CS_50018{UserIdList: []uint32{firstProfile.CommanderID, 999999, secondProfile.CommanderID}}
	batchBuffer, err := proto.Marshal(batchRequest)
	if err != nil {
		t.Fatalf("marshal batch request: %v", err)
	}

	if _, _, err := CommanderFriendBatchGet(&batchBuffer, client); err != nil {
		t.Fatalf("CommanderFriendBatchGet failed: %v", err)
	}
	batchResponse := decodeSinglePacket(t, client, 50019, &protobuf.SC_50019{})
	if len(batchResponse.GetUserList()) != 2 {
		t.Fatalf("expected 2 existing users, got %d", len(batchResponse.GetUserList()))
	}
	if batchResponse.GetUserList()[0].GetId() != firstProfile.CommanderID {
		t.Fatalf("expected first user id %d, got %d", firstProfile.CommanderID, batchResponse.GetUserList()[0].GetId())
	}
	if batchResponse.GetUserList()[1].GetId() != secondProfile.CommanderID {
		t.Fatalf("expected second user id %d, got %d", secondProfile.CommanderID, batchResponse.GetUserList()[1].GetId())
	}
}

func TestDeleteFriendSuccessAndFriendListHydration(t *testing.T) {
	client := setupHandlerCommander(t)
	targetProfile := seedSocialCommander(t, 910201, "Delete Friend")
	targetCommander := orm.Commander{CommanderID: targetProfile.CommanderID}
	if err := targetCommander.Load(); err != nil {
		t.Fatalf("load target commander: %v", err)
	}
	targetClient := &connection.Client{Commander: &targetCommander, Hash: targetProfile.CommanderID}
	client.Server.AddClient(targetClient)

	if err := orm.CreateCommanderFriendRelationPair(client.Commander.CommanderID, targetProfile.CommanderID); err != nil {
		t.Fatalf("seed friend relation: %v", err)
	}

	request := &protobuf.CS_50011{Id: proto.Uint32(targetProfile.CommanderID)}
	buffer, err := proto.Marshal(request)
	if err != nil {
		t.Fatalf("marshal delete request: %v", err)
	}

	if _, _, err := DeleteFriend(&buffer, client); err != nil {
		t.Fatalf("DeleteFriend failed: %v", err)
	}

	ack := decodeSinglePacket(t, client, 50012, &protobuf.SC_50012{})
	if ack.GetResult() != 0 {
		t.Fatalf("expected ack result 0, got %d", ack.GetResult())
	}
	notify := decodeSinglePacket(t, client, 50013, &protobuf.SC_50013{})
	if notify.GetId() != targetProfile.CommanderID {
		t.Fatalf("expected notify id %d, got %d", targetProfile.CommanderID, notify.GetId())
	}
	peerNotify := decodeSinglePacket(t, targetClient, 50013, &protobuf.SC_50013{})
	if peerNotify.GetId() != client.Commander.CommanderID {
		t.Fatalf("expected peer notify id %d, got %d", client.Commander.CommanderID, peerNotify.GetId())
	}

	requestListBuffer := []byte{}
	if _, _, err := CommanderFriendList(&requestListBuffer, client); err != nil {
		t.Fatalf("CommanderFriendList failed: %v", err)
	}
	friendList := decodeSinglePacket(t, client, 50000, &protobuf.SC_50000{})
	if len(friendList.GetFriendList()) != 0 {
		t.Fatalf("expected no friends after deletion, got %d", len(friendList.GetFriendList()))
	}
}

func TestDeleteFriendMissingRelation(t *testing.T) {
	client := setupHandlerCommander(t)
	request := &protobuf.CS_50011{Id: proto.Uint32(910301)}
	buffer, err := proto.Marshal(request)
	if err != nil {
		t.Fatalf("marshal delete request: %v", err)
	}

	if _, _, err := DeleteFriend(&buffer, client); err != nil {
		t.Fatalf("DeleteFriend failed: %v", err)
	}
	ack := decodeSinglePacket(t, client, 50012, &protobuf.SC_50012{})
	if ack.GetResult() == 0 {
		t.Fatalf("expected non-zero result when relation is missing")
	}
}
