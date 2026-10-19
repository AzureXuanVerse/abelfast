package game

import "slices"

// The rules of a world map's cells the map operations follow
// (mod/experiment/world/model/worldmapcell.lua, worldmap.lua,
// WorldConst.GetObstacleConfig): whether a fleet can leave a cell, stop on
// it or pass through it, and the ways through.

// WorldMapCell terrain types (LANDINFO.type).
const (
	worldTerrainStream = 1
	worldTerrainIce    = 2
	worldTerrainWind   = 3
	worldTerrainFog    = 4
)

// The obstacle keys (WorldConst.GetObstacleKey).
const (
	worldObstacleLeave  = 4
	worldObstacleArrive = 2
	worldObstaclePass   = 1
)

// worldObstacleMasks is WorldConst.ObstacleConfig: what a world_event_data
// obstacle value allows (leave, arrive, pass bits).
var worldObstacleMasks = map[uint32]uint32{0: 2, 1: 3, 2: 7, 3: 0, 4: 6, 5: 1, 6: 4, 7: 5}

// worldTerrainMasks is WorldMapCell.TerrainObstacleConfig (a fog of
// strength 0 is a Siren fog, 4).
var worldTerrainMasks = map[uint32]uint32{0: 7, worldTerrainStream: 6, worldTerrainIce: 6, worldTerrainWind: 2, worldTerrainFog: 6, 5: 7, 6: 7}

// alive is WorldMapAttachment:IsAlive.
func (a *worldAttachment) alive() bool {
	switch a.Type {
	case worldAttachEvent:
		return true
	case worldAttachEnemy, worldAttachBoss, worldAttachEnemyAI:
		return a.Flag != 1 && a.Data != 0
	case worldAttachTransport:
		return a.Data > 0
	case worldAttachArtifact:
		return false
	}
	return a.Flag != 1
}

// allows tells whether an attachment lets a fleet do key on its cell
// (WorldMapAttachment:CanLeave / CanArrive / CanPass).
func (a *worldAttachment) allows(key uint32) bool {
	switch a.Type {
	case worldAttachEnemy, worldAttachBoss, worldAttachEnemyAI:
		return key == worldObstacleArrive
	case worldAttachEvent:
		return worldObstacleMasks[worldEvents[a.ID].Obstacle]&key != 0
	}
	return true
}

// land is a cell's terrain, false for none.
func (m *worldMap) land(cell [2]uint32) (worldLand, bool) {
	for _, land := range m.Lands {
		if land.Row == cell[0] && land.Column == cell[1] {
			return land, true
		}
	}
	return worldLand{}, false
}

// cellAllows is WorldMapCell's CanLeave / CanArrive / CanPass: the cell is
// walkable, its terrain and every alive attachment on it allow key.
func (m *worldMap) cellAllows(cell [2]uint32, key uint32) bool {
	template := worldTemplates[m.TemplateID]
	if template == nil || !template.isWalkable(cell[0], cell[1]) {
		return false
	}
	terrain := uint32(0)
	if land, ok := m.land(cell); ok {
		terrain = land.Type
		if land.Type == worldTerrainFog && land.Distance == 0 {
			if 4&key == 0 {
				return false
			}
			terrain = ^uint32(0)
		}
	}
	if mask, ok := worldTerrainMasks[terrain]; ok && mask&key == 0 {
		return false
	}
	for _, a := range m.cellAttachments(cell[0], cell[1]) {
		if a.alive() && !a.allows(key) {
			return false
		}
	}
	return true
}

// fleetAt tells whether one of fleets other than self stands on cell.
func fleetAt(cell [2]uint32, fleets []*worldFleet, self *worldFleet) bool {
	return slices.ContainsFunc(fleets, func(fleet *worldFleet) bool {
		return fleet != self && !fleet.Submarine && fleetCell(fleet) == cell
	})
}

// stayPoint is WorldMap:IsStayPoint: a fleet can stop there.
func (m *worldMap) stayPoint(cell [2]uint32, fleets []*worldFleet, self *worldFleet) bool {
	return m.cellAllows(cell, worldObstacleArrive) && !fleetAt(cell, fleets, self)
}

// walkable is WorldMap:IsWalkable.
func (m *worldMap) walkable(cell [2]uint32, fleets []*worldFleet, self *worldFleet) bool {
	return m.cellAllows(cell, worldObstacleLeave) || m.stayPoint(cell, fleets, self)
}

var worldSteps = [4][2]int{{1, 0}, {-1, 0}, {0, 1}, {0, -1}}

func worldNeighbour(cell [2]uint32, step [2]int) ([2]uint32, bool) {
	r, c := int(cell[0])+step[0], int(cell[1])+step[1]
	if r < 0 || c < 0 {
		return [2]uint32{}, false
	}
	return [2]uint32{uint32(r), uint32(c)}, true
}

// findPath is WorldMap:FindPath from a fleet's cell: the shortest way to
// to through walkable cells that don't block (to itself must be a cell it
// can stop on), without from; nil for none.
func (m *worldMap) findPath(from, to [2]uint32, fleets []*worldFleet, self *worldFleet) [][2]uint32 {
	if !m.walkable(to, fleets, self) || !m.stayPoint(to, fleets, self) {
		return nil
	}
	parents := map[[2]uint32][2]uint32{from: from}
	queue := [][2]uint32{from}
	for len(queue) > 0 {
		cell := queue[0]
		queue = queue[1:]
		if cell == to {
			break
		}
		if cell != from && !m.cellAllows(cell, worldObstaclePass) {
			continue
		}
		for _, step := range worldSteps {
			next, ok := worldNeighbour(cell, step)
			if _, seen := parents[next]; !ok || seen || !m.walkable(next, fleets, self) {
				continue
			}
			parents[next] = cell
			queue = append(queue, next)
		}
	}
	if _, ok := parents[to]; !ok {
		return nil
	}
	var path [][2]uint32
	for cell := to; cell != from; cell = parents[cell] {
		path = append(path, cell)
	}
	slices.Reverse(path)
	return path
}

// slide carries a fleet on from the end of path: along a stream's
// direction while the next cell takes it, then over ice in the direction
// it came from (the client only animates what move_path says).
func (m *worldMap) slide(path [][2]uint32, fleets []*worldFleet, self *worldFleet) [][2]uint32 {
	for range 64 {
		end := path[len(path)-1]
		land, ok := m.land(end)
		if !ok {
			return path
		}
		var step [2]int
		switch land.Type {
		case worldTerrainStream:
			step = [2]int{int(land.DirRow), int(land.DirColumn)}
		case worldTerrainIce:
			if len(path) < 2 {
				return path
			}
			prev := path[len(path)-2]
			step = [2]int{int(end[0]) - int(prev[0]), int(end[1]) - int(prev[1])}
		default:
			return path
		}
		next, ok := worldNeighbour(end, step)
		if !ok || step == [2]int{} || !m.stayPoint(next, fleets, self) || slices.Contains(path, next) {
			return path
		}
		path = append(path, next)
	}
	return path
}

// stageEnemy is WorldMapCell:GetStageEnemy: the alive enemy on a cell.
func (m *worldMap) stageEnemy(cell [2]uint32) *worldAttachment {
	for _, a := range m.cellAttachments(cell[0], cell[1]) {
		if a.alive() && (a.Type == worldAttachEnemy || a.Type == worldAttachBoss || a.Type == worldAttachEnemyAI) {
			return a
		}
	}
	return nil
}

// occupied is WorldMap:ExistAny: something alive or a fleet on the cell.
func (m *worldMap) occupied(cell [2]uint32, fleets []*worldFleet) bool {
	if fleetAt(cell, fleets, nil) {
		return true
	}
	return slices.ContainsFunc(m.cellAttachments(cell[0], cell[1]), (*worldAttachment).alive)
}

// blankPath is the way from a cell to the nearest walkable one nothing
// stands on (WorldMap:FindNearestBlankPoint), nil for none.
func (m *worldMap) blankPath(from [2]uint32, fleets []*worldFleet) [][2]uint32 {
	parents := map[[2]uint32][2]uint32{from: from}
	queue := [][2]uint32{from}
	for len(queue) > 0 {
		cell := queue[0]
		queue = queue[1:]
		for _, step := range worldSteps {
			next, ok := worldNeighbour(cell, step)
			if _, seen := parents[next]; !ok || seen || !m.walkable(next, fleets, nil) {
				continue
			}
			parents[next] = cell
			if !m.occupied(next, fleets) {
				var path [][2]uint32
				for c := next; c != from; c = parents[c] {
					path = append(path, c)
				}
				slices.Reverse(path)
				return path
			}
			queue = append(queue, next)
		}
	}
	return nil
}

// stayPath is the way from a cell to the nearest one self can stop on
// that holds no alive enemy (an event may be there), nil for none.
func (m *worldMap) stayPath(from [2]uint32, fleets []*worldFleet, self *worldFleet) [][2]uint32 {
	parents := map[[2]uint32][2]uint32{from: from}
	queue := [][2]uint32{from}
	for len(queue) > 0 {
		cell := queue[0]
		queue = queue[1:]
		for _, step := range worldSteps {
			next, ok := worldNeighbour(cell, step)
			if _, seen := parents[next]; !ok || seen || !m.walkable(next, fleets, self) {
				continue
			}
			parents[next] = cell
			if m.stayPoint(next, fleets, self) && m.stageEnemy(next) == nil {
				var path [][2]uint32
				for c := next; c != from; c = parents[c] {
					path = append(path, c)
				}
				slices.Reverse(path)
				return path
			}
			queue = append(queue, next)
		}
	}
	return nil
}

// nearestFree are up to n walkable cells nothing stands on, nearest
// first, around a cell.
func (m *worldMap) nearestFree(from [2]uint32, n int) [][2]uint32 {
	var cells [][2]uint32
	seen := map[[2]uint32]bool{from: true}
	queue := [][2]uint32{from}
	for len(queue) > 0 && len(cells) < n {
		cell := queue[0]
		queue = queue[1:]
		for _, step := range worldSteps {
			next, ok := worldNeighbour(cell, step)
			if !ok || seen[next] || !worldTemplates[m.TemplateID].isWalkable(next[0], next[1]) {
				continue
			}
			seen[next] = true
			queue = append(queue, next)
			if len(cells) < n && m.stayPoint(next, nil, nil) && !m.occupied(next, nil) {
				cells = append(cells, next)
			}
		}
	}
	return cells
}

// removeAttachment takes an attachment off the map.
func (m *worldMap) removeAttachment(a *worldAttachment) {
	m.Attachments = slices.DeleteFunc(m.Attachments, func(other *worldAttachment) bool { return other == a })
}
