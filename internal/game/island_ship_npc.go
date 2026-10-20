package game

import (
	"context"
	"encoding/json"
	"maps"
	"slices"
	"time"

	"google.golang.org/protobuf/proto"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/gamedata"
	"github.com/ggmolly/belfast/internal/protobuf"
)

// The characters about the island: those following the commander
// (IslandFollowerAgency, private follow_ships), the actions shown to the
// other players (IslandAniamtionOpCommand), the greetings of strolling
// characters (island_strollnpc, IslandNpcActionPlayer) and the event
// characters of a scene (IslandActivityNpcAgency).

// IslandConst.FOLLOWER_OP_*
const (
	islandFollowerAdd = 1
	islandFollowerDel = 2
)

// island_action_feedback condition of a greeting answered with a reward
// (IslandStrollUnitVO:GetResponeAction)
const islandFeedbackRewarded = 1

// islandStrollConfig is an island_strollnpc row: action_feedback 1 for the
// strolling characters that can ask for a greeting.
type islandStrollConfig struct {
	ActionFeedback int32 `json:"action_feedback"`
}

// islandFeedbackConfig is an island_action_feedback row.
type islandFeedbackConfig struct {
	Condition uint32 `json:"condition"`
	DropID    uint32 `json:"drop_id"`
}

// islandWorldObjectConfig is an island_world_objects row: a scene's object;
// those whose param names an activity are its event characters.
type islandWorldObjectConfig struct {
	MapID uint32          `json:"mapId"`
	Param json.RawMessage `json:"param"`
}

// islandActionConfig is an island_action row.
type islandActionConfig struct {
	Type uint32 `json:"type"`
}

// islandEventNpc is an event character: a scene object shown while its
// activity runs (IslandActivityNpcAllocator:IsVisible).
type islandEventNpc struct {
	objectID, mapID, activityID uint32
}

var (
	islandStrollNpcs     map[uint32]islandStrollConfig
	islandFeedbacks      map[uint32]islandFeedbackConfig
	islandActions        map[uint32]islandActionConfig
	islandEventNpcs      []islandEventNpc
	islandDefaultActions []uint32
	islandFeedbackTimes  uint32
)

func loadIslandNpcData() error {
	var objects map[uint32]islandWorldObjectConfig
	err := gamedata.LoadAll(
		gamedata.Load("island_strollnpc", &islandStrollNpcs),
		gamedata.Load("island_action_feedback", &islandFeedbacks),
		gamedata.Load("island_action", &islandActions),
		gamedata.Load("island_world_objects", &objects),
		gamedata.Load("island_dress_template", &islandDressLinks),
	)
	if err != nil {
		return err
	}
	islandEventNpcs = nil
	for _, id := range slices.Sorted(maps.Keys(objects)) {
		var param struct {
			Activity json.RawMessage `json:"activity"`
		}
		if json.Unmarshal(objects[id].Param, &param) != nil {
			continue
		}
		if activities := gamedata.IDs(param.Activity); len(activities) > 0 {
			islandEventNpcs = append(islandEventNpcs, islandEventNpc{objectID: id, mapID: objects[id].MapID, activityID: activities[0]})
		}
	}
	islandFeedbackTimes = islandSetInt("island_feedback_award_times")
	return islandSetList("default_action", &islandDefaultActions)
}

// islandMapNpcs adds to an EnterMapCommand reply (SC_21214) the event
// characters about in scene mapID on island state: the island_world_objects
// of the scene whose param.activity runs (the client shows them only then,
// IslandActivityNpcAllocator). npc_list's id is the activity
// (IslandActivityNpcAgency keys its characters by it). The strolling
// characters are the client's own (IslandDataConvertor.CollectStrollUnits).
// GUESS: an event character is there whenever its activity runs (the Pearl
// Trade merchant, activity 990011, is the only one); belfast never moves or
// removes one (SC_21228).
func islandMapNpcs(tx *Tx, state *islandState, mapID uint32, reply *protobuf.SC_21214) error {
	now := time.Now()
	for _, npc := range islandEventNpcs {
		if npc.mapID != mapID {
			continue
		}
		if _, open := openActivity(npc.activityID, now); open {
			reply.NpcList = append(reply.NpcList, &protobuf.PB_ISLAND_NPC{Id: proto.Uint32(npc.activityID), ObjectId: proto.Uint32(npc.objectID)})
		}
	}
	return nil
}

// handleIslandFollower answers IslandFollowerOpCommand (CS_21630): a
// character starts (type 1) or stops (type 2) following the commander
// (follow_ships), at most island_set max_follower_cnt of them, only an idle
// one (IslandCharacterAgency:CanFollowPlayer). GUESS: the client alone
// checks the tasks that keep a character from following (in_task).
func (game *Game) handleIslandFollower(client *connection.Client, req *protobuf.CS_21630) error {
	ret, err := game.islandShipTx(client, func(tx *Tx, state *islandState, now time.Time) error {
		id := req.GetShipId()
		private := state.private()
		ship := state.islandShip(id)
		if ship == nil && id != islandNpcShipID {
			return errRefused
		}
		following := slices.Contains(private.FollowShips, id)
		switch req.GetType() {
		case islandFollowerAdd:
			if following || ship != nil && islandShipWorking(ship, now) || uint32(len(private.FollowShips)) >= islandMaxFollowers {
				return errRefused
			}
			private.FollowShips = append(private.FollowShips, id)
			return tx.islandTaskEvent(islandTaskFollow, 1, id)
		case islandFollowerDel:
			private.FollowShips = slices.DeleteFunc(private.FollowShips, func(other uint32) bool { return other == id })
			return nil
		}
		return errRefused
	})
	if err != nil {
		return err
	}
	return client.Send(&protobuf.SC_21631{Result: ret})
}

// handleIslandAnimationOp takes IslandAniamtionOpCommand (CS_21700, no
// reply): the commander plays an action (island_action) alone, invites to
// a two-player one (target 0), joins another player's (target its id) or
// stops (action 0). Everyone in its scene, itself included (the one
// joining plays the pair from it, IslandView:OnResponAniamtionOp), gets it
// as SC_21701. GUESS: only actions the commander has (action_list or
// island_set default_action) are relayed.
func (game *Game) handleIslandAnimationOp(client *connection.Client, req *protobuf.CS_21700) error {
	islandID, here := game.islandOf(client)
	action := req.GetActionId()
	if !here || islandID != req.GetIslandId() {
		return nil
	}
	if action != 0 {
		if _, ok := islandActions[action]; !ok {
			return nil
		}
		if !slices.Contains(islandDefaultActions, action) {
			owned := false
			err := game.tx(context.Background(), client, func(tx *Tx) error {
				state, err := tx.island()
				if err != nil {
					return err
				}
				owned = slices.Contains(state.private().GetActionList(), action)
				return nil
			})
			if err != nil || !owned {
				return err
			}
		}
	}
	msg := &protobuf.SC_21701{
		IslandId: proto.Uint32(islandID), PlayerId: proto.Uint32(client.CommanderID),
		TargetId: proto.Uint32(req.GetTargetId()), ActionId: proto.Uint32(action),
	}
	game.tellIslandScene(client, islandID, game.islandMapOf(client.CommanderID), msg)
	return client.Push(msg)
}

// handleIslandNpcActionAward answers IslandGetNpcActionAwardCommand
// (CS_21702): the commander greeted a strolling character with the action
// it asked for (npc_id, island_strollnpc action_feedback 1; 0 when only
// the character's skill answered) and it answered with action_feedback_id.
// A character that asked gives its reward once a day, for at most
// island_set island_feedback_award_times characters a day
// (action_feedback_npc_list, IslandDataConvertor.DistributeActionFeedbackAward4StrollUnits).
// The character greeted (ship_id), if its skill is a greeting one
// (IslandBuffType 701/702) not used today, recovers stamina (701, below
// type_use[1]: + type_use[2], IslandShipSkill:Apply) or gives its reward
// (702), and is used for the day (skill_use_state). Not done: the rewards
// themselves (island_action_feedback drop_id 3001, the 702 skills' drop
// 9001..9010) are server-side drop pools no table or wiki page gives, so
// drop_list stays empty.
func (game *Game) handleIslandNpcActionAward(client *connection.Client, req *protobuf.CS_21702) error {
	ret, err := game.islandShipTx(client, func(tx *Tx, state *islandState, now time.Time) error {
		feedback, ok := islandFeedbacks[req.GetActionFeedbackId()]
		if !ok {
			return errRefused
		}
		private := state.private()
		if npc := req.GetNpcId(); npc != 0 {
			if islandStrollNpcs[npc].ActionFeedback != 1 || feedback.Condition != islandFeedbackRewarded ||
				slices.Contains(private.ActionFeedbackNpcList, npc) || uint32(len(private.ActionFeedbackNpcList)) >= islandFeedbackTimes {
				return errRefused
			}
			private.ActionFeedbackNpcList = append(private.ActionFeedbackNpcList, npc)
			if err := tx.islandTaskEvent(islandTaskActionHello, 1, npc); err != nil {
				return err
			}
			if err := tx.islandAchievementEvent(islandAchieveNpcInteract, 0, 1); err != nil {
				return err
			}
		}
		ship := state.islandShip(req.GetShipId())
		config, ok := islandShipConfigs[req.GetShipId()]
		if ship == nil || !ok || ship.GetSkillUseState() == 1 {
			return nil
		}
		for _, id := range config.skillEffects(ship) {
			buff := islandBuffs[id]
			values := buff.numbers()
			switch {
			case buff.BuffType == islandBuffPowerRecoverByGreeting && len(values) >= 2 && float64(islandShipEnergy(ship, config, now)) <= values[0]:
				ship.Power = proto.Uint32(ship.GetPower() + uint32(values[1]))
				ship.SkillUseState = proto.Uint32(1)
			case buff.BuffType == islandBuffAwardByGreeting:
				ship.SkillUseState = proto.Uint32(1)
			}
		}
		return nil
	})
	if err != nil {
		return err
	}
	return client.Send(&protobuf.SC_21703{Result: ret})
}
