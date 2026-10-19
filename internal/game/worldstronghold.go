package game

import (
	"maps"
	"math/rand/v2"
	"slices"
	"time"

	"github.com/ggmolly/belfast/internal/gamedata"
)

// The Siren Strongholds (the wiki's Operation Siren page, "Siren
// Stronghold"; the client's "area_yaosai", the red vortexes): an entrance
// in WORLDINFO.sairen_chapter offers its world_chapter_colormask
// sairen_chapter map (WorldAtlas:SetSairenEntranceList,
// World:EntranceToReplacementMapList), world_chapter_random 30001-30064
// "Siren-Controlled Zone": hazard 7, enter_cost 200 (paid once: the map
// stays in chapter_list until it is cleared), templates is_sairen 50.
//
// From the tables:
//   - the zone's Siren control is the infection_value of its events
//     against is_sairen (WorldMap:GetEventPoisonRate): the four nodes'
//     "Siren Stronghold Flagship Fleet" (world_event_data 600x410-600x412)
//     and the final boss (6010410-6010413, a Defense Module III Upgraded:
//     sairenvalue 2000, 120 * 1.2 = the wiki's 144) are 10 each, 50 in
//     all; once they are gone the zone is cleared and starts over once left
//     (WorldMap:NeedClear: the client takes the entrance off sairen_chapter
//     then, WorldProxy:NetUpdateActiveMap);
//   - the zones are built from the structures of world_event_data
//     6000000-6099999 (the server puts them, the client runs nothing of
//     them by itself): a Siren node (6001000-6005000, one variant per node:
//     each has its own ids, which its effects name) walls a 2x2 inside in
//     (600x100, "qiangsairen1"), with two gates on its walls (a
//     Mass-Produced Siren Fleet, a "Suspicious Node" ambush or a resource)
//     and inside a device (current, fog, energy matrix, resource) and the
//     flagship, whose death runs its node's capture: its gates' fleets
//     gone, its walls broken (600x101), the node counter down (6000100 + n
//     Siren nodes left, "There are n Siren nodes remaining"), an allied
//     fleet on it (600x430); the final node (6010000) walls a 3x3 in with
//     one gate, a "Siren Blockade Device" ("disabled when Siren control
//     over the zone falls under 50%"), the boss and the Siren Transference
//     Device (6010500, working once the zone is dealt with: 6000303's
//     "The Siren Stronghold has been dealt with" and its swap to 6010501,
//     whose teleport goes to Liverpool); an allied node (6020000, 6021000)
//     is the one a retake goes for: a Siren fleet (6020410, "Obstructor
//     Mk. II") "on course toward node under allied control", which beaten
//     gives Siren Crates (6020440), else retakes the node (6020420: the
//     allied fleet becomes a Siren one, 6020430, the counter up, "Node has
//     been recaptured by Sirens. Siren control over this zone has risen.");
//   - the rewards are the drops their events end on, the wiki's columns:
//     10024 a flagship's ("Bosses"), 10023 a retaking fleet's
//     ("Reinforcement Boss"), 10026 the crates' ("Retake Bonus"), 10025
//     the final boss's (worldEventLoot);
//   - the server's settings the client never reads: gameset
//     world_sairen_openstage (the progress they need), world_sairen_max2
//     (3 shown at once, as the wiki says), world_sairen_weekly ({1, 1}:
//     one more each Monday, as the wiki says).
//
// Not in the tables (the wiki's, or guesses): where the structures go
// (strongholdSites: the final node nearest the centre, the four Siren
// nodes the farthest apart, in open water), which wall cells a gate opens
// (the cells a gate stands on: openGates), the two strongholds of a
// monthly reset (the wiki), when a retake comes (the wiki: "capturing the
// second or third node", on one captured node, turned into an allied
// node) and how many moves it gives (worldRetakeMoves, a guess), the
// timers of the server's effect types 25 and 26 (left out).

const (
	worldStrongholdsAtReset = 2 // the wiki: two at each monthly reset
	worldRetakeMoves        = 3 // the wiki's "limited moves": a guess
)

// the structures
var (
	strongholdNodes  = []uint32{6001000, 6002000, 6003000, 6004000, 6005000}
	strongholdAllied = []uint32{6020000, 6021000}
)

const (
	strongholdFinal    = 6010000
	strongholdCounter  = 6000100 // + n: n Siren nodes left
	strongholdDone     = 6000303
	strongholdBlockade = 6010210
	// offsets from a structure's root: its walls, its flagships (a
	// node's 410-412, the final's 410-413), an allied node's retaking
	// fleet (410) and retake (420)
	structureWalls    = 100
	structureFlagship = 410
	structureRetake   = 420
)

var worldSirenSettings struct {
	openStage uint32   // world_sairen_openstage
	visible   uint32   // world_sairen_max2
	weekly    []uint32 // world_sairen_weekly: {week day (1 Monday), count}
}

// loadWorldSirenData reads the Strongholds' gameset settings.
func loadWorldSirenData() error {
	var gameset map[string]uint32
	err := gamedata.LoadAll(
		func() (err error) {
			gameset, err = gamedata.Gameset()
			return err
		},
		func() error { return gamedata.GamesetDescription("world_sairen_weekly", &worldSirenSettings.weekly) },
	)
	if err != nil {
		return err
	}
	worldSirenSettings.openStage = gameset["world_sairen_openstage"]
	worldSirenSettings.visible = gameset["world_sairen_max2"]
	if worldSirenSettings.visible == 0 || len(worldSirenSettings.weekly) != 2 {
		return gamedata.Missing("gameset", "world_sairen_max2 / world_sairen_weekly")
	}
	return nil
}

// worldStrongholds is the month's Strongholds.
type worldStrongholds struct {
	// Since is when the count started: the monthly reset, or the progress
	// reaching world_sairen_openstage
	Since uint32 `json:"since,omitempty"`
	// Spawned are the strongholds due since then, Pending those of them
	// waiting for a place (only world_sairen_max2 are shown)
	Spawned uint32 `json:"spawned,omitempty"`
	Pending uint32 `json:"pending,omitempty"`
}

// updateStrongholds spawns the Strongholds due: two once they open (the
// monthly reset), one more each Monday, shown while fewer than
// world_sairen_max2 are; it tells whether sairen_chapter changed.
func (state *worldState) updateStrongholds(commanderID uint32, now time.Time) bool {
	if !state.activated() || state.Count.Progress < worldSirenSettings.openStage {
		return false
	}
	s := &state.Strongholds
	changed := false
	if s.Since == 0 {
		s.Since = uint32(now.Unix())
	}
	day := time.Duration(max(worldSirenSettings.weekly[0], 1)-1) * 24 * time.Hour
	weeks := periodStart(now.Add(-day), week).Sub(periodStart(time.Unix(int64(s.Since), 0).Add(-day), week)) / week
	due := worldStrongholdsAtReset + uint32(max(weeks, 0))*worldSirenSettings.weekly[1]
	for ; s.Spawned < due; s.Spawned++ {
		s.Pending++
	}
	for s.Pending > 0 && uint32(len(state.SairenChapters)) < worldSirenSettings.visible {
		entrance := state.pickStronghold(commanderID)
		if entrance == 0 {
			break
		}
		state.SairenChapters = append(state.SairenChapters, entrance)
		s.Pending--
		changed = true
	}
	return changed
}

// pickStronghold draws the entrance the next Stronghold appears on: one
// whose base map the progress opens and that has none.
func (state *worldState) pickStronghold(commanderID uint32) uint32 {
	var entrances []uint32
	for _, id := range slices.Sorted(maps.Keys(worldColormasks)) {
		mask := worldColormasks[id]
		if len(mask.SairenChapter) == 0 || mask.World != 1 || slices.Contains(state.SairenChapters, id) {
			continue
		}
		if stage := worldRandomMaps[mask.Chapter].OpenStage; len(stage) > 0 && stage[0] > state.Count.Progress {
			continue
		}
		entrances = append(entrances, id)
	}
	if len(entrances) == 0 {
		return 0
	}
	rng := worldRand(worldSeed(commanderID, state.Count.Activations, 30000+state.Strongholds.Spawned-state.Strongholds.Pending))
	return entrances[rng.IntN(len(entrances))]
}

// strongholdCleared takes a Stronghold cleared and left off sairen_chapter
// (the client does the same, NetUpdateActiveMap) and shows the next one.
func (state *worldState) strongholdCleared(randomID, commanderID uint32, now time.Time) {
	state.SairenChapters = slices.DeleteFunc(state.SairenChapters, func(id uint32) bool {
		return slices.Contains(worldColormasks[id].SairenChapter, randomID)
	})
	state.updateStrongholds(commanderID, now)
}

// worldStrongholdMap is what a Stronghold map knows of its structures.
type worldStrongholdMap struct {
	Nodes []worldStrongholdNode `json:"nodes"`
	Final [2]uint32             `json:"final"`
	// Retakes are the retakes begun, Retake the one under way
	Retakes uint32       `json:"retakes,omitempty"`
	Retake  *worldRetake `json:"retake,omitempty"`
	Done    bool         `json:"done,omitempty"`
}

// worldStrongholdNode is a Siren node: its root cell and structure (the
// allied node it became once a retake went for it).
type worldStrongholdNode struct {
	Root      [2]uint32 `json:"root"`
	Structure uint32    `json:"structure"`
	Captured  bool      `json:"captured,omitempty"`
}

// worldRetake is a retake under way: the node's index and the moves left.
type worldRetake struct {
	Node  int    `json:"node"`
	Moves uint32 `json:"moves"`
}

// structureBox is the cells a structure takes around its root: its walls'
// grid (offsets), {top, bottom, left, right}.
func structureBox(root uint32) [4]int32 {
	box := [4]int32{0, 0, 0, 0}
	for _, entry := range worldEvents[root+structureWalls].Grid {
		for i, v := range entry {
			if i%2 == 0 {
				box[0], box[1] = min(box[0], v), max(box[1], v)
			} else {
				box[2], box[3] = min(box[2], v), max(box[3], v)
			}
		}
	}
	return box
}

// boxCells are the cells of box around root, grown by margin.
func boxCells(root [2]uint32, box [4]int32, margin int32) [][2]int32 {
	var cells [][2]int32
	for r := int32(root[0]) + box[0] - margin; r <= int32(root[0])+box[1]+margin; r++ {
		for c := int32(root[1]) + box[2] - margin; c <= int32(root[1])+box[3]+margin; c++ {
			cells = append(cells, [2]int32{r, c})
		}
	}
	return cells
}

// strongholdSites picks the roots of the final node and of n Siren nodes:
// their structures and a cell around them on walkable cells (a way round
// each, nothing walled off), apart; the final node nearest the map's
// centre, each Siren node the farthest from the ones picked. nil when they
// don't fit.
func strongholdSites(template *worldTemplate, n int, rng *rand.Rand) [][2]uint32 {
	var cells [][2]uint32
	bottom, right := uint32(0), uint32(0)
	for _, cell := range template.Grids {
		bottom, right = max(bottom, cell.Row), max(right, cell.Column)
		if cell.Walkable {
			cells = append(cells, [2]uint32{cell.Row, cell.Column})
		}
	}
	slices.SortFunc(cells, func(a, b [2]uint32) int {
		if a[0] != b[0] {
			return int(a[0]) - int(b[0])
		}
		return int(a[1]) - int(b[1])
	})
	used := map[[2]int32]bool{}
	fits := func(root [2]uint32, box [4]int32) bool {
		for _, cell := range boxCells(root, box, 1) {
			if cell[0] < 0 || cell[1] < 0 || used[cell] || !template.isWalkable(uint32(cell[0]), uint32(cell[1])) {
				return false
			}
		}
		return true
	}
	take := func(root [2]uint32, box [4]int32) {
		for _, cell := range boxCells(root, box, 1) {
			used[cell] = true
		}
	}
	final := structureBox(strongholdFinal)
	centre := [2]uint32{bottom / 2, right / 2}
	var sites [][2]uint32
	best := -1
	for _, cell := range cells {
		if d := abs(int(cell[0])-int(centre[0])) + abs(int(cell[1])-int(centre[1])); fits(cell, final) && (best < 0 || d < best) {
			best = d
			if len(sites) == 0 {
				sites = append(sites, cell)
			}
			sites[0] = cell
		}
	}
	if len(sites) == 0 {
		return nil
	}
	take(sites[0], final)
	node := structureBox(strongholdNodes[0])
	for range n {
		var pick [2]uint32
		best := -1
		for _, cell := range cells {
			if !fits(cell, node) {
				continue
			}
			d := 1 << 30
			for _, site := range sites {
				d = min(d, chebyshev(cell, site))
			}
			// a little chance so the zones of a layout differ
			if d = 4*d + rng.IntN(4); d > best {
				pick, best = cell, d
			}
		}
		if best < 0 {
			return nil
		}
		take(pick, node)
		sites = append(sites, pick)
	}
	return sites
}

// placeStronghold builds a Stronghold: the final node, four Siren nodes
// (four of the five variants) and the node counter (5 nodes left); false
// when its structures don't fit the template.
func placeStronghold(m *worldMap, template *worldTemplate, rng *rand.Rand) bool {
	sites := strongholdSites(template, 4, rng)
	if sites == nil {
		return false
	}
	op := &worldOp{m: m, state: &worldState{}, rng: rng}
	sh := &worldStrongholdMap{Final: sites[0]}
	op.buildStructure(strongholdFinal, sites[0])
	variants := slices.Clone(strongholdNodes)
	rng.Shuffle(len(variants), func(i, j int) { variants[i], variants[j] = variants[j], variants[i] })
	for i, site := range sites[1:] {
		op.buildStructure(variants[i], site)
		sh.Nodes = append(sh.Nodes, worldStrongholdNode{Root: site, Structure: variants[i]})
	}
	m.Stronghold = sh
	// the counter, out of the way (its grid is the top left corner):
	// on a rock when there is one
	counter := strongholdCounter + uint32(len(sh.Nodes)) + 1
	var spots, rocks [][2]uint32
	for _, entry := range worldEvents[counter].Grid {
		for _, cell := range gridCells(template, entry) {
			spots = append(spots, cell)
			if !template.isWalkable(cell[0], cell[1]) {
				rocks = append(rocks, cell)
			}
		}
	}
	if len(rocks) > 0 {
		spots = rocks
	}
	if len(spots) > 0 {
		m.addAttachment(newWorldEvent(counter, spots[0][0], spots[0][1]))
	}
	return true
}

// buildStructure puts structure root at cell and runs its building
// effects (settle), then opens its gates.
func (op *worldOp) buildStructure(root uint32, cell [2]uint32) {
	before := slices.Clone(op.m.Attachments)
	op.m.addAttachment(newWorldEvent(root, cell[0], cell[1]))
	op.mark(cell)
	op.settle(func(a *worldAttachment) bool { return !slices.Contains(before, a) })
	op.m.openGates()
}

// worldBuildEffect tells whether an effect builds (spawns, branches,
// terrain, replacements, the client's camera moves) and runs by itself:
// what settle runs on the server's side, a tip or a battle being the
// client's to see.
func worldBuildEffect(id uint32) bool {
	effect, ok := worldEffects[id]
	if !ok || effect.AutoActivate == 0 {
		return false
	}
	switch effect.EffectType {
	case worldEffectSpawnEvents, worldEffectSpawnRelative, worldEffectRemoveEvents, worldEffectCondition,
		worldEffectRandom, worldEffectTerrain, worldEffectReplace, 25, 26, 28:
		return true
	}
	return effect.EffectType >= 1000
}

// settle runs the building effects of the events only accepts, what the
// client would ask for one by one (WorldMap:CheckInteractive), until none
// is left.
func (op *worldOp) settle(only func(*worldAttachment) bool) {
	for range 4096 {
		var ev *worldAttachment
		for _, a := range op.m.Attachments {
			if a.Type == worldAttachEvent && len(a.Effects) > 0 && only(a) && worldBuildEffect(a.Effects[0]) {
				ev = a
				break
			}
		}
		if ev == nil {
			return
		}
		id := ev.Effects[0]
		ev.Effects = slices.Delete(slices.Clone(ev.Effects), 0, 1)
		_ = op.runEffect(ev, id, 0)
		if len(ev.Effects) == 0 && slices.Contains(op.m.Attachments, ev) {
			// a gate that drew nothing (the 6005000 nodes' may) is
			// an opening still
			op.m.openGate(ev)
			op.m.removeAttachment(ev)
			op.mark([2]uint32{ev.Row, ev.Column})
		}
	}
}

// openGate takes the walls off the cell of a structure's gate event ev
// (openGates): not a structure's root (its id a thousand), nor the cell of
// a Blockade Device.
func (m *worldMap) openGate(ev *worldAttachment) {
	if ev.Type != worldAttachEvent || worldWall(ev.ID) || !worldScripted(ev.ID) || ev.ID%1000 == 0 ||
		worldObstacleMasks[worldEvents[ev.ID].Obstacle]&worldObstacleArrive == 0 {
		return
	}
	if slices.ContainsFunc(m.cellAttachments(ev.Row, ev.Column), func(a *worldAttachment) bool { return a.ID == strongholdBlockade }) {
		return
	}
	for _, a := range m.cellAttachments(ev.Row, ev.Column) {
		if a != ev && a.Type == worldAttachEvent && worldWall(a.ID) && worldScripted(a.ID) {
			m.removeAttachment(a)
		}
	}
}

// openGates takes the structures' walls off the cells their gates went
// to (a fleet, a resource, an allied node's opening), but the final
// node's Blockade Device, walled in until the zone's Siren control falls
// under half (strongholdStep).
func (m *worldMap) openGates() {
	for _, wall := range slices.Clone(m.Attachments) {
		if wall.Type != worldAttachEvent || !worldWall(wall.ID) || !worldScripted(wall.ID) {
			continue
		}
		for _, a := range m.cellAttachments(wall.Row, wall.Column) {
			if a != wall {
				m.openGate(a)
			}
		}
	}
}

// eventOn tells whether one of the events ids is on the map.
func (m *worldMap) eventOn(ids ...uint32) bool {
	return slices.ContainsFunc(m.Attachments, func(a *worldAttachment) bool {
		return a.Type == worldAttachEvent && slices.Contains(ids, a.ID)
	})
}

// flagships are a structure's flagship events.
func flagships(root uint32) []uint32 {
	var ids []uint32
	for id := root + structureFlagship; id < root+structureFlagship+4; id++ {
		if _, ok := worldEvents[id]; ok {
			ids = append(ids, id)
		}
	}
	return ids
}

// strongholdStep is what the server does on a Stronghold after an
// operation (finish): the nodes captured, the retake of the second and
// third capture begun, a retake beaten forgotten, the Blockade Devices
// gone once the Siren control is under half, the zone's end (6000303)
// once nothing is left of the Sirens.
func (op *worldOp) strongholdStep() {
	m := op.m
	sh, template := m.Stronghold, worldTemplates[m.TemplateID]
	if sh == nil || template == nil {
		return
	}
	captured := uint32(0)
	for i := range sh.Nodes {
		node := &sh.Nodes[i]
		if !node.Captured && !m.eventOn(flagships(node.Structure)...) {
			node.Captured = true
		}
		if node.Captured {
			captured++
		}
	}
	if sh.Retake != nil {
		root := sh.Nodes[sh.Retake.Node].Structure
		if !m.eventOn(root + structureFlagship) {
			sh.Retake = nil
		}
	}
	if sh.Retake == nil && sh.Retakes < 2 && captured >= sh.Retakes+2 && int(sh.Retakes) < len(strongholdAllied) {
		op.beginRetake()
	}
	if 2*m.infection() < template.IsSairen {
		for _, a := range slices.Clone(m.Attachments) {
			if a.Type == worldAttachEvent && a.ID == strongholdBlockade {
				for _, b := range m.cellAttachments(a.Row, a.Column) {
					m.removeAttachment(b)
				}
				op.mark([2]uint32{a.Row, a.Column})
			}
		}
	}
	if !sh.Done && m.infection() == 0 && !m.eventOn(flagships(strongholdFinal)...) {
		sh.Done = true
		spawnWorldEvent(m, template, strongholdDone, op.state.Fleets, op.rng)
		for _, a := range m.Attachments {
			if a.ID == strongholdDone {
				op.mark([2]uint32{a.Row, a.Column})
			}
		}
	}
}

// beginRetake turns a captured node into an allied node (the next of
// 6020000, 6021000) and sends its retaking fleet: the captured node
// farthest from the fleets, its walls and allied fleet replaced.
func (op *worldOp) beginRetake() {
	m, sh := op.m, op.m.Stronghold
	pick, best := -1, -1
	for i, node := range sh.Nodes {
		if !node.Captured || slices.Contains(strongholdAllied, node.Structure) {
			continue
		}
		d := 1 << 30
		for _, fleet := range op.state.Fleets {
			if !fleet.Submarine {
				d = min(d, chebyshev(node.Root, fleetCell(fleet)))
			}
		}
		if d > best {
			pick, best = i, d
		}
	}
	if pick < 0 {
		return
	}
	node := &sh.Nodes[pick]
	box := structureBox(node.Structure)
	for _, cell := range boxCells(node.Root, box, 0) {
		if cell[0] < 0 || cell[1] < 0 {
			continue
		}
		for _, a := range m.cellAttachments(uint32(cell[0]), uint32(cell[1])) {
			if a.Type == worldAttachEvent && worldScripted(a.ID) && a.ID/1000 == node.Structure/1000 {
				m.removeAttachment(a)
				op.mark([2]uint32{a.Row, a.Column})
			}
		}
	}
	allied := strongholdAllied[sh.Retakes]
	op.buildStructure(allied, node.Root)
	node.Structure = allied
	// the allied node's placeholder becomes the retaking fleet (60003010)
	op.replaceEvents(allied+structureFlagship-10, allied+structureFlagship)
	sh.Retakes++
	sh.Retake = &worldRetake{Node: pick, Moves: worldRetakeMoves}
}

// retakeMove counts a move against the retake under way: out of moves,
// the node is retaken (the allied node's 6020420 / 6021420, whose effects
// the client runs: the fleet gone, the allied fleet a Siren one, the
// counter up).
func (op *worldOp) retakeMove() {
	sh, template := op.m.Stronghold, worldTemplates[op.m.TemplateID]
	if sh == nil || sh.Retake == nil || template == nil {
		return
	}
	if sh.Retake.Moves > 1 {
		sh.Retake.Moves--
		return
	}
	node := sh.Nodes[sh.Retake.Node]
	sh.Retake = nil
	if !op.m.eventOn(node.Structure + structureFlagship) {
		return
	}
	retake := newWorldEvent(node.Structure+structureRetake, node.Root[0], node.Root[1])
	if op.m.addAttachment(retake) {
		op.mark(node.Root)
	}
}

// worldBattleOwnLoot tells whether a won battle against enemy on m gives
// loot of its own: not a Stronghold's bosses (expedition types 98, 99),
// whose events' drops are the wiki's rewards, nor the Arbiters, whose
// rewards are their hp phases'.
func worldBattleOwnLoot(m *worldMap, enemy uint32) bool {
	if _, ok := worldArbiterZones[m.RandomID]; ok {
		return false
	}
	if worldZones[m.RandomID] == worldZoneStronghold {
		kind := expeditionTemplates[enemy].Type
		return kind != 98 && kind != 99
	}
	return true
}
