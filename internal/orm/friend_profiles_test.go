package orm

import (
	"context"
	"testing"
	"time"

	"github.com/ggmolly/belfast/internal/db"
)

func TestCommanderSocialProfilesAndFriendRelations(t *testing.T) {
	initCommanderItemTestDB(t)
	clearTable(t, &CommanderFriendRelation{})
	clearTable(t, &Commander{})

	seedCommander := func(id uint32, name string, deleted bool) {
		t.Helper()
		if _, err := db.DefaultStore.Pool.Exec(context.Background(), `
INSERT INTO commanders (commander_id, account_id, level, exp, name, last_login, guide_index, new_guide_index, name_change_cooldown, room_id, exchange_count, draw_count1, draw_count10, support_requisition_count, support_requisition_month, collect_attack_count, acc_pay_lv, living_area_cover_id, selected_icon_frame_id, selected_chat_frame_id, selected_battle_ui_id, display_icon_id, display_skin_id, display_icon_theme_id, manifesto, dorm_name, random_ship_mode, random_flag_ship_enabled, deleted_at)
VALUES ($1, $2, 10, 0, $3, $4, 0, 0, '1970-01-01 00:00:00+00', 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, '', '', 0, false, $5)
`, int64(id), int64(id), name, time.Now().UTC(), nullableDeletedAt(deleted)); err != nil {
			t.Fatalf("seed commander: %v", err)
		}
	}

	seedCommander(7001, "FriendA", false)
	seedCommander(7002, "FriendB", false)
	seedCommander(7003, "FriendDeleted", true)

	profile, err := GetCommanderSocialProfileByName("FriendA")
	if err != nil {
		t.Fatalf("profile by name: %v", err)
	}
	if profile.CommanderID != 7001 {
		t.Fatalf("expected commander 7001, got %d", profile.CommanderID)
	}

	if _, err := GetCommanderSocialProfileByName("FriendDeleted"); err == nil {
		t.Fatalf("expected deleted commander to be filtered")
	}

	if err := CreateCommanderFriendRelationPair(7001, 7002); err != nil {
		t.Fatalf("create relation pair: %v", err)
	}

	friendsOfA, err := ListCommanderFriendIDs(7001)
	if err != nil {
		t.Fatalf("list friends A: %v", err)
	}
	if len(friendsOfA) != 1 || friendsOfA[0] != 7002 {
		t.Fatalf("unexpected friends for A: %+v", friendsOfA)
	}

	removed, err := DeleteCommanderFriendRelationPair(7001, 7002)
	if err != nil {
		t.Fatalf("delete relation pair: %v", err)
	}
	if !removed {
		t.Fatalf("expected relation to be removed")
	}

	friendsOfA, err = ListCommanderFriendIDs(7001)
	if err != nil {
		t.Fatalf("list friends A after delete: %v", err)
	}
	if len(friendsOfA) != 0 {
		t.Fatalf("expected no friends for A after delete, got %+v", friendsOfA)
	}
}

func TestGetCommanderSocialProfilesByIDsFiltersMissingAndDeleted(t *testing.T) {
	initCommanderItemTestDB(t)
	clearTable(t, &Commander{})

	seedCommander := func(id uint32, name string, deleted bool) {
		t.Helper()
		if _, err := db.DefaultStore.Pool.Exec(context.Background(), `
INSERT INTO commanders (commander_id, account_id, level, exp, name, last_login, guide_index, new_guide_index, name_change_cooldown, room_id, exchange_count, draw_count1, draw_count10, support_requisition_count, support_requisition_month, collect_attack_count, acc_pay_lv, living_area_cover_id, selected_icon_frame_id, selected_chat_frame_id, selected_battle_ui_id, display_icon_id, display_skin_id, display_icon_theme_id, manifesto, dorm_name, random_ship_mode, random_flag_ship_enabled, deleted_at)
VALUES ($1, $2, 10, 0, $3, $4, 0, 0, '1970-01-01 00:00:00+00', 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, '', '', 0, false, $5)
`, int64(id), int64(id), name, time.Now().UTC(), nullableDeletedAt(deleted)); err != nil {
			t.Fatalf("seed commander: %v", err)
		}
	}

	seedCommander(7101, "BatchA", false)
	seedCommander(7102, "BatchB", false)
	seedCommander(7103, "BatchDeleted", true)

	profiles, err := GetCommanderSocialProfilesByIDs([]uint32{7102, 999999, 7101, 7102, 7103})
	if err != nil {
		t.Fatalf("get social profiles by ids: %v", err)
	}
	if len(profiles) != 2 {
		t.Fatalf("expected 2 existing profiles, got %d", len(profiles))
	}
	if _, ok := profiles[7101]; !ok {
		t.Fatalf("expected profile for 7101")
	}
	if _, ok := profiles[7102]; !ok {
		t.Fatalf("expected profile for 7102")
	}
	if _, ok := profiles[7103]; ok {
		t.Fatalf("did not expect deleted profile 7103")
	}
	if _, ok := profiles[999999]; ok {
		t.Fatalf("did not expect missing profile 999999")
	}
}

func TestDeleteCommanderFriendRelationPairPreservesUnrelatedRelations(t *testing.T) {
	initCommanderItemTestDB(t)
	clearTable(t, &CommanderFriendRelation{})
	clearTable(t, &Commander{})

	seedCommander := func(id uint32, name string) {
		t.Helper()
		if _, err := db.DefaultStore.Pool.Exec(context.Background(), `
INSERT INTO commanders (commander_id, account_id, level, exp, name, last_login, guide_index, new_guide_index, name_change_cooldown, room_id, exchange_count, draw_count1, draw_count10, support_requisition_count, support_requisition_month, collect_attack_count, acc_pay_lv, living_area_cover_id, selected_icon_frame_id, selected_chat_frame_id, selected_battle_ui_id, display_icon_id, display_skin_id, display_icon_theme_id, manifesto, dorm_name, random_ship_mode, random_flag_ship_enabled, deleted_at)
VALUES ($1, $2, 10, 0, $3, $4, 0, 0, '1970-01-01 00:00:00+00', 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, '', '', 0, false, NULL)
`, int64(id), int64(id), name, time.Now().UTC()); err != nil {
			t.Fatalf("seed commander: %v", err)
		}
	}

	seedCommander(8001, "RelationA")
	seedCommander(8002, "RelationB")
	seedCommander(8003, "RelationC")

	if err := CreateCommanderFriendRelationPair(8001, 8002); err != nil {
		t.Fatalf("create first relation pair: %v", err)
	}
	if err := CreateCommanderFriendRelationPair(8001, 8003); err != nil {
		t.Fatalf("create second relation pair: %v", err)
	}

	removed, err := DeleteCommanderFriendRelationPair(8001, 8002)
	if err != nil {
		t.Fatalf("delete relation pair: %v", err)
	}
	if !removed {
		t.Fatalf("expected relation 8001<->8002 to be removed")
	}

	friendsOfFirst, err := ListCommanderFriendIDs(8001)
	if err != nil {
		t.Fatalf("list friends for 8001: %v", err)
	}
	if len(friendsOfFirst) != 1 || friendsOfFirst[0] != 8003 {
		t.Fatalf("expected only remaining relation to 8003, got %+v", friendsOfFirst)
	}

	friendsOfSecond, err := ListCommanderFriendIDs(8002)
	if err != nil {
		t.Fatalf("list friends for 8002: %v", err)
	}
	if len(friendsOfSecond) != 0 {
		t.Fatalf("expected 8002 to have no remaining relations, got %+v", friendsOfSecond)
	}

	friendsOfThird, err := ListCommanderFriendIDs(8003)
	if err != nil {
		t.Fatalf("list friends for 8003: %v", err)
	}
	if len(friendsOfThird) != 1 || friendsOfThird[0] != 8001 {
		t.Fatalf("expected mirrored relation 8003->8001 to remain, got %+v", friendsOfThird)
	}
}

func nullableDeletedAt(deleted bool) *time.Time {
	if !deleted {
		return nil
	}
	v := time.Now().UTC()
	return &v
}
