package orm

import (
	"encoding/json"
	"testing"

	"github.com/ggmolly/belfast/internal/rng"
)

func TestRollCommanderTemplateForPoolRandomizesMatchingRarity(t *testing.T) {
	initCommanderItemTestDB(t)
	clearTable(t, &ConfigEntry{})

	seed := func(key string, payload string) {
		t.Helper()
		if err := UpsertConfigEntry(commanderDataTemplateCategory, key, json.RawMessage(payload)); err != nil {
			t.Fatalf("seed template %s: %v", key, err)
		}
	}
	seed("10011", `{"id":10011,"rarity":5,"group_type":1,"exp":100,"exp_cost":10}`)
	seed("10012", `{"id":10012,"rarity":5,"group_type":2,"exp":100,"exp_cost":10}`)
	seed("10021", `{"id":10021,"rarity":4,"group_type":3,"exp":100,"exp_cost":10}`)

	originalRng := commanderMeowRng
	commanderMeowRng = rng.NewLockedRandFromSeed(1)
	t.Cleanup(func() {
		commanderMeowRng = originalRng
	})

	seen := map[uint32]struct{}{}
	for i := 0; i < 32; i++ {
		templateID, err := RollCommanderTemplateForPool(1)
		if err != nil {
			t.Fatalf("roll template: %v", err)
		}
		seen[templateID] = struct{}{}
	}

	if len(seen) < 2 {
		t.Fatalf("expected multiple matching templates to be selected, got %+v", seen)
	}
}
