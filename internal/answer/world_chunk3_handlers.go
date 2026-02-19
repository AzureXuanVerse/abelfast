package answer

import (
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/protobuf"
	"google.golang.org/protobuf/proto"
)

const (
	worldChunk3ResultFailure       = uint32(1)
	worldChunk3ResultRefused       = uint32(6)
	worldChunk3ResultSameTask      = uint32(20)
	worldChunk3PortGoodsDefault    = uint32(3)
	worldChunk3DailyRefreshSeconds = uint32(24 * 60 * 60)
)

type worldDailyTaskState struct {
	acceptedAt uint32
}

type worldCommanderState struct {
	dailyOffers       []uint32
	dailyRefreshAt    uint32
	activeDailyTasks  map[uint32]worldDailyTaskState
	shopGoodsCounts   map[uint32]uint32
	fleetGroupShips   map[uint32][]uint32
	achievementClaims map[string]struct{}
}

var (
	worldChunk3StateMu sync.Mutex
	worldChunk3State   = map[uint32]*worldCommanderState{}
)

func worldChunk3CommanderState(commanderID uint32) *worldCommanderState {
	state, ok := worldChunk3State[commanderID]
	if ok {
		return state
	}

	state = &worldCommanderState{
		dailyOffers:       []uint32{210200, 210201, 210202},
		dailyRefreshAt:    uint32(time.Now().Unix()) + worldChunk3DailyRefreshSeconds,
		activeDailyTasks:  map[uint32]worldDailyTaskState{},
		shopGoodsCounts:   map[uint32]uint32{},
		fleetGroupShips:   map[uint32][]uint32{},
		achievementClaims: map[string]struct{}{},
	}
	worldChunk3State[commanderID] = state
	return state
}

func WorldPortRequest(buffer *[]byte, client *connection.Client) (int, int, error) {
	var payload protobuf.CS_33401
	if err := proto.Unmarshal(*buffer, &payload); err != nil {
		return 0, 33402, err
	}

	response := &protobuf.SC_33402{
		Port: &protobuf.PORT_INFO{
			PortId:          proto.Uint32(0),
			TaskList:        []uint32{},
			GoodsList:       []*protobuf.GOODS_INFO_P33{},
			NextRefreshTime: proto.Uint32(0),
		},
	}

	if payload.GetMapId() == 0 {
		return client.SendMessage(33402, response)
	}

	now := uint32(time.Now().Unix())
	worldChunk3StateMu.Lock()
	state := worldChunk3CommanderState(client.Commander.CommanderID)
	if now >= state.dailyRefreshAt {
		state.dailyRefreshAt = now + worldChunk3DailyRefreshSeconds
	}
	goodsID := payload.GetMapId()*1000 + 1
	if _, ok := state.shopGoodsCounts[goodsID]; !ok {
		state.shopGoodsCounts[goodsID] = worldChunk3PortGoodsDefault
	}
	taskList := append([]uint32{0}, state.dailyOffers...)
	goodsCount := state.shopGoodsCounts[goodsID]
	nextRefresh := state.dailyRefreshAt
	worldChunk3StateMu.Unlock()

	response.Port.PortId = proto.Uint32(payload.GetMapId())
	response.Port.TaskList = taskList
	response.Port.GoodsList = []*protobuf.GOODS_INFO_P33{{
		GoodsId: proto.Uint32(goodsID),
		Count:   proto.Uint32(goodsCount),
	}}
	response.Port.NextRefreshTime = proto.Uint32(nextRefresh)
	return client.SendMessage(33402, response)
}

func WorldPortShopping(buffer *[]byte, client *connection.Client) (int, int, error) {
	var payload protobuf.CS_33403
	if err := proto.Unmarshal(*buffer, &payload); err != nil {
		return 0, 33404, err
	}

	response := &protobuf.SC_33404{
		Result:   proto.Uint32(worldChunk3ResultFailure),
		DropList: []*protobuf.DROPINFO{},
	}

	if payload.GetShopId() == 0 || payload.GetCount() == 0 {
		return client.SendMessage(33404, response)
	}
	if payload.GetShopType() != 1 && payload.GetShopType() != 2 {
		return client.SendMessage(33404, response)
	}

	worldChunk3StateMu.Lock()
	state := worldChunk3CommanderState(client.Commander.CommanderID)
	remaining, exists := state.shopGoodsCounts[payload.GetShopId()]
	if !exists {
		remaining = worldChunk3PortGoodsDefault
	}
	if payload.GetCount() > remaining {
		worldChunk3StateMu.Unlock()
		return client.SendMessage(33404, response)
	}
	state.shopGoodsCounts[payload.GetShopId()] = remaining - payload.GetCount()
	worldChunk3StateMu.Unlock()

	response.Result = proto.Uint32(0)
	response.DropList = []*protobuf.DROPINFO{{
		Type:   proto.Uint32(2),
		Id:     proto.Uint32(payload.GetShopId()),
		Number: proto.Uint32(payload.GetCount()),
	}}
	return client.SendMessage(33404, response)
}

func WorldFleetChangeCompatibility(buffer *[]byte, client *connection.Client) (int, int, error) {
	var payload protobuf.CS_33405
	if err := proto.Unmarshal(*buffer, &payload); err != nil {
		return 0, 33406, err
	}

	response := &protobuf.SC_33406{Result: proto.Uint32(worldChunk3ResultFailure)}
	if len(payload.GetFleetList()) == 0 {
		return client.SendMessage(33406, response)
	}

	seenGroups := map[uint32]struct{}{}
	worldChunk3StateMu.Lock()
	state := worldChunk3CommanderState(client.Commander.CommanderID)
	for _, fleet := range payload.GetFleetList() {
		if fleet == nil || fleet.GetGroupId() == 0 {
			worldChunk3StateMu.Unlock()
			return client.SendMessage(33406, response)
		}
		if _, exists := seenGroups[fleet.GetGroupId()]; exists {
			worldChunk3StateMu.Unlock()
			return client.SendMessage(33406, response)
		}
		seenGroups[fleet.GetGroupId()] = struct{}{}
		state.fleetGroupShips[fleet.GetGroupId()] = append([]uint32{}, fleet.GetShipId()...)
	}
	worldChunk3StateMu.Unlock()

	response.Result = proto.Uint32(0)
	return client.SendMessage(33406, response)
}

func WorldShipRepair(buffer *[]byte, client *connection.Client) (int, int, error) {
	var payload protobuf.CS_33407
	if err := proto.Unmarshal(*buffer, &payload); err != nil {
		return 0, 33408, err
	}

	response := &protobuf.SC_33408{Result: proto.Uint32(worldChunk3ResultFailure)}
	if len(payload.GetShipList()) == 0 {
		return client.SendMessage(33408, response)
	}

	seen := map[uint32]struct{}{}
	for _, shipID := range payload.GetShipList() {
		if shipID == 0 {
			return client.SendMessage(33408, response)
		}
		if _, ok := seen[shipID]; ok {
			continue
		}
		seen[shipID] = struct{}{}
	}

	response.Result = proto.Uint32(0)
	return client.SendMessage(33408, response)
}

func WorldFleetRedeploy(buffer *[]byte, client *connection.Client) (int, int, error) {
	var payload protobuf.CS_33409
	if err := proto.Unmarshal(*buffer, &payload); err != nil {
		return 0, 33410, err
	}

	response := &protobuf.SC_33410{
		Result:    proto.Uint32(worldChunk3ResultFailure),
		GroupList: []*protobuf.GROUPINCHAPTER_P33{},
	}
	if len(payload.GetEliteFleetList()) == 0 {
		return client.SendMessage(33410, response)
	}

	groups := make([]*protobuf.GROUPINCHAPTER_P33, 0, len(payload.GetEliteFleetList()))
	for index, fleet := range payload.GetEliteFleetList() {
		if fleet == nil {
			return client.SendMessage(33410, response)
		}

		shipList := make([]*protobuf.SHIPINCHAPTER_P33, 0, len(fleet.GetShipIdList()))
		for _, shipID := range fleet.GetShipIdList() {
			if shipID == 0 {
				return client.SendMessage(33410, response)
			}
			shipList = append(shipList, &protobuf.SHIPINCHAPTER_P33{
				Id:     proto.Uint32(shipID),
				HpRant: proto.Uint32(10000),
			})
		}

		for _, commander := range fleet.GetCommanders() {
			if commander == nil || commander.GetPos() == 0 || commander.GetId() == 0 {
				return client.SendMessage(33410, response)
			}
		}

		row := uint32(index + 1)
		position := &protobuf.CHAPTERCELLPOS_P33{Row: proto.Uint32(row), Column: proto.Uint32(1)}
		groups = append(groups, &protobuf.GROUPINCHAPTER_P33{
			Id:            proto.Uint32(row),
			ShipList:      shipList,
			Pos:           position,
			LossFlag:      proto.Uint32(0),
			Bullet:        proto.Uint32(0),
			StartPos:      &protobuf.CHAPTERCELLPOS_P33{Row: proto.Uint32(row), Column: proto.Uint32(1)},
			DamageLevel:   proto.Uint32(0),
			CommanderList: append([]*protobuf.COMMANDERSINFO{}, fleet.GetCommanders()...),
			KillCount:     proto.Uint32(0),
			BulletMax:     proto.Uint32(0),
		})
	}

	response.Result = proto.Uint32(0)
	response.GroupList = groups
	return client.SendMessage(33410, response)
}

func WorldDailyTaskRefresh(buffer *[]byte, client *connection.Client) (int, int, error) {
	var payload protobuf.CS_33413
	if err := proto.Unmarshal(*buffer, &payload); err != nil {
		return 0, 33414, err
	}

	response := &protobuf.SC_33414{
		Result:          proto.Uint32(worldChunk3ResultFailure),
		TaskList:        []uint32{},
		NextRefreshTime: proto.Uint32(0),
	}
	if payload.GetType() != 0 {
		return client.SendMessage(33414, response)
	}

	now := uint32(time.Now().Unix())
	worldChunk3StateMu.Lock()
	state := worldChunk3CommanderState(client.Commander.CommanderID)
	if now >= state.dailyRefreshAt {
		state.dailyRefreshAt = now + worldChunk3DailyRefreshSeconds
	}
	taskList := append([]uint32{0}, state.dailyOffers...)
	nextRefresh := state.dailyRefreshAt
	worldChunk3StateMu.Unlock()

	response.Result = proto.Uint32(0)
	response.TaskList = taskList
	response.NextRefreshTime = proto.Uint32(nextRefresh)
	return client.SendMessage(33414, response)
}

func WorldTriggerDailyTask(buffer *[]byte, client *connection.Client) (int, int, error) {
	var payload protobuf.CS_33415
	if err := proto.Unmarshal(*buffer, &payload); err != nil {
		return 0, 33416, err
	}

	response := &protobuf.SC_33416{
		Result:   proto.Uint32(worldChunk3ResultFailure),
		TaskList: []*protobuf.TASK_INFO{},
	}
	if len(payload.GetTaskList()) == 0 {
		return client.SendMessage(33416, response)
	}

	worldChunk3StateMu.Lock()
	state := worldChunk3CommanderState(client.Commander.CommanderID)
	offerSet := map[uint32]struct{}{}
	for _, taskID := range state.dailyOffers {
		offerSet[taskID] = struct{}{}
	}

	requested := make([]uint32, 0, len(payload.GetTaskList()))
	seen := map[uint32]struct{}{}
	for _, taskID := range payload.GetTaskList() {
		if taskID == 0 {
			worldChunk3StateMu.Unlock()
			return client.SendMessage(33416, response)
		}
		if _, ok := seen[taskID]; ok {
			continue
		}
		seen[taskID] = struct{}{}
		requested = append(requested, taskID)
	}

	for _, taskID := range requested {
		if _, ok := offerSet[taskID]; !ok {
			response.Result = proto.Uint32(worldChunk3ResultRefused)
			worldChunk3StateMu.Unlock()
			return client.SendMessage(33416, response)
		}
		if _, exists := state.activeDailyTasks[taskID]; exists {
			response.Result = proto.Uint32(worldChunk3ResultSameTask)
			worldChunk3StateMu.Unlock()
			return client.SendMessage(33416, response)
		}
	}

	now := uint32(time.Now().Unix())
	resultTasks := make([]*protobuf.TASK_INFO, 0, len(requested))
	for _, taskID := range requested {
		state.activeDailyTasks[taskID] = worldDailyTaskState{acceptedAt: now}
		resultTasks = append(resultTasks, &protobuf.TASK_INFO{
			Id:          proto.Uint32(taskID),
			Progress:    proto.Uint32(0),
			AcceptTime:  proto.Uint32(now),
			SubmiteTime: proto.Uint32(0),
			EventMapId:  proto.Uint32(0),
		})
	}
	worldChunk3StateMu.Unlock()

	sort.Slice(resultTasks, func(i, j int) bool {
		return resultTasks[i].GetId() < resultTasks[j].GetId()
	})
	response.Result = proto.Uint32(0)
	response.TaskList = resultTasks
	return client.SendMessage(33416, response)
}

func WorldBossSupport(buffer *[]byte, client *connection.Client) (int, int, error) {
	var payload protobuf.CS_33509
	if err := proto.Unmarshal(*buffer, &payload); err != nil {
		return 0, 33510, err
	}

	result := worldChunk3ResultFailure
	if payload.GetType() >= 1 && payload.GetType() <= 3 {
		result = 0
	}
	response := &protobuf.SC_33510{Result: proto.Uint32(result)}
	return client.SendMessage(33510, response)
}

func WorldAchieveClaim(buffer *[]byte, client *connection.Client) (int, int, error) {
	var payload protobuf.CS_33602
	if err := proto.Unmarshal(*buffer, &payload); err != nil {
		return 0, 33603, err
	}

	response := &protobuf.SC_33603{
		Result: proto.Uint32(worldChunk3ResultFailure),
		Drops:  []*protobuf.DROPINFO{},
	}
	if len(payload.GetList()) == 0 {
		return client.SendMessage(33603, response)
	}

	worldChunk3StateMu.Lock()
	state := worldChunk3CommanderState(client.Commander.CommanderID)
	requestClaims := map[string]struct{}{}
	for _, entry := range payload.GetList() {
		if entry == nil || entry.GetId() == 0 || len(entry.GetStarList()) == 0 {
			worldChunk3StateMu.Unlock()
			return client.SendMessage(33603, response)
		}
		for _, star := range entry.GetStarList() {
			if star == 0 {
				worldChunk3StateMu.Unlock()
				return client.SendMessage(33603, response)
			}
			claimKey := fmt.Sprintf("%d:%d", entry.GetId(), star)
			if _, duplicatedInRequest := requestClaims[claimKey]; duplicatedInRequest {
				worldChunk3StateMu.Unlock()
				return client.SendMessage(33603, response)
			}
			if _, alreadyClaimed := state.achievementClaims[claimKey]; alreadyClaimed {
				worldChunk3StateMu.Unlock()
				return client.SendMessage(33603, response)
			}
			requestClaims[claimKey] = struct{}{}
		}
	}

	drops := make([]*protobuf.DROPINFO, 0, len(requestClaims))
	for _, entry := range payload.GetList() {
		for _, star := range entry.GetStarList() {
			claimKey := fmt.Sprintf("%d:%d", entry.GetId(), star)
			state.achievementClaims[claimKey] = struct{}{}
			drops = append(drops, &protobuf.DROPINFO{
				Type:   proto.Uint32(2),
				Id:     proto.Uint32(entry.GetId()*100 + star),
				Number: proto.Uint32(1),
			})
		}
	}
	worldChunk3StateMu.Unlock()

	sort.Slice(drops, func(i, j int) bool {
		return drops[i].GetId() < drops[j].GetId()
	})
	response.Result = proto.Uint32(0)
	response.Drops = drops
	return client.SendMessage(33603, response)
}
