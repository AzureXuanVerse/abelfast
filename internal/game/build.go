package game

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"math/rand/v2"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"time"

	"google.golang.org/protobuf/proto"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/db"
	"github.com/ggmolly/belfast/internal/logger"
	"github.com/ggmolly/belfast/internal/protobuf"
)

const (
	// maxBuilds is the client's MAX_BUILD_WORK_COUNT: builds that can be
	// queued at once
	maxBuilds = 10
	// buildWorkCount is how many of them progress at the same time
	// (SC_12024.worklist_count as captured), the next one starts when one
	// finishes (BuildShipProxy:activeNextBuild)
	buildWorkCount = 2
	// the light pool (ship_data_create_material 2)
	lightPool = 2
)

// The new player guide's light build: the official server gave Hatsuharu
// (301211) in 1500 s. The light pool's drop list is server side, so the
// guide's build is pinned to what the capture got; later ones are random.
const (
	tutorialBuildShip     = 301211
	tutorialBuildDuration = 1500
)

// rarityByRateName maps the rate_tip labels to ship_data_statistics rarity.
var rarityByRateName = map[string]uint32{"Common": 2, "Rare": 3, "Elite": 4, "Super Rare": 5, "Ultra Rare": 6}

// buildTimes are the construction times of the ships (base template id ->
// seconds), server-side data the client never sees: server/build_times.json
// of the data dir, scraped from the wiki by internal/tools/wiki_build_times.py
// (Hatsuharu's 1500 s matches the official capture). Without it, or for a
// ship it doesn't list, buildDurations by rarity is used.
var buildTimes = sync.OnceValues(func() (map[uint32]uint32, error) {
	raw, err := os.ReadFile(filepath.Join(regionDataDir(), "server", "build_times.json"))
	if errors.Is(err, fs.ErrNotExist) {
		return map[uint32]uint32{}, nil
	}
	if err != nil {
		return nil, err
	}
	var times map[uint32]uint32
	if err := json.Unmarshal(raw, &times); err != nil {
		return nil, fmt.Errorf("build times: %w", err)
	}
	return times, nil
})

// buildDurations is how long a build takes by the rarity of what it yields,
// for ships without a known construction time (guessed, Rare's from the
// capture).
// The real times are per ship and server side (none of the client tables has
// them); rare matches the capture, the rest are approximations.
var buildDurations = map[uint32]uint32{2: 1200, 3: 1500, 4: 1800, 5: 3600, 6: 5400}

// buildDocks is SC_12024, BuildShipProxy's state, with nothing building and
// no draws or build points.
func buildDocks() *protobuf.SC_12024 {
	return &protobuf.SC_12024{
		WorklistCount: proto.Uint32(buildWorkCount),
	}
}

// buildDocksState is SC_12024 from storage: the builds in dock order (their
// index is the pos the client sends back) and the build points. draw_count_*
// count gem draws (draw_data_template), never done here.
func buildDocksState(ctx context.Context, queries *db.Queries, commanderID uint32) (*protobuf.SC_12024, error) {
	builds, err := queries.ListBuilds(ctx, commanderID)
	if err != nil {
		return nil, err
	}
	points, err := queries.GetBuildPoints(ctx, commanderID)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}
	msg := buildDocks()
	msg.ExchangeCount = proto.Uint32(points)
	for _, build := range builds {
		msg.WorklistList = append(msg.WorklistList, buildInfo(build))
	}
	return msg, nil
}

// buildStartTime is when a build started, or starts while it is queued.
func buildStartTime(build db.Build) int64 {
	return build.FinishTime - int64(build.Duration)
}

// buildInfo is a BUILDINFO, read into a BuildShip (model/vo/buildship.lua).
func buildInfo(build db.Build) *protobuf.BUILDINFO {
	return &protobuf.BUILDINFO{
		Time:       proto.Uint32(build.Duration),
		FinishTime: proto.Uint32(uint32(build.FinishTime)),
		BuildId:    proto.Uint32(build.PoolID),
	}
}

// pickBuildShip decides what a build yields: a rarity drawn with the pool's
// rate_tip, then one of the pool's ships of that rarity (ship_data_group
// descriptions linking to the pool).
func pickBuildShip(pool buildPool) (template uint32, duration uint32, ok bool) {
	candidates := map[uint32][]uint32{}
	for _, group := range pool.Groups {
		if ship, ok := shipTemplates[group*10+1]; ok {
			candidates[ship.Rarity] = append(candidates[ship.Rarity], group*10+1)
		}
	}
	// rarities that have ships, in a fixed order
	var rarities []uint32
	total := 0.0
	for name, rate := range pool.Rates {
		if rarity := rarityByRateName[name]; len(candidates[rarity]) > 0 {
			rarities = append(rarities, rarity)
			total += rate
		}
	}
	if len(rarities) == 0 {
		return 0, 0, false
	}
	slices.Sort(rarities)
	rates := map[uint32]float64{}
	for name, rate := range pool.Rates {
		rates[rarityByRateName[name]] = rate
	}
	roll := rand.Float64() * total
	rarity := rarities[len(rarities)-1] // if rounding leaves the roll positive
	for _, r := range rarities {
		if roll -= rates[r]; roll < 0 {
			rarity = r
			break
		}
	}
	ships := candidates[rarity]
	template = ships[rand.IntN(len(ships))]
	times, err := buildTimes()
	if err != nil {
		logger.WithFields("Build", logger.FieldValue("error", err)).Warn("build times not loaded")
	}
	if seconds, ok := times[template]; ok {
		return template, seconds, true
	}
	logger.WithFields("Build", logger.FieldValue("template", template)).Warn("no construction time, using the rarity's")
	return template, buildDurations[rarity], true
}

// buildStart is when a new build behind placed ones starts: when one of the
// buildWorkCount docks frees up (builds are started in order, so those are
// the latest finish times), now at the earliest.
func buildStart(placed []db.Build, now int64) int64 {
	finishes := make([]int64, 0, len(placed))
	for _, build := range placed {
		finishes = append(finishes, build.FinishTime)
	}
	slices.Sort(finishes)
	slices.Reverse(finishes)
	if len(finishes) < buildWorkCount {
		return now
	}
	return max(now, finishes[buildWorkCount-1])
}

// handleBuildShip starts constructions (BuildShipCommand, CS_12002): count
// builds in pool id (ship_data_create_material), paid with use_gold gold and
// number_1 use_item each; costtype 1 pays with an event build ticket, there
// is none. Each build adds exchange_count build points. The client takes the
// costs off its own state on result 0.
func (game *Game) handleBuildShip(client *connection.Client, req *protobuf.CS_12002) error {
	ctx := context.Background()
	now := time.Now().Unix()
	pool, ok := buildPools[req.GetId()]
	count := req.GetCount()
	var started []db.Build
	err := errRefused
	if ok && req.GetCosttype() == 0 && count >= 1 && count <= maxBuilds {
		err = game.tx(ctx, client, func(tx *Tx) error {
			builds, err := tx.ListBuilds(ctx, client.CommanderID)
			if err != nil {
				return err
			}
			if len(builds)+int(count) > maxBuilds {
				return errRefused
			}
			// the guide's build is the commander's first
			built, err := taskCounter(ctx, tx.Queries, client.CommanderID, TaskSubBuild, 0)
			if err != nil {
				return err
			}
			if err := tx.SpendResource(ResourceGold, pool.Gold*count); err != nil {
				return err
			}
			if err := tx.SpendItem(pool.Item, pool.ItemCount*count); err != nil {
				return err
			}
			for i := range count {
				template, duration, ok := uint32(tutorialBuildShip), uint32(tutorialBuildDuration), true
				if built > 0 || i > 0 || req.GetId() != lightPool {
					template, duration, ok = pickBuildShip(pool)
				}
				if !ok {
					return errRefused
				}
				start := buildStart(builds, now)
				build, err := tx.CreateBuild(ctx, db.CreateBuildParams{
					CommanderID: client.CommanderID,
					PoolID:      req.GetId(),
					TemplateID:  template,
					Duration:    duration,
					FinishTime:  start + int64(duration),
				})
				if err != nil {
					return err
				}
				builds = append(builds, build)
				started = append(started, build)
			}
			// capture: the task progress (SC_20102, SC_20202) comes before SC_12003
			tx.event(TaskEvent{SubType: TaskSubBuild, Count: count})
			return tx.AddBuildPoints(ctx, db.AddBuildPointsParams{CommanderID: client.CommanderID, ExchangeCount: pool.ExchangeCount * count})
		})
	}
	if err != nil && !refused(err) {
		return err
	}
	reply := &protobuf.SC_12003{Result: result(err)}
	if err == nil {
		for _, build := range started {
			reply.BuildInfo = append(reply.BuildInfo, buildInfo(build))
		}
	}
	return client.Send(reply)
}

// buildsAt maps the dock positions the client sends (1-based, in build
// order) to builds, dropping duplicates and positions with no build.
func buildsAt(builds []db.Build, positions []uint32) (picked []db.Build, picks []uint32) {
	seen := map[uint32]bool{}
	for _, pos := range positions {
		if pos < 1 || int(pos) > len(builds) || seen[pos] {
			continue
		}
		seen[pos] = true
		picked = append(picked, builds[pos-1])
		picks = append(picks, pos)
	}
	return picked, picks
}

// handleBuildQuickFinish finishes builds now with a Quick Finisher each
// (BuildShipImmediatelyCommand, CS_12008); queued builds behind them start
// right away. Replies the positions finished.
func (game *Game) handleBuildQuickFinish(client *connection.Client, req *protobuf.CS_12008) error {
	ctx := context.Background()
	now := time.Now().Unix()
	var finished []uint32
	// capture: the task progress of the Quick Finishers used (SC_20202) comes
	// before SC_12009
	err := game.tx(ctx, client, func(tx *Tx) error {
		builds, err := tx.ListBuilds(ctx, client.CommanderID)
		if err != nil {
			return err
		}
		picked, positions := buildsAt(builds, req.GetPosList())
		for i, build := range picked {
			if build.FinishTime <= now {
				continue
			}
			if err := tx.SpendItem(ItemQuickFinisher, 1); err != nil {
				return err
			}
			builds[positions[i]-1].FinishTime = now
			finished = append(finished, positions[i])
		}
		if len(finished) == 0 {
			return errRefused
		}
		return rescheduleBuilds(ctx, tx.Queries, builds, now)
	})
	if err != nil && !refused(err) {
		return err
	}
	return client.Send(&protobuf.SC_12009{Result: result(err), PosList: finished})
}

// rescheduleBuilds saves builds after some finished early: the ones not
// started yet move up behind the others.
func rescheduleBuilds(ctx context.Context, queries *db.Queries, builds []db.Build, now int64) error {
	var placed []db.Build
	for _, build := range builds {
		if buildStartTime(build) > now {
			build.FinishTime = buildStart(placed, now) + int64(build.Duration)
		}
		placed = append(placed, build)
		if err := queries.SetBuildFinishTime(ctx, db.SetBuildFinishTimeParams{FinishTime: build.FinishTime, ID: build.ID}); err != nil {
			return err
		}
	}
	return nil
}

// handleBuildResults tells which ship each finished build yields
// (GetShipCommand, CS_12043), so the client downloads their paintings before
// collecting them.
func (game *Game) handleBuildResults(client *connection.Client, req *protobuf.CS_12043) error {
	builds, err := game.DB.ListBuilds(context.Background(), client.CommanderID)
	if err != nil {
		return err
	}
	now := time.Now().Unix()
	reply := &protobuf.SC_12044{}
	for i, build := range builds {
		if build.FinishTime <= now {
			reply.InfoList = append(reply.InfoList, &protobuf.BUILD_INFO{Pos: proto.Uint32(uint32(i + 1)), Tid: proto.Uint32(build.TemplateID)})
		}
	}
	return client.Send(reply)
}

// handleCollectBuilds turns finished builds into ships (GetShipCommand,
// CS_12025). A ship of a new group first pushes the archive medal (SC_17002).
func (game *Game) handleCollectBuilds(client *connection.Client, req *protobuf.CS_12025) error {
	ctx := context.Background()
	now := time.Now().Unix()
	var ships []*protobuf.SHIPINFO
	err := game.tx(ctx, client, func(tx *Tx) error {
		builds, err := tx.ListBuilds(ctx, client.CommanderID)
		if err != nil {
			return err
		}
		picked, _ := buildsAt(builds, req.GetPosList())
		for _, build := range picked {
			if build.FinishTime > now {
				continue
			}
			ship, err := tx.AddShip(build.TemplateID)
			if err != nil {
				return err
			}
			if err := tx.DeleteBuild(ctx, build.ID); err != nil {
				return err
			}
			ships = append(ships, ship)
		}
		if len(ships) == 0 {
			return errRefused
		}
		return nil
	})
	if err != nil && !refused(err) {
		return err
	}
	return client.Send(&protobuf.SC_12026{Result: result(err), ShipList: ships})
}

// handleConfirmBuilds acknowledges the "ships collected while away"
// notification (GetShipConfirmCommand, CS_12045, SC_11003.taking_ship_list);
// builds are only ever collected by the player, so there is nothing to clear.
func (game *Game) handleConfirmBuilds(client *connection.Client, req *protobuf.CS_12045) error {
	return client.Send(&protobuf.SC_12046{Result: result(nil)})
}
