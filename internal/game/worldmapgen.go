package game

import (
	"encoding/binary"
	"hash/fnv"
	"maps"
	"math/rand/v2"
	"slices"
)

// The world map generator. What stands on a world map (enemies, events,
// boxes) is the official server's content: no client table lists it and
// nothing of the mode was captured, so belfast makes it up from the
// tables and the wiki, deterministically (the same commander, activation
// and map always give the same map):
//
//   - the template a map is played on is drawn among its
//     world_chapter_random.template_id entries whose progress range holds the
//     world's progress, by weight;
//   - the story maps (random ids 200 to 999, the 新手地图 tutorial 200-202
//     among them) have their own events, numbered map id * 100 + n in
//     world_event_data (map 200: 20001-20007), each at the cells its grid
//     names (a rectangle: one of its cells). The map starts with those of
//     them no effect spawns (world_effect_data type 9 lists the events an
//     effect adds): on map 200 the merchant 20001, the elite fleet 20002,
//     the cells around it 20007 and the way out 20005; the rest of the
//     script (stories, the battle with 314001, the teleport to 201) comes
//     from their effects as the fleet triggers them (CS_33103);
//   - the Explorable zones and their Safe Zones start with their template
//     family's controller event (worldMapController), whose effects
//     (CS_33103, run as soon as the client shows the map) spread the map's
//     Siren content: the zone is secured once the events with an
//     infection_value are all gone;
//   - the Obscured and Abyssal zones (the Coordinate Loggers' treasure
//     maps) have no controller: their content is placed from the wiki's
//     description (placeObscuredZone, placeAbyssalZone);
//   - the other maps without one that have a hazard level and no port get
//     enemies from world_expedition_data: the ones numbered (20 + the
//     template's difficulty) * 10000 + n that aren't bosses
//     (expedition_data_template type 99), about one per 15 walkable cells
//     plus the hazard level, 3 to 10, and a supply chest per 50 walkable
//     cells, 1 to 3 (no zone a fleet can reach today is one). Ports and
//     hazard 0 maps get nothing;
//   - the template's terrain becomes the map's land_list;
//   - a map whose template is always in sight (map_sight 1) starts
//     discovered, the others discover the cells around the fleets.

// worldSeed seeds a map's generation: the commander, the activation (a
// reset makes new maps) and the map.
func worldSeed(commanderID, activations, randomID uint32) uint64 {
	hash := fnv.New64a()
	var buf [12]byte
	binary.LittleEndian.PutUint32(buf[0:], commanderID)
	binary.LittleEndian.PutUint32(buf[4:], activations)
	binary.LittleEndian.PutUint32(buf[8:], randomID)
	hash.Write(buf[:])
	return hash.Sum64()
}

func worldRand(seed uint64) *rand.Rand {
	return rand.New(rand.NewPCG(seed, seed^0x9e3779b97f4a7c15))
}

// drawWorldTemplate picks the template randomID is played on at progress.
func drawWorldTemplate(randomID, progress uint32, seed uint64) uint32 {
	entries := worldRandomMaps[randomID].Templates
	var candidates [][4]uint32
	for _, entry := range entries {
		if entry[2] <= progress && progress <= entry[3] {
			candidates = append(candidates, entry)
		}
	}
	if len(candidates) == 0 {
		candidates = entries
	}
	if len(candidates) == 0 {
		return 0
	}
	rng := worldRand(seed)
	pick := weightedPick(func(n uint32) uint32 { return rng.Uint32N(n) }, candidates, func(entry [4]uint32) uint32 { return max(entry[1], 1) })
	return candidates[pick][0]
}

// generateWorldMap makes map randomID on templateID.
func generateWorldMap(randomID, templateID uint32, seed uint64) *worldMap {
	m := &worldMap{RandomID: randomID, TemplateID: templateID, Generation: worldMapGeneration}
	template := worldTemplates[templateID]
	if template == nil {
		return m
	}
	m.Lands = templateLands(template)
	m.AllDiscovered = template.MapSight == 1
	rng := worldRand(seed ^ 0x5bd1e995)
	if family := worldEventFamily(randomID); len(family) > 0 {
		placeFamilyEvents(m, template, family, rng)
		return m
	}
	if randomID == worldForwardBase || placeArbiterZone(m, template, rng) {
		// the Forward Base's device is the month's (refreshForwardBase)
		return m
	}
	switch worldZones[randomID] {
	case worldZoneStronghold:
		if placeStronghold(m, template, rng) {
			return m
		}
	case worldZoneObscured:
		placeObscuredZone(m, template, rng)
		return m
	case worldZoneAbyssal:
		if placeAbyssalZone(m, template, rng) {
			return m
		}
	}
	if id, ok := worldMapController(randomID, templateID); ok {
		if len(spawnWorldEvent(m, template, id, nil, rng)) > 0 {
			return m
		}
	}
	if worldRandomMaps[randomID].HazardLevel == 0 || template.port != 0 {
		return m
	}
	placeWorldEnemies(m, template, rng)
	placeWorldBoxes(m, template, rng)
	return m
}

// templateLands turns a template's terrain into land_list entries.
func templateLands(template *worldTemplate) []worldLand {
	var lands []worldLand
	for _, entry := range template.Terrain {
		if len(entry) < 3 || entry[0] < 0 || entry[1] < 0 {
			continue
		}
		land := worldLand{Row: uint32(entry[0]), Column: uint32(entry[1]), Type: uint32(entry[2])}
		arg := func(i int) int32 {
			if i < len(entry) {
				return entry[i]
			}
			return 0
		}
		switch land.Type {
		case 1: // stream
			land.DirRow, land.DirColumn = arg(3), arg(4)
		case 3: // wind
			land.DirRow, land.DirColumn, land.Distance = arg(3), arg(4), uint32(max(arg(5), 0))
		case 4, 6: // fog, poison
			land.Distance = uint32(max(arg(3), 0))
		}
		lands = append(lands, land)
	}
	return lands
}

// worldMapGeneration is the generator's version (worldMap.Generation): 1
// is the maps started by their template's controller event, 2 the safe
// zones started by their own controller and the Obscured and Abyssal
// zones' content.
const worldMapGeneration = 2

// The controllers' offsets in a template family's events (family * 10000
// + offset): the Explorable zone's content (0), the Safe Zone's (400),
// the Zone Control Reward that appears once an Explorable zone is secured
// (300, it spawns 301 around itself) and the event that ends a cleared
// Safe Zone (401: EffectEventMapClearFlag, the zone starts over once left).
const (
	worldControllerSafe       = 400
	worldControllerSecured    = 300
	worldControllerSafeClosed = 401
)

// worldMapController is the event a map on templateID starts with, the
// rest of its content coming from that event's effects: world_event_data
// has a set per template family (template id / 1000 * 10000, 10440000 for
// 1044001 and 1044002), on cell (0, 0) or (1, 1) and spawned by no effect,
// whose effects all run by themselves (autoactivate, WorldMap:
// CheckInteractive: the client asks for them as soon as the map is shown):
// edge placeholders on the rows and columns fleets arrive on, then
// resource nodes, "Pawn" enemy fleets (their infection_value is the zone's
// Siren infection, WorldMap:GetEventPoisonRate, which must fall to 0
// before the zone is secured: WorldMap:CheckMapPressing) and the other
// events of the family spread over the map, then the placeholders removed.
// The Explorable zone (the entrance's base map) starts with family * 10000
// (C4: 8 "Pawns", 2 Promoted "Pawns", 3 + 1 hidden resource nodes, a
// meowfficer search point), its Safe Zone (complete_chapter, a template of
// the same family) with family * 10000 + 400 (C4: 3 "Pawns", 4 resource
// nodes + 1 hidden cache, a search point): the counts the wiki's
// Explorable / Secured tables give for every corrosion level. That the
// official server starts its maps this way is a reading of the tables
// (nothing of the mode was captured).
func worldMapController(randomID, templateID uint32) (uint32, bool) {
	id := templateID / 1000 * 10000
	if worldZones[randomID] == worldZoneSafe {
		id += worldControllerSafe
	}
	_, ok := worldEvents[id]
	return id, ok && id != 0 && !worldSpawnedEvents[id]
}

// worldFamilyEvent is event offset of templateID's family, 0 for none.
func worldFamilyEvent(templateID, offset uint32) uint32 {
	id := templateID/1000*10000 + offset
	if _, ok := worldEvents[id]; !ok || templateID < 1000000 {
		return 0
	}
	return id
}

// worldEventFamily are a story map's own events (map id * 100 + n), by id.
func worldEventFamily(randomID uint32) []uint32 {
	if randomID < 200 || randomID >= 1000 {
		return nil
	}
	var ids []uint32
	for id := randomID * 100; id < randomID*100+100; id++ {
		if _, ok := worldEvents[id]; ok {
			ids = append(ids, id)
		}
	}
	return ids
}

// gridCells are the template cells a world_event_data grid entry names, nil
// for offsets (negative) from a spawning event.
func gridCells(template *worldTemplate, entry []int32) [][2]uint32 {
	if len(entry) == 2 {
		entry = []int32{entry[0], entry[1], entry[0], entry[1]}
	}
	if len(entry) != 4 || slices.ContainsFunc(entry, func(v int32) bool { return v < 0 }) {
		return nil
	}
	var cells [][2]uint32
	for row := min(entry[0], entry[2]); row <= max(entry[0], entry[2]); row++ {
		for column := min(entry[1], entry[3]); column <= max(entry[1], entry[3]); column++ {
			if template.hasCell(uint32(row), uint32(column)) {
				cells = append(cells, [2]uint32{uint32(row), uint32(column)})
			}
		}
	}
	return cells
}

// newWorldEvent is an event attachment as it starts: its effects all to run.
func newWorldEvent(id, row, column uint32) *worldAttachment {
	return &worldAttachment{Row: row, Column: column, Type: worldAttachEvent, ID: id, Effects: slices.Clone(worldEvents[id].Effect)}
}

// worldWall tells whether an event is a wall of signs (obstacle 3, nothing
// gets on it; is_guide: the client plays its story when a fleet walks up
// to it): it fills the rectangles of its grid, where the other events take
// one cell of each.
func worldWall(id uint32) bool {
	return worldEvents[id].Obstacle == 3 && worldEvents[id].IsGuide == 1
}

// spawnWorldEvent adds an event at a cell of each entry of its grid
// (spawnCell), what an effect of type 9 does (the later map operations
// reuse it), fleets standing where they are; it returns the attachments
// added.
func spawnWorldEvent(m *worldMap, template *worldTemplate, id uint32, fleets []*worldFleet, rng *rand.Rand) []*worldAttachment {
	var added []*worldAttachment
	var groups [][][2]uint32
	for _, entry := range worldEvents[id].Grid {
		if cells := gridCells(template, entry); len(cells) > 0 {
			groups = append(groups, cells)
		}
	}
	// a scripted event goes to one cell of its whole grid (spawnEvents)
	if worldScripted(id) && !worldWall(id) && len(groups) > 1 {
		groups = [][][2]uint32{slices.Concat(groups...)}
	}
	for _, cells := range groups {
		picks := [][2]uint32{spawnCell(m, template, cells, fleets, rng)}
		if worldWall(id) {
			picks = cells
		}
		for _, cell := range picks {
			if attachment := newWorldEvent(id, cell[0], cell[1]); m.addAttachment(attachment) {
				added = append(added, attachment)
			}
		}
	}
	return added
}

// spawnCell picks the cell of cells an event spawns on: a walkable one
// nothing (no attachment, no fleet) stands on, else any walkable one, else
// any; at random. An event a fleet can't reach (a "Pawn" on a rock) would
// keep its zone from ever being secured.
func spawnCell(m *worldMap, template *worldTemplate, cells [][2]uint32, fleets []*worldFleet, rng *rand.Rand) [2]uint32 {
	var free, walkable [][2]uint32
	for _, cell := range cells {
		if !template.isWalkable(cell[0], cell[1]) {
			continue
		}
		walkable = append(walkable, cell)
		if len(m.cellAttachments(cell[0], cell[1])) == 0 && !fleetAt(cell, fleets, nil) {
			free = append(free, cell)
		}
	}
	for _, pool := range [][][2]uint32{free, walkable, cells} {
		if len(pool) > 0 {
			return pool[rng.IntN(len(pool))]
		}
	}
	return cells[0]
}

func placeFamilyEvents(m *worldMap, template *worldTemplate, family []uint32, rng *rand.Rand) {
	for _, id := range family {
		if !worldSpawnedEvents[id] {
			spawnWorldEvent(m, template, id, nil, rng)
		}
	}
}

// freeWorldCells are the walkable cells nothing stands on, away from the
// port, in a stable order.
func freeWorldCells(m *worldMap, template *worldTemplate) [][2]uint32 {
	var cells [][2]uint32
	for _, cell := range template.Grids {
		if !cell.Walkable || len(m.cellAttachments(cell.Row, cell.Column)) > 0 {
			continue
		}
		if template.port != 0 && chebyshev([2]uint32{cell.Row, cell.Column}, template.portCenter) <= 2 {
			continue
		}
		cells = append(cells, [2]uint32{cell.Row, cell.Column})
	}
	slices.SortFunc(cells, func(a, b [2]uint32) int {
		if a[0] != b[0] {
			return int(a[0]) - int(b[0])
		}
		return int(a[1]) - int(b[1])
	})
	return cells
}

func chebyshev(a, b [2]uint32) int {
	return max(abs(int(a[0])-int(b[0])), abs(int(a[1])-int(b[1])))
}

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

func walkableCount(template *worldTemplate) int {
	n := 0
	for _, cell := range template.Grids {
		if cell.Walkable {
			n++
		}
	}
	return n
}

// worldEnemyPool are the enemies a template's difficulty draws from.
func worldEnemyPool(difficulty uint32) []uint32 {
	var pool []uint32
	for _, id := range slices.Sorted(maps.Keys(worldExpeditions)) {
		expedition, ok := expeditionTemplates[id]
		if ok && id/10000 == 20+difficulty && expedition.Type != 99 {
			pool = append(pool, id)
		}
	}
	return pool
}

// edgeCell tells whether a cell is on the rows or columns the map's cells
// start or end on, where fleets come in from the neighbouring maps.
func edgeCell(template *worldTemplate, cell [2]uint32) bool {
	minRow, minColumn, maxRow, maxColumn := ^uint32(0), ^uint32(0), uint32(0), uint32(0)
	for _, c := range template.Grids {
		minRow, maxRow = min(minRow, c.Row), max(maxRow, c.Row)
		minColumn, maxColumn = min(minColumn, c.Column), max(maxColumn, c.Column)
	}
	return cell[0] == minRow || cell[0] == maxRow || cell[1] == minColumn || cell[1] == maxColumn
}

func placeWorldEnemies(m *worldMap, template *worldTemplate, rng *rand.Rand) {
	pool := worldEnemyPool(template.Difficulty)
	if len(pool) == 0 {
		pool = worldEnemyPool(1)
	}
	if len(pool) == 0 {
		return
	}
	hazard := int(worldRandomMaps[m.RandomID].HazardLevel)
	count := min(max(walkableCount(template)/15+hazard, 3), 10)
	var placed [][2]uint32
	cells := slices.DeleteFunc(freeWorldCells(m, template), func(cell [2]uint32) bool { return edgeCell(template, cell) })
	rng.Shuffle(len(cells), func(i, j int) { cells[i], cells[j] = cells[j], cells[i] })
	for _, cell := range cells {
		if len(placed) == count {
			break
		}
		if slices.ContainsFunc(placed, func(other [2]uint32) bool { return chebyshev(cell, other) < 2 }) {
			continue
		}
		m.addAttachment(&worldAttachment{Row: cell[0], Column: cell[1], Type: worldAttachEnemy, ID: pool[rng.IntN(len(pool))], Data: 1})
		placed = append(placed, cell)
	}
}

func placeWorldBoxes(m *worldMap, template *worldTemplate, rng *rand.Rand) {
	var boxes []uint32
	for _, id := range slices.Sorted(maps.Keys(boxTemplates)) {
		if box := boxTemplates[id]; box.Type == 1 && box.Name == boxTemplates[1].Name {
			boxes = append(boxes, id)
		}
	}
	cells := freeWorldCells(m, template)
	if len(boxes) == 0 || len(cells) == 0 {
		return
	}
	count := min(max(walkableCount(template)/50, 1), 3)
	rng.Shuffle(len(cells), func(i, j int) { cells[i], cells[j] = cells[j], cells[i] })
	for _, cell := range cells[:min(count, len(cells))] {
		m.addAttachment(&worldAttachment{Row: cell[0], Column: cell[1], Type: worldAttachBox, ID: boxes[rng.IntN(len(boxes))]})
	}
}

// worldStartCells are where n fleets start on a map: the cells around its
// port when it has one (where WorldMap:InPort finds them in port), else the
// free walkable cell farthest from anything on the map, then the free cells
// nearest it.
func worldStartCells(m *worldMap, n int) [][2]uint32 {
	template := worldTemplates[m.TemplateID]
	if template == nil || n == 0 {
		return nil
	}
	var cells, port [][2]uint32
	for _, cell := range template.Grids {
		c := [2]uint32{cell.Row, cell.Column}
		// not in a Siren fog either (an Arbiter's zone is half one)
		if !cell.Walkable || len(m.cellAttachments(cell.Row, cell.Column)) > 0 || !m.cellAllows(c, worldObstacleArrive) {
			continue
		}
		if template.isPortCell(cell.Row, cell.Column) {
			port = append(port, c)
		} else {
			cells = append(cells, c)
		}
	}
	byPos := func(a, b [2]uint32) int {
		if a[0] != b[0] {
			return int(a[0]) - int(b[0])
		}
		return int(a[1]) - int(b[1])
	}
	slices.SortFunc(port, byPos)
	slices.SortFunc(cells, byPos)
	if len(port) > 0 {
		return append(port, cells...)[:min(n, len(port)+len(cells))]
	}
	if len(cells) == 0 {
		return nil
	}
	if worldZones[m.RandomID] == worldZoneAbyssal {
		// the Siren on the top rows, the fleets come in at the bottom
		// middle, its corners on either side of them
		bottom, middle := uint32(0), uint32(0)
		for _, cell := range template.Grids {
			bottom, middle = max(bottom, cell.Row), max(middle, cell.Column)
		}
		entry := [2]uint32{bottom, middle / 2}
		slices.SortStableFunc(cells, func(a, b [2]uint32) int { return chebyshev(a, entry) - chebyshev(b, entry) })
		return cells[:min(n, len(cells))]
	}
	first, best := cells[0], -1
	for _, cell := range cells {
		nearest := 1 << 30
		for _, attachment := range m.Attachments {
			nearest = min(nearest, chebyshev(cell, [2]uint32{attachment.Row, attachment.Column}))
		}
		if nearest > best {
			first, best = cell, nearest
		}
	}
	rest := slices.DeleteFunc(cells, func(c [2]uint32) bool { return c == first })
	slices.SortStableFunc(rest, func(a, b [2]uint32) int { return chebyshev(a, first) - chebyshev(b, first) })
	return append([][2]uint32{first}, rest...)[:min(n, len(rest)+1)]
}

// discoverAround marks discovered the cells in a fleet's sight at row,
// column (WorldConst.InFOVRange with world_move_initial_view), what the
// client would otherwise ask for with OpReqDiscover as it lands.
func discoverAround(m *worldMap, row, column uint32) {
	template := worldTemplates[m.TemplateID]
	if template == nil || m.AllDiscovered {
		return
	}
	radius := int(worldSettings.initialView)
	var cells [][2]uint32
	for dr := -radius; dr <= radius; dr++ {
		for dc := -radius; dc <= radius; dc++ {
			r, c := int(row)+dr, int(column)+dc
			if r < 0 || c < 0 || dr*dr+dc*dc > radius*radius {
				continue
			}
			if template.hasCell(uint32(r), uint32(c)) {
				cells = append(cells, [2]uint32{uint32(r), uint32(c)})
			}
		}
	}
	m.discover(cells...)
}

// The Obscured Zones (the Map Coordinates' treasure maps, "Hidden
// Zones"): the wiki gives R2 1 Siren Rally Point, R3 and R4 2, R5 and R6
// 3, each guarded by a fleet (its own effects: the battle, then the
// rally's drop), fleets around them from the start that give exp and
// rewards (world_event_data's "Mass-Produced Siren Fleet" 1009001-1009003,
// what the root Rally Points 1000105... spawn around themselves), and once
// every Rally Point is destroyed the rewards and the way out (a Siren
// Transference Device by the map's size: Resource Crates around it, then
// the story option that teleports the fleets back, EffectEventTeleportBack).
// The rally variants are the weather (1000200), fog (1000300) and current
// (1000400) ones, at the rally grid's cells. What the official server
// runs besides (the trigger effects 25/26/31 that bring reinforcements and
// the control devices) isn't in the client's tables: no reinforcement
// spawns here, the rallies are reachable from the start.
var (
	obscuredRallyCount  = map[uint32]int{1: 1, 2: 1, 3: 2, 4: 2, 5: 3, 6: 3}
	obscuredRallyEvents = []uint32{1000200, 1000300, 1000400}
	obscuredFleetEvents = []uint32{1009001, 1009002, 1009003}
	// by map size: up to 9, 15, 21 rows (grids [2, 2, 6, 6], [2, 2, 12, 12],
	// [2, 2, 18, 18])
	obscuredExitEvents = []uint32{1000032, 1000033, 1000034}
)

const worldRallyName = "Siren Rally Point"

func placeObscuredZone(m *worldMap, template *worldTemplate, rng *rand.Rand) {
	n := obscuredRallyCount[worldRandomMaps[m.RandomID].HazardLevel]
	if n == 0 {
		n = 1
	}
	var slots [][2]uint32
	for _, entry := range worldEvents[obscuredRallyEvents[0]].Grid {
		for _, cell := range gridCells(template, entry) {
			if template.isWalkable(cell[0], cell[1]) {
				slots = append(slots, cell)
			}
		}
	}
	rng.Shuffle(len(slots), func(i, j int) { slots[i], slots[j] = slots[j], slots[i] })
	if len(slots) < n {
		cells := slices.DeleteFunc(freeWorldCells(m, template), func(cell [2]uint32) bool { return edgeCell(template, cell) })
		rng.Shuffle(len(cells), func(i, j int) { cells[i], cells[j] = cells[j], cells[i] })
		for _, cell := range cells {
			if len(slots) == n {
				break
			}
			if !slices.ContainsFunc(slots, func(other [2]uint32) bool { return chebyshev(cell, other) < 4 }) {
				slots = append(slots, cell)
			}
		}
	}
	events := slices.Clone(obscuredRallyEvents)
	rng.Shuffle(len(events), func(i, j int) { events[i], events[j] = events[j], events[i] })
	for i, cell := range slots[:min(n, len(slots))] {
		rally := newWorldEvent(events[i%len(events)], cell[0], cell[1])
		rally.Effects = slices.DeleteFunc(rally.Effects, inertSpawn)
		if !m.addAttachment(rally) {
			continue
		}
		for _, id := range obscuredFleetEvents {
			spawnRelative(m, template, rally, id, nil, rng)
		}
	}
}

// inertSpawn tells whether an effect only spawns placeholders (unnamed
// events without effects, on the map's edges), which the official
// server's triggers take away again: left alone they would stay.
func inertSpawn(id uint32) bool {
	effect, ok := worldEffects[id]
	if !ok || effect.EffectType != worldEffectSpawnEvents {
		return false
	}
	ids := paramUints(effectParams(effect))
	return len(ids) > 0 && !slices.ContainsFunc(ids, func(id uint32) bool {
		ev := worldEvents[id]
		return ev.Name != "" || len(ev.Effect) > 0 || len(ev.EventOp) > 0
	})
}

// spawnRelative adds event id at one cell of its grid, read as offsets
// from the event from (a free cell first, spawnCell); nil when none fits.
func spawnRelative(m *worldMap, template *worldTemplate, from *worldAttachment, id uint32, fleets []*worldFleet, rng *rand.Rand) *worldAttachment {
	var cells [][2]uint32
	for _, entry := range worldEvents[id].Grid {
		if len(entry) == 2 {
			entry = []int32{entry[0], entry[1], entry[0], entry[1]}
		}
		if len(entry) != 4 {
			continue
		}
		for dr := min(entry[0], entry[2]); dr <= max(entry[0], entry[2]); dr++ {
			for dc := min(entry[1], entry[3]); dc <= max(entry[1], entry[3]); dc++ {
				r, c := int32(from.Row)+dr, int32(from.Column)+dc
				if r >= 0 && c >= 0 && template.hasCell(uint32(r), uint32(c)) {
					cells = append(cells, [2]uint32{uint32(r), uint32(c)})
				}
			}
		}
	}
	if len(cells) == 0 {
		return nil
	}
	cell := spawnCell(m, template, cells, fleets, rng)
	attachment := newWorldEvent(id, cell[0], cell[1])
	if !m.addAttachment(attachment) {
		return nil
	}
	return attachment
}

// obscuredExit spawns an Obscured Zone's way out once its Siren Rally
// Points are all destroyed (their events done), unless it is there; it
// returns the attachment added.
func obscuredExit(m *worldMap, rng *rand.Rand) *worldAttachment {
	template := worldTemplates[m.TemplateID]
	if worldZones[m.RandomID] != worldZoneObscured || template == nil {
		return nil
	}
	for _, a := range m.Attachments {
		if a.Type == worldAttachEvent && (worldEvents[a.ID].Name == worldRallyName || slices.Contains(obscuredExitEvents, a.ID)) {
			return nil
		}
	}
	rows := uint32(0)
	for _, cell := range template.Grids {
		rows = max(rows, cell.Row+1)
	}
	exit := obscuredExitEvents[0]
	switch {
	case rows > 15:
		exit = obscuredExitEvents[2]
	case rows > 9:
		exit = obscuredExitEvents[1]
	}
	if added := spawnWorldEvent(m, template, exit, nil, rng); len(added) > 0 {
		return added[0]
	}
	return nil
}

// The Abyssal Zones (the Siren Coordinates' treasure maps, templates
// 140004-140006): world_event_data 4100000 (cell (4, 6)) spawns the
// Abyssal Siren of the map's difficulty (one of five, 4100052... for C4),
// whose death spawns the way out (4100002, then 4100001); 4100010 and
// 4100011 a fleet each in the fog of the bottom corners, the fleets the
// wiki says clear the Low Resolve the Siren can't be fought with. The
// zone's walls and blockade devices (4100101-4100106, 4100200...) need the
// server's triggers to be lifted: they are left out, the corners open.
var abyssalRootEvents = []uint32{4100000, 4100010, 4100011}

func placeAbyssalZone(m *worldMap, template *worldTemplate, rng *rand.Rand) bool {
	if !template.hasCell(4, 6) {
		return false
	}
	for _, id := range abyssalRootEvents {
		// one each: the corners' grids are two rectangles of one corner
		var cells [][2]uint32
		for _, entry := range worldEvents[id].Grid {
			cells = append(cells, gridCells(template, entry)...)
		}
		if len(cells) > 0 {
			cell := spawnCell(m, template, cells, nil, rng)
			m.addAttachment(newWorldEvent(id, cell[0], cell[1]))
		}
	}
	return true
}
