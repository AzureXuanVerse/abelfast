package neweducate

import (
	"testing"

	"github.com/ggmolly/belfast/internal/protobuf"
	"google.golang.org/protobuf/proto"
)

func TestChooseNewEducateTalentCandidatePrefersUnusedOption(t *testing.T) {
	selected := []uint32{11, 12, 13}
	refreshed := []uint32{14}
	available := []uint32{11, 12, 13, 15, 16}

	if candidate := chooseNewEducateTalentCandidate(selected, refreshed, available, 12); candidate != 15 {
		t.Fatalf("expected replacement talent 15, got %d", candidate)
	}

	if candidate := chooseNewEducateTalentCandidate(selected, refreshed, []uint32{11, 12, 13}, 12); candidate != 12 {
		t.Fatalf("expected fallback to original talent, got %d", candidate)
	}
}

func TestApplyNewEducateConfigDropsAdjustsResourcesAndAttrs(t *testing.T) {
	state := &educateState{
		Info:      ensureTBInfoDefaults(tbInfoPlaceholder()),
		Permanent: ensureTBPermanentDefaults(tbPermanentPlaceholder()),
	}
	state.Info.Res.Resource = []*protobuf.KVDATA{{Key: proto.Uint32(2), Value: proto.Uint32(10)}}
	state.Info.Res.Attrs = []*protobuf.KVDATA{{Key: proto.Uint32(1), Value: proto.Uint32(8)}}

	applyNewEducateConfigDrops(state, [][]int32{{2, 2, 3}, {1, 1, 5}}, 2)

	if got := state.Info.Res.GetResource()[0].GetValue(); got != 4 {
		t.Fatalf("expected resource value 4, got %d", got)
	}
	if got := state.Info.Res.GetAttrs()[0].GetValue(); got != 0 {
		t.Fatalf("expected attr value 0, got %d", got)
	}
}

func TestApplyNewEducateTalentSelectionAddsBuffStateAndDrop(t *testing.T) {
	state := &educateState{
		Info:      ensureTBInfoDefaults(tbInfoPlaceholder()),
		Permanent: ensureTBPermanentDefaults(tbPermanentPlaceholder()),
	}
	state.Info.Round.Round = proto.Uint32(3)

	drop := applyNewEducateTalentSelection(state, 42)

	if len(state.Info.Benefit.GetActives()) != 1 {
		t.Fatalf("expected one active buff, got %d", len(state.Info.Benefit.GetActives()))
	}
	if state.Info.Benefit.GetActives()[0].GetId() != 42 {
		t.Fatalf("expected active buff 42, got %d", state.Info.Benefit.GetActives()[0].GetId())
	}
	if state.Info.Benefit.GetActives()[0].GetRound() != 3 {
		t.Fatalf("expected buff round 3, got %d", state.Info.Benefit.GetActives()[0].GetRound())
	}
	if len(state.Permanent.GetTarotArchive()) != 1 || state.Permanent.GetTarotArchive()[0] != 42 {
		t.Fatalf("expected tarot archive to contain 42, got %v", state.Permanent.GetTarotArchive())
	}
	if len(drop.GetBaseDrop()) != 1 || drop.GetBaseDrop()[0].GetType() != 4 || drop.GetBaseDrop()[0].GetId() != 42 || drop.GetBaseDrop()[0].GetNumber() != 1 {
		t.Fatalf("unexpected talent drop: %+v", drop.GetBaseDrop())
	}
	if drop.GetDisplay() == nil {
		t.Fatalf("expected drop display to be populated")
	}
}

func TestAdvanceNewEducateRoundHandlesTempRoundsAndMaxRound(t *testing.T) {
	state := &educateState{
		Info:      ensureTBInfoDefaults(tbInfoPlaceholder()),
		Permanent: ensureTBPermanentDefaults(tbPermanentPlaceholder()),
	}
	state.Info.Round.Round = proto.Uint32(5)
	state.Info.Round.TempRound = proto.Uint32(2)
	state.Info.Fsm.SystemNo = proto.Uint32(newEducateSystemMap)
	state.Info.Site.Characters = []uint32{10, 11}
	state.Info.EvalFail = proto.Uint32(1)
	state.Permanent.MaxRound = proto.Uint32(4)

	advanceNewEducateRound(state)

	if state.Info.Round.GetRound() != 5 {
		t.Fatalf("expected temp round to keep round 5, got %d", state.Info.Round.GetRound())
	}
	if state.Info.Round.GetInTemp() != 1 || state.Info.Round.GetTempRound() != 1 {
		t.Fatalf("expected in_temp=1 temp_round=1, got in_temp=%d temp_round=%d", state.Info.Round.GetInTemp(), state.Info.Round.GetTempRound())
	}
	if state.Permanent.GetMaxRound() != 5 {
		t.Fatalf("expected max round 5, got %d", state.Permanent.GetMaxRound())
	}
	if state.Info.GetEvalFail() != 0 {
		t.Fatalf("expected eval_fail reset")
	}
	if state.Info.Fsm.GetSystemNo() != 0 || state.Info.Fsm.GetCurrentNode() != 0 {
		t.Fatalf("expected FSM reset, got system=%d node=%d", state.Info.Fsm.GetSystemNo(), state.Info.Fsm.GetCurrentNode())
	}
	if len(state.Info.Site.GetCharacters()) != 0 {
		t.Fatalf("expected site characters cleared, got %v", state.Info.Site.GetCharacters())
	}

	advanceNewEducateRound(state)

	if state.Info.Round.GetRound() != 5 || state.Info.Round.GetInTemp() != 1 || state.Info.Round.GetTempRound() != 0 {
		t.Fatalf("expected second temp round consumption, got round=%d in_temp=%d temp_round=%d", state.Info.Round.GetRound(), state.Info.Round.GetInTemp(), state.Info.Round.GetTempRound())
	}

	advanceNewEducateRound(state)

	if state.Info.Round.GetRound() != 6 || state.Info.Round.GetInTemp() != 0 {
		t.Fatalf("expected normal round advance to 6, got round=%d in_temp=%d", state.Info.Round.GetRound(), state.Info.Round.GetInTemp())
	}
	if state.Permanent.GetMaxRound() != 6 {
		t.Fatalf("expected max round 6, got %d", state.Permanent.GetMaxRound())
	}
}

func TestNewEducateGetEndingsUsesActivatedEndings(t *testing.T) {
	state := &educateState{
		Info:      ensureTBInfoDefaults(tbInfoPlaceholder()),
		Permanent: ensureTBPermanentDefaults(&protobuf.TBPERMANENT{Endings: []uint32{7, 8}, ActiveEndings: []uint32{9}}),
	}

	endings := append([]uint32{}, state.Permanent.Endings...)
	if len(endings) != 2 || endings[0] != 7 || endings[1] != 8 {
		t.Fatalf("expected activated endings [7 8], got %v", endings)
	}
}
