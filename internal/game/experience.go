package game

import (
	"cmp"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"slices"
	"strconv"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/db"
	"github.com/ggmolly/belfast/internal/gamedata"
	"github.com/ggmolly/belfast/internal/logger"
	"github.com/ggmolly/belfast/internal/protobuf"
)

// ys.Battle.BattleConst.BattleScore letters, as server/battle_exp.json keys
// its rates
var scoreLetters = map[string]uint32{"D": 0, "C": 1, "B": 2, "A": 3, "S": 4}

// shipRarityUR is ShipRarity.SSR: ships of that rarity level on
// ship_level.exp_ur (Ship:getLevelExpConfig).
const shipRarityUR = 6

// battleExpRates are the server-side battle exp rates of
// server/battle_exp.json (wiki-derived, see its _source fields). Without
// the file only what the official capture shows is known: S-rank ship exp
// (x1.2), the happy morale bonus (x1.2) and the commander exp of a won
// campaign battle (scenarioPlayerExp).
type battleExpRates struct {
	ship   map[uint32]float64 // ship exp by battle score
	energy map[uint32]float64 // ship exp by energy_template id, x1 when absent
	// commander exp: the A-rank exp by ships sortied, the rate by score, by
	// level gap above a bound (rank ignored there)
	playerByShips map[uint32]float64
	playerRank    map[uint32]float64
	playerGaps    []levelGapRate
	fromFile      bool
}

type levelGapRate struct {
	Over uint32  `json:"over"`
	Rate float64 `json:"rate"`
}

var battleExp battleExpRates

// user_level row: a commander level's exp to the next one and what the
// level holds
type commanderLevelConfig struct {
	Exp           uint32 `json:"exp"`
	MaxCollection uint32 `json:"max_collection"`
	// what the academy's harvests may fill gold and oil up to
	// (Player:getLevelMaxGold / getLevelMaxOil, without a guild's bonus)
	MaxGold uint32 `json:"max_gold"`
	MaxOil  uint32 `json:"max_oil"`
}

var (
	commanderLevels   map[uint32]commanderLevelConfig
	commanderMaxLevel uint32 // the last user_level row (Player:getMaxLevel)
)

// shipLevelExpUR is ship_level exp_ur by level, as shipLevelExp.
var shipLevelExpUR []uint32

func loadExperienceData() error {
	if err := gamedata.Load("user_level", &commanderLevels)(); err != nil {
		return err
	}
	commanderMaxLevel = 0
	for level := range commanderLevels {
		commanderMaxLevel = max(commanderMaxLevel, level)
	}
	if _, ok := commanderLevels[1]; !ok {
		return gamedata.Missing("user_level", "level 1")
	}
	return loadBattleExp()
}

func loadBattleExp() error {
	battleExp = battleExpRates{
		ship:   map[uint32]float64{scoreS: 1.2},
		energy: map[uint32]float64{energyHappy: 1.2},
	}
	raw, err := gamedata.ServerFile("battle_exp.json")
	if err != nil {
		return err
	}
	if raw == nil {
		logger.WithFields("Battle").Warn("no server/battle_exp.json: ship exp only for S rank, commander exp only the capture's for a win")
		return nil
	}
	var file struct {
		Ship   map[string]any `json:"ship_rank_rates"`
		Energy map[string]any `json:"ship_energy_rates"`
		Player struct {
			ByShips map[string]any `json:"by_ships"`
			Rank    map[string]any `json:"rank_rates"`
			Gaps    []levelGapRate `json:"level_gaps"`
		} `json:"player_exp"`
	}
	if err := json.Unmarshal(raw, &file); err != nil {
		return fmt.Errorf("battle exp: %w", err)
	}
	battleExp = battleExpRates{
		ship:          rateTable(file.Ship, scoreKey),
		energy:        rateTable(file.Energy, numberKey),
		playerByShips: rateTable(file.Player.ByShips, numberKey),
		playerRank:    rateTable(file.Player.Rank, scoreKey),
		playerGaps:    file.Player.Gaps,
		fromFile:      true,
	}
	// the widest gap first
	slices.SortFunc(battleExp.playerGaps, func(a, b levelGapRate) int { return cmp.Compare(b.Over, a.Over) })
	return nil
}

func scoreKey(key string) (uint32, bool) {
	score, ok := scoreLetters[key]
	return score, ok
}

func numberKey(key string) (uint32, bool) {
	n, err := strconv.ParseUint(key, 10, 32)
	return uint32(n), err == nil
}

// rateTable keeps the numbers of a battle_exp.json table, leaving out its
// _source notes.
func rateTable(rows map[string]any, key func(string) (uint32, bool)) map[uint32]float64 {
	table := map[uint32]float64{}
	for name, value := range rows {
		id, okKey := key(name)
		rate, okRate := value.(float64)
		if okKey && okRate {
			table[id] = rate
		}
	}
	return table
}

// energyState is the energy_template row a morale falls in.
func energyState(energy uint32) uint32 {
	for id, row := range energyStates {
		if energy >= row.LowerBound && energy <= row.UpperBound {
			return id
		}
	}
	return 0
}

// shipBattleExp is the exp a ship gets for a battle: expedition exp x rank
// x the system's rate x morale, x mvp_exp_percent for the MVP and
// x flagship_exp_percent for the flagship, floored (a campaign battle,
// scenarioExpRate: the capture's 230 = 80 x 1.2 x 2 x 1.2). A score without
// a rate gives none.
func shipBattleExp(expedition expeditionTemplate, systemRate float64, score uint32, ship db.Ship, flagship, mvp bool) uint32 {
	rate, known := battleExp.ship[score]
	if !known {
		return 0
	}
	exp := float64(expedition.Exp) * rate * systemRate
	if energyRate, ok := battleExp.energy[energyState(ship.Energy)]; ok {
		exp *= energyRate
	}
	if mvp {
		exp = exp * float64(mvpExpPercent) / 100
	}
	if flagship {
		exp = exp * float64(flagshipExpPercent) / 100
	}
	return uint32(math.Floor(exp))
}

// playerBattleExp is the commander exp of a campaign battle (SC_40004
// player_exp): the A-rank exp of the fleet's size, x the rank's rate when the
// commander is at most 20 levels above the enemy, the level gap's otherwise,
// x scenarioExpRate, rounded (the capture's 65 = 27 x 1.2 x 2). Sizes the
// wiki doesn't list take the nearest one, logged.
func playerBattleExp(expedition expeditionTemplate, score, ships, commanderLevel uint32) uint32 {
	if !battleExp.fromFile {
		if score > scoreC {
			return scenarioPlayerExp
		}
		return 0
	}
	rate, known := battleExp.playerRank[score]
	if !known {
		logger.WithFields("Battle", logger.FieldValue("score", score)).Warn("no commander exp rate for this score, the commander gets no exp")
		return 0
	}
	for _, gap := range battleExp.playerGaps {
		if commanderLevel > expedition.Level+gap.Over {
			rate = gap.Rate
			break
		}
	}
	base, ok := battleExp.playerByShips[ships]
	if !ok {
		nearest := uint32(0)
		for size := range battleExp.playerByShips {
			if nearest == 0 || absDiff(size, ships) < absDiff(nearest, ships) {
				nearest = size
			}
		}
		base = battleExp.playerByShips[nearest]
		logger.WithFields("Battle", logger.FieldValue("ships", ships), logger.FieldValue("as", nearest)).Warn("no commander exp for this fleet size, using the nearest listed")
	}
	return uint32(math.Round(base * rate * scenarioExpRate))
}

func absDiff(a, b uint32) uint32 {
	if a > b {
		return a - b
	}
	return b - a
}

// addShipExp is Ship:addExp for a battle's exp (accumulate true, as every
// campaign gate passes): exp is the progress into the current level,
// ship_level exp (exp_ur for UR ships) per level, up to the ship's
// max_level. At max_level a ship that can_get_proficency keeps the exp,
// up to gameset exp_overflow_max; any other ship's exp is dropped (0 once
// it levels up to it).
func addShipExp(ship *db.Ship, gained uint32) {
	curve := shipLevelExp
	leveling := shipLevelings[ship.TemplateID]
	if leveling.ur {
		curve = shipLevelExpUR
	}
	accumulate := leveling.accumulate
	if ship.Level >= ship.MaxLevel {
		if ship.Exp >= expOverflowMax {
			return
		}
		if !accumulate {
			gained = 0
		}
	}
	ship.Exp += gained
	leveled := false
	for ship.Level < ship.MaxLevel && int(ship.Level) < len(curve) && ship.Exp >= curve[ship.Level-1] {
		ship.Exp -= curve[ship.Level-1]
		ship.Level++
		leveled = true
	}
	if ship.Level >= ship.MaxLevel {
		if accumulate {
			ship.Exp = min(ship.Exp, expOverflowMax)
		} else if leveled {
			ship.Exp = 0
		}
	}
}

// addCommanderExp is Player:addExp: exp is the progress into the current
// level, user_level exp per level; at the last level it keeps adding up.
// A level up counts for the level tasks (SC_20002), and when the level
// holds more commission teams (user_level max_collection, which EventProxy
// only learns from SC_13002) the commissions are sent again: the client
// levels the commander itself from player_exp, nothing else is pushed in
// the capture.
func addCommanderExp(tx *Tx, gained uint32) error {
	if gained == 0 {
		return nil
	}
	commander, err := tx.GetCommander(tx.ctx, tx.commanderID)
	if err != nil {
		return err
	}
	level, exp := commander.Level, commander.Exp+gained
	for level < commanderMaxLevel {
		config, ok := commanderLevels[level]
		if !ok || exp < config.Exp {
			break
		}
		exp -= config.Exp
		level++
	}
	if err := tx.SetCommanderExp(tx.ctx, db.SetCommanderExpParams{Level: level, Exp: exp, ID: tx.commanderID}); err != nil {
		return err
	}
	if level == commander.Level {
		return nil
	}
	tx.event(TaskEvent{SubType: TaskSubLevel, Count: level})
	if commanderLevels[level].MaxCollection == commanderLevels[commander.Level].MaxCollection {
		return nil
	}
	commissions, err := commissionList(tx.ctx, tx.Queries, tx.commanderID, level)
	if err != nil {
		return err
	}
	tx.push(commissions)
	return nil
}

// shipExpBooks is the exp one ship exp book gives, by item: the
// item_data_statistics usage_arg of the gameset ship_exp_books items
// (ShipExpItemUsagePage.GetAllItemIDs, the ones this region's table has).
var shipExpBooks map[uint32]uint32

func readShipExpBooks(items map[uint32]itemConfig) error {
	var ids []uint32
	if err := gamedata.GamesetDescription("ship_exp_books", &ids); err != nil {
		return err
	}
	shipExpBooks = map[uint32]uint32{}
	for _, id := range ids {
		row, ok := items[id]
		if !ok {
			continue
		}
		if exp := gamedata.IDs(row.UsageArg); len(exp) > 0 {
			shipExpBooks[id] = exp[0]
		}
	}
	return nil
}

// handleUseShipExpBooks answers UseAddShipExpCommand
// (controller/command/item/useaddshipexpcommand.lua, the ship's level bar
// in its detail page, ShipExpItemUsagePage): books are ship exp books and
// their counts, used on ship_id. On result 0 the client adds their
// usage_arg x num to the ship with Ship:addExp (without accumulating, as
// trainShip) and removes the books itself; nothing else is read. The page
// is only offered below the ship's max level. No official capture has it:
// the reply is built from the Lua alone.
func (game *Game) handleUseShipExpBooks(client *connection.Client, req *protobuf.CS_22011) error {
	err := game.tx(context.Background(), client, func(tx *Tx) error {
		ship, err := tx.GetShip(tx.ctx, db.GetShipParams{ID: req.GetShipId(), CommanderID: tx.commanderID})
		if errors.Is(err, sql.ErrNoRows) {
			return errRefused
		} else if err != nil {
			return err
		}
		if ship.Level >= ship.MaxLevel {
			return errRefused
		}
		exp := uint32(0)
		for _, book := range req.GetBooks() {
			per, ok := shipExpBooks[book.GetId()]
			if !ok {
				return errRefused
			}
			if err := tx.SpendItem(book.GetId(), book.GetNum()); err != nil {
				return err
			}
			exp += per * book.GetNum()
		}
		if exp == 0 {
			return errRefused
		}
		if trainShip(&ship, exp) {
			tx.techPointsChanged()
		}
		if err := tx.UpdateShipBattleState(tx.ctx, db.UpdateShipBattleStateParams{
			Level: ship.Level, Exp: ship.Exp, Intimacy: ship.Intimacy, Energy: ship.Energy, ID: ship.ID, CommanderID: tx.commanderID,
		}); err != nil {
			return err
		}
		_, err = raiseShipGroup(tx.ctx, tx.Queries, ship)
		return err
	})
	if err != nil && !refused(err) {
		return err
	}
	return client.Send(&protobuf.SC_22012{Result: result(err)})
}
