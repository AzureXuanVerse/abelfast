package answer

import (
	"testing"
	"time"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/orm"
	"github.com/ggmolly/belfast/internal/protobuf"
	"google.golang.org/protobuf/proto"
)

var socialMiscCommanderID uint32 = 980000

func setupSocialMiscCommander(t *testing.T, name string) *orm.Commander {
	t.Helper()
	orm.InitDatabase()
	socialMiscCommanderID++
	commanderID := socialMiscCommanderID
	if err := orm.CreateCommanderRoot(commanderID, commanderID, name, 0, 0); err != nil {
		t.Fatalf("create commander: %v", err)
	}
	commander := &orm.Commander{CommanderID: commanderID}
	if err := commander.Load(); err != nil {
		t.Fatalf("load commander: %v", err)
	}
	return commander
}

func setupSocialMiscState(t *testing.T) {
	t.Helper()
	orm.InitDatabase()
	clearTable(t, &orm.PlayerInform{})
	clearTable(t, &orm.FriendDirectMessage{})
	clearTable(t, &orm.FriendRelationship{})
	clearTable(t, &orm.Commander{})
}

func TestSendFriendMessageSuccess(t *testing.T) {
	setupSocialMiscState(t)
	sender := setupSocialMiscCommander(t, "Sender")
	recipient := setupSocialMiscCommander(t, "Recipient")

	if err := orm.CreateFriendRelationship(sender.CommanderID, recipient.CommanderID, uint32(time.Now().Unix())); err != nil {
		t.Fatalf("create relationship: %v", err)
	}

	server := connection.NewServer("127.0.0.1", 0, nil)
	senderClient := &connection.Client{Hash: 1, Commander: sender}
	recipientClient := &connection.Client{Hash: 2, Commander: recipient}
	server.AddClient(senderClient)
	server.AddClient(recipientClient)

	request := protobuf.CS_50105{Id: proto.Uint32(recipient.CommanderID), Content: proto.String("hello friend")}
	buffer, err := proto.Marshal(&request)
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}

	if _, _, err := SendFriendMessage(&buffer, senderClient); err != nil {
		t.Fatalf("SendFriendMessage failed: %v", err)
	}

	var ack protobuf.SC_50106
	decodePacketAt(t, senderClient, 0, 50106, &ack)
	if ack.GetResult() != 0 {
		t.Fatalf("expected result 0, got %d", ack.GetResult())
	}

	var push protobuf.SC_50104
	decodePacketAt(t, recipientClient, 0, 50104, &push)
	if push.GetMsg().GetPlayer().GetId() != sender.CommanderID {
		t.Fatalf("expected sender id %d, got %d", sender.CommanderID, push.GetMsg().GetPlayer().GetId())
	}
	if push.GetMsg().GetContent() != "hello friend" {
		t.Fatalf("unexpected push content: %q", push.GetMsg().GetContent())
	}

	count := queryAnswerTestInt64(t, "SELECT COUNT(*) FROM friend_direct_messages WHERE sender_id = $1 AND receiver_id = $2", int64(sender.CommanderID), int64(recipient.CommanderID))
	if count != 1 {
		t.Fatalf("expected one friend direct message row, got %d", count)
	}
}

func TestSendFriendMessageOffline(t *testing.T) {
	setupSocialMiscState(t)
	sender := setupSocialMiscCommander(t, "Sender Offline")
	recipient := setupSocialMiscCommander(t, "Recipient Offline")

	if err := orm.CreateFriendRelationship(sender.CommanderID, recipient.CommanderID, uint32(time.Now().Unix())); err != nil {
		t.Fatalf("create relationship: %v", err)
	}

	server := connection.NewServer("127.0.0.1", 0, nil)
	senderClient := &connection.Client{Hash: 3, Commander: sender}
	server.AddClient(senderClient)

	request := protobuf.CS_50105{Id: proto.Uint32(recipient.CommanderID), Content: proto.String("ping")}
	buffer, err := proto.Marshal(&request)
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}

	if _, _, err := SendFriendMessage(&buffer, senderClient); err != nil {
		t.Fatalf("SendFriendMessage failed: %v", err)
	}

	var ack protobuf.SC_50106
	decodePacketAt(t, senderClient, 0, 50106, &ack)
	if ack.GetResult() != 28 {
		t.Fatalf("expected offline result 28, got %d", ack.GetResult())
	}

	count := queryAnswerTestInt64(t, "SELECT COUNT(*) FROM friend_direct_messages")
	if count != 0 {
		t.Fatalf("expected no friend direct messages, got %d", count)
	}
}

func TestSendFriendMessageNotFriend(t *testing.T) {
	setupSocialMiscState(t)
	sender := setupSocialMiscCommander(t, "Sender NotFriend")
	recipient := setupSocialMiscCommander(t, "Recipient NotFriend")

	server := connection.NewServer("127.0.0.1", 0, nil)
	senderClient := &connection.Client{Hash: 4, Commander: sender}
	recipientClient := &connection.Client{Hash: 5, Commander: recipient}
	server.AddClient(senderClient)
	server.AddClient(recipientClient)

	request := protobuf.CS_50105{Id: proto.Uint32(recipient.CommanderID), Content: proto.String("blocked")}
	buffer, err := proto.Marshal(&request)
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}

	if _, _, err := SendFriendMessage(&buffer, senderClient); err != nil {
		t.Fatalf("SendFriendMessage failed: %v", err)
	}

	var ack protobuf.SC_50106
	decodePacketAt(t, senderClient, 0, 50106, &ack)
	if ack.GetResult() == 0 {
		t.Fatalf("expected non-zero result for non-friend")
	}
	if recipientClient.Buffer.Len() != 0 {
		t.Fatalf("expected no push packet for non-friend send")
	}
	count := queryAnswerTestInt64(t, "SELECT COUNT(*) FROM friend_direct_messages")
	if count != 0 {
		t.Fatalf("expected no friend direct messages, got %d", count)
	}
}

func TestSendFriendMessageDecodeFailure(t *testing.T) {
	setupSocialMiscState(t)
	sender := setupSocialMiscCommander(t, "Sender Decode")
	client := &connection.Client{Commander: sender}
	buffer := []byte{0xff, 0x00, 0x42}
	_, outID, err := SendFriendMessage(&buffer, client)
	if err == nil {
		t.Fatalf("expected decode error")
	}
	if outID != 50106 {
		t.Fatalf("expected outgoing packet id 50106, got %d", outID)
	}
	count := queryAnswerTestInt64(t, "SELECT COUNT(*) FROM friend_direct_messages")
	if count != 0 {
		t.Fatalf("expected no friend direct messages, got %d", count)
	}
}

func TestReportPlayerSuccess(t *testing.T) {
	setupSocialMiscState(t)
	reporter := setupSocialMiscCommander(t, "Reporter")
	target := setupSocialMiscCommander(t, "Reported")
	client := &connection.Client{Commander: reporter}

	request := protobuf.CS_50111{Id: proto.Uint32(target.CommanderID), Info: proto.String("spam"), Content: proto.String("offensive language")}
	buffer, err := proto.Marshal(&request)
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}

	if _, _, err := ReportPlayer(&buffer, client); err != nil {
		t.Fatalf("ReportPlayer failed: %v", err)
	}

	var response protobuf.SC_50112
	decodePacketAt(t, client, 0, 50112, &response)
	if response.GetResult() != 0 {
		t.Fatalf("expected result 0, got %d", response.GetResult())
	}

	count := queryAnswerTestInt64(t, "SELECT COUNT(*) FROM player_informs WHERE reporter_id = $1 AND target_id = $2 AND info = $3 AND content = $4", int64(reporter.CommanderID), int64(target.CommanderID), "spam", "offensive language")
	if count != 1 {
		t.Fatalf("expected one player inform row, got %d", count)
	}
}

func TestReportPlayerDecodeFailure(t *testing.T) {
	setupSocialMiscState(t)
	reporter := setupSocialMiscCommander(t, "Reporter Decode")
	client := &connection.Client{Commander: reporter}
	buffer := []byte{0xff, 0x00, 0x42}
	_, outID, err := ReportPlayer(&buffer, client)
	if err == nil {
		t.Fatalf("expected decode error")
	}
	if outID != 50112 {
		t.Fatalf("expected outgoing packet id 50112, got %d", outID)
	}
	count := queryAnswerTestInt64(t, "SELECT COUNT(*) FROM player_informs")
	if count != 0 {
		t.Fatalf("expected no player informs, got %d", count)
	}
}

func TestGetThemeTemplatePlayerInfoSuccess(t *testing.T) {
	setupSocialMiscState(t)
	requester := setupSocialMiscCommander(t, "Requester")
	target := setupSocialMiscCommander(t, "Theme Owner")
	client := &connection.Client{Commander: requester}

	request := protobuf.CS_50113{UserId: proto.Uint32(target.CommanderID)}
	buffer, err := proto.Marshal(&request)
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}

	if _, _, err := GetThemeTemplatePlayerInfo(&buffer, client); err != nil {
		t.Fatalf("GetThemeTemplatePlayerInfo failed: %v", err)
	}

	var response protobuf.SC_50114
	decodePacketAt(t, client, 0, 50114, &response)
	if response.GetResult() != 0 {
		t.Fatalf("expected result 0, got %d", response.GetResult())
	}
	if response.GetPlayer().GetId() != target.CommanderID {
		t.Fatalf("expected player id %d, got %d", target.CommanderID, response.GetPlayer().GetId())
	}
	if response.GetPlayer().GetName() != target.Name {
		t.Fatalf("expected player name %q, got %q", target.Name, response.GetPlayer().GetName())
	}
}

func TestGetThemeTemplatePlayerInfoNotFound(t *testing.T) {
	setupSocialMiscState(t)
	requester := setupSocialMiscCommander(t, "Requester Missing")
	client := &connection.Client{Commander: requester}

	request := protobuf.CS_50113{UserId: proto.Uint32(99999999)}
	buffer, err := proto.Marshal(&request)
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}

	if _, _, err := GetThemeTemplatePlayerInfo(&buffer, client); err != nil {
		t.Fatalf("GetThemeTemplatePlayerInfo failed: %v", err)
	}

	var response protobuf.SC_50114
	decodePacketAt(t, client, 0, 50114, &response)
	if response.GetResult() == 0 {
		t.Fatalf("expected non-zero result for missing commander")
	}
	if response.GetPlayer().GetId() != 0 {
		t.Fatalf("expected player id 0 in failure response, got %d", response.GetPlayer().GetId())
	}
}

func TestGetThemeTemplatePlayerInfoDecodeFailure(t *testing.T) {
	setupSocialMiscState(t)
	requester := setupSocialMiscCommander(t, "Requester Decode")
	client := &connection.Client{Commander: requester}
	buffer := []byte{0xff, 0x00, 0x42}
	_, outID, err := GetThemeTemplatePlayerInfo(&buffer, client)
	if err == nil {
		t.Fatalf("expected decode error")
	}
	if outID != 50114 {
		t.Fatalf("expected outgoing packet id 50114, got %d", outID)
	}
}
