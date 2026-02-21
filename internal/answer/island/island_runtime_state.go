package island

import (
	"sort"
	"sync"

	"google.golang.org/protobuf/proto"

	"github.com/ggmolly/belfast/internal/protobuf"
)

const islandVisitCapacity = 1

type islandRuntimeState struct {
	mu          sync.Mutex
	sessions    map[uint32]uint32
	visitors    map[uint32]map[uint32]struct{}
	queues      map[uint32][]uint32
	cooldowns   map[uint32]uint32
	visitorFeed map[uint32][]*protobuf.PB_VISITOR
	objects     map[uint32]map[islandObjectKey]*islandObjectState
}

type islandObjectKey struct {
	objectType uint32
	objectID   uint32
}

type islandObjectState struct {
	status uint32
	slots  map[uint32]uint32
}

type islandObjectTemplate struct {
	ID       uint32
	Type     uint32
	SlotIDs  []uint32
	Status   uint32
	MapID    uint32
	KnownMap bool
}

var globalIslandRuntimeState = newIslandRuntimeState()

func newIslandRuntimeState() *islandRuntimeState {
	return &islandRuntimeState{
		sessions:    make(map[uint32]uint32),
		visitors:    make(map[uint32]map[uint32]struct{}),
		queues:      make(map[uint32][]uint32),
		cooldowns:   make(map[uint32]uint32),
		visitorFeed: make(map[uint32][]*protobuf.PB_VISITOR),
		objects:     make(map[uint32]map[islandObjectKey]*islandObjectState),
	}
}

func (s *islandRuntimeState) resetForTest() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sessions = make(map[uint32]uint32)
	s.visitors = make(map[uint32]map[uint32]struct{})
	s.queues = make(map[uint32][]uint32)
	s.cooldowns = make(map[uint32]uint32)
	s.visitorFeed = make(map[uint32][]*protobuf.PB_VISITOR)
	s.objects = make(map[uint32]map[islandObjectKey]*islandObjectState)
}

func (s *islandRuntimeState) setCooldownForTest(commanderID uint32, until uint32) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cooldowns[commanderID] = until
}

func (s *islandRuntimeState) clearSessionForTest(commanderID uint32) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.clearSessionLocked(commanderID)
}

func (s *islandRuntimeState) setSessionForTest(commanderID uint32, islandID uint32) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.assignSessionLocked(commanderID, islandID)
}

func (s *islandRuntimeState) hasMatchingSession(commanderID uint32, islandID uint32) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	activeIslandID, ok := s.sessions[commanderID]
	return ok && activeIslandID == islandID
}

func (s *islandRuntimeState) releaseSession(commanderID uint32, islandID uint32) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	activeIslandID, ok := s.sessions[commanderID]
	if !ok || activeIslandID != islandID {
		return false
	}
	s.releaseObjectOwnershipLocked(commanderID, islandID)
	s.clearSessionLocked(commanderID)
	return true
}

func (s *islandRuntimeState) seedIslandObjects(islandID uint32, templates []islandObjectTemplate) []*protobuf.PB_OBJECT {
	s.mu.Lock()
	defer s.mu.Unlock()

	objects := s.objects[islandID]
	if objects == nil {
		objects = make(map[islandObjectKey]*islandObjectState)
		s.objects[islandID] = objects
	}

	out := make([]*protobuf.PB_OBJECT, 0, len(templates))
	for _, template := range templates {
		if template.ID == 0 || template.Type == 0 {
			continue
		}
		key := islandObjectKey{objectType: template.Type, objectID: template.ID}
		state := objects[key]
		if state == nil {
			state = &islandObjectState{status: template.Status, slots: make(map[uint32]uint32)}
			for _, slotID := range template.SlotIDs {
				if slotID == 0 {
					continue
				}
				state.slots[slotID] = 0
			}
			objects[key] = state
		} else {
			if template.Status != 0 {
				state.status = template.Status
			}
			for _, slotID := range template.SlotIDs {
				if slotID == 0 {
					continue
				}
				if _, exists := state.slots[slotID]; !exists {
					state.slots[slotID] = 0
				}
			}
		}
		out = append(out, buildIslandObjectMessage(template.ID, template.Type, state))
	}

	return out
}

func (s *islandRuntimeState) applyIslandControl(commanderID uint32, islandID uint32, objectType uint32, objectID uint32, slotID uint32, op uint32, status uint32) (*protobuf.PB_OBJECT, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if objectType != 1 && objectType != 2 {
		return nil, false
	}
	if objectID == 0 || slotID == 0 {
		return nil, false
	}

	objects := s.objects[islandID]
	if objects == nil {
		return nil, false
	}

	key := islandObjectKey{objectType: objectType, objectID: objectID}
	state := objects[key]
	if state == nil {
		return nil, false
	}

	currentOwner, slotExists := state.slots[slotID]
	if !slotExists {
		return nil, false
	}

	switch op {
	case 1:
		if currentOwner != 0 && currentOwner != commanderID {
			return nil, false
		}
		state.slots[slotID] = commanderID
	case 0:
		if currentOwner != commanderID {
			return nil, false
		}
		state.slots[slotID] = 0
	default:
		return nil, false
	}

	state.status = status
	return buildIslandObjectMessage(objectID, objectType, state), true
}

func (s *islandRuntimeState) islandIDByCommander(commanderID uint32) (uint32, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	islandID, ok := s.sessions[commanderID]
	return islandID, ok
}

func (s *islandRuntimeState) enter(commanderID uint32, commanderName string, islandID uint32, now uint32) (uint32, uint32, uint32) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if until := s.cooldowns[commanderID]; until > now {
		return 19, 0, until
	}

	if currentIslandID, ok := s.sessions[commanderID]; ok && currentIslandID == islandID {
		return 0, 0, 0
	}

	queue := s.queues[islandID]
	if !containsIslandRuntimeUint32(queue, commanderID) {
		queue = append(queue, commanderID)
		s.queues[islandID] = queue
	}

	visitors := s.visitors[islandID]
	if visitors == nil {
		visitors = make(map[uint32]struct{})
		s.visitors[islandID] = visitors
	}

	if len(visitors) < islandVisitCapacity && len(queue) > 0 && queue[0] == commanderID {
		s.assignSessionLocked(commanderID, islandID)
		s.queues[islandID] = queue[1:]
		s.enqueueVisitorEventLocked(islandID, commanderID, commanderName, 1)
		return 0, 0, 0
	}

	position := uint32(indexOfUint32(queue, commanderID) + 1)
	if position == 0 {
		position = 1
	}
	return 6, position, 0
}

func (s *islandRuntimeState) poll(commanderID uint32, commanderName string, islandID uint32, now uint32) (uint32, uint32, uint32) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if until := s.cooldowns[commanderID]; until > now {
		return 19, 0, until
	}

	if currentIslandID, ok := s.sessions[commanderID]; ok && currentIslandID == islandID {
		return 0, 0, 0
	}

	queue := s.queues[islandID]
	if !containsIslandRuntimeUint32(queue, commanderID) {
		return 1, 0, 0
	}

	visitors := s.visitors[islandID]
	if visitors == nil {
		visitors = make(map[uint32]struct{})
		s.visitors[islandID] = visitors
	}

	if len(visitors) < islandVisitCapacity && len(queue) > 0 && queue[0] == commanderID {
		s.assignSessionLocked(commanderID, islandID)
		s.queues[islandID] = queue[1:]
		s.enqueueVisitorEventLocked(islandID, commanderID, commanderName, 1)
		return 0, 0, 0
	}

	position := uint32(indexOfUint32(queue, commanderID) + 1)
	if position == 0 {
		position = 1
	}
	return 6, position, 0
}

func (s *islandRuntimeState) drainVisitorFeed(commanderID uint32) []*protobuf.PB_VISITOR {
	s.mu.Lock()
	defer s.mu.Unlock()

	feed := s.visitorFeed[commanderID]
	if len(feed) == 0 {
		return []*protobuf.PB_VISITOR{}
	}
	out := make([]*protobuf.PB_VISITOR, 0, len(feed))
	for _, item := range feed {
		out = append(out, proto.Clone(item).(*protobuf.PB_VISITOR))
	}
	s.visitorFeed[commanderID] = []*protobuf.PB_VISITOR{}
	return out
}

func (s *islandRuntimeState) enqueueVisitorEventLocked(islandID uint32, visitorID uint32, visitorName string, cmd uint32) {
	visitor := &protobuf.PB_VISITOR{
		Id:   proto.Uint32(visitorID),
		Name: proto.String(visitorName),
		Time: proto.Uint32(0),
		Cmd:  proto.Uint32(cmd),
	}
	for commanderID := range s.visitors[islandID] {
		if commanderID == visitorID {
			continue
		}
		s.visitorFeed[commanderID] = append(s.visitorFeed[commanderID], visitor)
	}
}

func (s *islandRuntimeState) clearSessionLocked(commanderID uint32) {
	islandID, ok := s.sessions[commanderID]
	if !ok {
		return
	}
	delete(s.sessions, commanderID)
	if visitors := s.visitors[islandID]; visitors != nil {
		delete(visitors, commanderID)
	}
}

func (s *islandRuntimeState) releaseObjectOwnershipLocked(commanderID uint32, islandID uint32) {
	objects := s.objects[islandID]
	if objects == nil {
		return
	}
	for _, state := range objects {
		for slotID, ownerID := range state.slots {
			if ownerID == commanderID {
				state.slots[slotID] = 0
			}
		}
	}
}

func (s *islandRuntimeState) assignSessionLocked(commanderID uint32, islandID uint32) {
	s.clearSessionLocked(commanderID)
	if s.visitors[islandID] == nil {
		s.visitors[islandID] = make(map[uint32]struct{})
	}
	s.visitors[islandID][commanderID] = struct{}{}
	s.sessions[commanderID] = islandID
}

func containsIslandRuntimeUint32(values []uint32, target uint32) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func indexOfUint32(values []uint32, target uint32) int {
	for index, value := range values {
		if value == target {
			return index
		}
	}
	return -1
}

func buildIslandObjectMessage(objectID uint32, objectType uint32, state *islandObjectState) *protobuf.PB_OBJECT {
	slotIDs := make([]uint32, 0, len(state.slots))
	for slotID := range state.slots {
		slotIDs = append(slotIDs, slotID)
	}
	sort.Slice(slotIDs, func(i, j int) bool { return slotIDs[i] < slotIDs[j] })

	slots := make([]*protobuf.PB_SLOT, 0, len(slotIDs))
	for _, slotID := range slotIDs {
		ownerID := state.slots[slotID]
		slots = append(slots, &protobuf.PB_SLOT{
			SlotId:  proto.Uint32(slotID),
			OwnerId: proto.Uint32(ownerID),
		})
	}
	return &protobuf.PB_OBJECT{
		Id:     proto.Uint32(objectID),
		Type:   proto.Uint32(objectType),
		Slots:  slots,
		Status: proto.Uint32(state.status),
	}
}
