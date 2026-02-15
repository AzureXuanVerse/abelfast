package orm

import (
	"encoding/json"
	"testing"
	"time"
)

func TestListCommanderActiveBuffs(t *testing.T) {
	initRandomFlagShipTestDB(t)
	commanderID := uint32(1010)
	otherCommanderID := uint32(2020)
	now := time.Date(2026, 1, 22, 12, 0, 0, 0, time.UTC)

	clearTable(t, &CommanderBuff{})

	entries := []CommanderBuff{
		{CommanderID: commanderID, BuffID: 10, ExpiresAt: now.Add(-time.Hour)},
		{CommanderID: commanderID, BuffID: 11, ExpiresAt: now.Add(time.Hour)},
		{CommanderID: otherCommanderID, BuffID: 12, ExpiresAt: now.Add(time.Hour)},
	}
	for i := range entries {
		if err := UpsertCommanderBuff(entries[i].CommanderID, entries[i].BuffID, entries[i].ExpiresAt); err != nil {
			t.Fatalf("create commander buffs: %v", err)
		}
	}

	active, err := ListCommanderActiveBuffs(commanderID, now)
	if err != nil {
		t.Fatalf("list commander buffs: %v", err)
	}
	if len(active) != 1 {
		t.Fatalf("expected 1 active buff, got %d", len(active))
	}
	if active[0].BuffID != 11 {
		t.Fatalf("expected buff id 11, got %d", active[0].BuffID)
	}
}

func TestGetCommanderSkillLearnTimeAllowance(t *testing.T) {
	initRandomFlagShipTestDB(t)
	clearTable(t, &CommanderBuff{})
	clearTable(t, &ConfigEntry{})

	now := time.Date(2026, 2, 15, 12, 0, 0, 0, time.UTC)
	commanderID := uint32(3030)

	if err := UpsertConfigEntry(benefitBuffCategory, "11", json.RawMessage(`{"id":11,"benefit_type":"skill_learn_time","benefit_effect":"2"}`)); err != nil {
		t.Fatalf("seed buff config 11: %v", err)
	}
	if err := UpsertConfigEntry(benefitBuffCategory, "12", json.RawMessage(`{"id":12,"benefit_type":"skill_learn_time","benefit_effect":"3"}`)); err != nil {
		t.Fatalf("seed buff config 12: %v", err)
	}
	if err := UpsertConfigEntry(benefitBuffCategory, "13", json.RawMessage(`{"id":13,"benefit_type":"other","benefit_effect":"99"}`)); err != nil {
		t.Fatalf("seed buff config 13: %v", err)
	}

	if err := UpsertCommanderBuff(commanderID, 11, now.Add(2*time.Hour)); err != nil {
		t.Fatalf("seed active commander buff 11: %v", err)
	}
	if err := UpsertCommanderBuff(commanderID, 12, now.Add(2*time.Hour)); err != nil {
		t.Fatalf("seed active commander buff 12: %v", err)
	}
	if err := UpsertCommanderBuff(commanderID, 13, now.Add(2*time.Hour)); err != nil {
		t.Fatalf("seed active commander buff 13: %v", err)
	}
	if err := UpsertCommanderBuff(commanderID, 14, now.Add(-2*time.Hour)); err != nil {
		t.Fatalf("seed expired commander buff 14: %v", err)
	}

	allowance, err := GetCommanderSkillLearnTimeAllowance(commanderID, now)
	if err != nil {
		t.Fatalf("get allowance: %v", err)
	}
	if allowance != 3 {
		t.Fatalf("expected allowance 3, got %d", allowance)
	}
}
