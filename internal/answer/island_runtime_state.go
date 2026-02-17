package answer

import (
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
}

var globalIslandRuntimeState = newIslandRuntimeState()

func newIslandRuntimeState() *islandRuntimeState {
	return &islandRuntimeState{
		sessions:    make(map[uint32]uint32),
		visitors:    make(map[uint32]map[uint32]struct{}),
		queues:      make(map[uint32][]uint32),
		cooldowns:   make(map[uint32]uint32),
		visitorFeed: make(map[uint32][]*protobuf.PB_VISITOR),
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
	s.clearSessionLocked(commanderID)
	if s.visitors[islandID] == nil {
		s.visitors[islandID] = make(map[uint32]struct{})
	}
	s.visitors[islandID][commanderID] = struct{}{}
	s.sessions[commanderID] = islandID
}

func (s *islandRuntimeState) hasMatchingSession(commanderID uint32, islandID uint32) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	activeIslandID, ok := s.sessions[commanderID]
	return ok && activeIslandID == islandID
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
		s.sessions[commanderID] = islandID
		visitors[commanderID] = struct{}{}
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
		s.sessions[commanderID] = islandID
		visitors[commanderID] = struct{}{}
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
