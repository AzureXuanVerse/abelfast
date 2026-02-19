package answer_test

import (
	"encoding/json"
	"fmt"
	"os"
	"testing"

	"github.com/ggmolly/belfast/internal/answer"
	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/orm"
	"github.com/ggmolly/belfast/internal/packets"
	"github.com/ggmolly/belfast/internal/protobuf"
	"google.golang.org/protobuf/proto"
)

func createDorm3dCommander(t *testing.T, commanderID uint32) *orm.Commander {
	os.Setenv("MODE", "test")
	orm.InitDatabase()
	commander := &orm.Commander{
		CommanderID: commanderID,
		AccountID:   commanderID,
		Name:        fmt.Sprintf("Dorm3d Tester %d", commanderID),
	}
	if err := orm.CreateCommanderRoot(commanderID, commanderID, commander.Name, 0, 0); err != nil {
		t.Fatalf("failed to create commander: %v", err)
	}
	if err := commander.Load(); err != nil {
		t.Fatalf("failed to load commander: %v", err)
	}
	return commander
}

func TestDorm3dApartmentDataUsesStoredData(t *testing.T) {
	commander := createDorm3dCommander(t, 9100)
	stored := orm.NewDorm3dApartment(commander.CommanderID)
	stored.DailyVigorMax = 120
	stored.Gifts = orm.Dorm3dGiftList{{GiftID: 101, Number: 5, UsedNumber: 2}}
	stored.GiftDaily = orm.Dorm3dGiftShopList{{GiftID: 202, Count: 3}}
	stored.Rooms = orm.Dorm3dRoomList{{
		ID:          7,
		Furnitures:  []orm.Dorm3dFurniture{{FurnitureID: 44, SlotID: 1}},
		Collections: []uint32{9},
		Ships:       []uint32{10},
	}}
	stored.Ships = orm.Dorm3dShipList{{
		ShipGroup:      301,
		FavorLv:        2,
		FavorExp:       123,
		RegularTrigger: []uint32{1},
		DailyFavor:     3,
		Dialogues:      []uint32{4},
		Skins:          []uint32{5},
		CurSkin:        6,
		Name:           "Dorm3d",
		NameCd:         7,
		VisitTime:      8,
		HiddenInfo:     []orm.Dorm3dSkinHiddenInfo{{SkinID: 6, HiddenParts: []uint32{1}}},
	}}
	stored.Ins = orm.Dorm3dInsList{{
		ShipGroup: 1,
		CareFlag:  1,
		CurBack:   2,
		CurCommId: 3,
		CommList: []orm.Dorm3dCommInfo{{
			ID:       11,
			Time:     100,
			ReadFlag: 1,
			ReplyList: []orm.Dorm3dKeyValue{{
				Key:   1,
				Value: 2,
			}},
		}},
		PhoneList: []orm.Dorm3dPhoneInfo{{ID: 12, Time: 200, ReadFlag: 0}},
		FriendList: []orm.Dorm3dFriendCircleInfo{{
			ID:       13,
			Time:     300,
			ReadFlag: 1,
			GoodFlag: 1,
			ExitTime: 400,
			ReplyList: []orm.Dorm3dReplyFriend{{
				Key:   3,
				Value: 4,
				Time:  300,
			}},
		}},
	}}
	if err := orm.CreateDorm3dApartment(&stored); err != nil {
		t.Fatalf("failed to create dorm3d apartment: %v", err)
	}

	client := &connection.Client{Commander: commander}
	buffer := []byte{}
	if _, _, err := answer.Dorm3dApartmentData(&buffer, client); err != nil {
		t.Fatalf("Dorm3dApartmentData failed: %v", err)
	}
	response := &protobuf.SC_28000{}
	decodeTestPacket(t, client, 28000, response)
	if response.GetDailyVigorMax() != 120 {
		t.Fatalf("expected daily vigor max 120, got %d", response.GetDailyVigorMax())
	}
	if len(response.GetGifts()) != 1 {
		t.Fatalf("expected 1 gift, got %d", len(response.GetGifts()))
	}
	if response.GetGifts()[0].GetGiftId() != 101 {
		t.Fatalf("expected gift id 101, got %d", response.GetGifts()[0].GetGiftId())
	}
	if len(response.GetIns()) != 1 {
		t.Fatalf("expected 1 ins entry, got %d", len(response.GetIns()))
	}
	if len(response.GetIns()[0].GetFriendList()) != 1 {
		t.Fatalf("expected 1 friend list entry, got %d", len(response.GetIns()[0].GetFriendList()))
	}
}

func TestDorm3dInstagramOpsPersist(t *testing.T) {
	commander := createDorm3dCommander(t, 9101)
	stored := orm.NewDorm3dApartment(commander.CommanderID)
	stored.Ins = orm.Dorm3dInsList{{
		ShipGroup: 100,
		FriendList: []orm.Dorm3dFriendCircleInfo{{
			ID:        55,
			Time:      10,
			ReadFlag:  0,
			GoodFlag:  0,
			ExitTime:  0,
			ReplyList: []orm.Dorm3dReplyFriend{},
		}},
	}}
	if err := orm.CreateDorm3dApartment(&stored); err != nil {
		t.Fatalf("failed to create dorm3d apartment: %v", err)
	}
	client := &connection.Client{Commander: commander}

	readPayload := &protobuf.CS_28026{
		ShipId: proto.Uint32(100),
		Type:   proto.Uint32(orm.Dorm3dInstagramOpRead),
		IdList: []uint32{55},
	}
	readBuf, err := proto.Marshal(readPayload)
	if err != nil {
		t.Fatalf("failed to marshal payload: %v", err)
	}
	if _, _, err := answer.HandleDorm3dInstagramAction(&readBuf, client); err != nil {
		t.Fatalf("Dorm3dInstagramOp failed: %v", err)
	}
	readResponse := &protobuf.SC_28027{}
	decodeTestPacket(t, client, 28027, readResponse)
	if readResponse.GetResult() != 0 {
		t.Fatalf("expected result 0, got %d", readResponse.GetResult())
	}

	discussPayload := &protobuf.CS_28028{
		ShipId: proto.Uint32(100),
		Type:   proto.Uint32(2),
		Id:     proto.Uint32(55),
		ChatId: proto.Uint32(9),
		Value:  proto.Uint32(3),
	}
	discussBuf, err := proto.Marshal(discussPayload)
	if err != nil {
		t.Fatalf("failed to marshal payload: %v", err)
	}
	if _, _, err := answer.Dorm3dInstagramDiscuss(&discussBuf, client); err != nil {
		t.Fatalf("Dorm3dInstagramDiscuss failed: %v", err)
	}
	discussResponse := &protobuf.SC_28029{}
	decodeTestPacket(t, client, 28029, discussResponse)
	if discussResponse.GetResult() != 0 {
		t.Fatalf("expected result 0, got %d", discussResponse.GetResult())
	}

	updated, err := orm.GetDorm3dApartment(commander.CommanderID)
	if err != nil {
		t.Fatalf("failed to load dorm3d apartment: %v", err)
	}
	if len(updated.Ins) != 1 {
		t.Fatalf("expected 1 ins entry, got %d", len(updated.Ins))
	}
	friend := updated.Ins[0].FriendList[0]
	if friend.ReadFlag != 1 {
		t.Fatalf("expected read flag 1, got %d", friend.ReadFlag)
	}
	if len(friend.ReplyList) != 1 {
		t.Fatalf("expected 1 reply, got %d", len(friend.ReplyList))
	}
	if friend.ReplyList[0].Key != 9 || friend.ReplyList[0].Value != 3 {
		t.Fatalf("expected reply key 9 value 3, got %d/%d", friend.ReplyList[0].Key, friend.ReplyList[0].Value)
	}
	if friend.ReplyList[0].Time == 0 {
		t.Fatalf("expected reply time to be set")
	}
	if friend.Time == 0 {
		t.Fatalf("expected friend time to be set")
	}
}

func seedDorm3dConfigEntry(t *testing.T, category string, key string, raw string) {
	t.Helper()
	if err := orm.UpsertConfigEntry(category, key, json.RawMessage(raw)); err != nil {
		t.Fatalf("failed to seed config entry %s/%s: %v", category, key, err)
	}
}

func decodePacketSequenceIDs(t *testing.T, client *connection.Client) []int {
	t.Helper()
	buffer := client.Buffer.Bytes()
	ids := make([]int, 0)
	offset := 0
	for offset < len(buffer) {
		packetID := packets.GetPacketId(offset, &buffer)
		ids = append(ids, packetID)
		size := packets.GetPacketSize(offset, &buffer) + 2
		offset += size
	}
	return ids
}

func TestDorm3dChatSettingsPersist(t *testing.T) {
	commander := createDorm3dCommander(t, 9102)
	seedDorm3dConfigEntry(t, "ShareCfg/dorm3d_ins_chat_group.json", "3001", `{"id":3001,"ship_group":100}`)
	stored := orm.NewDorm3dApartment(commander.CommanderID)
	stored.Ships = orm.Dorm3dShipList{{ShipGroup: 100, Skins: []uint32{2001}, CurSkin: 2001}}
	stored.Ins = orm.Dorm3dInsList{{ShipGroup: 100, CommList: []orm.Dorm3dCommInfo{{ID: 3001, Time: 10, ReadFlag: 0, ReplyList: []orm.Dorm3dKeyValue{}}}}}
	if err := orm.CreateDorm3dApartment(&stored); err != nil {
		t.Fatalf("failed to create dorm3d apartment: %v", err)
	}
	client := &connection.Client{Commander: commander}

	backgroundPayload := protobuf.CS_28030{ShipId: proto.Uint32(100), BackId: proto.Uint32(2001)}
	backgroundBuf, _ := proto.Marshal(&backgroundPayload)
	if _, _, err := answer.Dorm3dChatSetBackground(&backgroundBuf, client); err != nil {
		t.Fatalf("Dorm3dChatSetBackground failed: %v", err)
	}
	backgroundResp := &protobuf.SC_28031{}
	decodeTestPacket(t, client, 28031, backgroundResp)
	if backgroundResp.GetResult() != 0 {
		t.Fatalf("expected background result 0, got %d", backgroundResp.GetResult())
	}

	carePayload := protobuf.CS_28032{ShipId: proto.Uint32(100), Value: proto.Uint32(1)}
	careBuf, _ := proto.Marshal(&carePayload)
	if _, _, err := answer.Dorm3dChatSetCare(&careBuf, client); err != nil {
		t.Fatalf("Dorm3dChatSetCare failed: %v", err)
	}
	careResp := &protobuf.SC_28033{}
	decodeTestPacket(t, client, 28033, careResp)
	if careResp.GetResult() != 0 {
		t.Fatalf("expected care result 0, got %d", careResp.GetResult())
	}

	topicPayload := protobuf.CS_28034{ShipId: proto.Uint32(100), CommId: proto.Uint32(3001)}
	topicBuf, _ := proto.Marshal(&topicPayload)
	if _, _, err := answer.Dorm3dInstagramSetTopic(&topicBuf, client); err != nil {
		t.Fatalf("Dorm3dInstagramSetTopic failed: %v", err)
	}
	topicResp := &protobuf.SC_28035{}
	decodeTestPacket(t, client, 28035, topicResp)
	if topicResp.GetResult() != 0 {
		t.Fatalf("expected topic result 0, got %d", topicResp.GetResult())
	}

	visitPayload := protobuf.CS_28036{ShipId: proto.Uint32(100)}
	visitBuf, _ := proto.Marshal(&visitPayload)
	if _, _, err := answer.Dorm3dRecordVisit(&visitBuf, client); err != nil {
		t.Fatalf("Dorm3dRecordVisit failed: %v", err)
	}
	visitResp := &protobuf.SC_28037{}
	decodeTestPacket(t, client, 28037, visitResp)
	if visitResp.GetResult() != 0 {
		t.Fatalf("expected visit result 0, got %d", visitResp.GetResult())
	}

	updated, err := orm.GetDorm3dApartment(commander.CommanderID)
	if err != nil {
		t.Fatalf("failed to load apartment: %v", err)
	}
	if updated.Ins[0].CurBack != 2001 || updated.Ins[0].CareFlag != 1 || updated.Ins[0].CurCommId != 3001 {
		t.Fatalf("unexpected ins state %+v", updated.Ins[0])
	}
	if updated.Ships[0].VisitTime == 0 {
		t.Fatalf("expected visit time to be set")
	}
}

func TestDorm3dChatSettingsFailureResults(t *testing.T) {
	commander := createDorm3dCommander(t, 9103)
	stored := orm.NewDorm3dApartment(commander.CommanderID)
	stored.Ships = orm.Dorm3dShipList{{ShipGroup: 100, Skins: []uint32{2001}, CurSkin: 2001}}
	if err := orm.CreateDorm3dApartment(&stored); err != nil {
		t.Fatalf("failed to create dorm3d apartment: %v", err)
	}
	client := &connection.Client{Commander: commander}

	backgroundPayload := protobuf.CS_28030{ShipId: proto.Uint32(100), BackId: proto.Uint32(9999)}
	backgroundBuf, _ := proto.Marshal(&backgroundPayload)
	if _, _, err := answer.Dorm3dChatSetBackground(&backgroundBuf, client); err != nil {
		t.Fatalf("Dorm3dChatSetBackground failed: %v", err)
	}
	backgroundResp := &protobuf.SC_28031{}
	decodeTestPacket(t, client, 28031, backgroundResp)
	if backgroundResp.GetResult() == 0 {
		t.Fatalf("expected non-zero background result")
	}

	topicPayload := protobuf.CS_28034{ShipId: proto.Uint32(100), CommId: proto.Uint32(5555)}
	topicBuf, _ := proto.Marshal(&topicPayload)
	if _, _, err := answer.Dorm3dInstagramSetTopic(&topicBuf, client); err != nil {
		t.Fatalf("Dorm3dInstagramSetTopic failed: %v", err)
	}
	topicResp := &protobuf.SC_28035{}
	decodeTestPacket(t, client, 28035, topicResp)
	if topicResp.GetResult() == 0 {
		t.Fatalf("expected non-zero topic result")
	}

	visitPayload := protobuf.CS_28036{ShipId: proto.Uint32(200)}
	visitBuf, _ := proto.Marshal(&visitPayload)
	if _, _, err := answer.Dorm3dRecordVisit(&visitBuf, client); err != nil {
		t.Fatalf("Dorm3dRecordVisit failed: %v", err)
	}
	visitResp := &protobuf.SC_28037{}
	decodeTestPacket(t, client, 28037, visitResp)
	if visitResp.GetResult() == 0 {
		t.Fatalf("expected non-zero visit result")
	}
}

func TestDorm3dTriggerEventUnlockAndDedupe(t *testing.T) {
	commander := createDorm3dCommander(t, 9104)
	seedDorm3dConfigEntry(t, "ShareCfg/dorm3d_ins_unlock.json", "1", `{"id":1,"type":1,"content":4001,"trigger_type":152,"trigger_num":2}`)
	seedDorm3dConfigEntry(t, "ShareCfg/dorm3d_ins_chat_group.json", "4001", `{"id":4001,"ship_group":100}`)
	stored := orm.NewDorm3dApartment(commander.CommanderID)
	stored.Ships = orm.Dorm3dShipList{{ShipGroup: 100}}
	if err := orm.CreateDorm3dApartment(&stored); err != nil {
		t.Fatalf("failed to create dorm3d apartment: %v", err)
	}
	client := &connection.Client{Commander: commander}

	payload := protobuf.CS_28023{EventList: []*protobuf.APA_EVENT_INFO{{
		EventType: proto.Uint32(152),
		Value:     proto.Uint32(2),
		ShipId:    proto.Uint32(100),
	}}}
	buf, _ := proto.Marshal(&payload)
	if _, _, err := answer.Dorm3dChatTriggerEvent(&buf, client); err != nil {
		t.Fatalf("Dorm3dChatTriggerEvent failed: %v", err)
	}
	ids := decodePacketSequenceIDs(t, client)
	if len(ids) != 2 || ids[0] != 28024 || ids[1] != 28025 {
		t.Fatalf("expected packet sequence [28024 28025], got %v", ids)
	}
	client.Buffer.Reset()

	buf, _ = proto.Marshal(&payload)
	if _, _, err := answer.Dorm3dChatTriggerEvent(&buf, client); err != nil {
		t.Fatalf("Dorm3dChatTriggerEvent replay failed: %v", err)
	}
	ids = decodePacketSequenceIDs(t, client)
	if len(ids) != 1 || ids[0] != 28024 {
		t.Fatalf("expected only ack packet, got %v", ids)
	}
	client.Buffer.Reset()

	updated, err := orm.GetDorm3dApartment(commander.CommanderID)
	if err != nil {
		t.Fatalf("failed to load apartment: %v", err)
	}
	if len(updated.Ins) != 1 || len(updated.Ins[0].CommList) != 1 || updated.Ins[0].CommList[0].ID != 4001 {
		t.Fatalf("expected unlocked comm topic persisted")
	}
}

func TestDorm3dCollectionItemPersists(t *testing.T) {
	commander := createDorm3dCommander(t, 9110)
	seedDorm3dConfigEntry(t, "ShareCfg/dorm3d_collection_template.json", "7001", `{"id":7001,"room_id":10}`)
	seedDorm3dConfigEntry(t, "ShareCfg/dorm3d_rooms.json", "10", `{"id":10,"type":2,"character":[100]}`)
	stored := orm.NewDorm3dApartment(commander.CommanderID)
	stored.Rooms = orm.Dorm3dRoomList{{ID: 10, Collections: []uint32{}}}
	if err := orm.CreateDorm3dApartment(&stored); err != nil {
		t.Fatalf("create apartment: %v", err)
	}
	client := &connection.Client{Commander: commander}
	payload := protobuf.CS_28011{RoomId: proto.Uint32(10), CollectionId: proto.Uint32(7001), ShipGroup: proto.Uint32(100)}
	buf, _ := proto.Marshal(&payload)
	if _, _, err := answer.Dorm3dCollectionItem(&buf, client); err != nil {
		t.Fatalf("Dorm3dCollectionItem failed: %v", err)
	}
	resp := &protobuf.SC_28012{}
	decodeTestPacket(t, client, 28012, resp)
	if resp.GetResult() != 0 {
		t.Fatalf("expected result 0")
	}

	buf, _ = proto.Marshal(&payload)
	if _, _, err := answer.Dorm3dCollectionItem(&buf, client); err != nil {
		t.Fatalf("Dorm3dCollectionItem retry failed: %v", err)
	}
	decodeTestPacket(t, client, 28012, resp)

	updated, err := orm.GetDorm3dApartment(commander.CommanderID)
	if err != nil {
		t.Fatalf("get apartment: %v", err)
	}
	if len(updated.Rooms[0].Collections) != 1 || updated.Rooms[0].Collections[0] != 7001 {
		t.Fatalf("unexpected collections: %+v", updated.Rooms[0].Collections)
	}
}

func TestDorm3dApartmentOpsPersist(t *testing.T) {
	commander := createDorm3dCommander(t, 9111)
	seedDorm3dConfigEntry(t, "ShareCfg/dorm3d_dialogue_group.json", "8001", `{"id":8001,"char_id":100}`)
	stored := orm.NewDorm3dApartment(commander.CommanderID)
	stored.Ships = orm.Dorm3dShipList{{ShipGroup: 100, Skins: []uint32{2001}, CurSkin: 2001, HiddenInfo: []orm.Dorm3dSkinHiddenInfo{}}}
	if err := orm.CreateDorm3dApartment(&stored); err != nil {
		t.Fatalf("create apartment: %v", err)
	}
	client := &connection.Client{Commander: commander}

	skinPayload := protobuf.CS_28013{ShipGroup: proto.Uint32(100), Skin: proto.Uint32(2001)}
	skinBuf, _ := proto.Marshal(&skinPayload)
	if _, _, err := answer.Dorm3dChangeSkin(&skinBuf, client); err != nil {
		t.Fatalf("Dorm3dChangeSkin failed: %v", err)
	}
	skinResp := &protobuf.SC_28014{}
	decodeTestPacket(t, client, 28014, skinResp)
	if skinResp.GetResult() != 0 {
		t.Fatalf("expected skin result 0")
	}

	talkPayload := protobuf.CS_28015{DialogId: proto.Uint32(8001)}
	talkBuf, _ := proto.Marshal(&talkPayload)
	if _, _, err := answer.Dorm3dTalk(&talkBuf, client); err != nil {
		t.Fatalf("Dorm3dTalk failed: %v", err)
	}
	talkResp := &protobuf.SC_28016{}
	decodeTestPacket(t, client, 28016, talkResp)
	if talkResp.GetResult() != 0 || len(talkResp.GetDropList()) != 0 {
		t.Fatalf("expected talk success with empty drops")
	}

	callPayload := protobuf.CS_28021{ShipGroup: proto.Uint32(100), Name: proto.String("Commander")}
	callBuf, _ := proto.Marshal(&callPayload)
	if _, _, err := answer.Dorm3dSetCall(&callBuf, client); err != nil {
		t.Fatalf("Dorm3dSetCall failed: %v", err)
	}
	callResp := &protobuf.SC_28022{}
	decodeTestPacket(t, client, 28022, callResp)
	if callResp.GetResult() != 0 {
		t.Fatalf("expected call-name result 0")
	}

	hiddenPayload := protobuf.CS_28038{ShipGroup: proto.Uint32(100), SkinId: proto.Uint32(2001), HiddenParts: []uint32{3, 9}}
	hiddenBuf, _ := proto.Marshal(&hiddenPayload)
	if _, _, err := answer.Dorm3dSetSkinHiddenParts(&hiddenBuf, client); err != nil {
		t.Fatalf("Dorm3dSetSkinHiddenParts failed: %v", err)
	}
	hiddenResp := &protobuf.SC_28039{}
	decodeTestPacket(t, client, 28039, hiddenResp)
	if hiddenResp.GetResult() != 0 {
		t.Fatalf("expected hidden-parts result 0")
	}

	updated, err := orm.GetDorm3dApartment(commander.CommanderID)
	if err != nil {
		t.Fatalf("get apartment: %v", err)
	}
	ship := updated.Ships[0]
	if ship.Name != "Commander" || ship.NameCd == 0 {
		t.Fatalf("expected call name + cooldown persisted")
	}
	if len(ship.Dialogues) != 1 || ship.Dialogues[0] != 8001 {
		t.Fatalf("expected dialogue persisted")
	}
	if len(ship.HiddenInfo) != 1 || len(ship.HiddenInfo[0].HiddenParts) != 2 {
		t.Fatalf("expected hidden info persisted")
	}
}

func TestDorm3dApartmentOpsFailureResults(t *testing.T) {
	commander := createDorm3dCommander(t, 9112)
	stored := orm.NewDorm3dApartment(commander.CommanderID)
	stored.Ships = orm.Dorm3dShipList{{ShipGroup: 100, Skins: []uint32{2001}}}
	stored.Rooms = orm.Dorm3dRoomList{{ID: 10}}
	if err := orm.CreateDorm3dApartment(&stored); err != nil {
		t.Fatalf("create apartment: %v", err)
	}
	client := &connection.Client{Commander: commander}

	collectionPayload := protobuf.CS_28011{RoomId: proto.Uint32(10), CollectionId: proto.Uint32(9999), ShipGroup: proto.Uint32(0)}
	collectionBuf, _ := proto.Marshal(&collectionPayload)
	if _, _, err := answer.Dorm3dCollectionItem(&collectionBuf, client); err != nil {
		t.Fatalf("Dorm3dCollectionItem failed: %v", err)
	}
	collectionResp := &protobuf.SC_28012{}
	decodeTestPacket(t, client, 28012, collectionResp)
	if collectionResp.GetResult() == 0 {
		t.Fatalf("expected non-zero collection result")
	}

	changeSkinPayload := protobuf.CS_28013{ShipGroup: proto.Uint32(100), Skin: proto.Uint32(9999)}
	changeSkinBuf, _ := proto.Marshal(&changeSkinPayload)
	if _, _, err := answer.Dorm3dChangeSkin(&changeSkinBuf, client); err != nil {
		t.Fatalf("Dorm3dChangeSkin failed: %v", err)
	}
	changeSkinResp := &protobuf.SC_28014{}
	decodeTestPacket(t, client, 28014, changeSkinResp)
	if changeSkinResp.GetResult() == 0 {
		t.Fatalf("expected non-zero skin result")
	}

	talkPayload := protobuf.CS_28015{DialogId: proto.Uint32(9999)}
	talkBuf, _ := proto.Marshal(&talkPayload)
	if _, _, err := answer.Dorm3dTalk(&talkBuf, client); err != nil {
		t.Fatalf("Dorm3dTalk failed: %v", err)
	}
	talkResp := &protobuf.SC_28016{}
	decodeTestPacket(t, client, 28016, talkResp)
	if talkResp.GetResult() == 0 {
		t.Fatalf("expected non-zero talk result")
	}
}
