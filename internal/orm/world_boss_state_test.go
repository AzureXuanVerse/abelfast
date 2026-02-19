package orm

import "testing"

func TestWorldBossStateRoundTrip(t *testing.T) {
	state, err := GetOrCreateCommanderWorldBossState(31001)
	if err != nil {
		t.Fatalf("get or create failed: %v", err)
	}
	state.DefaultBossID = 99
	state.SelfBoss = &WorldBossBossState{ID: 99, TemplateID: 1234, Lv: 1, Hp: 5000, Owner: 31001, LastTime: 9999}
	state.SetRankings(99, []WorldBossRankEntry{{CommanderID: 1, Name: "a", Damage: 2}})
	state.SetRewardClaimed(99, true)

	if err := SaveCommanderWorldBossState(state); err != nil {
		t.Fatalf("save failed: %v", err)
	}

	loaded, err := GetCommanderWorldBossState(31001)
	if err != nil {
		t.Fatalf("reload failed: %v", err)
	}
	if loaded.DefaultBossID != 99 {
		t.Fatalf("expected default boss id 99, got %d", loaded.DefaultBossID)
	}
	if loaded.SelfBoss == nil || loaded.SelfBoss.ID != 99 {
		t.Fatalf("expected persisted self boss")
	}
	ranks := loaded.GetRankings(99)
	if len(ranks) != 1 || ranks[0].Damage != 2 {
		t.Fatalf("expected persisted rankings")
	}
	if !loaded.IsRewardClaimed(99) {
		t.Fatalf("expected reward claimed flag")
	}
}
