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
// tables, deterministically (the same commander, activation and map always
// give the same map):
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
//   - the other maps start with their template's controller event
//     (worldMapController), whose effects (CS_33103, run as soon as the
//     client shows the map) spread the map's Siren content: the zone is
//     secured once the events with an infection_value are all gone;
//   - the maps without one (none of the open sea zones lacks one) that
//     have a hazard level and no port get enemies from
//     world_expedition_data: the ones numbered (20 + the template's
//     difficulty) * 10000 + n that aren't bosses (expedition_data_template
//     type 99), about one per 15 walkable cells plus the hazard level, 3 to
//     10, at least a cell apart, away from the map's edge rows/columns
//     fleets arrive on; and a supply chest (box_data_template "Supply
//     Chest") per 50 walkable cells, 1 to 3. Ports and safe zones (hazard 0)
//     get nothing;
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
	if id, ok := worldMapController(templateID); ok {
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
// is the maps started by their template's controller event.
const worldMapGeneration = 1

// worldMapController is the event a map on templateID starts with, the
// rest of its content coming from that event's effects: world_event_data
// has one per template family (template id / 1000 * 10000, 10440000 for
// 1044001 and 1044002), on cell (0, 0) and spawned by no effect, whose
// effects all run by themselves (autoactivate, WorldMap:CheckInteractive:
// the client asks for them as soon as the map is shown): edge placeholders
// on the rows and columns fleets arrive on, then resource nodes, "Pawn"
// enemy fleets (their infection_value is the zone's Siren infection,
// WorldMap:GetEventPoisonRate, which must fall to 0 before the zone is
// secured: WorldMap:CheckMapPressing) and the other events of the family
// spread over the map, then the placeholders removed. That the official
// server starts its maps this way is a reading of the tables (nothing of
// the mode was captured).
func worldMapController(templateID uint32) (uint32, bool) {
	id := templateID / 1000 * 10000
	_, ok := worldEvents[id]
	return id, ok && id != 0 && !worldSpawnedEvents[id]
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
	for _, entry := range worldEvents[id].Grid {
		cells := gridCells(template, entry)
		if len(cells) == 0 {
			continue
		}
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
		if !cell.Walkable || len(m.cellAttachments(cell.Row, cell.Column)) > 0 {
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
