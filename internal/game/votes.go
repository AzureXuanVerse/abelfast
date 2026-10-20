package game

import (
	"encoding/json"
	"fmt"
	"slices"

	"google.golang.org/protobuf/proto"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/gamedata"
	"github.com/ggmolly/belfast/internal/protobuf"
)

// The popularity election (view/vote, VoteProxy): stages of activity_vote
// (qualifiers, groups, repechage, finals, special votes), each run by an
// ACTIVITY_TYPE_VOTE activity whose config_id is the stage. Commanders cast
// votes with CS_11202 cmd 1 (VoteCommand), which belfast doesn't take: no
// vote is stored, every candidate has 0 votes.

// activity_vote row, what the ranking needs: the candidates of a stage are
// its group_data (ship groups or activity_vote_virtual_ship_data ids), or,
// when it has none, the ranks of earlier stages that rank_to_next sends to it
// ({stage, ranks} pairs). Neither field is read by the client.
type voteStageConfig struct {
	ID         uint32            `json:"id"`
	GroupData  []uint32          `json:"group_data"`
	RankToNext []json.RawMessage `json:"rank_to_next"`
}

var (
	voteStageConfigs map[uint32]voteStageConfig

	// voteRankings is the ranking of each stage, best first
	voteRankings map[uint32][]uint32
)

// voteAdvance is a rank_to_next pair: the stage the ranks go on to.
type voteAdvance struct {
	stage uint32
	ranks []uint32
}

func loadVoteData() error {
	if err := gamedata.Load("activity_vote", &voteStageConfigs)(); err != nil {
		return err
	}
	// the stages each stage's ranks go on to, by the stage they come from
	advances := map[uint32][]voteAdvance{}
	ids := make([]uint32, 0, len(voteStageConfigs))
	for id, stage := range voteStageConfigs {
		ids = append(ids, id)
		for _, raw := range stage.RankToNext {
			var pair []json.RawMessage
			if json.Unmarshal(raw, &pair) != nil || len(pair) != 2 {
				return gamedata.Missing("activity_vote", "%d rank_to_next %s", id, raw)
			}
			var next uint32
			var ranks []uint32
			if json.Unmarshal(pair[0], &next) != nil || json.Unmarshal(pair[1], &ranks) != nil {
				return gamedata.Missing("activity_vote", "%d rank_to_next %s", id, raw)
			}
			if next != 0 {
				advances[id] = append(advances[id], voteAdvance{next, ranks})
			}
		}
	}
	slices.Sort(ids)
	voteRankings = make(map[uint32][]uint32, len(ids))
	visiting := map[uint32]bool{}
	var rank func(id uint32) ([]uint32, error)
	rank = func(id uint32) ([]uint32, error) {
		if ranking, ok := voteRankings[id]; ok {
			return ranking, nil
		}
		if visiting[id] {
			return nil, fmt.Errorf("activity_vote %d: rank_to_next loops", id)
		}
		visiting[id] = true
		candidates := slices.Clone(voteStageConfigs[id].GroupData)
		if len(candidates) == 0 {
			// earlier stages in id order, each in the order rank_to_next
			// lists the ranks
			for _, from := range ids {
				for _, advance := range advances[from] {
					if advance.stage != id {
						continue
					}
					previous, err := rank(from)
					if err != nil {
						return nil, err
					}
					for _, place := range advance.ranks {
						if place >= 1 && int(place) <= len(previous) {
							candidates = append(candidates, previous[place-1])
						}
					}
				}
			}
		}
		// no votes are stored: the ranking is the candidates' order
		voteRankings[id] = candidates
		return candidates, nil
	}
	for _, id := range ids {
		if _, err := rank(id); err != nil {
			return err
		}
	}
	return nil
}

// handleFetchVoteRank answers FetchVoteInfoCommand and FetchVoteRankCommand
// (controller/command/vote/, CS_17203): the ranking of stage type, the
// activity_vote id, as VoteGroup builds it (VoteProxy.Data2VoteShip,
// model/vo/vote): key is the ship group, or the
// activity_vote_virtual_ship_data id; value1 the total votes (shown once the
// stage closed), value2 the votes cast in game (shown and sorted on while it
// is open), value3 the web votes (stored, never shown). Out of the vote the
// client takes the list order as the ranks (VoteGroup.updateRankMap), so the
// list is best first. Belfast has no web vote and stores no in-game vote yet:
// every candidate has 0. 17203 was never captured: the reply is built from
// the Lua alone. An unknown stage gets an empty list (SC_17204 has no result).
func (game *Game) handleFetchVoteRank(client *connection.Client, req *protobuf.CS_17203) error {
	ranking := voteRankings[req.GetType()]
	reply := &protobuf.SC_17204{List: make([]*protobuf.MULKEYVALUE_P17, 0, len(ranking))}
	for _, key := range ranking {
		reply.List = append(reply.List, &protobuf.MULKEYVALUE_P17{
			Key:    proto.Uint32(key),
			Value1: proto.Uint32(0),
			Value2: proto.Uint32(0),
			Value3: proto.Uint32(0),
		})
	}
	return client.Send(reply)
}
