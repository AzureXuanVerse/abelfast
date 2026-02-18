package answer

import (
	"sort"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/consts"
	"github.com/ggmolly/belfast/internal/orm"
	"github.com/ggmolly/belfast/internal/protobuf"
	"google.golang.org/protobuf/proto"
)

const (
	miniGameOpResultSuccess = uint32(0)
	miniGameOpResultFailure = uint32(1)

	miniGameCmdComplete    = uint32(1)
	miniGameCmdUltimate    = uint32(2)
	miniGameCmdSpecialGame = uint32(3)
	miniGameCmdHighScore   = uint32(4)
	miniGameCmdPlay        = uint32(5)
	miniGameCmdSuccessData = uint32(101)
)

func MiniGameOperation(buffer *[]byte, client *connection.Client) (int, int, error) {
	req := &protobuf.CS_26103{}
	if err := proto.Unmarshal(*buffer, req); err != nil {
		return 0, 26104, err
	}

	response := runMiniGameOperation(client, req)
	return connection.SendProtoMessage(26104, client, response)
}

func runMiniGameOperation(client *connection.Client, req *protobuf.CS_26103) *protobuf.SC_26104 {
	response := &protobuf.SC_26104{
		Result:    proto.Uint32(miniGameOpResultFailure),
		AwardList: []*protobuf.DROPINFO{},
	}
	if client == nil || client.Commander == nil {
		return response
	}
	hubID := req.GetHubid()
	if hubID == 0 || req.GetCmd() == 0 {
		return response
	}

	hubConfig, err := orm.GetMiniGameHubConfig(hubID)
	if err != nil {
		return response
	}
	hubState, err := orm.GetOrCreateMiniGameHubState(client.Commander.CommanderID, hubConfig)
	if err != nil {
		return response
	}

	dataState, awards, ok := applyMiniGameCommand(hubConfig, hubState, req.GetCmd(), req.GetArgs1())
	if !ok {
		return response
	}
	if err := orm.SaveMiniGameHubState(hubState); err != nil {
		return response
	}
	if dataState != nil {
		if err := orm.SaveMiniGameDataState(dataState); err != nil {
			return response
		}
	}
	if err := grantMiniGameDrops(client, awards); err != nil {
		return response
	}

	response.Result = proto.Uint32(miniGameOpResultSuccess)
	response.Hub = buildMiniGameHubProto(hubState)
	if dataState != nil {
		response.Data = buildMiniGameDataProto(dataState)
	}
	response.AwardList = awards
	return response
}

func applyMiniGameCommand(config *orm.MiniGameHubConfig, hubState *orm.MiniGameHubState, cmd uint32, args []uint32) (*orm.MiniGameDataState, []*protobuf.DROPINFO, bool) {
	awards := []*protobuf.DROPINFO{}
	switch cmd {
	case miniGameCmdComplete:
		if len(args) < 3 || args[2] == 0 {
			return nil, nil, false
		}
		if hubState.AvailableCnt == 0 {
			return nil, nil, false
		}
		hubState.AvailableCnt--
		hubState.UsedCnt++
		if config.RewardNeed > 0 && hubState.UsedCnt >= config.RewardNeed && hubState.Ultimate == 0 {
			hubState.Ultimate = 1
			if drop, ok := miniGameRewardDrop(config.RewardDisplay); ok {
				awards = append(awards, drop)
			}
		}
		dataState, err := orm.GetOrCreateMiniGameDataState(hubState.CommanderID, args[2])
		if err != nil {
			return nil, nil, false
		}
		dataState.Datas = append([]uint32(nil), args...)
		return dataState, awards, true
	case miniGameCmdPlay:
		if len(args) < 1 || args[0] == 0 {
			return nil, nil, false
		}
		if hubState.AvailableCnt == 0 {
			return nil, nil, false
		}
		hubState.AvailableCnt--
		hubState.UsedCnt++
		dataState, err := orm.GetOrCreateMiniGameDataState(hubState.CommanderID, args[0])
		if err != nil {
			return nil, nil, false
		}
		dataState.Datas = append([]uint32(nil), args...)
		return dataState, awards, true
	case miniGameCmdSpecialGame, miniGameCmdSuccessData:
		if len(args) < 1 || args[0] == 0 {
			return nil, nil, false
		}
		dataState, err := orm.GetOrCreateMiniGameDataState(hubState.CommanderID, args[0])
		if err != nil {
			return nil, nil, false
		}
		if len(args) > 1 {
			dataState.Datas = append([]uint32(nil), args[1:]...)
		}
		return dataState, awards, true
	case miniGameCmdHighScore:
		if len(args) < 2 || args[0] == 0 {
			return nil, nil, false
		}
		extra := uint32(0)
		if len(args) > 2 {
			extra = args[2]
		}
		current := hubState.MaxScores[args[0]]
		if args[1] > current.Score || (args[1] == current.Score && (current.Extra == 0 || extra < current.Extra)) {
			hubState.MaxScores[args[0]] = orm.MiniGameScoreEntry{Score: args[1], Extra: extra}
		}
		return nil, awards, true
	case miniGameCmdUltimate:
		hubState.Ultimate = 1
		return nil, awards, true
	default:
		return nil, nil, false
	}
}

func miniGameRewardDrop(rewardDisplay []uint32) (*protobuf.DROPINFO, bool) {
	if len(rewardDisplay) < 3 || rewardDisplay[2] == 0 {
		return nil, false
	}
	return &protobuf.DROPINFO{
		Type:   proto.Uint32(rewardDisplay[0]),
		Id:     proto.Uint32(rewardDisplay[1]),
		Number: proto.Uint32(rewardDisplay[2]),
	}, true
}

func grantMiniGameDrops(client *connection.Client, awards []*protobuf.DROPINFO) error {
	for _, award := range awards {
		switch award.GetType() {
		case consts.DROP_TYPE_RESOURCE:
			if err := client.Commander.AddResource(award.GetId(), award.GetNumber()); err != nil {
				return err
			}
		case consts.DROP_TYPE_ITEM, consts.DROP_TYPE_VITEM:
			if err := client.Commander.AddItem(award.GetId(), award.GetNumber()); err != nil {
				return err
			}
		}
	}
	return nil
}

func buildMiniGameHubProto(state *orm.MiniGameHubState) *protobuf.MINIGAMEHUB {
	maxscores := make([]*protobuf.KVDATA2, 0, len(state.MaxScores))
	keys := make([]uint32, 0, len(state.MaxScores))
	for key := range state.MaxScores {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i int, j int) bool {
		return keys[i] < keys[j]
	})
	for _, key := range keys {
		score := state.MaxScores[key]
		maxscores = append(maxscores, &protobuf.KVDATA2{
			Key:    proto.Uint32(key),
			Value1: proto.Uint32(score.Score),
			Value2: proto.Uint32(score.Extra),
		})
	}
	return &protobuf.MINIGAMEHUB{
		Id:           proto.Uint32(state.HubID),
		AvailableCnt: proto.Uint32(state.AvailableCnt),
		UsedCnt:      proto.Uint32(state.UsedCnt),
		Ultimate:     proto.Uint32(state.Ultimate),
		Maxscores:    maxscores,
	}
}

func buildMiniGameDataProto(state *orm.MiniGameDataState) *protobuf.MINIGAMEDATA {
	lists := make([]*protobuf.KEYVALUELIST_P26, 0, len(state.KVLists))
	for _, list := range state.KVLists {
		values := make([]*protobuf.KEYVALUE_P26, 0, len(list.Values))
		for _, value := range list.Values {
			values = append(values, &protobuf.KEYVALUE_P26{
				Key:    proto.Uint32(value.Key),
				Value:  proto.Uint32(value.Value),
				Value2: proto.Uint32(value.Value2),
			})
		}
		lists = append(lists, &protobuf.KEYVALUELIST_P26{
			Key:       proto.Uint32(list.Key),
			ValueList: values,
		})
	}
	return &protobuf.MINIGAMEDATA{
		Id:                proto.Uint32(state.GameID),
		Datas:             append([]uint32(nil), state.Datas...),
		Date1KeyValueList: lists,
	}
}
