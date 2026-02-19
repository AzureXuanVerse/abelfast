package orm

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5"
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

func TestDorm3dRoomHelpers(t *testing.T) {
	apartment := NewDorm3dApartment(9)
	if !apartment.AddRoom(Dorm3dRoom{ID: 4}) {
		t.Fatalf("expected first add room to succeed")
	}
	if apartment.AddRoom(Dorm3dRoom{ID: 4}) {
		t.Fatalf("expected duplicate add room to fail")
	}
	if apartment.RoomByID(4) == nil {
		t.Fatalf("expected room lookup by id to succeed")
	}
}

func TestSaveDorm3dApartmentTx(t *testing.T) {
	initCommanderItemTestDB(t)
	clearTable(t, &Dorm3dApartment{})

	apartment := NewDorm3dApartment(33)
	apartment.DailyVigorMax = 42
	if err := WithPGXTx(context.Background(), func(tx pgx.Tx) error {
		return SaveDorm3dApartmentTx(context.Background(), tx, &apartment)
	}); err != nil {
		t.Fatalf("save dorm3d apartment tx: %v", err)
	}
	stored, err := GetDorm3dApartment(33)
	if err != nil {
		t.Fatalf("get dorm3d apartment: %v", err)
	}
	if stored.DailyVigorMax != 42 {
		t.Fatalf("expected daily vigor max 42, got %d", stored.DailyVigorMax)
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

func TestDorm3dInsMutations(t *testing.T) {
	initCommanderItemTestDB(t)
	clearTable(t, &Dorm3dApartment{})
	clearTable(t, &ConfigEntry{})

	apartment := NewDorm3dApartment(3)
	apartment.Ships = Dorm3dShipList{{ShipGroup: 100, Skins: []uint32{2001}, CurSkin: 2001}}
	apartment.Ins = Dorm3dInsList{{ShipGroup: 100, CommList: []Dorm3dCommInfo{{ID: 3001, ReplyList: []Dorm3dKeyValue{}}}}}
	if err := CreateDorm3dApartment(&apartment); err != nil {
		t.Fatalf("create apartment: %v", err)
	}
	if err := UpsertConfigEntry(dorm3dInsChatGroupCategory, "3001", json.RawMessage(`{"id":3001,"ship_group":100}`)); err != nil {
		t.Fatalf("seed chat group config: %v", err)
	}

	if err := UpdateDorm3dInsBackground(3, 100, 2001); err != nil {
		t.Fatalf("update background: %v", err)
	}
	if err := UpdateDorm3dInsCareFlag(3, 100, 1); err != nil {
		t.Fatalf("update care: %v", err)
	}
	if err := SetDorm3dCurrentCommID(3, 100, 3001); err != nil {
		t.Fatalf("set current comm: %v", err)
	}
	if err := UpdateDorm3dVisitTime(3, 100, 1234); err != nil {
		t.Fatalf("update visit time: %v", err)
	}

	updated, err := GetDorm3dApartment(3)
	if err != nil {
		t.Fatalf("get apartment: %v", err)
	}
	if updated.Ins[0].CurBack != 2001 || updated.Ins[0].CareFlag != 1 || updated.Ins[0].CurCommId != 3001 {
		t.Fatalf("unexpected ins values: %+v", updated.Ins[0])
	}
	if updated.Ships[0].VisitTime != 1234 {
		t.Fatalf("expected visit time 1234, got %d", updated.Ships[0].VisitTime)
	}
}

func TestDorm3dInsMutationValidationFailures(t *testing.T) {
	initCommanderItemTestDB(t)
	clearTable(t, &Dorm3dApartment{})
	clearTable(t, &ConfigEntry{})

	apartment := NewDorm3dApartment(4)
	apartment.Ships = Dorm3dShipList{{ShipGroup: 100, Skins: []uint32{2001}, CurSkin: 2001}}
	apartment.Ins = Dorm3dInsList{{ShipGroup: 100, CommList: []Dorm3dCommInfo{{ID: 3001, ReplyList: []Dorm3dKeyValue{}}}}}
	if err := CreateDorm3dApartment(&apartment); err != nil {
		t.Fatalf("create apartment: %v", err)
	}

	if err := UpdateDorm3dInsBackground(4, 100, 9999); !errors.Is(err, ErrDorm3dInvalidBackground) {
		t.Fatalf("expected invalid background error, got %v", err)
	}
	if err := SetDorm3dCurrentCommID(4, 100, 5555); !errors.Is(err, ErrDorm3dCommNotFound) {
		t.Fatalf("expected missing comm error, got %v", err)
	}
	if err := UpdateDorm3dVisitTime(4, 999, 1); !errors.Is(err, ErrDorm3dShipNotFound) {
		t.Fatalf("expected missing ship error, got %v", err)
	}
}

func TestApplyDorm3dTriggerEventsUnlockAndDedupe(t *testing.T) {
	initCommanderItemTestDB(t)
	clearTable(t, &Dorm3dApartment{})
	clearTable(t, &ConfigEntry{})

	apartment := NewDorm3dApartment(5)
	apartment.Ships = Dorm3dShipList{{ShipGroup: 100}}
	if err := CreateDorm3dApartment(&apartment); err != nil {
		t.Fatalf("create apartment: %v", err)
	}
	if err := UpsertConfigEntry(dorm3dInsUnlockCategory, "1", json.RawMessage(`{"id":1,"type":1,"content":4001,"trigger_type":152,"trigger_num":2}`)); err != nil {
		t.Fatalf("seed unlock config: %v", err)
	}
	if err := UpsertConfigEntry(dorm3dInsChatGroupCategory, "4001", json.RawMessage(`{"id":4001,"ship_group":100}`)); err != nil {
		t.Fatalf("seed chat group config: %v", err)
	}

	unlocks, err := ApplyDorm3dTriggerEvents(5, []Dorm3dEventInfo{{EventType: 152, Value: 2, ShipGroup: 100}}, 88)
	if err != nil {
		t.Fatalf("apply trigger events: %v", err)
	}
	if len(unlocks) != 1 || unlocks[0].ActID != 4001 || unlocks[0].Type != 1 {
		t.Fatalf("unexpected unlocks: %+v", unlocks)
	}
	unlocks, err = ApplyDorm3dTriggerEvents(5, []Dorm3dEventInfo{{EventType: 152, Value: 99, ShipGroup: 999}}, 90)
	if err != nil {
		t.Fatalf("apply trigger events unknown ship: %v", err)
	}
	if len(unlocks) != 0 {
		t.Fatalf("expected unknown ship to be ignored, got %+v", unlocks)
	}
	unlocks, err = ApplyDorm3dTriggerEvents(5, []Dorm3dEventInfo{{EventType: 152, Value: 3, ShipGroup: 100}}, 99)
	if err != nil {
		t.Fatalf("apply trigger events second pass: %v", err)
	}
	if len(unlocks) != 0 {
		t.Fatalf("expected deduped unlocks, got %+v", unlocks)
	}

	updated, err := GetDorm3dApartment(5)
	if err != nil {
		t.Fatalf("get apartment: %v", err)
	}
	if len(updated.Ins) != 1 || len(updated.Ins[0].CommList) != 1 || updated.Ins[0].CommList[0].ID != 4001 {
		t.Fatalf("unexpected unlocked state: %+v", updated.Ins)
	}
	if updated.Ins[0].ShipGroup != 100 {
		t.Fatalf("unexpected ship group in ins state: %+v", updated.Ins)
	}
}
