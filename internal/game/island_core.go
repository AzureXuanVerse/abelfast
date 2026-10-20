package game

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"slices"

	"google.golang.org/protobuf/proto"

	"github.com/ggmolly/belfast/internal/db"
	"github.com/ggmolly/belfast/internal/gamedata"
	"github.com/ggmolly/belfast/internal/protobuf"
	"github.com/ggmolly/belfast/internal/protocol"
)

// The Island (IslandProxy, Island / SharedIsland, mod/island, protocol
// module 21): the commander's own island life-sim, opened from the main
// screen's living area (MainLiveAreaPage, IslandMediator, commander level
// 50). Its state is the PB_ISLAND the client gets in SC_21201 and keeps up to
// date itself from each reply (IslandDropHelper.AddItems for rewards), so
// belfast keeps exactly that message per commander (commander_islands), plus
// what only the server knows per subsystem (islandState.extra). Built from
// the Lua and the island_* tables alone: no official capture enters the
// island.
//
// The subsystems live in island_*.go: each registers its handlers
// (registerIsland*) and loads its tables (loadIsland*Data), and works on the
// state through Tx.island (the commander's own, saved when the request
// commits) or Tx.otherIsland (another commander's, read only).

// islandState is a commander's island as stored.
type islandState struct {
	commanderID uint32
	island      *protobuf.PB_ISLAND
	position    *protobuf.PB_PLAYER_POS_RECORD // nil until CS_21229
	extra       map[string]json.RawMessage     // by subsystem
}

// public is the island's public data (what visitors see too).
func (state *islandState) public() *protobuf.PB_ISLAND_PUBLIC {
	return state.island.PublicData
}

// private is the island's private data (only its owner gets it).
func (state *islandState) private() *protobuf.PB_ISLAND_PRIVATE {
	return state.island.PrivateData
}

// getExtra decodes the server-only state a subsystem stored under key into
// value; false when there is none yet (value is left alone).
func (state *islandState) getExtra(key string, value any) (bool, error) {
	raw, ok := state.extra[key]
	if !ok {
		return false, nil
	}
	if err := json.Unmarshal(raw, value); err != nil {
		return false, fmt.Errorf("island extra %s: %w", key, err)
	}
	return true, nil
}

// setExtra stores a subsystem's server-only state under key.
func (state *islandState) setExtra(key string, value any) error {
	raw, err := json.Marshal(value)
	if err != nil {
		return err
	}
	state.extra[key] = raw
	return nil
}

// islandSettings are island_set's rows by key: key_value_int, and
// key_value_varchar for the lists.
var islandSettings map[string]gamedata.Setting

// islandSetInt is an island_set key_value_int.
func islandSetInt(key string) uint32 {
	return islandSettings[key].KeyValueInt
}

// islandSetList decodes an island_set key_value_varchar into value.
func islandSetList(key string, value any) error {
	setting, ok := islandSettings[key]
	if !ok || json.Unmarshal(setting.KeyValueVarchar, value) != nil {
		return gamedata.Missing("island_set", "%s key_value_varchar", key)
	}
	return nil
}

// islandMapConfig is an island_map row: a scene of the island and when it
// is open.
type islandMapConfig struct {
	Time gamedata.Window `json:"time"`
}

// islandDressConfig is an island_dress_template row: an outfit part (type)
// of the commander (belongto 1) or of a character.
type islandDressConfig struct {
	Type     uint32 `json:"type"`
	BelongTo uint32 `json:"belongto"`
	IsHide   uint32 `json:"is_hide"`
}

// islandDressColorConfig is an island_dress_colordiff_template row: a color
// of an outfit part.
type islandDressColorConfig struct {
	Dress uint32 `json:"belongto_dress"`
}

// islandPlaceConfig is an island_production_place row: a production site,
// there from the start (unlock_type 0) or unlocked by an ability.
type islandPlaceConfig struct {
	UnlockType uint32 `json:"unlock_type"`
	MapID      uint32 `json:"map_id"`
}

var (
	islandPlaces      map[uint32]islandPlaceConfig
	islandMaps        map[uint32]islandMapConfig
	islandDresses     map[uint32]islandDressConfig
	islandDressColors map[uint32]islandDressColorConfig
	// a new island's outfit: what the commander wears (by part type) and
	// owns, with the colors it owns (island_set default_dress, initial_dress,
	// initial_dress_colordiff)
	islandStart struct {
		dress      []uint32
		ownedDress []uint32
		colors     []uint32
		season     uint32
		scene      uint32
	}
)

// loadIslandLifeData reads the tables of every island subsystem.
func loadIslandLifeData() error {
	return gamedata.LoadAll(
		loadIslandCoreData,
		loadIslandInventoryData,
		loadIslandTaskData,
		loadIslandOrderData,
		loadIslandProductionData,
		loadIslandShipData,
		loadIslandSocialData,
	)
}

func loadIslandCoreData() error {
	err := gamedata.LoadAll(
		func() (err error) {
			islandSettings, err = gamedata.Settings("island_set")
			return err
		},
		gamedata.Load("island_map", &islandMaps),
		gamedata.Load("island_production_place", &islandPlaces),
		gamedata.Load("island_dress_template", &islandDresses),
		gamedata.Load("island_dress_colordiff_template", &islandDressColors),
		loadIslandDropData,
	)
	if err != nil {
		return err
	}
	for _, key := range []struct {
		name  string
		value *[]uint32
	}{
		{"default_dress", &islandStart.dress},
		{"initial_dress", &islandStart.ownedDress},
		{"initial_dress_colordiff", &islandStart.colors},
	} {
		if err := islandSetList(key.name, key.value); err != nil {
			return err
		}
	}
	for _, id := range append(slices.Clone(islandStart.dress), islandStart.ownedDress...) {
		if _, ok := islandDresses[id]; !ok {
			return gamedata.Missing("island_dress_template", "dress %d", id)
		}
	}
	islandStart.season = islandSetInt("season_now")
	islandStart.scene = islandSetInt("initial_scene")
	if _, ok := islandMaps[islandStart.scene]; !ok {
		return gamedata.Missing("island_map", "initial_scene %d", islandStart.scene)
	}
	return nil
}

// newIsland is the island a commander starts with: level 1, the first
// warehouse and plaza levels, the current development season, and the
// starting outfit (worn and owned, seen), the production sites there from
// the start (islandNewBuilding). Everything else the client fills
// in itself (the plaza's starting furniture: Island.AddDefaultAgoraData) or
// is earned. GUESS: no capture shows a new island; the open flag, invite
// code and the rest start empty.
func newIsland(commanderID uint32, now uint32) *islandState {
	dress := &protobuf.PB_ISLAND_USER_DRESS_SYS{}
	for _, id := range islandStart.dress {
		dress.CurDress = append(dress.CurDress, &protobuf.PB_ISLAND_CUR_DRESS{
			Type: proto.Uint32(islandDresses[id].Type), Id: proto.Uint32(id),
		})
	}
	for _, id := range islandStart.ownedDress {
		owned := &protobuf.PB_ISLAND_DRESS_USER{Id: proto.Uint32(id), State: proto.Uint32(1)}
		for _, color := range islandStart.colors {
			if islandDressColors[color].Dress == id {
				owned.ColorList = append(owned.ColorList, color)
			}
		}
		dress.HadDress = append(dress.HadDress, owned)
	}
	island := &protobuf.PB_ISLAND{
		PublicData: &protobuf.PB_ISLAND_PUBLIC{
			Id:           proto.Uint32(commanderID),
			Level:        proto.Uint32(1),
			StorageLevel: proto.Uint32(1),
			AgoraLevel:   proto.Uint32(1),
		},
		PrivateData: &protobuf.PB_ISLAND_PRIVATE{
			DailyTimestamp: proto.Uint32(now),
			Season:         &protobuf.PB_ISLAND_SEASON{Id: proto.Uint32(islandStart.season)},
			UserDress:      dress,
		},
	}
	for _, id := range slices.Sorted(maps.Keys(islandPlaces)) {
		if islandPlaces[id].UnlockType == 0 {
			island.PublicData.BuildList = append(island.PublicData.BuildList, islandNewBuilding(id))
		}
	}
	state := &islandState{
		commanderID: commanderID,
		island:      protocol.FillRequired(island),
		extra:       map[string]json.RawMessage{},
	}
	islandFishingDefaults(state)
	islandSocialRefresh(state, serverNow())
	return state
}

// readIsland loads a commander's island; nil when it has none yet.
func readIsland(ctx context.Context, queries *db.Queries, commanderID uint32) (*islandState, error) {
	row, err := queries.GetIsland(ctx, commanderID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	state := &islandState{commanderID: commanderID, island: &protobuf.PB_ISLAND{}, extra: map[string]json.RawMessage{}}
	if err := proto.Unmarshal(row.Island, state.island); err != nil {
		return nil, fmt.Errorf("island %d: %w", commanderID, err)
	}
	if state.island.PrivateData == nil {
		state.island.PrivateData = &protobuf.PB_ISLAND_PRIVATE{}
	}
	protocol.FillRequired(state.island)
	islandFishingDefaults(state)
	// the sites there from the start, also on an island made before one was
	for _, id := range slices.Sorted(maps.Keys(islandPlaces)) {
		public := state.public()
		if islandPlaces[id].UnlockType == 0 && !slices.ContainsFunc(public.BuildList, func(build *protobuf.PB_ISLAND_BUILD) bool { return build.GetId() == id }) {
			public.BuildList = append(public.BuildList, islandNewBuilding(id))
		}
	}
	if len(row.Position) > 0 {
		state.position = &protobuf.PB_PLAYER_POS_RECORD{}
		if err := proto.Unmarshal(row.Position, state.position); err != nil {
			return nil, fmt.Errorf("island %d position: %w", commanderID, err)
		}
	}
	if err := json.Unmarshal([]byte(row.Extra), &state.extra); err != nil {
		return nil, fmt.Errorf("island %d extra: %w", commanderID, err)
	}
	islandSocialRefresh(state, serverNow())
	return state, nil
}

// saveIsland stores a commander's island.
func saveIsland(ctx context.Context, queries *db.Queries, state *islandState) error {
	island, err := proto.Marshal(state.island)
	if err != nil {
		return err
	}
	position := []byte{}
	if state.position != nil {
		if position, err = proto.Marshal(state.position); err != nil {
			return err
		}
	}
	extra, err := json.Marshal(state.extra)
	if err != nil {
		return err
	}
	return queries.SaveIsland(ctx, db.SaveIslandParams{
		CommanderID: state.commanderID, Island: island, Position: position, Extra: string(extra),
		InviteCode: state.private().GetInviteCode(),
	})
}

// island is the commander's own island, created on first use, loaded once
// per request; Game.tx saves it when the request commits.
func (tx *Tx) island() (*islandState, error) {
	if tx.ownIsland != nil {
		return tx.ownIsland, nil
	}
	state, err := readIsland(tx.ctx, tx.Queries, tx.commanderID)
	if err != nil {
		return nil, err
	}
	if state == nil {
		state = newIsland(tx.commanderID, uint32(serverNow().Unix()))
	}
	tx.ownIsland = state
	return state, nil
}

// otherIsland is another commander's island as it is stored, read only
// (changes to it are not saved); nil when that commander has none.
func (tx *Tx) otherIsland(commanderID uint32) (*islandState, error) {
	if commanderID == tx.commanderID {
		return tx.island()
	}
	return readIsland(tx.ctx, tx.Queries, commanderID)
}

// saveOwnIsland stores the island Tx.island loaded, if any.
func (tx *Tx) saveOwnIsland() error {
	if tx.ownIsland == nil {
		return nil
	}
	return saveIsland(tx.ctx, tx.Queries, tx.ownIsland)
}
