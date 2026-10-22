package game

// The back-office's backyard edits beyond the dorm view of admin_social.go:
// the room's name, floors, slots and food capacity, the ships training or
// resting (added, moved, what they popped, collected), the furniture owned
// (taken off the floors when it is removed) and the theme templates. Every
// edit runs through editDorm: the commander kicked, the training settled up
// to now, one transaction.

import (
	"context"
	"database/sql"
	"errors"
	"maps"
	"slices"
	"strconv"
	"time"

	"google.golang.org/protobuf/proto"

	"github.com/ggmolly/belfast/internal/db"
	"github.com/ggmolly/belfast/internal/protobuf"
)

// dormNameMax is CourtYardRenamePage's nameValidityCheck(name, 0, 20).
const dormNameMax = 20

// BackyardShip is a ship in the backyard with what the back-office edits of
// it: its exp (the bound its level allows) and what the training would give
// it if settled now.
type BackyardShip struct {
	DormShipView
	Exp        uint32 `json:"exp"`
	ExpMax     uint32 `json:"exp_max"`
	MaxLevel   uint32 `json:"max_level"`
	Intimacy   uint32 `json:"intimacy"`
	PendingExp uint32 `json:"pending_exp"`
}

// BackyardFurniture is a furniture owned, how many (at most its
// furniture_data_template count) and how many stand on the floors.
type BackyardFurniture struct {
	ID          uint32 `json:"id"`
	Count       uint32 `json:"count"`
	Max         uint32 `json:"max"`
	Placed      uint32 `json:"placed"`
	Comfortable uint32 `json:"comfortable"`
}

// BackyardCandidate is a ship that may go to the backyard, Busy saying why
// the client's selection hides it (TAG_HIDE_BACKYARD, TAG_BLOCK_BACKYARD).
type BackyardCandidate struct {
	AdminShip
	Busy string `json:"busy,omitempty"`
}

// BackyardFurnitureEntry is a furniture_data_template row the back-office
// may grant.
type BackyardFurnitureEntry struct {
	ID          uint32 `json:"id"`
	Max         uint32 `json:"max"`
	Comfortable uint32 `json:"comfortable"`
}

// BackyardAdmin is the backyard as the back-office edits it.
type BackyardAdmin struct {
	DormView
	Ships      []BackyardShip      `json:"ships"`
	Furniture  []BackyardFurniture `json:"furniture_list"`
	Candidates []BackyardCandidate `json:"candidates"`
	// the bounds of the edits
	NameMax           int      `json:"name_max"`
	MinSlots          uint32   `json:"min_slots"`
	FloorLevel        uint32   `json:"floor_level"`
	FoodExtensionsMax uint32   `json:"food_extensions_max"`
	FoodCapacities    []uint32 `json:"food_capacities"`
	PopCoinsMax       uint32   `json:"pop_coins_max"`
	PopIntimacyMax    uint32   `json:"pop_intimacy_max"`
	SharedMax         uint32   `json:"shared_max"`
	// the training: each dorm_data_template time, the exp each trainee gets
	// and the food eaten; what settling now would give; when the food runs
	// out (0: nobody trains)
	TickSeconds uint32  `json:"tick_seconds"`
	TickExp     float64 `json:"tick_exp"`
	TickFood    uint32  `json:"tick_food"`
	ExpCap      uint32  `json:"exp_cap"`
	PendingExp  uint32  `json:"pending_exp"`
	PendingFood uint32  `json:"pending_food"`
	FoodUntil   int64   `json:"food_until"`
	LoadTime    int64   `json:"load_time"`
	DormMoney   uint32  `json:"dorm_money"`
}

// furnitureMax is how many of a furniture one may own
// (furniture_data_template count, Furniture:canPurchase), 0 for no such
// furniture.
func furnitureMax(id uint32) uint32 {
	config, ok := furnitures[id]
	if !ok {
		return 0
	}
	return max(config.Count, 1)
}

// BackyardFurnitureCatalog is every furniture the back-office may grant.
func BackyardFurnitureCatalog() []BackyardFurnitureEntry {
	list := make([]BackyardFurnitureEntry, 0, len(furnitures))
	for id, config := range furnitures {
		if top := furnitureMax(id); top > 0 {
			list = append(list, BackyardFurnitureEntry{ID: id, Max: top, Comfortable: config.Comfortable})
		}
	}
	slices.SortFunc(list, func(a, b BackyardFurnitureEntry) int { return int(a.ID) - int(b.ID) })
	return list
}

// dormFoodGoods are the shop_template dorm_food_max extensions
// (BackYardFeedExtendPage), each the range of extensions bought before it
// applies to.
func dormFoodGoods() []shopGoods {
	var list []shopGoods
	for _, goods := range shopTemplate {
		if goods.effect == effectDormFoodMax && goods.hasCount {
			list = append(list, goods)
		}
	}
	return list
}

// dormFoodCapacities is the food added by 0, 1, ... extensions, up to the
// last one the shop sells (ExtendCommand: num per extension, the goods
// whose count range holds the extensions bought before).
func dormFoodCapacities() []uint32 {
	goods := dormFoodGoods()
	capacities := []uint32{0}
	for n := uint32(0); ; n++ {
		i := slices.IndexFunc(goods, func(g shopGoods) bool { return g.countAllows(n) })
		if i < 0 {
			return capacities
		}
		capacities = append(capacities, capacities[n]+uint32(goods[i].Num))
	}
}

// dormMinSlots is the fewest slots the backyard may have: the starting two
// (shop_template dorm_exp_pos from number 2), and room for the ships on
// each floor.
func dormMinSlots(ships []db.DormShip) uint32 {
	floors := map[uint32]uint32{}
	least := uint32(2)
	for _, ship := range ships {
		floors[ship.Floor]++
		least = max(least, floors[ship.Floor])
	}
	return least
}

// shipExpMax is the most exp a ship may hold at its level (editShip's
// check): a level's worth less one, the overflow at the max level for the
// ships that keep it, else none.
func shipExpMax(ship db.Ship) uint32 {
	curve := shipLevelCurve(ship.TemplateID)
	switch {
	case ship.Level < ship.MaxLevel && int(ship.Level) <= len(curve):
		return curve[ship.Level-1] - 1
	case shipLevelings[ship.TemplateID].accumulate:
		return expOverflowMax
	}
	return 0
}

// BackyardAdmin is the commander's backyard with its bounds and the
// training as of now.
func (game *Game) BackyardAdmin(ctx context.Context, id uint32) (BackyardAdmin, error) {
	queries := game.DB.Queries
	view, err := game.Dorm(ctx, id)
	if err != nil {
		return BackyardAdmin{}, err
	}
	admin := BackyardAdmin{
		DormView: view, Ships: []BackyardShip{}, Furniture: []BackyardFurniture{}, Candidates: []BackyardCandidate{},
		NameMax: dormNameMax, FloorLevel: dormMaxLevel, FoodCapacities: dormFoodCapacities(),
		PopCoinsMax: dormMaxPopRolls * dormSettings.coinPop.max, PopIntimacyMax: dormMaxPopRolls * dormSettings.intimacyPop.max,
		SharedMax: maxSharedThemes,
	}
	admin.FoodExtensionsMax = uint32(len(admin.FoodCapacities) - 1)
	commander, err := queries.GetCommander(ctx, id)
	if err != nil {
		return admin, err
	}
	dorm, err := queries.GetDorm(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		dorm, err = db.CommanderDorm{CommanderID: id, Level: 1, FloorNum: 1, ExpPos: 2, LoadTime: time.Now().Unix()}, nil
	}
	if err != nil {
		return admin, err
	}
	admin.LoadTime = dorm.LoadTime
	inDorm, err := queries.ListDormShips(ctx, id)
	if err != nil {
		return admin, err
	}
	admin.MinSlots = dormMinSlots(inDorm)
	training := 0
	for _, ship := range inDorm {
		if ship.Floor == dormFloorTrain {
			training++
		}
	}
	owned, err := queries.ListFurniture(ctx, id)
	if err != nil {
		return admin, err
	}
	level := dormLevels[dorm.Level]
	admin.TickSeconds = level.Time
	admin.TickExp = dormTickExp(dorm, max(training, 1), dormComfort(dorm, owned), commander.Level)
	admin.TickFood = dormTickFood(dorm, max(training, 1))
	admin.ExpCap = dormSettings.onceLimitBase + dormSettings.onceLimitByLevel*commander.Level
	t, err := newTx(ctx, queries, id).dormTraining(dorm, commander, training, time.Now().Unix())
	if err != nil {
		return admin, err
	}
	admin.PendingExp, admin.PendingFood = t.exp, t.food
	if training > 0 && admin.TickFood > 0 {
		admin.FoodUntil = dorm.LoadTime + int64(commander.DormFood/admin.TickFood)*int64(level.Time)
	}

	ids := make([]uint32, 0, len(inDorm))
	for _, row := range inDorm {
		ids = append(ids, row.ShipID)
	}
	ships, err := shipsByID(ctx, queries, id, ids)
	if err != nil {
		return admin, err
	}
	for _, row := range view.Ships {
		ship := ships[row.ID]
		entry := BackyardShip{DormShipView: row, Exp: ship.Exp, ExpMax: shipExpMax(ship), MaxLevel: ship.MaxLevel, Intimacy: ship.Intimacy}
		if row.Floor == dormFloorTrain && ship.Level < ship.MaxLevel {
			entry.PendingExp = t.exp
		}
		admin.Ships = append(admin.Ships, entry)
	}

	layouts, err := shownLayouts(ctx, queries, id)
	if err != nil {
		return admin, err
	}
	placed := placedFurniture(layouts)
	for _, row := range owned {
		admin.Furniture = append(admin.Furniture, BackyardFurniture{
			ID: row.FurnitureID, Count: row.Count, Max: max(furnitureMax(row.FurnitureID), row.Count),
			Placed: placed[row.FurnitureID], Comfortable: furnitures[row.FurnitureID].Comfortable,
		})
	}

	busy, err := backyardBusyShips(ctx, queries, id)
	if err != nil {
		return admin, err
	}
	all, err := queries.ListShipsByCommander(ctx, id)
	if err != nil {
		return admin, err
	}
	for _, ship := range all {
		if _, in := ships[ship.ID]; in {
			continue
		}
		admin.Candidates = append(admin.Candidates, BackyardCandidate{
			AdminShip: AdminShip{ID: ship.ID, TemplateID: ship.TemplateID, SkinID: ship.SkinID, Name: ship.Name, Level: ship.Level},
			Busy:      busy[ship.ID],
		})
	}

	resources, err := queries.ListResources(ctx, id)
	if err != nil {
		return admin, err
	}
	for _, row := range resources {
		if row.ResourceID == ResourceDormMoney {
			admin.DormMoney = row.Amount
		}
	}
	return admin, nil
}

// backyardBusyShips are the ships the client's backyard selection hides or
// blocks (NewBackYardShipInfoMediator: TAG_HIDE_BACKYARD inEvent, inTactics,
// inChapter, inPvP; TAG_BLOCK_BACKYARD inClass), with why.
func backyardBusyShips(ctx context.Context, queries *db.Queries, id uint32) (map[uint32]string, error) {
	busy := map[uint32]string{}
	commissions, err := queries.ListCommissionShips(ctx, id)
	if err != nil {
		return nil, err
	}
	for _, row := range commissions {
		busy[row.ShipID] = "Commission"
	}
	classes, err := queries.ListSkillClasses(ctx, id)
	if err != nil {
		return nil, err
	}
	for _, row := range classes {
		busy[row.ShipID] = "Tactical class"
	}
	defense, err := fleetMembers(ctx, queries, id, fleetExercise)
	if err != nil {
		return nil, err
	}
	for _, ship := range defense {
		busy[ship] = "Defense fleet"
	}
	inChapter, err := shipsInChapter(ctx, queries, id)
	if err != nil {
		return nil, err
	}
	for ship := range inChapter {
		busy[ship] = "In a chapter"
	}
	return busy, nil
}

// shownLayouts are the floors as the client shows them: floor 1 never
// arranged has the starter arrangement (handleBackyardVisitor).
func shownLayouts(ctx context.Context, queries *db.Queries, id uint32) (map[uint32]*protobuf.FURFLOORPUTINFO, error) {
	layouts, err := floorLayouts(ctx, queries, id)
	if err != nil {
		return nil, err
	}
	if _, ok := layouts[dormFloorTrain]; !ok {
		starter := make([]*protobuf.FURNITUREPUTINFO, len(furnitureStart))
		for i, put := range furnitureStart {
			starter[i] = proto.Clone(put).(*protobuf.FURNITUREPUTINFO)
		}
		layouts[dormFloorTrain] = &protobuf.FURFLOORPUTINFO{Floor: proto.Uint32(dormFloorTrain), FurniturePutList: starter}
	}
	return layouts, nil
}

// placedFurniture is how many of each furniture stand on the floors (the
// floors share the furniture owned, BackYardDecorationMediator
// GetAllFloorFurnitures).
func placedFurniture(layouts map[uint32]*protobuf.FURFLOORPUTINFO) map[uint32]uint32 {
	placed := map[uint32]uint32{}
	for _, layout := range layouts {
		for _, put := range layout.GetFurniturePutList() {
			if config, ok := furnitureID(put.GetId()); ok {
				placed[config]++
			}
		}
	}
	return placed
}

// SetDormName renames the backyard as BackYardRenameCommand does, empty for
// the client's default name.
func (game *Game) SetDormName(ctx context.Context, id uint32, name string) (bool, error) {
	if name != "" && !validNameUpTo(name, dormNameMax) {
		return false, adminRefused("the name takes 1 to %d characters, no spaces", dormNameMax)
	}
	return game.editDorm(ctx, id, func(tx *Tx, _ db.CommanderDorm) error {
		return tx.SetDormName(tx.ctx, db.SetDormNameParams{Name: name, CommanderID: id})
	})
}

// SetDormFloors opens the second floor (shop_template dorm_floor), which
// the client offers once the room is at Dorm.MAX_LEVEL
// (CourtYardTopPanel:UnLockTip). A floor is never closed: its furniture
// and its ships would be lost.
func (game *Game) SetDormFloors(ctx context.Context, id, floors uint32) (bool, error) {
	return game.editDorm(ctx, id, func(tx *Tx, dorm db.CommanderDorm) error {
		switch {
		case floors < dorm.FloorNum:
			return AdminError{Msg: "a floor can't be closed"}
		case floors > dormMaxFloor:
			return adminRefused("the backyard has at most %d floors", dormMaxFloor)
		case floors > 1 && dorm.Level < dormMaxLevel:
			return adminRefused("the second floor opens at level %d", dormMaxLevel)
		}
		return tx.SetDormFloorNum(tx.ctx, db.SetDormFloorNumParams{FloorNum: floors, CommanderID: id})
	})
}

// SetDormSlots sets how many ships each floor holds (exp_pos, rest_pos
// alike, ExtendCommand dorm_exp_pos): from 2 up to the level's
// training_ship_number, never fewer than the ships on a floor.
func (game *Game) SetDormSlots(ctx context.Context, id, slots uint32) (bool, error) {
	return game.editDorm(ctx, id, func(tx *Tx, dorm db.CommanderDorm) error {
		ships, err := tx.ListDormShips(tx.ctx, id)
		if err != nil {
			return err
		}
		least, most := dormMinSlots(ships), dormLevels[dorm.Level].TrainingShips
		if slots < least || slots > most {
			return adminRefused("the slots go from %d to %d", least, most)
		}
		return tx.SetDormExpPos(tx.ctx, db.SetDormExpPosParams{ExpPos: slots, CommanderID: id})
	})
}

// SetDormFoodExtensions sets how many food capacity extensions were bought,
// the capacity they add as the shop's dorm_food_max goods give it; food
// past the new capacity is lost.
func (game *Game) SetDormFoodExtensions(ctx context.Context, id, count uint32) (bool, error) {
	capacities := dormFoodCapacities()
	if int(count) >= len(capacities) {
		return false, adminRefused("the shop sells %d extensions", len(capacities)-1)
	}
	return game.editDorm(ctx, id, func(tx *Tx, dorm db.CommanderDorm) error {
		dorm.FoodMaxIncrease, dorm.FoodMaxIncreaseCount = capacities[count], count
		if err := tx.AdminBackyardSetFoodMax(tx.ctx, db.AdminBackyardSetFoodMaxParams{FoodMaxIncrease: dorm.FoodMaxIncrease, FoodMaxIncreaseCount: count, CommanderID: id}); err != nil {
			return err
		}
		commander, err := tx.GetCommander(tx.ctx, id)
		if err != nil {
			return err
		}
		if limit := dormFoodMax(dorm); commander.DormFood > limit {
			return tx.SetDormFood(tx.ctx, db.SetDormFoodParams{DormFood: limit, ID: id})
		}
		return nil
	})
}

// SettleDorm gives the training ships the exp of the ticks elapsed and takes
// the food they ate, as BackYardRequestShipExpCommand does.
func (game *Game) SettleDorm(ctx context.Context, id uint32) (bool, error) {
	return game.editDorm(ctx, id, func(*Tx, db.CommanderDorm) error { return nil })
}

// dormFloorRoom refuses a ship on a floor that isn't open or is full.
func dormFloorRoom(dorm db.CommanderDorm, ships []db.DormShip, floor uint32) error {
	if floor != dormFloorTrain && floor != dormFloorRest {
		return AdminError{Msg: "floor 1 trains, floor 2 rests"}
	}
	if floor > dorm.FloorNum {
		return AdminError{Msg: "the second floor isn't open"}
	}
	on := uint32(0)
	for _, ship := range ships {
		if ship.Floor == floor {
			on++
		}
	}
	if on >= dorm.ExpPos {
		return adminRefused("floor %d holds %d ships", floor, dorm.ExpPos)
	}
	return nil
}

// startTraining starts the settlement clock when the first trainee comes,
// so the time nobody trained isn't paid (AddShipCommand).
func (tx *Tx) startTraining(dorm db.CommanderDorm, ships []db.DormShip, now int64) error {
	if slices.ContainsFunc(ships, func(ship db.DormShip) bool { return ship.Floor == dormFloorTrain }) {
		return nil
	}
	return tx.SetDormSettlement(tx.ctx, db.SetDormSettlementParams{LoadTime: now, NextTimestamp: dorm.NextTimestamp, CommanderID: tx.commanderID})
}

// AddDormShip puts a ship to train (floor 1) or rest (floor 2) as
// AddShipCommand does, unless the client's selection hides or blocks it.
func (game *Game) AddDormShip(ctx context.Context, id, shipID, floor uint32) (bool, error) {
	return game.editDorm(ctx, id, func(tx *Tx, dorm db.CommanderDorm) error {
		if _, err := tx.GetShip(tx.ctx, db.GetShipParams{ID: shipID, CommanderID: id}); errors.Is(err, sql.ErrNoRows) {
			return adminRefused("no ship %d", shipID)
		} else if err != nil {
			return err
		}
		ships, err := tx.ListDormShips(tx.ctx, id)
		if err != nil {
			return err
		}
		if slices.ContainsFunc(ships, func(ship db.DormShip) bool { return ship.ShipID == shipID }) {
			return AdminError{Msg: "the ship is already in the backyard"}
		}
		busy, err := backyardBusyShips(tx.ctx, tx.Queries, id)
		if err != nil {
			return err
		}
		if why := busy[shipID]; why != "" {
			return AdminError{Msg: "the ship is busy: " + why}
		}
		if err := dormFloorRoom(dorm, ships, floor); err != nil {
			return err
		}
		now := time.Now().Unix()
		if floor == dormFloorTrain {
			if err := tx.startTraining(dorm, ships, now); err != nil {
				return err
			}
		}
		return tx.AddDormShip(tx.ctx, db.AddDormShipParams{ShipID: shipID, CommanderID: id, Floor: floor, PopAt: now + dormSettings.popTime})
	})
}

// MoveDormShip sends a ship to the other floor with what it popped, as the
// client's exit then add would without losing it.
func (game *Game) MoveDormShip(ctx context.Context, id, shipID, floor uint32) (bool, error) {
	return game.editDorm(ctx, id, func(tx *Tx, dorm db.CommanderDorm) error {
		ships, err := tx.ListDormShips(tx.ctx, id)
		if err != nil {
			return err
		}
		i := slices.IndexFunc(ships, func(ship db.DormShip) bool { return ship.ShipID == shipID })
		if i < 0 {
			return adminRefused("ship %d isn't in the backyard", shipID)
		}
		if ships[i].Floor == floor {
			return nil
		}
		if err := dormFloorRoom(dorm, ships, floor); err != nil {
			return err
		}
		if floor == dormFloorTrain {
			if err := tx.startTraining(dorm, ships, time.Now().Unix()); err != nil {
				return err
			}
		}
		_, err = tx.AdminBackyardSetShipFloor(tx.ctx, db.AdminBackyardSetShipFloorParams{Floor: floor, ShipID: shipID, CommanderID: id})
		return err
	})
}

// SetDormShipPop sets the coins and affection waiting above a ship, at most
// a day of dorm_pop_time rolls (dormMaxPopRolls) of each.
func (game *Game) SetDormShipPop(ctx context.Context, id, shipID, coins, intimacy uint32) (bool, error) {
	coinsMax, intimacyMax := dormMaxPopRolls*dormSettings.coinPop.max, dormMaxPopRolls*dormSettings.intimacyPop.max
	if coins > coinsMax || intimacy > intimacyMax {
		return false, adminRefused("a ship pops at most %d coins and %d affection", coinsMax, intimacyMax)
	}
	return game.editDorm(ctx, id, func(tx *Tx, _ db.CommanderDorm) error {
		ships, err := tx.ListDormShips(tx.ctx, id)
		if err != nil {
			return err
		}
		i := slices.IndexFunc(ships, func(ship db.DormShip) bool { return ship.ShipID == shipID })
		if i < 0 {
			return adminRefused("ship %d isn't in the backyard", shipID)
		}
		return tx.Queries.SetDormShipPop(tx.ctx, db.SetDormShipPopParams{PopIcon: coins, PopIntimacy: intimacy, PopAt: ships[i].PopAt, ShipID: shipID, CommanderID: id})
	})
}

// CollectDorm collects what a ship (0: every ship) popped, as
// ShipAddIntimacyAndMoneyCommand does: the coins to dormMoney, the
// affection to the ship.
func (game *Game) CollectDorm(ctx context.Context, id, shipID uint32) (bool, error) {
	return game.editDorm(ctx, id, func(tx *Tx, _ db.CommanderDorm) error {
		if err := tx.harvestDorm(shipID); errors.Is(err, errRefused) {
			return adminRefused("ship %d isn't in the backyard", shipID)
		} else if err != nil {
			return err
		}
		return nil
	})
}

// SetDormFurniture sets how many of a furniture the commander owns, at most
// its furniture_data_template count. The pieces past it are taken off the
// floors, with what stood on them.
func (game *Game) SetDormFurniture(ctx context.Context, id, piece, count uint32) (bool, error) {
	top := furnitureMax(piece)
	if top == 0 {
		return false, adminRefused("no furniture %d", piece)
	}
	if count > top {
		return false, adminRefused("one owns at most %d of it", top)
	}
	return game.editDorm(ctx, id, func(tx *Tx, _ db.CommanderDorm) error {
		if count == 0 {
			if err := tx.AdminBackyardDeleteFurniture(tx.ctx, db.AdminBackyardDeleteFurnitureParams{CommanderID: id, FurnitureID: piece}); err != nil {
				return err
			}
		} else if err := tx.AdminBackyardSetFurniture(tx.ctx, db.AdminBackyardSetFurnitureParams{CommanderID: id, FurnitureID: piece, Count: count}); err != nil {
			return err
		}
		return tx.unplaceFurniture(piece, count)
	})
}

// unplaceFurniture takes pieces of a furniture off the floors, the last
// placed first, until no more than count stand.
func (tx *Tx) unplaceFurniture(piece, count uint32) error {
	layouts, err := shownLayouts(tx.ctx, tx.Queries, tx.commanderID)
	if err != nil {
		return err
	}
	extra := int(placedFurniture(layouts)[piece]) - int(count)
	if extra <= 0 {
		return nil
	}
	floors := slices.Sorted(maps.Keys(layouts))
	slices.Reverse(floors)
	for _, floor := range floors {
		if extra <= 0 {
			break
		}
		layout := layouts[floor]
		list := layout.GetFurniturePutList()
		removed := map[string]bool{}
		for i := len(list) - 1; i >= 0 && extra > 0; i-- {
			if config, ok := furnitureID(list[i].GetId()); ok && config == piece {
				removed[list[i].GetId()] = true
				extra--
			}
		}
		if len(removed) == 0 {
			continue
		}
		layout.FurniturePutList = withoutPieces(list, removed)
		raw, err := proto.Marshal(layout)
		if err != nil {
			return err
		}
		if err := tx.SetDormFloor(tx.ctx, db.SetDormFloorParams{CommanderID: tx.commanderID, Floor: floor, Layout: raw}); err != nil {
			return err
		}
	}
	return nil
}

// withoutPieces is a floor's pieces less those removed, the pieces standing
// on them (parent) and the children lists naming them.
func withoutPieces(list []*protobuf.FURNITUREPUTINFO, removed map[string]bool) []*protobuf.FURNITUREPUTINFO {
	for grew := true; grew; {
		grew = false
		for _, put := range list {
			if removed[put.GetId()] || put.GetParent() == 0 {
				continue
			}
			if removed[strconv.FormatUint(put.GetParent(), 10)] {
				removed[put.GetId()] = true
				grew = true
			}
		}
	}
	kept := make([]*protobuf.FURNITUREPUTINFO, 0, len(list))
	for _, put := range list {
		if removed[put.GetId()] {
			continue
		}
		put.Child = slices.DeleteFunc(put.Child, func(child *protobuf.CHILDINFO) bool { return removed[child.GetId()] })
		kept = append(kept, put)
	}
	return kept
}

// ResetDormFloor puts floor 1 back to the starter arrangement
// (furniture_init_data_template), what a floor never arranged shows.
func (game *Game) ResetDormFloor(ctx context.Context, id, floor uint32) (bool, error) {
	if floor != dormFloorTrain {
		return false, AdminError{Msg: "only floor 1 has a starter arrangement"}
	}
	return game.editDorm(ctx, id, func(tx *Tx, _ db.CommanderDorm) error {
		return tx.AdminBackyardDeleteFloor(tx.ctx, db.AdminBackyardDeleteFloorParams{CommanderID: id, Floor: floor})
	})
}

// ShareDormTheme puts a theme template in the theme shop
// (BackYardUploadThemeTemplateCommand, at most MAX_UPLOAD_THEME_CNT) or
// takes it out with its likes and collections
// (BackYardUnloadThemeTemplateCommand).
func (game *Game) ShareDormTheme(ctx context.Context, id, pos uint32, shared bool) (bool, error) {
	return game.editDorm(ctx, id, func(tx *Tx, _ db.CommanderDorm) error {
		theme, err := tx.GetDormTheme(tx.ctx, db.GetDormThemeParams{CommanderID: id, Pos: pos})
		if errors.Is(err, sql.ErrNoRows) {
			return adminRefused("no theme in slot %d", pos)
		}
		if err != nil {
			return err
		}
		if shared == (theme.UploadTime > 0) {
			return nil
		}
		uploadTime := int64(0)
		if shared {
			count, err := tx.CountSharedDormThemes(tx.ctx, id)
			if err != nil {
				return err
			}
			if count >= maxSharedThemes {
				return adminRefused("at most %d themes are shared", maxSharedThemes)
			}
			uploadTime = time.Now().Unix()
		}
		if _, err := tx.SetDormThemeUploadTime(tx.ctx, db.SetDormThemeUploadTimeParams{UploadTime: uploadTime, CommanderID: id, Pos: pos}); err != nil {
			return err
		}
		if shared {
			return nil
		}
		return unshareTheme(tx.ctx, tx.Queries, theme.ThemeID)
	})
}

// DeleteDormTheme empties a theme template slot, as
// BackYardDeleteThemeTemplateCommand does.
func (game *Game) DeleteDormTheme(ctx context.Context, id, pos uint32) (bool, error) {
	return game.editDorm(ctx, id, func(tx *Tx, _ db.CommanderDorm) error {
		deleted, err := tx.Queries.DeleteDormTheme(tx.ctx, db.DeleteDormThemeParams{CommanderID: id, Pos: pos})
		if err != nil {
			return err
		}
		if deleted == 0 {
			return adminRefused("no theme in slot %d", pos)
		}
		return unshareTheme(tx.ctx, tx.Queries, themeID(id, pos))
	})
}
