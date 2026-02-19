package orm

import (
	"encoding/json"
	"errors"
	"testing"
)

func TestDorm3dApartmentLifecycle(t *testing.T) {
	initCommanderItemTestDB(t)
	clearTable(t, &Dorm3dApartment{})

	if _, err := GetDorm3dApartment(1); err == nil {
		t.Fatalf("expected error for missing apartment")
	}
	apartment, err := GetOrCreateDorm3dApartment(1)
	if err != nil {
		t.Fatalf("get or create apartment: %v", err)
	}
	if apartment.CommanderID != 1 {
		t.Fatalf("unexpected commander id")
	}
	if apartment.Gifts == nil || apartment.Ships == nil || apartment.Ins == nil {
		t.Fatalf("expected defaults initialized")
	}
}

func TestDorm3dInstagramUpdatesAndReplies(t *testing.T) {
	initCommanderItemTestDB(t)
	clearTable(t, &Dorm3dApartment{})

	if err := UpdateDorm3dInstagramFlags(2, 10, []uint32{55}, Dorm3dInstagramOpRead, 100); err != nil {
		t.Fatalf("update instagram flags: %v", err)
	}
	if err := UpdateDorm3dInstagramFlags(2, 10, []uint32{55}, Dorm3dInstagramOpLike, 100); err != nil {
		t.Fatalf("update instagram like: %v", err)
	}
	if err := AddDorm3dInstagramReply(2, 10, 55, 7, 9, 100); err != nil {
		t.Fatalf("add instagram reply: %v", err)
	}
	apartment, err := GetDorm3dApartment(2)
	if err != nil {
		t.Fatalf("get apartment: %v", err)
	}
	if len(apartment.Ins) != 1 || len(apartment.Ins[0].FriendList) != 1 {
		t.Fatalf("expected ins entries")
	}
	entry := apartment.Ins[0].FriendList[0]
	if entry.ReadFlag != 1 || entry.GoodFlag != 1 {
		t.Fatalf("expected read and like flags set")
	}
	if len(entry.ReplyList) != 1 {
		t.Fatalf("expected reply list")
	}
}

func TestDorm3dEnsureDefaults(t *testing.T) {
	apartment := Dorm3dApartment{}
	apartment.EnsureDefaults()
	if apartment.Gifts == nil || apartment.Ships == nil || apartment.Ins == nil {
		t.Fatalf("expected defaults set")
	}
}

func TestDorm3dJSONScan(t *testing.T) {
	list := Dorm3dGiftList{{GiftID: 1}}
	value, err := list.Value()
	if err != nil {
		t.Fatalf("value: %v", err)
	}
	var decoded Dorm3dGiftList
	if err := decoded.Scan(value); err != nil {
		t.Fatalf("scan string: %v", err)
	}
	if len(decoded) != 1 {
		t.Fatalf("expected decoded list")
	}
	var decodedBytes Dorm3dGiftList
	if err := decodedBytes.Scan([]byte("[]")); err != nil {
		t.Fatalf("scan bytes: %v", err)
	}
	if err := decodedBytes.Scan(nil); err != nil {
		t.Fatalf("scan nil: %v", err)
	}
	if err := decodedBytes.Scan(123); err == nil {
		t.Fatalf("expected scan error for unsupported type")
	}

	giftShop := Dorm3dGiftShopList{{GiftID: 1, Count: 2}}
	value, err = giftShop.Value()
	if err != nil {
		t.Fatalf("gift shop value: %v", err)
	}
	var decodedGiftShop Dorm3dGiftShopList
	if err := decodedGiftShop.Scan(value); err != nil {
		t.Fatalf("gift shop scan: %v", err)
	}

	rooms := Dorm3dRoomList{{ID: 1}}
	value, err = rooms.Value()
	if err != nil {
		t.Fatalf("room value: %v", err)
	}
	var decodedRooms Dorm3dRoomList
	if err := decodedRooms.Scan(value); err != nil {
		t.Fatalf("room scan: %v", err)
	}

	ships := Dorm3dShipList{{ShipGroup: 1, Name: "X"}}
	value, err = ships.Value()
	if err != nil {
		t.Fatalf("ship value: %v", err)
	}
	var decodedShips Dorm3dShipList
	if err := decodedShips.Scan(value); err != nil {
		t.Fatalf("ship scan: %v", err)
	}

	ins := Dorm3dInsList{{ShipGroup: 1}}
	value, err = ins.Value()
	if err != nil {
		t.Fatalf("ins value: %v", err)
	}
	var decodedIns Dorm3dInsList
	if err := decodedIns.Scan(value); err != nil {
		t.Fatalf("ins scan: %v", err)
	}
}

func TestDorm3dApartmentOps(t *testing.T) {
	initCommanderItemTestDB(t)
	clearTable(t, &Dorm3dApartment{})
	clearTable(t, &ConfigEntry{})

	apartment := NewDorm3dApartment(6)
	apartment.Ships = Dorm3dShipList{{ShipGroup: 100, Skins: []uint32{2001}, HiddenInfo: []Dorm3dSkinHiddenInfo{}}}
	apartment.Rooms = Dorm3dRoomList{{ID: 10, Collections: []uint32{}}}
	if err := CreateDorm3dApartment(&apartment); err != nil {
		t.Fatalf("create apartment: %v", err)
	}
	if err := UpsertConfigEntry(dorm3dDialogueGroupCategory, "8001", json.RawMessage(`{"id":8001,"char_id":100}`)); err != nil {
		t.Fatalf("seed dialogue config: %v", err)
	}
	if err := UpsertConfigEntry(dorm3dCollectionTemplateCategory, "7001", json.RawMessage(`{"id":7001,"room_id":10}`)); err != nil {
		t.Fatalf("seed collection config: %v", err)
	}
	if err := UpsertConfigEntry(dorm3dRoomsCategory, "10", json.RawMessage(`{"id":10,"type":2,"character":[100]}`)); err != nil {
		t.Fatalf("seed room config: %v", err)
	}

	if err := SetDorm3dCallName(6, 100, "Commander", 1, 55); err != nil {
		t.Fatalf("set call name: %v", err)
	}
	if err := ChangeDorm3dShipSkin(6, 100, 2001); err != nil {
		t.Fatalf("change skin: %v", err)
	}
	if err := UpdateDorm3dSkinHiddenParts(6, 100, 2001, []uint32{1, 2}); err != nil {
		t.Fatalf("update hidden parts: %v", err)
	}
	if err := MarkDorm3dDialogueSeen(6, 8001); err != nil {
		t.Fatalf("mark dialogue seen: %v", err)
	}
	if err := MarkDorm3dCollection(6, 10, 7001, 100); err != nil {
		t.Fatalf("mark collection: %v", err)
	}
	if err := MarkDorm3dCollection(6, 10, 7001, 100); err != nil {
		t.Fatalf("mark collection second time: %v", err)
	}

	updated, err := GetDorm3dApartment(6)
	if err != nil {
		t.Fatalf("get apartment: %v", err)
	}
	if updated.Ships[0].Name != "Commander" || updated.Ships[0].NameCd != 55 {
		t.Fatalf("expected call name persisted")
	}
	if len(updated.Ships[0].Dialogues) != 1 || updated.Ships[0].Dialogues[0] != 8001 {
		t.Fatalf("expected dialogue persisted")
	}
	if len(updated.Ships[0].HiddenInfo) != 1 || len(updated.Ships[0].HiddenInfo[0].HiddenParts) != 2 {
		t.Fatalf("expected hidden parts persisted")
	}
	if len(updated.Rooms[0].Collections) != 1 || updated.Rooms[0].Collections[0] != 7001 {
		t.Fatalf("expected collection persisted once")
	}
}

func TestDorm3dApartmentOpValidationErrors(t *testing.T) {
	initCommanderItemTestDB(t)
	clearTable(t, &Dorm3dApartment{})
	clearTable(t, &ConfigEntry{})

	apartment := NewDorm3dApartment(7)
	apartment.Ships = Dorm3dShipList{{ShipGroup: 100, Skins: []uint32{2001}}}
	apartment.Rooms = Dorm3dRoomList{{ID: 10, Collections: []uint32{}}}
	if err := CreateDorm3dApartment(&apartment); err != nil {
		t.Fatalf("create apartment: %v", err)
	}
	if err := UpsertConfigEntry(dorm3dRoomsCategory, "10", json.RawMessage(`{"id":10,"type":2,"character":[100]}`)); err != nil {
		t.Fatalf("seed room config: %v", err)
	}

	if err := SetDorm3dCallName(7, 0, "", 0, 0); !errors.Is(err, ErrDorm3dInvalidCallName) {
		t.Fatalf("expected invalid call name error, got %v", err)
	}
	if err := SetDorm3dCallName(7, 999, "Commander", 0, 100); !errors.Is(err, ErrDorm3dShipNotFound) {
		t.Fatalf("expected ship not found error, got %v", err)
	}
	if err := SetDorm3dCallName(7, 100, "Commander", 10, 100); err != nil {
		t.Fatalf("expected initial call name set to succeed, got %v", err)
	}
	if err := SetDorm3dCallName(7, 100, "Commander2", 50, 150); !errors.Is(err, ErrDorm3dInvalidCallName) {
		t.Fatalf("expected cooldown validation failure, got %v", err)
	}
	if err := ChangeDorm3dShipSkin(7, 100, 9999); !errors.Is(err, ErrDorm3dSkinNotAvailable) {
		t.Fatalf("expected skin unavailable error, got %v", err)
	}
	if err := UpdateDorm3dSkinHiddenParts(7, 100, 0, nil); !errors.Is(err, ErrDorm3dHiddenSkinInvalid) {
		t.Fatalf("expected invalid hidden skin error, got %v", err)
	}
	if err := MarkDorm3dDialogueSeen(7, 9999); !errors.Is(err, ErrDorm3dDialogueInvalid) {
		t.Fatalf("expected dialogue invalid error, got %v", err)
	}
	if err := MarkDorm3dCollection(7, 10, 9999, 100); !errors.Is(err, ErrDorm3dCollectionInvalid) {
		t.Fatalf("expected collection invalid error, got %v", err)
	}
}
