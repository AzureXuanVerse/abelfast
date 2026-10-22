package game

// The back-office's views and edits of a commander's exercises, Operation
// Siren, META Showdown, backyard and friends. Every edit of what the client
// keeps a copy of kicks the commander first (see AdminWrite).

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"google.golang.org/protobuf/proto"

	"github.com/ggmolly/belfast/internal/db"
	"github.com/ggmolly/belfast/internal/protobuf"
)

// Exercises

// ExerciseRank is an arena_data_rank row.
type ExerciseRank struct {
	ID    uint32 `json:"id"`
	Name  string `json:"name"`
	Point uint32 `json:"point"`
	Order uint32 `json:"order"`
}

// ExerciseRival is a current opponent.
type ExerciseRival struct {
	ID       uint32 `json:"id"`
	Name     string `json:"name"`
	Level    uint32 `json:"level"`
	Score    uint32 `json:"score"`
	Standing uint32 `json:"standing"`
	Rank     string `json:"rank"`
}

// AdminShip is a ship as the back-office lists it.
type AdminShip struct {
	ID         uint32 `json:"id"`
	TemplateID uint32 `json:"template_id"`
	SkinID     uint32 `json:"skin_id"`
	Name       string `json:"name"`
	Level      uint32 `json:"level"`
}

func adminShips(ships []db.Ship) []AdminShip {
	list := make([]AdminShip, len(ships))
	for i, ship := range ships {
		list[i] = AdminShip{ID: ship.ID, TemplateID: ship.TemplateID, SkinID: ship.SkinID, Name: ship.Name, Level: ship.Level}
	}
	return list
}

// ExerciseView is a commander's exercise season as of now (loadExercise).
type ExerciseView struct {
	Season        int64           `json:"season"`
	SeasonEnd     int64           `json:"season_end"`
	Score         uint32          `json:"score"`
	Standing      uint32          `json:"standing"`
	Rank          ExerciseRank    `json:"rank"`
	Next          *ExerciseRank   `json:"next,omitempty"`
	BestRank      uint32          `json:"best_rank"`
	FightCount    uint32          `json:"fight_count"`
	FightMax      uint32          `json:"fight_max"`
	RefilledAt    int64           `json:"refilled_at"`
	NextRefill    int64           `json:"next_refill"`
	FlashCount    uint32          `json:"flash_count"`
	FlashMax      uint32          `json:"flash_max"`
	ShopRefreshes uint32          `json:"shop_refreshes"`
	Vanguard      []AdminShip     `json:"vanguard"`
	Main          []AdminShip     `json:"main"`
	Rivals        []ExerciseRival `json:"rivals"`
	Ranks         []ExerciseRank  `json:"ranks"`
}

func exerciseRank(row arenaRank) ExerciseRank {
	return ExerciseRank{ID: row.ID, Name: row.Name, Point: row.Point, Order: row.Order}
}

// Exercise is the commander's exercise season.
func (game *Game) Exercise(ctx context.Context, id uint32) (ExerciseView, error) {
	now := time.Now()
	queries := game.DB.Queries
	state, err := loadExercise(ctx, queries, id, now)
	if err != nil {
		return ExerciseView{}, err
	}
	standings, err := listExerciseStandings(ctx, queries, now)
	if err != nil {
		return ExerciseView{}, err
	}
	standing := standings.rank(id)
	held := militaryRank(state.Score, standing)
	view := ExerciseView{
		Season: state.Season, SeasonEnd: state.Season + int64(exerciseSeasonLength/time.Second),
		Score: state.Score, Standing: standing, Rank: exerciseRank(held), BestRank: state.BestRank,
		FightCount: state.FightCount, FightMax: exerciseFightCount, RefilledAt: state.RefilledAt, NextRefill: nextExerciseReset(now).Unix(),
		FlashCount: state.FlashCount, FlashMax: exerciseRivalReplace, ShopRefreshes: state.ShopRefreshes,
		Rivals: []ExerciseRival{}, Ranks: []ExerciseRank{},
	}
	for _, row := range arenaRanks {
		view.Ranks = append(view.Ranks, exerciseRank(row))
		if row.ID == held.ID+1 {
			next := exerciseRank(row)
			view.Next = &next
		}
	}
	vanguard, main, err := defenseFleet(ctx, queries, state)
	if err != nil {
		return view, err
	}
	view.Vanguard, view.Main = adminShips(vanguard), adminShips(main)
	for _, rival := range state.Rivals {
		for i, row := range standings {
			if row.ID == rival {
				score := uint32(row.Score)
				view.Rivals = append(view.Rivals, ExerciseRival{
					ID: row.ID, Name: row.Name, Level: row.Level, Score: score, Standing: uint32(i + 1),
					Rank: militaryRank(score, uint32(i+1)).Name,
				})
			}
		}
	}
	return view, nil
}

// editExercise changes the commander's exercise state and saves it.
func (game *Game) editExercise(ctx context.Context, id uint32, fn func(queries *db.Queries, state *db.CommanderExercise) error) (bool, error) {
	return game.AdminWrite(ctx, id, func(tx *Tx) error {
		state, err := loadExercise(ctx, tx.Queries, id, time.Now())
		if err != nil {
			return err
		}
		if err := fn(tx.Queries, &state); err != nil {
			return err
		}
		return saveExercise(ctx, tx.Queries, state)
	})
}

// SetExerciseScore sets the season's score. The ranks it reaches count as
// reached (best_rank), so the next battle doesn't pay their promotions.
func (game *Game) SetExerciseScore(ctx context.Context, id, score uint32) (bool, error) {
	return game.editExercise(ctx, id, func(queries *db.Queries, state *db.CommanderExercise) error {
		state.Score = score
		if err := saveExercise(ctx, queries, *state); err != nil {
			return err
		}
		standings, err := listExerciseStandings(ctx, queries, time.Now())
		if err != nil {
			return err
		}
		state.BestRank = max(state.BestRank, militaryRank(score, standings.rank(id)).ID)
		return nil
	})
}

// RefillExercise gives back every attempt (SeasonInfo MAX_FIGHTCOUNT).
func (game *Game) RefillExercise(ctx context.Context, id uint32) (bool, error) {
	return game.editExercise(ctx, id, func(_ *db.Queries, state *db.CommanderExercise) error {
		state.FightCount = exerciseFightCount
		return nil
	})
}

// NewExerciseRivals forgets the current opponents: new ones are drawn when
// the client next asks for the season (rivalList). The day's replacements
// aren't spent.
func (game *Game) NewExerciseRivals(ctx context.Context, id uint32) (bool, error) {
	return game.editExercise(ctx, id, func(_ *db.Queries, state *db.CommanderExercise) error {
		state.Rivals = nil
		return nil
	})
}

// Operation Siren

// WorldMapView is a map the world has entered.
type WorldMapView struct {
	RandomID uint32 `json:"random_id"`
	Name     string `json:"name"`
	Clear    bool   `json:"clear"`
	Active   bool   `json:"active"`
}

// WorldView is a commander's Operation Siren.
type WorldView struct {
	Started     bool           `json:"started"`
	Activated   bool           `json:"activated"`
	Progress    uint32         `json:"progress"`
	ResetStage  uint32         `json:"reset_stage"`
	ExpiredTime int64          `json:"expired_time"`
	Stamina     uint32         `json:"stamina"`
	StaminaMax  uint32         `json:"stamina_max"`
	AtlasMaps   int            `json:"atlas_maps"`
	Cleared     int            `json:"cleared"`
	Maps        []WorldMapView `json:"maps"`
	Resettable  bool           `json:"resettable"`
}

// resettable is when the monthly reset may run (handleWorldKill): the
// world activated and the story at world_resetting_stage.
func (state *worldState) resettable() bool {
	return state.activated() && state.Count.Progress >= worldSettings.resetStage
}

// World is the commander's Operation Siren.
func (game *Game) World(ctx context.Context, id uint32) (WorldView, error) {
	now := serverNow()
	state, err := loadWorldState(ctx, game.DB.Queries, id, now)
	if err != nil {
		return WorldView{}, err
	}
	view := WorldView{
		Started: state.ActivateTime != 0 || state.Count.Activations != 0, Activated: state.activated(),
		Progress: state.Count.Progress, ResetStage: worldSettings.resetStage, ExpiredTime: int64(state.ExpiredTime),
		Stamina: state.Stamina.total(), StaminaMax: worldSettings.maxStamina, AtlasMaps: len(state.Maps),
		Cleared: len(state.CleanChapters), Maps: []WorldMapView{}, Resettable: state.resettable(),
	}
	rows, err := game.DB.AdminSocialWorldMaps(ctx, id)
	if err != nil {
		return view, err
	}
	for _, row := range rows {
		var m worldMap
		if err := json.Unmarshal(row.State, &m); err != nil {
			return view, fmt.Errorf("world map %d: %w", row.RandomID, err)
		}
		view.Maps = append(view.Maps, WorldMapView{
			RandomID: row.RandomID, Name: worldRandomMaps[row.RandomID].Name, Clear: m.Clear, Active: row.RandomID == state.MapID,
		})
	}
	return view, nil
}

// ResetWorld runs the monthly reset now (handleWorldKill's reset, without
// waiting for its due date): the world inventory turns into what
// worldResetDrops gives, granted, and the maps start over. It is refused
// where the game refuses it: a world not activated or before the story's
// reset stage. It tells whether the commander was kicked.
func (game *Game) ResetWorld(ctx context.Context, id uint32) ([]*protobuf.DROPINFO, bool, error) {
	var drops []*protobuf.DROPINFO
	kicked, err := game.AdminWrite(ctx, id, func(tx *Tx) error {
		now := serverNow()
		state, err := tx.worldState(now)
		if err != nil {
			return err
		}
		if !state.resettable() {
			return AdminError{Msg: "the world can't be reset: not activated or before the reset stage"}
		}
		if drops, err = tx.worldResetDrops(state); err != nil {
			return err
		}
		if err := tx.DeleteWorldMaps(tx.ctx, id); err != nil {
			return err
		}
		return tx.saveWorldState(state.reset(now))
	})
	return drops, kicked, err
}

// DefenseFleet is the commander's exercise defense fleet, vanguard then
// main ship ids, and whether it was set (else it is fleet 1's ships, as
// rivals see it).
func (game *Game) DefenseFleet(ctx context.Context, id uint32) ([]uint32, bool, error) {
	queries := game.DB.Queries
	state, err := loadExercise(ctx, queries, id, time.Now())
	if err != nil {
		return nil, false, err
	}
	vanguard, main, err := defenseFleet(ctx, queries, state)
	if err != nil {
		return nil, false, err
	}
	ids := []uint32{}
	for _, ship := range append(vanguard, main...) {
		ids = append(ids, ship.ID)
	}
	return ids, len(state.Vanguard)+len(state.Main) > 0, nil
}

// SetDefenseFleet sets the exercise defense fleet, as UpdateExerciseFleetCommand
// does: ships are split into its sides by their team.
func (game *Game) SetDefenseFleet(ctx context.Context, id uint32, ships []uint32) (bool, error) {
	return game.AdminWrite(ctx, id, func(tx *Tx) error {
		byID, err := shipsByID(tx.ctx, tx.Queries, id, ships)
		if err != nil {
			return err
		}
		var vanguard, main []uint32
		for _, shipID := range ships {
			if ship, ok := byID[shipID]; ok && shipTeam(ship) == teamVanguard {
				vanguard = append(vanguard, shipID)
			} else {
				main = append(main, shipID)
			}
		}
		if err := checkDefenseFleet(tx.ctx, tx.Queries, id, vanguard, main); errors.Is(err, errRefused) {
			return AdminError{Msg: "the defense fleet takes 1 to 3 vanguard and 1 to 3 main ships, each once"}
		} else if err != nil {
			return err
		}
		state, err := loadExercise(tx.ctx, tx.Queries, id, time.Now())
		if err != nil {
			return err
		}
		state.Vanguard, state.Main = vanguard, main
		return saveExercise(tx.ctx, tx.Queries, state)
	})
}

// SetWorldStamina sets the world's action points: up to the max as the
// points that recover (action_power), the rest as extra points above it
// (action_power_extra), as WorldStaminaManager splits them.
func (game *Game) SetWorldStamina(ctx context.Context, id, total uint32) (bool, error) {
	return game.AdminWrite(ctx, id, func(tx *Tx) error {
		state, err := tx.worldState(serverNow())
		if err != nil {
			return err
		}
		if !state.activated() {
			return AdminError{Msg: "the world isn't activated"}
		}
		state.Stamina.Power = min(total, worldSettings.maxStamina)
		state.Stamina.Extra = total - state.Stamina.Power
		return tx.saveWorldState(state)
	})
}

// META Showdown

// SetMetaPt sets a META ship group's Sync Points, at most its last reward's
// (ship_strengthen_meta target). The rewards taken stay taken.
func (game *Game) SetMetaPt(ctx context.Context, id, group, pt uint32) (bool, error) {
	config, ok := metaShips[group]
	if !ok || len(config.Target) == 0 {
		return false, adminRefused("no META ship group %d", group)
	}
	if top := config.Target[len(config.Target)-1]; pt > top {
		return false, adminRefused("its Sync Points go up to %d", top)
	}
	return game.AdminWrite(ctx, id, func(tx *Tx) error {
		return tx.AdminSocialSetMetaPt(tx.ctx, db.AdminSocialSetMetaPtParams{CommanderID: id, GroupID: group, Pt: pt})
	})
}

// MetaBossView is the boss the commander summoned.
type MetaBossView struct {
	TemplateID uint32 `json:"template_id"`
	MetaShip   uint32 `json:"meta_ship"`
	Level      uint32 `json:"level"`
	HP         uint32 `json:"hp"`
	MaxHP      uint32 `json:"max_hp"`
	ExpiresAt  int64  `json:"expires_at"`
	KilledAt   int64  `json:"killed_at"`
	Fights     uint32 `json:"fights"`
	Damage     uint32 `json:"damage"`
	Guild      bool   `json:"guild_support"`
	Friend     bool   `json:"friend_support"`
	World      bool   `json:"world_support"`
}

// MetaGroupView is a META ship group's Sync Points.
type MetaGroupView struct {
	Group   uint32 `json:"group"`
	Pt      uint32 `json:"pt"`
	Claimed int    `json:"claimed"`
	Steps   int    `json:"steps"`
	Max     uint32 `json:"max"`
}

// MetaView is a commander's META Showdown.
type MetaView struct {
	SummonPt    uint32          `json:"summon_pt"`
	SummonPtOld uint32          `json:"summon_pt_old"`
	SelfBossLv  uint32          `json:"self_boss_lv"`
	CurrentShip uint32          `json:"current_ship"`
	Boss        *MetaBossView   `json:"boss,omitempty"`
	Groups      []MetaGroupView `json:"groups"`
}

// Meta is the commander's META Showdown, read without creating its row.
func (game *Game) Meta(ctx context.Context, id uint32) (MetaView, error) {
	view := MetaView{SelfBossLv: 1, Groups: []MetaGroupView{}}
	if current := currentWorldBoss(time.Now()); current != 0 {
		view.CurrentShip = worldBossTemplates[current].MetaID
	}
	row, err := game.DB.GetWorldBoss(ctx, id)
	switch {
	case errors.Is(err, sql.ErrNoRows):
	case err != nil:
		return view, err
	default:
		view.SummonPt, view.SummonPtOld = row.SummonPt, row.SummonPtOld
		if row.SelfBossTemplateID == currentWorldBoss(time.Now()) {
			view.SelfBossLv = row.SelfBossLv
		}
		if row.BossTemplateID != 0 {
			now := time.Now().Unix()
			view.Boss = &MetaBossView{
				TemplateID: row.BossTemplateID, MetaShip: worldBossTemplates[row.BossTemplateID].MetaID,
				Level: row.BossLv, HP: row.BossHp, MaxHP: worldBossLevel(row.BossTemplateID, row.BossLv).HP,
				ExpiresAt: row.BossLastTime, KilledAt: row.BossKillTime, Fights: row.BossFightCount, Damage: row.BossDamage,
				Guild: row.GuildSupport > now, Friend: row.FriendSupport > now, World: row.WorldSupport > now,
			}
		}
	}
	pts, err := game.DB.ListMetaPts(ctx, id)
	if err != nil {
		return view, err
	}
	awards, err := game.DB.ListMetaPtAwards(ctx, id)
	if err != nil {
		return view, err
	}
	for _, pt := range pts {
		config := metaShips[pt.GroupID]
		group := MetaGroupView{Group: pt.GroupID, Pt: pt.Pt, Steps: len(config.Target)}
		if len(config.Target) > 0 {
			group.Max = config.Target[len(config.Target)-1]
		}
		for _, award := range awards {
			if award.GroupID == pt.GroupID {
				group.Claimed++
			}
		}
		view.Groups = append(view.Groups, group)
	}
	return view, nil
}

// Backyard

// DormLevel is a dorm_data_template row.
type DormLevel struct {
	ID       uint32 `json:"id"`
	Capacity uint32 `json:"capacity"`
	Slots    uint32 `json:"slots"`
}

// DormShipView is a ship in the backyard.
type DormShipView struct {
	AdminShip
	Floor       uint32 `json:"floor"`
	PopIcon     uint32 `json:"pop_icon"`
	PopIntimacy uint32 `json:"pop_intimacy"`
}

// DormFloorView is a floor's arrangement.
type DormFloorView struct {
	Floor  uint32 `json:"floor"`
	Placed int    `json:"placed"`
	// Starter is the starter arrangement of a floor 1 never arranged
	Starter bool `json:"starter"`
}

// DormThemeView is a saved theme template.
type DormThemeView struct {
	Pos      uint32 `json:"pos"`
	Name     string `json:"name"`
	Pieces   int    `json:"pieces"`
	SharedAt int64  `json:"shared_at"`
	Likes    int64  `json:"likes"`
	Favs     int64  `json:"favs"`
}

// DormView is a commander's backyard.
type DormView struct {
	Name           string          `json:"name"`
	Level          uint32          `json:"level"`
	MaxLevel       uint32          `json:"max_level"`
	Floors         uint32          `json:"floors"`
	MaxFloors      uint32          `json:"max_floors"`
	Slots          uint32          `json:"slots"`
	MaxSlots       uint32          `json:"max_slots"`
	Food           uint32          `json:"food"`
	FoodMax        uint32          `json:"food_max"`
	FoodExtensions uint32          `json:"food_extensions"`
	Comfort        uint32          `json:"comfort"`
	Furniture      uint32          `json:"furniture"`
	NextSettle     int64           `json:"next_settle"`
	Ships          []DormShipView  `json:"ships"`
	FloorList      []DormFloorView `json:"floor_list"`
	Themes         []DormThemeView `json:"themes"`
}

// Dorm is the commander's backyard, a new commander's when it has no row.
func (game *Game) Dorm(ctx context.Context, id uint32) (DormView, error) {
	queries := game.DB.Queries
	commander, err := queries.GetCommander(ctx, id)
	if err != nil {
		return DormView{}, err
	}
	dorm, err := queries.GetDorm(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		dorm, err = db.CommanderDorm{CommanderID: id, Level: 1, FloorNum: 1, ExpPos: 2}, nil
	}
	if err != nil {
		return DormView{}, err
	}
	owned, err := queries.ListFurniture(ctx, id)
	if err != nil {
		return DormView{}, err
	}
	view := DormView{
		Name: dorm.Name, Level: dorm.Level, MaxLevel: dormMaxLevel, Floors: dorm.FloorNum, MaxFloors: dormMaxFloor,
		Slots: dorm.ExpPos, MaxSlots: dormLevels[dorm.Level].TrainingShips, Food: commander.DormFood, FoodMax: dormFoodMax(dorm),
		FoodExtensions: dorm.FoodMaxIncreaseCount, Comfort: dormComfort(dorm, owned), NextSettle: dorm.NextTimestamp,
		Ships: []DormShipView{}, FloorList: []DormFloorView{}, Themes: []DormThemeView{},
	}
	for _, row := range owned {
		view.Furniture += row.Count
	}
	inDorm, err := queries.ListDormShips(ctx, id)
	if err != nil {
		return view, err
	}
	ids := make([]uint32, len(inDorm))
	for i, row := range inDorm {
		ids[i] = row.ShipID
	}
	ships, err := shipsByID(ctx, queries, id, ids)
	if err != nil {
		return view, err
	}
	for _, row := range inDorm {
		ship := ships[row.ShipID]
		view.Ships = append(view.Ships, DormShipView{
			AdminShip: AdminShip{ID: ship.ID, TemplateID: ship.TemplateID, SkinID: ship.SkinID, Name: ship.Name, Level: ship.Level},
			Floor:     row.Floor, PopIcon: row.PopIcon, PopIntimacy: row.PopIntimacy,
		})
	}
	layouts, err := floorLayouts(ctx, queries, id)
	if err != nil {
		return view, err
	}
	for floor := uint32(1); floor <= dorm.FloorNum; floor++ {
		layout, ok := layouts[floor]
		switch {
		case ok:
			view.FloorList = append(view.FloorList, DormFloorView{Floor: floor, Placed: len(layout.GetFurniturePutList())})
		case floor == dormFloorTrain:
			view.FloorList = append(view.FloorList, DormFloorView{Floor: floor, Placed: len(furnitureStart), Starter: true})
		default:
			view.FloorList = append(view.FloorList, DormFloorView{Floor: floor})
		}
	}
	themes, err := queries.AdminSocialDormThemes(ctx, id)
	if err != nil {
		return view, err
	}
	for _, row := range themes {
		theme := &protobuf.DORMTHEME{}
		if err := proto.Unmarshal(row.Theme, theme); err != nil {
			return view, fmt.Errorf("dorm theme %d: %w", row.Pos, err)
		}
		view.Themes = append(view.Themes, DormThemeView{
			Pos: row.Pos, Name: theme.GetName(), Pieces: len(theme.GetFurniturePutList()),
			SharedAt: row.UploadTime, Likes: row.LikeCount, Favs: row.FavCount,
		})
	}
	return view, nil
}

// DormLevels are the backyard's levels.
func DormLevels() []DormLevel {
	levels := []DormLevel{}
	for id := uint32(1); id <= dormMaxLevel; id++ {
		if row, ok := dormLevels[id]; ok {
			levels = append(levels, DormLevel{ID: id, Capacity: row.Capacity, Slots: row.TrainingShips})
		}
	}
	return levels
}

// editDorm settles the training up to now, the rates it ran at, then runs
// fn on the backyard.
func (game *Game) editDorm(ctx context.Context, id uint32, fn func(tx *Tx, dorm db.CommanderDorm) error) (bool, error) {
	return game.AdminWrite(ctx, id, func(tx *Tx) error {
		now := time.Now().Unix()
		if _, err := tx.settleDorm(now); err != nil {
			return err
		}
		dorm, err := tx.dorm(now)
		if err != nil {
			return err
		}
		return fn(tx, dorm)
	})
}

// SetDormLevel raises the backyard to level, as Expansion Licenses do
// (expandDorm). Lowering it is refused: the furniture placed may not fit a
// smaller room.
func (game *Game) SetDormLevel(ctx context.Context, id, level uint32) (bool, error) {
	return game.editDorm(ctx, id, func(tx *Tx, dorm db.CommanderDorm) error {
		if level < dorm.Level {
			return AdminError{Msg: "the backyard can't shrink"}
		}
		if level > dormMaxLevel {
			return adminRefused("the backyard's highest level is %d", dormMaxLevel)
		}
		return tx.SetDormLevel(tx.ctx, db.SetDormLevelParams{Level: level, CommanderID: id})
	})
}

// SetDormFood sets the backyard's food, at most what it holds
// (Dorm:GetCapcity).
func (game *Game) SetDormFood(ctx context.Context, id, food uint32) (bool, error) {
	return game.editDorm(ctx, id, func(tx *Tx, dorm db.CommanderDorm) error {
		if limit := dormFoodMax(dorm); food > limit {
			return adminRefused("the backyard holds %d food", limit)
		}
		return tx.SetDormFood(tx.ctx, db.SetDormFoodParams{DormFood: food, ID: id})
	})
}

// FillDormFood fills the backyard's food.
func (game *Game) FillDormFood(ctx context.Context, id uint32) (bool, error) {
	return game.editDorm(ctx, id, func(tx *Tx, dorm db.CommanderDorm) error {
		return tx.SetDormFood(tx.ctx, db.SetDormFoodParams{DormFood: dormFoodMax(dorm), ID: id})
	})
}

// RemoveDormShip takes a ship out of the backyard as ShipExitCommand does,
// with the training settled up to now; what it hadn't popped is lost.
func (game *Game) RemoveDormShip(ctx context.Context, id, shipID uint32) (bool, error) {
	return game.editDorm(ctx, id, func(tx *Tx, _ db.CommanderDorm) error {
		removed, err := tx.DeleteDormShip(tx.ctx, db.DeleteDormShipParams{ShipID: shipID, CommanderID: id})
		if err == nil && removed == 0 {
			err = adminRefused("ship %d isn't in the backyard", shipID)
		}
		return err
	})
}

// ClearDormFloor takes every piece of furniture off a floor (the furniture
// stays owned), as a PutFurnitureCommand of an empty floor would.
func (game *Game) ClearDormFloor(ctx context.Context, id, floor uint32) (bool, error) {
	return game.editDorm(ctx, id, func(tx *Tx, dorm db.CommanderDorm) error {
		if floor < 1 || floor > dorm.FloorNum {
			return adminRefused("the backyard has %d floors", dorm.FloorNum)
		}
		layout, err := proto.Marshal(&protobuf.FURFLOORPUTINFO{Floor: proto.Uint32(floor)})
		if err != nil {
			return err
		}
		return tx.SetDormFloor(tx.ctx, db.SetDormFloorParams{CommanderID: id, Floor: floor, Layout: layout})
	})
}

// Friends

// MaxFriends is how many friends a commander may have.
const MaxFriends = maxFriends

// RemoveFriend ends a friendship on both sides and tells whoever plays, as
// DeleteFriendCommand does: no one is kicked.
func (game *Game) RemoveFriend(ctx context.Context, id, friendID uint32) (bool, error) {
	_, _, err := game.commit(ctx, id, func(tx *Tx) error {
		removed, err := tx.DeleteFriendship(ctx, db.DeleteFriendshipParams{CommanderID: id, FriendID: friendID})
		if err == nil && removed == 0 {
			err = adminRefused("%d isn't a friend", friendID)
		}
		return err
	})
	if err != nil {
		return false, err
	}
	game.deliver(nil, id, &protobuf.SC_50013{Id: proto.Uint32(friendID)})
	game.deliver(nil, friendID, &protobuf.SC_50013{Id: proto.Uint32(id)})
	return false, nil
}

// AcceptFriendRequest accepts sender's request as AcceptFriendRequestCommand
// does: both become friends (each list at most maxFriends), the requests
// both ways are gone and the sender leaves the blacklist. The commander is
// kicked (its client keeps the request), the sender told when it plays.
func (game *Game) AcceptFriendRequest(ctx context.Context, id, senderID uint32) (bool, error) {
	var self, sender db.Commander
	kicked, err := game.AdminWrite(ctx, id, func(tx *Tx) error {
		removed, err := tx.DeleteFriendRequest(ctx, db.DeleteFriendRequestParams{CommanderID: id, SenderID: senderID})
		if err != nil {
			return err
		}
		if removed == 0 {
			return adminRefused("no request from %d", senderID)
		}
		if sender, err = tx.GetCommander(ctx, senderID); err != nil {
			return err
		}
		if self, err = tx.GetCommander(ctx, id); err != nil {
			return err
		}
		friend, err := tx.CountFriend(ctx, db.CountFriendParams{CommanderID: id, FriendID: senderID})
		if err != nil || friend > 0 {
			return err
		}
		now := time.Now().Unix()
		for _, side := range [][2]uint32{{id, senderID}, {senderID, id}} {
			friends, err := tx.CountFriends(ctx, side[0])
			if err != nil {
				return err
			}
			if friends >= maxFriends {
				return adminRefused("commander %d already has %d friends", side[0], maxFriends)
			}
			if err := tx.AddFriend(ctx, db.AddFriendParams{CommanderID: side[0], FriendID: side[1], Since: now}); err != nil {
				return err
			}
		}
		if _, err := tx.DeleteFriendRequest(ctx, db.DeleteFriendRequestParams{CommanderID: senderID, SenderID: id}); err != nil {
			return err
		}
		_, err = tx.DeleteBlacklist(ctx, db.DeleteBlacklistParams{CommanderID: id, BlockedID: senderID})
		return err
	})
	if err == nil && sender.ID != 0 {
		game.deliver(nil, senderID, &protobuf.SC_50008{Player: game.friendInfo(self)})
	}
	return kicked, err
}

// RejectFriendRequest drops sender's request; the sender isn't told.
func (game *Game) RejectFriendRequest(ctx context.Context, id, senderID uint32) (bool, error) {
	return game.AdminWrite(ctx, id, func(tx *Tx) error {
		removed, err := tx.DeleteFriendRequest(ctx, db.DeleteFriendRequestParams{CommanderID: id, SenderID: senderID})
		if err == nil && removed == 0 {
			err = adminRefused("no request from %d", senderID)
		}
		return err
	})
}

// Unblock takes a player off the commander's blacklist.
func (game *Game) Unblock(ctx context.Context, id, blockedID uint32) (bool, error) {
	return game.AdminWrite(ctx, id, func(tx *Tx) error {
		removed, err := tx.DeleteBlacklist(ctx, db.DeleteBlacklistParams{CommanderID: id, BlockedID: blockedID})
		if err == nil && removed == 0 {
			err = adminRefused("%d isn't blocked", blockedID)
		}
		return err
	})
}
