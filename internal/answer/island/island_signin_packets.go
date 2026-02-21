package island

import (
	"time"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/orm"
	"github.com/ggmolly/belfast/internal/protobuf"
	"google.golang.org/protobuf/proto"
)

func IslandSignInInvitation(buffer *[]byte, client *connection.Client) (int, int, error) {
	var request protobuf.CS_21312
	if err := proto.Unmarshal(*buffer, &request); err != nil {
		return 0, 21313, err
	}

	response := protobuf.SC_21313{Result: proto.Uint32(1)}
	inviterState, err := orm.GetOrCreateCommanderIslandSocialState(client.Commander.CommanderID)
	if err != nil {
		return client.SendMessage(21313, &response)
	}

	normalizedTargets := normalizeCommanderIDs(request.GetFriendList(), client.Commander.CommanderID)
	if len(normalizedTargets) > 0 {
		inviterState.InvitedCommanderIDs = mergeUniqueCommanderIDs(inviterState.InvitedCommanderIDs, normalizedTargets)
		if err := orm.SaveCommanderIslandSocialState(inviterState); err != nil {
			return client.SendMessage(21313, &response)
		}
	}

	now := uint32(time.Now().UTC().Unix())
	for _, targetCommanderID := range normalizedTargets {
		if existsErr := orm.CommanderExists(targetCommanderID); existsErr != nil {
			continue
		}
		targetState, getErr := orm.GetOrCreateCommanderIslandSocialState(targetCommanderID)
		if getErr != nil {
			continue
		}
		targetState.GiftCount++
		targetState.GiftTimestamp = now + 86400
		targetState.GiftVisitors = mergeUniqueCommanderIDs(targetState.GiftVisitors, []uint32{client.Commander.CommanderID})
		if saveErr := orm.SaveCommanderIslandSocialState(targetState); saveErr != nil {
			continue
		}

		push := &protobuf.SC_21314{
			IslandId:      proto.Uint32(client.Commander.CommanderID),
			GiftCount:     proto.Uint32(targetState.GiftCount),
			GiftVisitor:   append([]uint32(nil), targetState.GiftVisitors...),
			GiftTimestamp: proto.Uint32(targetState.GiftTimestamp),
			Cmd:           proto.Uint32(2),
		}
		if client.Server != nil {
			if peer, ok := client.Server.FindClientByCommander(targetCommanderID); ok {
				peer.SendMessage(21314, push)
			}
		}
	}

	response.Result = proto.Uint32(0)
	return client.SendMessage(21313, &response)
}

func HandleIslandGetGiftTag(buffer *[]byte, client *connection.Client) (int, int, error) {
	var request protobuf.CS_21315
	if err := proto.Unmarshal(*buffer, &request); err != nil {
		return 0, 21316, err
	}

	requestedIDs := normalizeCommanderIDs(request.GetUserIdList(), 0)
	stateByCommanderID, err := orm.BatchGetCommanderIslandSocialStates(requestedIDs)
	if err != nil {
		response := protobuf.SC_21316{GiftList: []*protobuf.KVDATA2{}}
		return client.SendMessage(21316, &response)
	}

	giftList := make([]*protobuf.KVDATA2, 0, len(requestedIDs))
	for _, commanderID := range requestedIDs {
		state := stateByCommanderID[commanderID]
		if state == nil {
			continue
		}
		giftList = append(giftList, &protobuf.KVDATA2{
			Key:    proto.Uint32(commanderID),
			Value1: proto.Uint32(state.GiftTimestamp),
			Value2: proto.Uint32(state.GiftCount),
		})
	}

	response := protobuf.SC_21316{GiftList: giftList}
	return client.SendMessage(21316, &response)
}

func normalizeCommanderIDs(ids []uint32, exclude uint32) []uint32 {
	seen := make(map[uint32]struct{}, len(ids))
	out := make([]uint32, 0, len(ids))
	for _, id := range ids {
		if id == 0 || id == exclude {
			continue
		}
		if _, exists := seen[id]; exists {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out
}

func mergeUniqueCommanderIDs(existing []uint32, additions []uint32) []uint32 {
	if len(additions) == 0 {
		return existing
	}
	seen := make(map[uint32]struct{}, len(existing)+len(additions))
	out := make([]uint32, 0, len(existing)+len(additions))
	for _, id := range existing {
		if id == 0 {
			continue
		}
		if _, exists := seen[id]; exists {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	for _, id := range additions {
		if id == 0 {
			continue
		}
		if _, exists := seen[id]; exists {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out
}
