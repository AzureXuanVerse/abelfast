package game

import (
	"cmp"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"math"
	"math/rand/v2"
	"slices"
	"strconv"
	"time"

	"google.golang.org/protobuf/proto"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/db"
	"github.com/ggmolly/belfast/internal/gamedata"
	"github.com/ggmolly/belfast/internal/logger"
	"github.com/ggmolly/belfast/internal/protobuf"
)

// The backyard (DormProxy, model/vo/dorm/dorm.lua). Ships train on floor 1
// (exp, eating the food) and rest on floor 2 (DormShip.FLOOR_1 / FLOOR_2);
// both pop coins and affection. None of it is in an official capture but
// the login's SC_19001 / SC_19009 / SC_19010: the replies are built from
// the Lua alone.

const (
	dormFloorTrain = 1 // DormShip.FLOOR_1
	dormFloorRest  = 2 // DormShip.FLOOR_2
	dormMaxFloor   = 2 // Dorm.MAX_FLOOR
	dormMaxLevel   = 4 // Dorm.MAX_LEVEL

	// Dorm.DORM_2_FLOOR_COMFORTABLE_ADDITION
	dorm2FloorComfort = 20

	// ResourceDormMoney is player_resource dormMoney, the furniture coins
	// the ships pop.
	ResourceDormMoney = 6

	// dormThemeSlots is how many theme templates a commander keeps
	// (DormProxy:GetTemplateNewID).
	dormThemeSlots = 10
	// themeTemplateCustom is BackYardConst.THEME_TEMPLATE_TYPE_CUSTOM.
	themeTemplateCustom = 2

	// dormMaxPopRolls bounds the pop rolls made for a ship in one CS_19009
	// (a day of dorm_pop_time periods).
	dormMaxPopRolls = 48

	// furnitureTokensTrophy is medal_template 10041, the Renovation Merit
	// Medal: decor tokens spent on furniture.
	furnitureTokensTrophy = 10041
)

// dorm_data_template row
type dormConfig struct {
	Capacity         uint32      `json:"capacity"`
	Comfortable      uint32      `json:"comfortable"`
	ComfortableCount [][2]uint32 `json:"comfortable_count"`
	Consume          uint32      `json:"consume"`
	Exp              uint32      `json:"exp"`
	Time             uint32      `json:"time"`
	TrainingShips    uint32      `json:"training_ship_number"`
}

// furniture_data_template row
type furnitureConfig struct {
	Type        uint32 `json:"type"`
	Comfortable uint32 `json:"comfortable"`
	Count       uint32 `json:"count"`
}

// furniture_shop_template row, what BuyFurnitureCommand prices
// (Furniture:getPrice, inTime, model/vo/newbackyard/furniture.lua)
type furnitureGoods struct {
	GemPrice      uint32          `json:"gem_price"`
	DormIconPrice uint32          `json:"dorm_icon_price"`
	Discount      uint32          `json:"discount"`
	DiscountTime  gamedata.Window `json:"discount_time"`
	Time          gamedata.Window `json:"time"`
	// a collaboration's furniture sells only while it is its time
	CollaborationTime json.RawMessage `json:"collaboration_furniture_time"`
	RawTime           json.RawMessage `json:"-"`
}

// furniture_init_data_template row: where the starter furniture stands
type furnitureInitConfig struct {
	X      uint32 `json:"x"`
	Y      uint32 `json:"y"`
	Dir    uint32 `json:"dir"`
	Parent uint64 `json:"parent"`
}

// dormGameset is the gameset rows of the backyard.
type dormGameset struct {
	expBase, comfortDegree uint32
	expRatio, foodRatio    []uint32 // by the training ship count, from 1
	onceLimitBase          uint32
	onceLimitByLevel       uint32
	popTime                int64
	coinPop, intimacyPop   dormPop
}

// dormPop is a gameset dorm_<kind>_pop_{min,max,rant}: what a ship may pop
// every dorm_pop_time, rant in thousandths.
type dormPop struct{ min, max, rant uint32 }

var (
	dormLevels     map[uint32]dormConfig
	furnitures     map[uint32]furnitureConfig
	furnitureShop  map[uint32]furnitureGoods
	furnitureStart []*protobuf.FURNITUREPUTINFO
	dormSettings   dormGameset
	// dormOpenLevel is open_systems_limited's CourtYardMediator level, the
	// commander level the backyard opens at (SystemOpenMgr:isOpenSystem)
	dormOpenLevel uint32
)

func loadDormData() error {
	var shop map[uint32]json.RawMessage
	var start map[uint32]furnitureInitConfig
	var gameset map[string]uint32
	var systems map[uint32]struct {
		Mediator string `json:"mediator"`
		Level    uint32 `json:"level"`
	}
	err := gamedata.LoadAll(
		gamedata.Load("open_systems_limited", &systems),
		gamedata.Load("dorm_data_template", &dormLevels),
		gamedata.Load("furniture_data_template", &furnitures),
		gamedata.Load("furniture_shop_template", &shop),
		gamedata.Load("furniture_init_data_template", &start),
		func() (err error) {
			gameset, err = gamedata.Gameset()
			return err
		},
	)
	if err != nil {
		return err
	}
	dormOpenLevel = 0
	for _, row := range systems {
		if row.Mediator == "CourtYardMediator" {
			dormOpenLevel = row.Level
		}
	}
	if dormOpenLevel == 0 {
		return gamedata.Missing("open_systems_limited", "CourtYardMediator")
	}
	for level := uint32(1); level <= dormMaxLevel; level++ {
		if row, ok := dormLevels[level]; !ok || row.Time == 0 {
			return gamedata.Missing("dorm_data_template", "level %d", level)
		}
	}
	furnitureShop = make(map[uint32]furnitureGoods, len(shop))
	for id, raw := range shop {
		var goods furnitureGoods
		if err := json.Unmarshal(raw, &goods); err != nil {
			return fmt.Errorf("furniture_shop_template %d: %w", id, err)
		}
		var row struct {
			Time json.RawMessage `json:"time"`
		}
		_ = json.Unmarshal(raw, &row)
		goods.RawTime = row.Time
		furnitureShop[id] = goods
	}
	furnitureStart = furnitureStart[:0]
	for _, id := range slices.Sorted(maps.Keys(start)) {
		row := start[id]
		furnitureStart = append(furnitureStart, &protobuf.FURNITUREPUTINFO{
			Id: proto.String(strconv.FormatUint(uint64(id), 10)), X: proto.Uint32(row.X), Y: proto.Uint32(row.Y),
			Dir: proto.Uint32(row.Dir), Parent: proto.Uint64(row.Parent), ShipId: proto.Uint32(0),
		})
	}
	get := func(key string) (uint32, error) {
		value, ok := gameset[key]
		if !ok {
			return 0, gamedata.Missing("gameset", "%s", key)
		}
		return value, nil
	}
	settings := dormGameset{}
	for _, field := range []struct {
		key   string
		value *uint32
	}{
		{"dorm_exp_base", &settings.expBase},
		{"dorm_exp_ratio_comfort_degree", &settings.comfortDegree},
		{"dorm_exp_once_limit_base", &settings.onceLimitBase},
		{"dorm_exp_once_limit_by_level", &settings.onceLimitByLevel},
		{"dorm_dorm_pop_min", &settings.coinPop.min},
		{"dorm_dorm_pop_max", &settings.coinPop.max},
		{"dorm_dorm_pop_rant", &settings.coinPop.rant},
		{"dorm_intimacy_pop_min", &settings.intimacyPop.min},
		{"dorm_intimacy_pop_max", &settings.intimacyPop.max},
		{"dorm_intimacy_pop_rant", &settings.intimacyPop.rant},
	} {
		if *field.value, err = get(field.key); err != nil {
			return err
		}
	}
	popTime, err := get("dorm_pop_time")
	if err != nil {
		return err
	}
	settings.popTime = int64(max(popTime, 1))
	for n := 1; ; n++ {
		exp, okExp := gameset[fmt.Sprintf("dorm_exp_ratio_by_%d", n)]
		food, okFood := gameset[fmt.Sprintf("dorm_food_ratio_by_%d", n)]
		if !okExp || !okFood {
			break
		}
		settings.expRatio = append(settings.expRatio, exp)
		settings.foodRatio = append(settings.foodRatio, food)
	}
	if len(settings.expRatio) == 0 {
		return gamedata.Missing("gameset", "dorm_exp_ratio_by_1")
	}
	dormSettings = settings
	return nil
}

// ratio is a dorm_*_ratio_by_<n> value, the last one past the table.
func ratio(values []uint32, n int) uint32 {
	return values[min(max(n, 1), len(values))-1]
}

// dormState is the commander's backyard row, created on first use with
// nothing settled before since.
func dormState(ctx context.Context, queries *db.Queries, commanderID uint32, since int64) (db.CommanderDorm, error) {
	if err := queries.EnsureDorm(ctx, db.EnsureDormParams{CommanderID: commanderID, LoadTime: since}); err != nil {
		return db.CommanderDorm{}, err
	}
	return queries.GetDorm(ctx, commanderID)
}

func (tx *Tx) dorm(now int64) (db.CommanderDorm, error) {
	return dormState(tx.ctx, tx.Queries, tx.commanderID, now)
}

// dormFoodMax is how much food the backyard holds (Dorm:GetCapcity).
func dormFoodMax(dorm db.CommanderDorm) uint32 {
	return dormLevels[dorm.Level].Capacity + dorm.FoodMaxIncrease
}

// typeForComfortable is Furniture:getTypeForComfortable: the comfort group
// of a furniture_data_template type (INDEX_TO_COMFORTABLE_TYPE).
func typeForComfortable(kind uint32) uint32 {
	groups := [...]uint32{1, 2, 3, 4, 5, 6, 7, 2, 2, 6, 2, 2, 2, 2, 2, 2}
	if kind >= 1 && int(kind) <= len(groups) {
		return groups[kind-1]
	}
	return 2 // TYPE_FURNITURE
}

// dormComfort is Dorm:getComfortable: for each comfort group of the
// level's comfortable_count, the best comfortable of the owned furniture
// (every piece counts, placed or not), plus the level's own and the second
// floor's.
func dormComfort(dorm db.CommanderDorm, owned []db.ListFurnitureRow) uint32 {
	groups := map[uint32][]uint32{}
	for _, row := range owned {
		config := furnitures[row.FurnitureID]
		group := typeForComfortable(config.Type)
		for range row.Count {
			groups[group] = append(groups[group], config.Comfortable)
		}
	}
	level := dormLevels[dorm.Level]
	comfort := level.Comfortable
	for _, count := range level.ComfortableCount {
		values := groups[count[0]]
		slices.SortFunc(values, func(a, b uint32) int { return cmp.Compare(b, a) })
		for _, value := range values[:min(int(count[1]), len(values))] {
			comfort += value
		}
	}
	if dorm.FloorNum >= 2 {
		comfort += dorm2FloorComfort
	}
	return comfort
}

// dormTickExp is the exp a training ship gets each dorm_data_template time
// (BackYardShipCard:GetBaseExp, view/backyard/ship/cards/): the ratio for
// the training ship count times dorm_exp_base plus the level's exp scaled
// by comfort, times 1 + 5% per commander level. Belfast has no backyard exp
// buffs.
func dormTickExp(dorm db.CommanderDorm, training int, comfort, commanderLevel uint32) float64 {
	level := dormLevels[dorm.Level]
	scaled := float64(level.Exp) * float64(comfort) / float64(comfort+dormSettings.comfortDegree)
	return float64(ratio(dormSettings.expRatio, training)) / 100 * (float64(dormSettings.expBase) + scaled) * (1 + 0.05*float64(commanderLevel))
}

// dormTickFood is the food the training ships eat each tick
// (Dorm:getFoodLeftTime): the level's consume times the ratio for their
// count.
func dormTickFood(dorm db.CommanderDorm, training int) uint32 {
	return ratio(dormSettings.foodRatio, training) * dormLevels[dorm.Level].Consume / 100
}

// dormTraining is what the training ships earn since the last settlement:
// the ticks elapsed (the tick count, those the food lasts) and the exp each
// ship gets, capped at dorm_exp_once_limit_base + _by_level per commander
// level.
type dormTraining struct {
	ticks, fed int64
	exp, food  uint32
}

func (tx *Tx) dormTraining(dorm db.CommanderDorm, commander db.Commander, training int, now int64) (dormTraining, error) {
	period := int64(dormLevels[dorm.Level].Time)
	var t dormTraining
	if now > dorm.LoadTime {
		t.ticks = (now - dorm.LoadTime) / period
	}
	if training == 0 || t.ticks == 0 {
		return t, nil
	}
	owned, err := tx.ListFurniture(tx.ctx, tx.commanderID)
	if err != nil {
		return t, err
	}
	perTick := dormTickFood(dorm, training)
	t.fed = t.ticks
	if perTick > 0 {
		t.fed = min(t.ticks, int64(commander.DormFood/perTick))
	}
	t.food = uint32(t.fed) * perTick
	exp := math.Floor(dormTickExp(dorm, training, dormComfort(dorm, owned), commander.Level) * float64(t.fed))
	t.exp = uint32(min(exp, float64(dormSettings.onceLimitBase+dormSettings.onceLimitByLevel*commander.Level)))
	return t, nil
}

// trainShip gives a training ship its exp as BackYardRequestShipExpCommand
// (and UseAddShipExpCommand for exp books) does: nothing at max level, Ship:addExp without accumulating (a ship
// reaching its max level drops the rest). It tells whether it reached
// the Fleet Technology level.
func trainShip(ship *db.Ship, exp uint32) bool {
	if exp == 0 || ship.Level >= ship.MaxLevel {
		return false
	}
	before := ship.Level
	addShipExp(ship, exp)
	if ship.Level >= ship.MaxLevel {
		ship.Exp = 0
	}
	return before < techShipLevel && ship.Level >= techShipLevel
}

func (tx *Tx) saveDormShip(ship db.Ship) error {
	if err := tx.UpdateShipBattleState(tx.ctx, db.UpdateShipBattleStateParams{
		Level: ship.Level, Exp: ship.Exp, Intimacy: ship.Intimacy, Energy: ship.Energy, ID: ship.ID, CommanderID: tx.commanderID,
	}); err != nil {
		return err
	}
	_, err := raiseShipGroup(tx.ctx, tx.Queries, ship)
	return err
}

// settleDorm settles the training since the last settlement: every
// training ship gets the same exp, the food eaten is taken, and the next
// settlement is a tick after the last one counted (SC_19027).
func (tx *Tx) settleDorm(now int64) (*protobuf.SC_19027, error) {
	dorm, err := tx.dorm(now)
	if err != nil {
		return nil, err
	}
	commander, err := tx.GetCommander(tx.ctx, tx.commanderID)
	if err != nil {
		return nil, err
	}
	ships, err := tx.ListDormShips(tx.ctx, tx.commanderID)
	if err != nil {
		return nil, err
	}
	var training []uint32
	for _, ship := range ships {
		if ship.Floor == dormFloorTrain {
			training = append(training, ship.ShipID)
		}
	}
	t, err := tx.dormTraining(dorm, commander, len(training), now)
	if err != nil {
		return nil, err
	}
	if t.exp > 0 {
		rows, err := tx.ListShipsByID(tx.ctx, db.ListShipsByIDParams{CommanderID: tx.commanderID, Ids: training})
		if err != nil {
			return nil, err
		}
		for _, ship := range rows {
			if trainShip(&ship, t.exp) {
				tx.techPointsChanged()
			}
			if err := tx.saveDormShip(ship); err != nil {
				return nil, err
			}
		}
	}
	if t.food > 0 {
		if err := tx.SetDormFood(tx.ctx, db.SetDormFoodParams{DormFood: commander.DormFood - t.food, ID: tx.commanderID}); err != nil {
			return nil, err
		}
	}
	period := int64(dormLevels[dorm.Level].Time)
	loaded := max(dorm.LoadTime, 0) + t.ticks*period
	if loaded > now || loaded == 0 {
		loaded = now
	}
	next := loaded + period
	if err := tx.SetDormSettlement(tx.ctx, db.SetDormSettlementParams{LoadTime: loaded, NextTimestamp: next, CommanderID: tx.commanderID}); err != nil {
		return nil, err
	}
	return &protobuf.SC_19027{Exp: proto.Uint32(t.exp), Food: proto.Uint32(t.food), NextTimestamp: proto.Uint32(uint32(next))}, nil
}

// floorLayouts are the furniture placed on each floor, as saved.
func floorLayouts(ctx context.Context, queries *db.Queries, commanderID uint32) (map[uint32]*protobuf.FURFLOORPUTINFO, error) {
	rows, err := queries.ListDormFloors(ctx, commanderID)
	if err != nil {
		return nil, err
	}
	layouts := make(map[uint32]*protobuf.FURFLOORPUTINFO, len(rows))
	for _, row := range rows {
		layout := &protobuf.FURFLOORPUTINFO{}
		if err := proto.Unmarshal(row.Layout, layout); err != nil {
			return nil, fmt.Errorf("dorm floor %d: %w", row.Floor, err)
		}
		layouts[row.Floor] = layout
	}
	return layouts, nil
}

// dorm is SC_19001, the backyard (GetBackYardDataCommand): its level,
// floors, slots, food, furniture, ships and saved layouts, the training
// settled up to load_time, next settlement next_timestamp (0: on entering).
func (game *Game) dorm(ctx context.Context, queries *db.Queries, commander db.Commander, ships []db.Ship) (*protobuf.SC_19001, error) {
	dorm, err := dormState(ctx, queries, commander.ID, commander.RegisteredAt)
	if err != nil {
		return nil, err
	}
	furniture, err := queries.ListFurniture(ctx, commander.ID)
	if err != nil {
		return nil, err
	}
	list := make([]*protobuf.FURNITUREINFO, len(furniture))
	for i, row := range furniture {
		list[i] = &protobuf.FURNITUREINFO{Id: proto.Uint32(row.FurnitureID), Count: proto.Uint32(row.Count)}
	}
	inDorm, err := queries.ListDormShips(ctx, commander.ID)
	if err != nil {
		return nil, err
	}
	byID := make(map[uint32]db.Ship, len(ships))
	for _, ship := range ships {
		byID[ship.ID] = ship
	}
	var shipList []*protobuf.SHIPINFO_IN_DORM
	for _, row := range inDorm {
		ship := byID[row.ShipID]
		shipList = append(shipList, &protobuf.SHIPINFO_IN_DORM{
			Id: proto.Uint32(row.ShipID), Floor: proto.Uint32(row.Floor),
			PopIcon: proto.Uint32(row.PopIcon), PopIntimacy: proto.Uint32(row.PopIntimacy),
			Tid: proto.Uint32(ship.TemplateID), SkinId: proto.Uint32(ship.SkinID),
		})
	}
	layouts, err := floorLayouts(ctx, queries, commander.ID)
	if err != nil {
		return nil, err
	}
	var putList []*protobuf.FURFLOORPUTINFO
	for _, floor := range slices.Sorted(maps.Keys(layouts)) {
		putList = append(putList, layouts[floor])
	}
	return &protobuf.SC_19001{
		Lv:                   proto.Uint32(dorm.Level),
		Food:                 proto.Uint32(commander.DormFood),
		FoodMaxIncrease:      proto.Uint32(dorm.FoodMaxIncrease),
		FoodMaxIncreaseCount: proto.Uint32(dorm.FoodMaxIncreaseCount),
		ShipList:             shipList,
		FurnitureIdList:      list,
		FloorNum:             proto.Uint32(dorm.FloorNum),
		ExpPos:               proto.Uint32(dorm.ExpPos),
		FurniturePutList:     putList,
		NextTimestamp:        proto.Uint32(uint32(dorm.NextTimestamp)),
		LoadTime:             proto.Uint32(uint32(dorm.LoadTime)),
		Name:                 proto.String(dorm.Name),
	}, nil
}

// handleBackyardVisitor answers GetBackYardVisitorCommand (CS_19024, type
// 0), sent on entering the backyard (CourtYardScene:SetUpCourtYard): the
// furniture each floor shows, which the client sets as the floors' themes,
// and a friend's ship visiting (FriendProxy, SHOW_FIREND_BACKYARD_SHIP_FLAG).
// A floor never arranged shows the starter arrangement on floor 1
// (furniture_init_data_template, which the client never reads itself and
// SC_19001 doesn't carry for a new commander). Belfast has no friends: no
// visitor.
func (game *Game) handleBackyardVisitor(client *connection.Client, req *protobuf.CS_19024) error {
	ctx := context.Background()
	layouts, err := floorLayouts(ctx, game.DB.Queries, client.CommanderID)
	if err != nil {
		return err
	}
	if _, ok := layouts[dormFloorTrain]; !ok {
		layouts[dormFloorTrain] = &protobuf.FURFLOORPUTINFO{Floor: proto.Uint32(dormFloorTrain), FurniturePutList: furnitureStart}
	}
	reply := &protobuf.SC_19025{}
	for _, floor := range slices.Sorted(maps.Keys(layouts)) {
		reply.FurniturePutList = append(reply.FurniturePutList, layouts[floor])
	}
	return client.Send(reply)
}

// handleDormShipExp answers BackYardRequestShipExpCommand (CS_19026, type
// 0), sent on entering the backyard and at each next_timestamp while in
// it: the exp each training ship got, the food eaten (the client adds and
// takes them itself) and when to ask again.
func (game *Game) handleDormShipExp(client *connection.Client, req *protobuf.CS_19026) error {
	var reply *protobuf.SC_19027
	err := game.tx(context.Background(), client, func(tx *Tx) (err error) {
		reply, err = tx.settleDorm(time.Now().Unix())
		return err
	})
	if err != nil {
		return err
	}
	return client.Send(reply)
}

// handleDormAddShip answers AddShipCommand (CS_19002): ship_id goes to
// train (type 1) or rest (type 2, once the second floor is bought), each
// floor holding exp_pos ships (Dorm rest_pos is exp_pos). A first trainee
// starts the settlement clock, so the time nobody trained isn't paid.
func (game *Game) handleDormAddShip(client *connection.Client, req *protobuf.CS_19002) error {
	err := game.tx(context.Background(), client, func(tx *Tx) error {
		now := time.Now().Unix()
		dorm, err := tx.dorm(now)
		if err != nil {
			return err
		}
		floor := req.GetType()
		if floor != dormFloorTrain && (floor != dormFloorRest || dorm.FloorNum < 2) {
			return errRefused
		}
		if _, err := tx.GetShip(tx.ctx, db.GetShipParams{ID: req.GetShipId(), CommanderID: tx.commanderID}); errors.Is(err, sql.ErrNoRows) {
			return errRefused
		} else if err != nil {
			return err
		}
		ships, err := tx.ListDormShips(tx.ctx, tx.commanderID)
		if err != nil {
			return err
		}
		onFloor := 0
		for _, ship := range ships {
			if ship.ShipID == req.GetShipId() {
				return errRefused
			}
			if ship.Floor == floor {
				onFloor++
			}
		}
		if onFloor >= int(dorm.ExpPos) {
			return errRefused
		}
		if floor == dormFloorTrain && onFloor == 0 {
			if err := tx.SetDormSettlement(tx.ctx, db.SetDormSettlementParams{LoadTime: now, NextTimestamp: dorm.NextTimestamp, CommanderID: tx.commanderID}); err != nil {
				return err
			}
		}
		return tx.AddDormShip(tx.ctx, db.AddDormShipParams{ShipID: req.GetShipId(), CommanderID: tx.commanderID, Floor: floor, PopAt: now + dormSettings.popTime})
	})
	if err != nil && !refused(err) {
		return err
	}
	return client.Send(&protobuf.SC_19003{Result: result(err)})
}

// handleDormShipExit answers ShipExitCommand (CS_19004): the ship leaves,
// with the exp it trained since the last settlement (the client adds it).
// The other trainees are settled later together, so the food it ate is
// not taken, and the coins and affection it hadn't popped are lost (the
// client drops them too).
func (game *Game) handleDormShipExit(client *connection.Client, req *protobuf.CS_19004) error {
	var exp uint32
	err := game.tx(context.Background(), client, func(tx *Tx) error {
		now := time.Now().Unix()
		dorm, err := tx.dorm(now)
		if err != nil {
			return err
		}
		ships, err := tx.ListDormShips(tx.ctx, tx.commanderID)
		if err != nil {
			return err
		}
		i := slices.IndexFunc(ships, func(ship db.DormShip) bool { return ship.ShipID == req.GetShipId() })
		if i < 0 {
			return errRefused
		}
		if ships[i].Floor == dormFloorTrain {
			training := 0
			for _, ship := range ships {
				if ship.Floor == dormFloorTrain {
					training++
				}
			}
			commander, err := tx.GetCommander(tx.ctx, tx.commanderID)
			if err != nil {
				return err
			}
			t, err := tx.dormTraining(dorm, commander, training, now)
			if err != nil {
				return err
			}
			ship, err := tx.GetShip(tx.ctx, db.GetShipParams{ID: req.GetShipId(), CommanderID: tx.commanderID})
			if err != nil {
				return err
			}
			if ship.Level < ship.MaxLevel {
				exp = t.exp
			}
			if trainShip(&ship, exp) {
				tx.techPointsChanged()
			}
			if err := tx.saveDormShip(ship); err != nil {
				return err
			}
		}
		_, err = tx.DeleteDormShip(tx.ctx, db.DeleteDormShipParams{ShipID: req.GetShipId(), CommanderID: tx.commanderID})
		return err
	})
	if err != nil && !refused(err) {
		return err
	}
	if err != nil {
		exp = 0
	}
	return client.Send(&protobuf.SC_19005{Result: result(err), Exp: proto.Uint32(exp)})
}

// rollPop is one dorm_pop_time roll of what a ship pops.
func rollPop(pop dormPop) uint32 {
	if pop.max == 0 || rand.Uint32N(1000) >= pop.rant {
		return 0
	}
	return pop.min + rand.Uint32N(pop.max-min(pop.min, pop.max)+1)
}

// handleDormPopEvents answers BackYardRequestPopEventCommand (CS_19009),
// sent at login and every dorm_pop_time: the coins and affection waiting
// above each ship (the totals, the client sets them). Every dorm_pop_time
// in the backyard a ship may pop dorm_dorm_pop_min..max coins and
// dorm_intimacy_pop_min..max affection, each with a dorm_*_pop_rant in a
// thousand chance; the gameset names them, the server-side rule is a guess.
func (game *Game) handleDormPopEvents(client *connection.Client, req *protobuf.CS_19009) error {
	reply := &protobuf.SC_19010{}
	err := game.tx(context.Background(), client, func(tx *Tx) error {
		now := time.Now().Unix()
		ships, err := tx.ListDormShips(tx.ctx, tx.commanderID)
		if err != nil {
			return err
		}
		for _, ship := range ships {
			if ship.PopAt <= now {
				for rolls := 0; ship.PopAt <= now && rolls < dormMaxPopRolls; rolls++ {
					ship.PopAt += dormSettings.popTime
					ship.PopIcon += rollPop(dormSettings.coinPop)
					ship.PopIntimacy += rollPop(dormSettings.intimacyPop)
				}
				if ship.PopAt <= now {
					ship.PopAt = now + dormSettings.popTime
				}
				if err := tx.SetDormShipPop(tx.ctx, db.SetDormShipPopParams{
					PopIcon: ship.PopIcon, PopIntimacy: ship.PopIntimacy, PopAt: ship.PopAt, ShipID: ship.ShipID, CommanderID: tx.commanderID,
				}); err != nil {
					return err
				}
			}
			if ship.PopIcon > 0 || ship.PopIntimacy > 0 {
				reply.PopList = append(reply.PopList, &protobuf.POP_INFO{
					Id: proto.Uint32(ship.ShipID), Intimacy: proto.Uint32(ship.PopIntimacy), DormIcon: proto.Uint32(ship.PopIcon),
				})
			}
		}
		return nil
	})
	if err != nil {
		return err
	}
	return client.Send(reply)
}

// harvestDorm collects what the ship (0: every ship) popped, as
// Dorm:HarvestInimacyAndMoney does whichever of the two was tapped: the
// affection to the ship (up to maxIntimacy, as battles give it), the
// coins to dormMoney.
func (tx *Tx) harvestDorm(shipID uint32) error {
	ships, err := tx.ListDormShips(tx.ctx, tx.commanderID)
	if err != nil {
		return err
	}
	found := false
	coins := uint32(0)
	for _, row := range ships {
		if shipID != 0 && row.ShipID != shipID {
			continue
		}
		found = true
		if row.PopIntimacy > 0 {
			ship, err := tx.GetShip(tx.ctx, db.GetShipParams{ID: row.ShipID, CommanderID: tx.commanderID})
			if err != nil {
				return err
			}
			ship.Intimacy = max(ship.Intimacy, min(ship.Intimacy+row.PopIntimacy, maxIntimacy))
			if err := tx.saveDormShip(ship); err != nil {
				return err
			}
		}
		coins += row.PopIcon
		if row.PopIcon > 0 || row.PopIntimacy > 0 {
			if err := tx.SetDormShipPop(tx.ctx, db.SetDormShipPopParams{PopAt: row.PopAt, ShipID: row.ShipID, CommanderID: tx.commanderID}); err != nil {
				return err
			}
		}
	}
	if !found && shipID != 0 {
		return errRefused
	}
	if coins > 0 {
		return tx.AddResource(ResourceDormMoney, coins)
	}
	return nil
}

// handleDormIntimacy answers ShipAddInimacyCommand and, with id 0,
// ShipAddIntimacyAndMoneyCommand (CS_19011).
func (game *Game) handleDormIntimacy(client *connection.Client, req *protobuf.CS_19011) error {
	err := game.tx(context.Background(), client, func(tx *Tx) error { return tx.harvestDorm(req.GetId()) })
	if err != nil && !refused(err) {
		return err
	}
	return client.Send(&protobuf.SC_19012{Result: result(err)})
}

// handleDormMoney answers ShipAddMoneyCommand (CS_19013).
func (game *Game) handleDormMoney(client *connection.Client, req *protobuf.CS_19013) error {
	if req.GetId() == 0 {
		return client.Send(&protobuf.SC_19014{Result: result(errRefused)})
	}
	err := game.tx(context.Background(), client, func(tx *Tx) error { return tx.harvestDorm(req.GetId()) })
	if err != nil && !refused(err) {
		return err
	}
	return client.Send(&protobuf.SC_19014{Result: result(err)})
}

// furnitureID is the furniture_data_template id of a placed piece: its id
// is the config id (PutFurnitureCommand) or a unique id, config id * 100 +
// n (BackyardThemeFurniture.GetUniqueId).
func furnitureID(id string) (uint32, bool) {
	n, err := strconv.ParseUint(id, 10, 32)
	if err != nil {
		return 0, false
	}
	if _, ok := furnitures[uint32(n)]; ok {
		return uint32(n), true
	}
	_, ok := furnitures[uint32(n/100)]
	return uint32(n / 100), ok
}

// ownsLayout is whether every piece of a layout is owned furniture.
func ownsLayout(owned []db.ListFurnitureRow, list []*protobuf.FURNITUREPUTINFO) bool {
	has := make(map[uint32]bool, len(owned))
	for _, row := range owned {
		has[row.FurnitureID] = row.Count > 0
	}
	for _, put := range list {
		ids := []string{put.GetId()}
		for _, child := range put.GetChild() {
			ids = append(ids, child.GetId())
		}
		for _, id := range ids {
			if config, ok := furnitureID(id); !ok || !has[config] {
				return false
			}
		}
	}
	return true
}

// handlePutFurniture saves a floor's arrangement (PutFurnitureCommand,
// CS_19008, no reply: the client already set it as the floor's theme).
func (game *Game) handlePutFurniture(client *connection.Client, req *protobuf.CS_19008) error {
	ctx := context.Background()
	return game.DB.InTx(ctx, func(queries *db.Queries) error {
		dorm, err := dormState(ctx, queries, client.CommanderID, time.Now().Unix())
		if err != nil {
			return err
		}
		owned, err := queries.ListFurniture(ctx, client.CommanderID)
		if err != nil {
			return err
		}
		if req.GetFloor() < 1 || req.GetFloor() > dorm.FloorNum || !ownsLayout(owned, req.GetFurniturePutList()) {
			logger.WithFields("Dorm", logger.FieldValue("floor", req.GetFloor())).Warn("furniture arrangement refused")
			return nil
		}
		layout, err := proto.Marshal(&protobuf.FURFLOORPUTINFO{Floor: proto.Uint32(req.GetFloor()), FurniturePutList: req.GetFurniturePutList()})
		if err != nil {
			return err
		}
		return queries.SetDormFloor(ctx, db.SetDormFloorParams{CommanderID: client.CommanderID, Floor: req.GetFloor(), Layout: layout})
	})
}

// handleDormRename answers BackYardRenameCommand (CS_19016); the client
// filters the name itself (CourtYardRenamePage).
func (game *Game) handleDormRename(client *connection.Client, req *protobuf.CS_19016) error {
	var err error
	if req.GetName() == "" {
		err = errRefused
	} else {
		err = game.tx(context.Background(), client, func(tx *Tx) error {
			if _, err := tx.dorm(time.Now().Unix()); err != nil {
				return err
			}
			return tx.SetDormName(tx.ctx, db.SetDormNameParams{Name: req.GetName(), CommanderID: tx.commanderID})
		})
	}
	if err != nil && !refused(err) {
		return err
	}
	return client.Send(&protobuf.SC_19017{Result: result(err)})
}

// price is Furniture:getPrice in a currency (4 gems, 6 dormMoney): the
// row's price less its running discount, at least 1; false when it isn't
// sold for it or isn't on sale now (Furniture:inTime).
func (goods furnitureGoods) price(currency uint32, now time.Time) (uint32, bool) {
	var base uint32
	switch currency {
	case ResourceGem:
		base = goods.GemPrice
	case ResourceDormMoney:
		base = goods.DormIconPrice
	}
	if base == 0 || !goods.Time.Open(now) {
		return 0, false
	}
	var collaboration string
	if json.Unmarshal(goods.CollaborationTime, &collaboration) != nil || collaboration != "" {
		if string(goods.CollaborationTime) != string(goods.RawTime) {
			return 0, false
		}
	}
	discount := uint32(0)
	if goods.Discount > 0 && goods.Discount <= 100 && goods.DiscountTime.Open(now) {
		discount = goods.Discount
	}
	return max(base*(100-discount)/100, 1), true
}

// handleBuyFurniture answers BuyFurnitureCommand (CS_19006): one of each
// furniture_id, paid in currency; the client takes the price and adds them.
// Decor tokens spent count for the Renovation Merit Medal.
func (game *Game) handleBuyFurniture(client *connection.Client, req *protobuf.CS_19006) error {
	err := game.tx(context.Background(), client, func(tx *Tx) error {
		now := time.Now()
		currency := req.GetCurrency()
		if len(req.GetFurnitureId()) == 0 {
			return errRefused
		}
		total := uint32(0)
		for _, id := range req.GetFurnitureId() {
			goods, ok := furnitureShop[id]
			if !ok {
				return errRefused
			}
			price, ok := goods.price(currency, now)
			if !ok {
				return errRefused
			}
			total += price
		}
		if err := tx.SpendShopResource(currency, total); err != nil {
			return err
		}
		for _, id := range req.GetFurnitureId() {
			if err := tx.AddFurniture(tx.ctx, db.AddFurnitureParams{CommanderID: tx.commanderID, FurnitureID: id, Count: 1}); err != nil {
				return err
			}
		}
		if currency != ResourceDormMoney {
			return nil
		}
		return tx.addTrophyProgress(furnitureTokensTrophy, total)
	})
	if err != nil && !refused(err) {
		return err
	}
	return client.Send(&protobuf.SC_19007{Result: result(err)})
}

// addTrophyProgress adds to a medal_template progress kept in
// commander_trophies, pushing it (SC_17002).
func (tx *Tx) addTrophyProgress(trophyID, amount uint32) error {
	trophies, err := tx.ListTrophies(tx.ctx, tx.commanderID)
	if err != nil {
		return err
	}
	progress := amount
	for _, trophy := range trophies {
		if trophy.TrophyID == trophyID {
			progress += trophy.Progress
		}
	}
	if err := tx.SetTrophy(tx.ctx, db.SetTrophyParams{CommanderID: tx.commanderID, TrophyID: trophyID, Progress: progress}); err != nil {
		return err
	}
	push, err := trophyProgress(tx.ctx, tx.Queries, tx.commanderID, trophyID)
	if err != nil {
		return err
	}
	tx.push(push)
	return nil
}

// extendDorm applies the backyard's shop_template type 0 extensions
// (ExtendCommand): a training and resting slot (dorm_exp_pos, limit_args
// "number" the slots before), food capacity (dorm_food_max, num, "count"
// the extensions before) and the second floor (dorm_floor, "count" the
// floors before). It tells whether effect is one of them.
func (tx *Tx) extendDorm(goods shopGoods, amount uint32) (bool, error) {
	dorm, err := tx.dorm(time.Now().Unix())
	if err != nil {
		return true, err
	}
	switch goods.effect {
	case effectDormExpPos:
		if !goods.countAllows(dorm.ExpPos) || dorm.ExpPos >= dormLevels[dorm.Level].TrainingShips {
			return true, errRefused
		}
		return true, tx.SetDormExpPos(tx.ctx, db.SetDormExpPosParams{ExpPos: dorm.ExpPos + 1, CommanderID: tx.commanderID})
	case effectDormFoodMax:
		if !goods.countAllows(dorm.FoodMaxIncreaseCount) {
			return true, errRefused
		}
		return true, tx.AddDormFoodMax(tx.ctx, db.AddDormFoodMaxParams{FoodMaxIncrease: amount, CommanderID: tx.commanderID})
	case effectDormFloor:
		if !goods.countAllows(dorm.FloorNum) || dorm.FloorNum >= dormMaxFloor {
			return true, errRefused
		}
		return true, tx.SetDormFloorNum(tx.ctx, db.SetDormFloorNumParams{FloorNum: dorm.FloorNum + 1, CommanderID: tx.commanderID})
	}
	return false, nil
}

// expandDorm is the Expansion License (usage_dorm_lv_up): the backyard's
// next level, a bigger room (ExtendBackYardAreaCommand).
func (tx *Tx) expandDorm() error {
	dorm, err := tx.dorm(time.Now().Unix())
	if err != nil {
		return err
	}
	if dorm.Level >= dormMaxLevel {
		return errRefused
	}
	return tx.SetDormLevel(tx.ctx, db.SetDormLevelParams{Level: dorm.Level + 1, CommanderID: tx.commanderID})
}

// addDormFood fills the backyard with food, what doesn't fit is lost
// (AddFoodCommand).
func (tx *Tx) addDormFood(food uint32) error {
	dorm, err := tx.dorm(time.Now().Unix())
	if err != nil {
		return err
	}
	commander, err := tx.GetCommander(tx.ctx, tx.commanderID)
	if err != nil {
		return err
	}
	return tx.SetDormFood(tx.ctx, db.SetDormFoodParams{DormFood: uint32(min(uint64(commander.DormFood)+uint64(food), uint64(dormFoodMax(dorm)))), ID: tx.commanderID})
}
