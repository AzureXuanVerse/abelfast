package orm

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/ggmolly/belfast/internal/db"
)

func TestGetPlayerSummaryStatsDefaults(t *testing.T) {
	initCommanderItemTestDB(t)
	clearTable(t, &OwnedSkin{})
	clearTable(t, &CommanderMedalDisplay{})
	clearTable(t, &CommanderFurniture{})
	clearTable(t, &ChapterProgress{})
	clearTable(t, &OwnedShip{})
	clearTable(t, &Ship{})
	clearTable(t, &Commander{})
	clearTable(t, &Account{})

	if err := CreateCommanderRoot(9001, 9001, "Summary Default", 0, 0); err != nil {
		t.Fatalf("create commander root: %v", err)
	}
	registerAt := time.Unix(1700000100, 0).UTC()
	commanderID := uint32(9001)
	if err := CreateAccount(&Account{
		ID:                fmt.Sprintf("summary-%d", commanderID),
		CommanderID:       &commanderID,
		PasswordHash:      "hash",
		PasswordAlgo:      "argon2id",
		PasswordUpdatedAt: registerAt,
		CreatedAt:         registerAt,
		UpdatedAt:         registerAt,
	}); err != nil {
		t.Fatalf("create account: %v", err)
	}
	if _, err := db.DefaultStore.Pool.Exec(context.Background(), `UPDATE commanders SET last_login = $2 WHERE commander_id = $1`, int64(9001), registerAt.Add(48*time.Hour)); err != nil {
		t.Fatalf("update commander last_login: %v", err)
	}

	stats, err := GetPlayerSummaryStats(9001)
	if err != nil {
		t.Fatalf("get player summary stats: %v", err)
	}
	if stats.ChapterID != 101 {
		t.Fatalf("expected chapter fallback 101, got %d", stats.ChapterID)
	}
	if stats.CharacterID != 100001 {
		t.Fatalf("expected default character 100001, got %d", stats.CharacterID)
	}
	if stats.FirstLadyID != 0 || stats.FirstLadyName != "" || stats.FirstLadyTime != 0 {
		t.Fatalf("expected empty first lady fields")
	}
	if stats.RegisterDate == 0 || stats.FirstOnline == 0 {
		t.Fatalf("expected non-zero register/first online")
	}
	if stats.RegisterDate != uint32(registerAt.Unix()) {
		t.Fatalf("expected register date from immutable account creation time, got %d", stats.RegisterDate)
	}
	if stats.FirstOnline != stats.RegisterDate {
		t.Fatalf("expected first online to match register date")
	}
}

func TestGetPlayerSummaryStatsAggregates(t *testing.T) {
	initCommanderItemTestDB(t)
	clearTable(t, &OwnedSkin{})
	clearTable(t, &CommanderMedalDisplay{})
	clearTable(t, &CommanderFurniture{})
	clearTable(t, &ChapterProgress{})
	clearTable(t, &OwnedShip{})
	clearTable(t, &Ship{})
	clearTable(t, &Commander{})

	if err := CreateCommanderRoot(9002, 9002, "Summary Aggregate", 0, 0); err != nil {
		t.Fatalf("create commander root: %v", err)
	}

	shipA := Ship{TemplateID: 10011, Name: "Ship A", EnglishName: "Ship A", RarityID: 2, Star: 1, Type: 1, Nationality: 1, BuildTime: 1}
	shipB := Ship{TemplateID: 10021, Name: "Ship B", EnglishName: "Ship B", RarityID: 2, Star: 1, Type: 1, Nationality: 1, BuildTime: 1}
	if err := shipA.Create(); err != nil {
		t.Fatalf("create ship A: %v", err)
	}
	if err := shipB.Create(); err != nil {
		t.Fatalf("create ship B: %v", err)
	}

	ownedA := OwnedShip{OwnerID: 9002, ShipID: 10011}
	ownedB := OwnedShip{OwnerID: 9002, ShipID: 10021}
	if err := ownedA.Create(); err != nil {
		t.Fatalf("create owned ship A: %v", err)
	}
	if err := ownedB.Create(); err != nil {
		t.Fatalf("create owned ship B: %v", err)
	}

	firstProposeAt := time.Unix(1700000000, 0).UTC()
	if _, err := db.DefaultStore.Pool.Exec(context.Background(), `
UPDATE owned_ships
SET propose = TRUE,
	max_level = 125,
	intimacy = 25000,
	custom_name = 'First Name',
	create_time = $3
WHERE owner_id = $1
	AND id = $2
`, int64(9002), int64(ownedA.ID), firstProposeAt); err != nil {
		t.Fatalf("update owned ship A: %v", err)
	}
	if _, err := db.DefaultStore.Pool.Exec(context.Background(), `
UPDATE owned_ships
SET max_level = 120,
	is_secretary = TRUE,
	secretary_position = 0
WHERE owner_id = $1
	AND id = $2
`, int64(9002), int64(ownedB.ID)); err != nil {
		t.Fatalf("update owned ship B: %v", err)
	}

	if err := UpsertChapterProgress(&ChapterProgress{CommanderID: 9002, ChapterID: 110}); err != nil {
		t.Fatalf("upsert chapter progress: %v", err)
	}
	if _, err := db.DefaultStore.Pool.Exec(context.Background(), `INSERT INTO commander_furnitures (commander_id, furniture_id, count, get_time) VALUES ($1, $2, $3, $4)`, int64(9002), int64(1), int64(4), int64(1)); err != nil {
		t.Fatalf("insert commander furniture: %v", err)
	}
	if err := SetCommanderMedalDisplay(9002, []uint32{1, 2, 3}); err != nil {
		t.Fatalf("set medal display: %v", err)
	}
	if _, err := db.DefaultStore.Pool.Exec(context.Background(), `INSERT INTO skins (id, name, ship_group) VALUES ($1, $2, $3)`, int64(50011), "Skin A", int64(1001)); err != nil {
		t.Fatalf("insert skin A: %v", err)
	}
	if _, err := db.DefaultStore.Pool.Exec(context.Background(), `INSERT INTO owned_skins (commander_id, skin_id, expires_at) VALUES ($1, $2, NULL)`, int64(9002), int64(50011)); err != nil {
		t.Fatalf("insert owned skin: %v", err)
	}

	stats, err := GetPlayerSummaryStats(9002)
	if err != nil {
		t.Fatalf("get player summary stats: %v", err)
	}
	if stats.ChapterID != 110 {
		t.Fatalf("expected chapter 110, got %d", stats.ChapterID)
	}
	if stats.MarryNumber != 1 {
		t.Fatalf("expected marry number 1, got %d", stats.MarryNumber)
	}
	if stats.MedalNumber != 3 {
		t.Fatalf("expected medal number 3, got %d", stats.MedalNumber)
	}
	if stats.FurnitureNumber != 4 {
		t.Fatalf("expected furniture number 4, got %d", stats.FurnitureNumber)
	}
	if stats.CharacterID != 10021 {
		t.Fatalf("expected character id 10021, got %d", stats.CharacterID)
	}
	if stats.FirstLadyID != 10011 || stats.FirstLadyName != "First Name" || stats.FirstLadyTime != uint32(firstProposeAt.Unix()) {
		t.Fatalf("unexpected first lady stats: id=%d name=%q time=%d", stats.FirstLadyID, stats.FirstLadyName, stats.FirstLadyTime)
	}
	if stats.ShipNumTotal != 2 || stats.ShipNum120 != 2 || stats.ShipNum125 != 1 {
		t.Fatalf("unexpected ship summary counts: total=%d lvl120=%d lvl125=%d", stats.ShipNumTotal, stats.ShipNum120, stats.ShipNum125)
	}
	if stats.Love200Num != 1 {
		t.Fatalf("expected love200 count 1, got %d", stats.Love200Num)
	}
	if stats.CollectNum != 2 {
		t.Fatalf("expected collect num 2, got %d", stats.CollectNum)
	}
	if stats.SkinNum != 1 || stats.SkinShipNum != 1 {
		t.Fatalf("unexpected skin summary counts: skin_num=%d skin_ship_num=%d", stats.SkinNum, stats.SkinShipNum)
	}
}
