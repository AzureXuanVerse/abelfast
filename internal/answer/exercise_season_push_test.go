package answer

import "testing"

func TestBuildExerciseSeasonPushUpdate_UsesSeasonScoreAndRankSource(t *testing.T) {
	targets := buildExerciseRivalTargetList()
	push := buildExerciseSeasonPushUpdate(targets)

	if push.GetScore() != 0 {
		t.Fatalf("expected score 0, got %d", push.GetScore())
	}
	if push.GetRank() != 0 {
		t.Fatalf("expected rank 0, got %d", push.GetRank())
	}
	if len(push.GetTargetList()) != len(targets) {
		t.Fatalf("expected %d targets, got %d", len(targets), len(push.GetTargetList()))
	}
}

func TestBuildExerciseRivalTargetList_ContainsRequiredRivalIdentityFields(t *testing.T) {
	targets := buildExerciseRivalTargetList()
	if len(targets) != exerciseRivalCount {
		t.Fatalf("expected %d rivals, got %d", exerciseRivalCount, len(targets))
	}
	for _, target := range targets {
		if target.GetId() == 0 || target.GetLevel() == 0 || target.GetName() == "" {
			t.Fatalf("expected id, level, and name to be set for every rival")
		}
	}
}
