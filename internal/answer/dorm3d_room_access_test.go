package answer_test

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/ggmolly/belfast/internal/answer"
	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/orm"
	"github.com/ggmolly/belfast/internal/protobuf"
	"google.golang.org/protobuf/proto"
)

func seedDorm3dConfig(t *testing.T, roomID uint32) {
	t.Helper()
	roomConfig := fmt.Sprintf(`{"id":%d,"unlock_item":[[1,1,100],[2,60001,2]],"character_pay":[20220],"invite_cost":[[20220,270110]]}`,
		roomID,
	)
	if err := orm.UpsertConfigEntry("ShareCfg/dorm3d_rooms.json", fmt.Sprintf("%d", roomID), json.RawMessage(roomConfig)); err != nil {
		t.Fatalf("failed to seed dorm3d room config: %v", err)
	}
	if err := orm.UpsertConfigEntry("ShareCfg/shop_template.json", "270110", json.RawMessage(`{"id":270110,"resource_type":14,"resource_num":800}`)); err != nil {
		t.Fatalf("failed to seed shop template config: %v", err)
	}
}

func TestSelectDorm3dEnter(t *testing.T) {
	client := &connection.Client{}
	payload := &protobuf.CS_28017{Type: proto.Uint32(1)}
	buf, err := proto.Marshal(payload)
	if err != nil {
		t.Fatalf("failed to marshal payload: %v", err)
	}

	if _, _, err := answer.SelectDorm3dEnter(&buf, client); err != nil {
		t.Fatalf("SelectDorm3dEnter failed: %v", err)
	}
	response := &protobuf.SC_28018{}
	decodeTestPacket(t, client, 28018, response)
	if response.GetResult() != 0 {
		t.Fatalf("expected result 0, got %d", response.GetResult())
	}

	malformed := []byte{0xFF, 0x01}
	if _, _, err := answer.SelectDorm3dEnter(&malformed, client); err == nil {
		t.Fatalf("expected malformed payload error")
	}
}

func TestDorm3dRoomUnlockSuccessAndSnapshot(t *testing.T) {
	orm.InitDatabase()
	seedDorm3dConfig(t, 4)

	commander := createDorm3dCommander(t, 9301)
	if err := commander.SetResource(1, 500); err != nil {
		t.Fatalf("failed to seed resource: %v", err)
	}
	if err := commander.SetItem(60001, 5); err != nil {
		t.Fatalf("failed to seed item: %v", err)
	}

	client := &connection.Client{Commander: commander}
	payload := &protobuf.CS_28001{RoomId: proto.Uint32(4)}
	buf, err := proto.Marshal(payload)
	if err != nil {
		t.Fatalf("failed to marshal payload: %v", err)
	}

	if _, _, err := answer.Dorm3dRoomUnlock(&buf, client); err != nil {
		t.Fatalf("Dorm3dRoomUnlock failed: %v", err)
	}
	response := &protobuf.SC_28002{}
	decodeTestPacket(t, client, 28002, response)
	if response.GetResult() != 0 {
		t.Fatalf("expected result 0, got %d", response.GetResult())
	}
	if response.GetRoom() == nil || response.GetRoom().GetId() != 4 {
		t.Fatalf("expected unlocked room 4 in response")
	}

	if commander.OwnedResourcesMap[1].Amount != 400 {
		t.Fatalf("expected resource amount 400, got %d", commander.OwnedResourcesMap[1].Amount)
	}
	if commander.CommanderItemsMap[60001].Count != 3 {
		t.Fatalf("expected item count 3, got %d", commander.CommanderItemsMap[60001].Count)
	}

	apartment, err := orm.GetDorm3dApartment(commander.CommanderID)
	if err != nil {
		t.Fatalf("failed to load dorm3d apartment: %v", err)
	}
	if apartment.RoomByID(4) == nil {
		t.Fatalf("expected room to be persisted")
	}

	empty := []byte{}
	if _, _, err := answer.Dorm3dApartmentData(&empty, client); err != nil {
		t.Fatalf("Dorm3dApartmentData failed: %v", err)
	}
	snapshot := &protobuf.SC_28000{}
	decodeTestPacket(t, client, 28000, snapshot)
	if len(snapshot.GetRooms()) != 1 || snapshot.GetRooms()[0].GetId() != 4 {
		t.Fatalf("expected snapshot to include unlocked room")
	}
}

func TestDorm3dRoomUnlockFailures(t *testing.T) {
	orm.InitDatabase()
	seedDorm3dConfig(t, 4)

	duplicateCommander := createDorm3dCommander(t, 9302)
	if err := duplicateCommander.SetResource(1, 500); err != nil {
		t.Fatalf("failed to seed resource: %v", err)
	}
	if err := duplicateCommander.SetItem(60001, 5); err != nil {
		t.Fatalf("failed to seed item: %v", err)
	}
	apartment := orm.NewDorm3dApartment(duplicateCommander.CommanderID)
	apartment.Rooms = append(apartment.Rooms, orm.Dorm3dRoom{ID: 4, Furnitures: []orm.Dorm3dFurniture{}, Collections: []uint32{}, Ships: []uint32{}})
	if err := orm.SaveDorm3dApartment(&apartment); err != nil {
		t.Fatalf("failed to seed apartment: %v", err)
	}
	duplicateClient := &connection.Client{Commander: duplicateCommander}
	unlockPayload := &protobuf.CS_28001{RoomId: proto.Uint32(4)}
	unlockBuf, err := proto.Marshal(unlockPayload)
	if err != nil {
		t.Fatalf("failed to marshal payload: %v", err)
	}
	if _, _, err := answer.Dorm3dRoomUnlock(&unlockBuf, duplicateClient); err != nil {
		t.Fatalf("Dorm3dRoomUnlock failed: %v", err)
	}
	duplicateResp := &protobuf.SC_28002{}
	decodeTestPacket(t, duplicateClient, 28002, duplicateResp)
	if duplicateResp.GetResult() == 0 {
		t.Fatalf("expected duplicate unlock failure")
	}

	insufficientCommander := createDorm3dCommander(t, 9303)
	if err := insufficientCommander.SetResource(1, 50); err != nil {
		t.Fatalf("failed to seed resource: %v", err)
	}
	if err := insufficientCommander.SetItem(60001, 1); err != nil {
		t.Fatalf("failed to seed item: %v", err)
	}
	insufficientClient := &connection.Client{Commander: insufficientCommander}
	if _, _, err := answer.Dorm3dRoomUnlock(&unlockBuf, insufficientClient); err != nil {
		t.Fatalf("Dorm3dRoomUnlock failed: %v", err)
	}
	insufficientResp := &protobuf.SC_28002{}
	decodeTestPacket(t, insufficientClient, 28002, insufficientResp)
	if insufficientResp.GetResult() == 0 {
		t.Fatalf("expected insufficient cost failure")
	}
	updated, err := orm.GetDorm3dApartment(insufficientCommander.CommanderID)
	if err != nil {
		t.Fatalf("failed to load apartment: %v", err)
	}
	if updated.RoomByID(4) != nil {
		t.Fatalf("expected room not to persist on insufficient cost")
	}
}

func TestDorm3dReplaceFurniture(t *testing.T) {
	orm.InitDatabase()
	commander := createDorm3dCommander(t, 9304)
	stored := orm.NewDorm3dApartment(commander.CommanderID)
	stored.Rooms = orm.Dorm3dRoomList{{
		ID: 9,
		Furnitures: []orm.Dorm3dFurniture{
			{FurnitureID: 11, SlotID: 1},
			{FurnitureID: 22, SlotID: 2},
			{FurnitureID: 33, SlotID: 0},
		},
		Collections: []uint32{},
		Ships:       []uint32{},
	}}
	if err := orm.SaveDorm3dApartment(&stored); err != nil {
		t.Fatalf("failed to seed apartment: %v", err)
	}

	client := &connection.Client{Commander: commander}
	payload := &protobuf.CS_28007{
		RoomId: proto.Uint32(9),
		Furnitures: []*protobuf.APARTMENT_FURNITURE{
			{FurnitureId: proto.Uint32(33), SlotId: proto.Uint32(1)},
			{FurnitureId: proto.Uint32(0), SlotId: proto.Uint32(2)},
			{FurnitureId: proto.Uint32(0), SlotId: proto.Uint32(1)},
		},
	}
	buf, err := proto.Marshal(payload)
	if err != nil {
		t.Fatalf("failed to marshal payload: %v", err)
	}
	if _, _, err := answer.Dorm3dReplaceFurniture(&buf, client); err != nil {
		t.Fatalf("Dorm3dReplaceFurniture failed: %v", err)
	}
	response := &protobuf.SC_28008{}
	decodeTestPacket(t, client, 28008, response)
	if response.GetResult() != 0 {
		t.Fatalf("expected success result")
	}

	updated, err := orm.GetDorm3dApartment(commander.CommanderID)
	if err != nil {
		t.Fatalf("failed to load apartment: %v", err)
	}
	room := updated.RoomByID(9)
	if room == nil {
		t.Fatalf("expected room to exist")
	}
	if room.Furnitures[0].SlotID != 0 || room.Furnitures[1].SlotID != 0 || room.Furnitures[2].SlotID != 0 {
		t.Fatalf("expected clear and ordering semantics to leave all slots empty")
	}

	unknown := &protobuf.CS_28007{RoomId: proto.Uint32(9999), Furnitures: []*protobuf.APARTMENT_FURNITURE{}}
	unknownBuf, err := proto.Marshal(unknown)
	if err != nil {
		t.Fatalf("failed to marshal unknown payload: %v", err)
	}
	if _, _, err := answer.Dorm3dReplaceFurniture(&unknownBuf, client); err != nil {
		t.Fatalf("unexpected handler error for unknown room: %v", err)
	}
	unknownResp := &protobuf.SC_28008{}
	decodeTestPacket(t, client, 28008, unknownResp)
	if unknownResp.GetResult() == 0 {
		t.Fatalf("expected unknown room failure")
	}
}

func TestDorm3dRoomInviteUnlock(t *testing.T) {
	orm.InitDatabase()
	seedDorm3dConfig(t, 4)

	commander := createDorm3dCommander(t, 9305)
	if err := commander.SetResource(4, 1000); err != nil {
		t.Fatalf("failed to seed resource: %v", err)
	}
	stored := orm.NewDorm3dApartment(commander.CommanderID)
	stored.Rooms = orm.Dorm3dRoomList{{ID: 4, Furnitures: []orm.Dorm3dFurniture{}, Collections: []uint32{}, Ships: []uint32{}}}
	if err := orm.SaveDorm3dApartment(&stored); err != nil {
		t.Fatalf("failed to seed apartment: %v", err)
	}
	client := &connection.Client{Commander: commander}
	payload := &protobuf.CS_28019{RoomId: proto.Uint32(4), ShipGroup: proto.Uint32(20220)}
	buf, err := proto.Marshal(payload)
	if err != nil {
		t.Fatalf("failed to marshal payload: %v", err)
	}
	if _, _, err := answer.Dorm3dRoomInviteUnlock(&buf, client); err != nil {
		t.Fatalf("Dorm3dRoomInviteUnlock failed: %v", err)
	}
	response := &protobuf.SC_28020{}
	decodeTestPacket(t, client, 28020, response)
	if response.GetResult() != 0 {
		t.Fatalf("expected result 0, got %d", response.GetResult())
	}
	if commander.OwnedResourcesMap[4].Amount != 200 {
		t.Fatalf("expected resource amount 200 after invite unlock, got %d", commander.OwnedResourcesMap[4].Amount)
	}
	updated, err := orm.GetDorm3dApartment(commander.CommanderID)
	if err != nil {
		t.Fatalf("failed to load apartment: %v", err)
	}
	room := updated.RoomByID(4)
	if room == nil || len(room.Ships) != 1 || room.Ships[0] != 20220 {
		t.Fatalf("expected invite ship to be persisted")
	}

	if _, _, err := answer.Dorm3dRoomInviteUnlock(&buf, client); err != nil {
		t.Fatalf("Dorm3dRoomInviteUnlock duplicate failed unexpectedly: %v", err)
	}
	duplicate := &protobuf.SC_28020{}
	decodeTestPacket(t, client, 28020, duplicate)
	if duplicate.GetResult() == 0 {
		t.Fatalf("expected duplicate unlock failure")
	}

	badTarget := &protobuf.CS_28019{RoomId: proto.Uint32(4), ShipGroup: proto.Uint32(99999)}
	badTargetBuf, err := proto.Marshal(badTarget)
	if err != nil {
		t.Fatalf("failed to marshal payload: %v", err)
	}
	if _, _, err := answer.Dorm3dRoomInviteUnlock(&badTargetBuf, client); err != nil {
		t.Fatalf("Dorm3dRoomInviteUnlock bad target failed unexpectedly: %v", err)
	}
	badTargetResp := &protobuf.SC_28020{}
	decodeTestPacket(t, client, 28020, badTargetResp)
	if badTargetResp.GetResult() == 0 {
		t.Fatalf("expected invalid target failure")
	}

	insufficientCommander := createDorm3dCommander(t, 9306)
	if err := insufficientCommander.SetResource(4, 100); err != nil {
		t.Fatalf("failed to seed resource: %v", err)
	}
	insufficientApartment := orm.NewDorm3dApartment(insufficientCommander.CommanderID)
	insufficientApartment.Rooms = orm.Dorm3dRoomList{{ID: 4, Furnitures: []orm.Dorm3dFurniture{}, Collections: []uint32{}, Ships: []uint32{}}}
	if err := orm.SaveDorm3dApartment(&insufficientApartment); err != nil {
		t.Fatalf("failed to seed apartment: %v", err)
	}
	insufficientClient := &connection.Client{Commander: insufficientCommander}
	if _, _, err := answer.Dorm3dRoomInviteUnlock(&buf, insufficientClient); err != nil {
		t.Fatalf("Dorm3dRoomInviteUnlock insufficient failed unexpectedly: %v", err)
	}
	insufficientResp := &protobuf.SC_28020{}
	decodeTestPacket(t, insufficientClient, 28020, insufficientResp)
	if insufficientResp.GetResult() != 2 {
		t.Fatalf("expected insufficient resource result 2, got %d", insufficientResp.GetResult())
	}

	missingRoomCommander := createDorm3dCommander(t, 9307)
	if err := missingRoomCommander.SetResource(4, 1000); err != nil {
		t.Fatalf("failed to seed resource: %v", err)
	}
	missingRoomClient := &connection.Client{Commander: missingRoomCommander}
	if _, _, err := answer.Dorm3dRoomInviteUnlock(&buf, missingRoomClient); err != nil {
		t.Fatalf("Dorm3dRoomInviteUnlock missing room failed unexpectedly: %v", err)
	}
	missingRoomResp := &protobuf.SC_28020{}
	decodeTestPacket(t, missingRoomClient, 28020, missingRoomResp)
	if missingRoomResp.GetResult() == 0 {
		t.Fatalf("expected missing room failure")
	}

	malformed := []byte{0xAA}
	if _, _, err := answer.Dorm3dRoomInviteUnlock(&malformed, client); err == nil {
		t.Fatalf("expected malformed payload error")
	}
}
