package game

import (
	"encoding/json"
	"math/rand/v2"
	"slices"
	"time"

	"github.com/ggmolly/belfast/internal/db"
	"github.com/ggmolly/belfast/internal/gamedata"
)

// The Arbiters (the wiki's Operation Siren page, "Arbiter"): the monthly
// bosses of the NA Ocean Core Sector, fought from its Forward Base. Almost
// all of it is the client's tables:
//   - the Forward Base is entrance 154's base map, world_chapter_random
//     40000 (open_stage 200: task 4102's complete_stage, the end of the
//     story's chapter 5), no action points; its device (world_event_data
//     7000000 closed, 7000010 Normal, 7000011 Hard, 7000012 practice, on
//     7000000's cell) teleports the fleets to the month's battle zone
//     (effects 70000100 / 70000110: a "task" branch on world_task_data
//     4200 "Arbiter (Normal)" / 4201 "Arbiter (Hard)", then a "month"
//     branch to world_chapter_random 43000-43002 / 43010-43012, or 43020
//     for the practice);
//   - the zones' content is the 703xxxx family: a month branch spawns the
//     month's setting (7030040 / 7030050 / 7030060: walls, fires), whose
//     pedestal, once a fleet steps on it, summons the box that lifts the
//     Siren fog (effect type 23) and the month's Arbiter (7032000-7032015
//     Normal, 7032020-7032035 Hard, 7032040-7032055 practice: their own
//     skills, slg_bufflist, the wiki's per month table); its death fires
//     the trigger that completes the task (7000400 / 7000401) and opens a
//     Siren Transference Device back to the Forward Base;
//   - the Arbiters are world_expedition_data 315010-315035: phase_limit 1
//     for the Normal ones (WorldMapAttachment:IsPeriodEnemy, their lowest
//     hp kept the month: month_boss), 2 for the Hard ones, the phases' hp
//     and rewards in phase_story / phase_drop_display (the wiki's 80% to
//     0% Normal, 75% to 0% Hard tables), modifiers 3000 (120 * 1.3: the
//     wiki's 156), the Hard zones is_clear (left, they start over: back to
//     full hp) and without maintenance (instruction_available, "Regroup
//     and Rest"), each zone's way out world_chapter_template_reset's.
// The "month" value the branches read is the server's: 0 the commander's
// first Arbiter month (the wiki: "always Arbiter: The Hermit IX"), else
// the month of the year, 1-12 (October: Temperance XIV, as the wiki says).
// What isn't in the tables and is done here: the root events the server
// puts in each zone and on the Forward Base, and the month: on the 1st the
// Normal task is handed out again (the Hard one taken back), the zones
// start over, month_boss empties.

const (
	worldForwardBase       = 40000
	worldArbiterTaskNormal = 4200
	worldArbiterTaskHard   = 4201
)

// the Forward Base's devices
const (
	arbiterDeviceClosed   = 7000000
	arbiterDeviceNormal   = 7000010
	arbiterDeviceHard     = 7000011
	arbiterDevicePractice = 7000012
)

// arbiterTips are the Forward Base's tips (world_event_data 7000040-7000042:
// "Use the devices around the map to commence an Arbiter Battle on Normal
// difficulty"...) that come with each device.
var arbiterTips = map[uint32]uint32{arbiterDeviceNormal: 7000040, arbiterDeviceHard: 7000041, arbiterDevicePractice: 7000042}

// worldArbiterZones are the Arbiters' battle zones (the devices'
// teleports) with the events the server starts them with: the setting's
// month branch (7030000 Normal, 7030001 Hard, 7031200 the practice's
// Arbiter itself, all on cell (5, 6)) and the zone's tip ("Currently
// challenging an Arbiter on Normal difficulty..."), 7030010-7030012.
var worldArbiterZones = map[uint32][2]uint32{
	43000: {7030000, 7030010}, 43001: {7030000, 7030010}, 43002: {7030000, 7030010},
	43010: {7030001, 7030011}, 43011: {7030001, 7030011}, 43012: {7030001, 7030011},
	43020: {7031200, 7030012},
}

// worldArbiter is the commander's Arbiter months.
type worldArbiter struct {
	// First is the start of the first month the Arbiters were open
	First uint32 `json:"first,omitempty"`
	// Month is the start of the month they were last set up for
	Month uint32 `json:"month,omitempty"`
	// Device is the Forward Base device last put there
	Device uint32 `json:"device,omitempty"`
}

// arbiterMonth is the "month" a branch effect compares: 0 in the first
// Arbiter month, else the month of the year.
func (state *worldState) arbiterMonth(now time.Time) uint32 {
	start := uint32(startOfMonth(now).Unix())
	if state.Arbiter.First == 0 || state.Arbiter.First == start {
		return 0
	}
	return uint32(now.In(gamedata.Zone).Month())
}

// arbiterOpen tells whether the Forward Base is open (its open_stage).
func (state *worldState) arbiterOpen() bool {
	stage := worldRandomMaps[worldForwardBase].OpenStage
	return state.activated() && len(stage) > 0 && state.Count.Progress >= stage[0]
}

// updateArbiter sets the month's Arbiters up once the Forward Base is open:
// the Normal task (again), the Hard one gone, month_boss emptied, the
// battle zones started over (the one the fleets are in left as it is).
// It tells whether the world changed.
func (tx *Tx) updateArbiter(state *worldState, now time.Time) (bool, error) {
	start := uint32(startOfMonth(now).Unix())
	if !state.arbiterOpen() || state.Arbiter.Month == start {
		return false, nil
	}
	if state.Arbiter.First == 0 {
		state.Arbiter.First = start
	}
	state.Arbiter.Month = start
	state.MonthBoss = nil
	state.Tasks = slices.DeleteFunc(state.Tasks, func(task worldTask) bool { return task.ID == worldArbiterTaskHard })
	rng := rand.New(rand.NewPCG(uint64(now.UnixNano()), uint64(tx.commanderID)))
	tx.acceptWorldTask(state, worldArbiterTaskNormal, now, rng)
	for id := range worldArbiterZones {
		if id == state.MapID {
			continue
		}
		state.Maps = slices.DeleteFunc(state.Maps, func(m worldMapID) bool { return m.RandomID == id })
		if err := tx.DeleteWorldMap(tx.ctx, db.DeleteWorldMapParams{CommanderID: tx.commanderID, RandomID: id}); err != nil {
			return false, err
		}
	}
	return true, nil
}

// arbiterDevice is the Forward Base's device for the month's progress: the
// Normal battle's while its task is under way, then the Hard one's, then
// the practice's once that is done; closed before the Arbiters open.
func (state *worldState) arbiterDevice() uint32 {
	if task := state.task(worldArbiterTaskNormal); task != nil && task.alive() {
		return arbiterDeviceNormal
	}
	if task := state.task(worldArbiterTaskHard); task != nil {
		if task.alive() {
			return arbiterDeviceHard
		}
		return arbiterDevicePractice
	}
	return arbiterDeviceClosed
}

// refreshForwardBase puts the month's device on the Forward Base (a device
// is used up by its teleport) and, when it changes, its tip; it tells
// whether the map changed.
func refreshForwardBase(m *worldMap, state *worldState, rng *rand.Rand) bool {
	template := worldTemplates[m.TemplateID]
	closed, ok := worldEvents[arbiterDeviceClosed]
	if template == nil || !ok || len(closed.Grid) == 0 {
		return false
	}
	cells := gridCells(template, closed.Grid[0])
	if len(cells) == 0 {
		return false
	}
	cell := cells[0]
	want := state.arbiterDevice()
	devices := []uint32{arbiterDeviceClosed, arbiterDeviceNormal, arbiterDeviceHard, arbiterDevicePractice}
	changed, have := false, false
	for _, a := range slices.Clone(m.Attachments) {
		if a.Type != worldAttachEvent || !slices.Contains(devices, a.ID) {
			continue
		}
		if a.ID == want && a.Row == cell[0] && a.Column == cell[1] {
			have = true
			continue
		}
		m.removeAttachment(a)
		changed = true
	}
	if !have {
		m.addAttachment(newWorldEvent(want, cell[0], cell[1]))
		changed = true
	}
	if state.Arbiter.Device != want {
		state.Arbiter.Device = want
		for _, a := range slices.Clone(m.Attachments) {
			for _, tip := range arbiterTips {
				if a.Type == worldAttachEvent && a.ID == tip {
					m.removeAttachment(a)
				}
			}
		}
		if tip := arbiterTips[want]; tip != 0 {
			spawnWorldEvent(m, template, tip, state.Fleets, rng)
		}
		changed = true
	}
	return changed
}

// placeArbiterZone starts an Arbiter battle zone with its root event and
// tip; false for a map that isn't one.
func placeArbiterZone(m *worldMap, template *worldTemplate, rng *rand.Rand) bool {
	zone, ok := worldArbiterZones[m.RandomID]
	if !ok {
		return false
	}
	for _, id := range zone {
		spawnWorldEvent(m, template, id, nil, rng)
	}
	return true
}

// monthBossHP is the lowest hp the month left a periodic boss at
// (month_boss, World.lowestHP), full when it wasn't fought.
func (state *worldState) monthBossHP(id uint32) uint32 {
	for _, kv := range state.MonthBoss {
		if kv.ID == id {
			return kv.Count
		}
	}
	return fullHP
}

func (state *worldState) setMonthBossHP(id, hp uint32) {
	for i := range state.MonthBoss {
		if state.MonthBoss[i].ID == id {
			state.MonthBoss[i].Count = hp
			return
		}
	}
	state.MonthBoss = append(state.MonthBoss, worldCount{ID: id, Count: hp})
}

// phaseDrops are a boss's phase_drop_display: what each hp phase gives,
// by hp (1/10000); nil for a boss without (the Abyssal Sirens').
func (expedition worldExpedition) phaseDrops() map[uint32][]worldLoot {
	var drops map[uint32][]worldLoot
	for _, raw := range expedition.PhaseDropDisplay {
		var entry []json.RawMessage
		if json.Unmarshal(raw, &entry) != nil || len(entry) < 2 {
			continue
		}
		var items [][]uint32
		if json.Unmarshal(entry[1], &items) != nil {
			continue
		}
		var loot []worldLoot
		for _, item := range items {
			if len(item) == 3 {
				loot = append(loot, worldLoot{kind: item[0], id: item[1], lo: item[2], hi: item[2]})
			}
		}
		if drops == nil {
			drops = map[uint32][]worldLoot{}
		}
		drops[rawUint(entry[0])] = loot
	}
	return drops
}
