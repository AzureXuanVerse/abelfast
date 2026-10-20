package game

import (
	"context"
	"slices"

	"google.golang.org/protobuf/proto"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/db"
	"github.com/ggmolly/belfast/internal/gamedata"
	"github.com/ggmolly/belfast/internal/protobuf"
)

const (
	// medalTargetComingSoon is the target_type of the tiers the client shows
	// as "Coming Soon" (hide 3), never won.
	medalTargetComingSoon = 9999
	// maxDisplayMedals is how many medals a profile shows
	// (AttireAchievementPanel, "/5").
	maxDisplayMedals = 5
	// loveLetterMedalBase starts the love letter medal ids
	// (LoveLetter:GetTrophyList: base + group*100 + level, every 10 levels
	// from 1; Trophy.IsLoveLetterID).
	loveLetterMedalBase = 1000000000
	// medalAlwaysHidden is the hide of the sub-medals (Trophy.ALWAYS_HIDE),
	// never listed for display.
	medalAlwaysHidden = 1
)

// trophyMetrics maps every medal_template tier to the medal whose stored
// progress it shows: a medal's tiers count what its first tier counts (the
// next tier of the Renovation Merit Medal is "a total of" the same decor
// tokens, the archive medals the same ship count), and the sub-medals of a
// medal's later tiers what the sub-medal at the same place in its first
// tier counts (100612 is 100611's ships at level 120, with a higher
// target_num).
var trophyMetrics map[uint32]uint32

// loadTrophyChains builds trophyMetrics from medalTemplates.
func loadTrophyChains() {
	trophyMetrics = map[uint32]uint32{}
	for id, medal := range medalTemplates {
		if medal.Start != 0 {
			continue
		}
		firstSubs := trophySubMedals(id)
		for tier := id; tier != 0; tier = medalTemplates[tier].Next {
			if _, ok := medalTemplates[tier]; !ok || trophyMetrics[tier] != 0 {
				break
			}
			trophyMetrics[tier] = id
			for i, sub := range trophySubMedals(tier) {
				if i < len(firstSubs) {
					trophyMetrics[sub] = firstSubs[i]
				}
			}
		}
	}
}

// trophySubMedals are the sub-medals a complex medal (target_type 160,
// Trophy:isComplexTrophy) is won with, nil for the others.
func trophySubMedals(id uint32) []uint32 {
	medal := medalTemplates[id]
	if medal.TargetType != medalTargetSubMedals {
		return nil
	}
	return gamedata.IDs(medal.TargetID)
}

// trophyState is what the commander has of the medals: progress stored by
// metric (commander_trophies) and the tiers claimed, with their time.
type trophyState struct {
	progress map[uint32]uint32
	claimed  map[uint32]int64
}

func loadTrophyState(ctx context.Context, queries *db.Queries, commanderID uint32) (trophyState, error) {
	state := trophyState{progress: map[uint32]uint32{}, claimed: map[uint32]int64{}}
	trophies, err := queries.ListTrophies(ctx, commanderID)
	if err != nil {
		return state, err
	}
	for _, trophy := range trophies {
		state.progress[trophy.TrophyID] = trophy.Progress
	}
	claims, err := queries.ListTrophyClaims(ctx, commanderID)
	if err != nil {
		return state, err
	}
	for _, claim := range claims {
		state.claimed[claim.TrophyID] = claim.ClaimedAt
	}
	return state, nil
}

// info is a medal tier's ACHIEVEMENT_INFO: its metric's progress and the
// time it was claimed (0 when it wasn't).
func (state trophyState) info(id uint32) *protobuf.ACHIEVEMENT_INFO {
	metric := trophyMetrics[id]
	if metric == 0 {
		metric = id
	}
	return &protobuf.ACHIEVEMENT_INFO{
		Id:        proto.Uint32(id),
		Progress:  proto.Uint32(state.progress[metric]),
		Timestamp: proto.Uint32(uint32(state.claimed[id])),
	}
}

// withSubMedals is a tier followed by its sub-medals, as CollectionProxy
// binds them (bindComplexTrophy).
func (state trophyState) withSubMedals(id uint32) []*protobuf.ACHIEVEMENT_INFO {
	list := []*protobuf.ACHIEVEMENT_INFO{state.info(id)}
	for _, sub := range trophySubMedals(id) {
		list = append(list, state.info(sub))
	}
	return list
}

// unlocked is SC_17001 progress_list: every medal's tiers up to its first
// one not claimed, each followed by its sub-medals, first tiers in id order
// (the captured order). A tier is only known to the client once the one
// before it is claimed (TrophyClaimCommand's next).
func (state trophyState) unlocked() []*protobuf.ACHIEVEMENT_INFO {
	var list []*protobuf.ACHIEVEMENT_INFO
	for _, id := range archiveTrophies {
		if medalTemplates[id].Start != 0 {
			continue
		}
		for tier := id; tier != 0; tier = medalTemplates[tier].Next {
			if _, ok := medalTemplates[tier]; !ok {
				break
			}
			list = append(list, state.withSubMedals(tier)...)
			if state.claimed[tier] == 0 {
				break
			}
		}
	}
	return list
}

// claimable tells whether a medal tier can be claimed: the client knows it
// (unlocked), it isn't claimed and it reached its target_num, the claimed
// sub-medals for a complex medal (Trophy:getProgress). Coming Soon tiers
// never are.
func (state trophyState) claimable(id uint32) bool {
	medal, ok := medalTemplates[id]
	if !ok || medal.TargetType == medalTargetComingSoon || state.claimed[id] != 0 {
		return false
	}
	if !slices.ContainsFunc(state.unlocked(), func(info *protobuf.ACHIEVEMENT_INFO) bool { return info.GetId() == id }) {
		return false
	}
	if medal.TargetType == medalTargetSubMedals {
		won := uint32(0)
		for _, sub := range trophySubMedals(id) {
			if state.claimed[sub] != 0 {
				won++
			}
		}
		return won >= medal.TargetNum
	}
	return state.info(id).GetProgress() >= medal.TargetNum
}

// handleTrophyClaim answers TrophyClaimCommand (CS_17301): the commander
// claims a medal tier it won, by hand in the medal list or automatically
// for the hidden sub-medals (CollectionProxy:hiddenTrophyAutoClaim). The
// reply's timestamp is the claim time (Trophy:updateTimeStamp, shown as
// new), next the tiers it unlocks (the medal's next tier and its
// sub-medals), which CollectionProxy:unlockNewTrophy adds. Medals give no
// reward. Built from the Lua alone, no capture of it.
func (game *Game) handleTrophyClaim(client *connection.Client, req *protobuf.CS_17301) error {
	ctx := context.Background()
	reply := &protobuf.SC_17302{}
	err := game.tx(ctx, client, func(tx *Tx) error {
		state, err := loadTrophyState(ctx, tx.Queries, tx.commanderID)
		if err != nil {
			return err
		}
		if !state.claimable(req.GetId()) {
			return errRefused
		}
		now := serverNow().Unix()
		if _, err := tx.ClaimTrophy(ctx, db.ClaimTrophyParams{CommanderID: tx.commanderID, TrophyID: req.GetId(), ClaimedAt: now}); err != nil {
			return err
		}
		reply.Timestamp = proto.Uint32(uint32(now))
		if next := medalTemplates[req.GetId()].Next; next != 0 {
			if _, ok := medalTemplates[next]; ok {
				reply.Next = state.withSubMedals(next)
			}
		}
		return nil
	})
	if err != nil && !refused(err) {
		return err
	}
	reply.Result = result(err)
	return client.Send(reply)
}

// displayMedals is SC_11003 medal_id (Player.displayTrophyList), also what
// other commanders see on the profile card (DETAIL_INFO).
func displayMedals(ctx context.Context, queries *db.Queries, commanderID uint32) ([]uint32, error) {
	return queries.ListDisplayMedals(ctx, commanderID)
}

// handleChangeMedalDisplay answers ChangeMedalDisplayCommand (CS_17401):
// the medals (up to 5, in order) the commander shows on its profile, picked
// in the Attire medal page among its claimed medal tiers and its love
// letter medals; fixed_const is always 1. On result 0 the client keeps the
// list and tips change_display_medal_success. Built from the Lua alone.
func (game *Game) handleChangeMedalDisplay(client *connection.Client, req *protobuf.CS_17401) error {
	ctx := context.Background()
	err := game.tx(ctx, client, func(tx *Tx) error {
		ids := req.GetMedalId()
		if len(ids) > maxDisplayMedals {
			return errRefused
		}
		claims, err := tx.ListTrophyClaims(ctx, tx.commanderID)
		if err != nil {
			return err
		}
		claimed := map[uint32]bool{}
		for _, claim := range claims {
			claimed[claim.TrophyID] = true
		}
		letters, err := tx.ListLoveLetterMedals(ctx, tx.commanderID)
		if err != nil {
			return err
		}
		letterLevels := map[uint32]uint32{}
		for _, letter := range letters {
			letterLevels[letter.GroupID] = letter.Level
		}
		seen := map[uint32]bool{}
		for _, id := range ids {
			if seen[id] {
				return errRefused
			}
			seen[id] = true
			if id > loveLetterMedalBase {
				group, level := (id-loveLetterMedalBase)/100, (id-loveLetterMedalBase)%100
				if level%10 != 1 || level > letterLevels[group] {
					return errRefused
				}
			} else if !claimed[id] || medalTemplates[id].Hide == medalAlwaysHidden {
				return errRefused
			}
		}
		if err := tx.ClearDisplayMedals(ctx, tx.commanderID); err != nil {
			return err
		}
		for i, id := range ids {
			if err := tx.AddDisplayMedal(ctx, db.AddDisplayMedalParams{CommanderID: tx.commanderID, Position: uint32(i), MedalID: id}); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil && !refused(err) {
		return err
	}
	return client.Send(&protobuf.SC_17402{Result: result(err)})
}

// trophyProgress is the SC_17002 for a metric whose stored progress
// changed: every tier the client knows that shows it, with its claim time
// kept (Trophy:update replaces both).
func trophyProgress(ctx context.Context, queries *db.Queries, commanderID, metric uint32) (*protobuf.SC_17002, error) {
	state, err := loadTrophyState(ctx, queries, commanderID)
	if err != nil {
		return nil, err
	}
	push := &protobuf.SC_17002{}
	for _, info := range state.unlocked() {
		if trophyMetrics[info.GetId()] == metric {
			push.ProgressList = append(push.ProgressList, info)
		}
	}
	return push, nil
}
