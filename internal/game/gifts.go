package game

import (
	"database/sql"
	"encoding/json"
	"errors"
	"slices"

	"github.com/ggmolly/belfast/internal/db"
	"github.com/ggmolly/belfast/internal/gamedata"
)

// giftDislikes are the usage_favorability gifts each ship_data_statistics
// id dislikes (gift_dislike): given one, it gains no affection
// (ShipGiftTools.GetItemIntimacyValue, view/ship/gift/shipgifttools.lua).
var giftDislikes map[uint32][]uint32

func readGiftDislikes(stats map[uint32]shipStatisticsConfig) {
	giftDislikes = map[uint32][]uint32{}
	for id, stat := range stats {
		if dislikes := gamedata.IDs(stat.GiftDislike); len(dislikes) > 0 {
			giftDislikes[id] = dislikes
		}
	}
}

// readShipGift reads a usage_favorability usage_arg: [item, affection] the
// affection (1/100) one gift gives a ship that likes it.
func readShipGift(raw json.RawMessage) uint32 {
	if arg := gamedata.IDs(raw); len(arg) >= 2 {
		return arg[1]
	}
	return 0
}

// giveShipGift gives count of a gift to the ship arg names (ShipGiftScene,
// view/ship/gift/shipgiftscene.lua). Every gift is spent; a ship that
// doesn't dislike it gains affection per gift, past the cap wasted
// (shipyard_favorability_exceed) as Ship:addLikability clamps it: 100.00
// (maxIntimacy) unoathed, the last intimacy_template upper_bound oathed.
// The client adds the affection itself on result 0 ((count -
// drop_list[1].count) * affection, ShipGiftScene:OnGiftSuccess), so nothing
// goes back in drop_list. A ship at its cap is refused, as the client does
// (shipyard_favorability_max).
func (tx *Tx) giveShipGift(id, count, affection uint32, arg []uint32) error {
	if len(arg) != 1 || affection == 0 {
		return errRefused
	}
	ship, err := tx.GetShip(tx.ctx, db.GetShipParams{ID: arg[0], CommanderID: tx.commanderID})
	if errors.Is(err, sql.ErrNoRows) {
		return errRefused
	}
	if err != nil {
		return err
	}
	limit := uint32(maxIntimacy)
	if ship.Propose != 0 {
		limit = intimacies[len(intimacies)-1].UpperBound
	}
	if ship.Intimacy >= limit {
		return errRefused
	}
	if err := tx.SpendItem(id, count); err != nil {
		return err
	}
	if slices.Contains(giftDislikes[ship.TemplateID], id) {
		return nil
	}
	ship.Intimacy = min(ship.Intimacy+affection*count, limit)
	if err := tx.UpdateShipBattleState(tx.ctx, db.UpdateShipBattleStateParams{
		Level: ship.Level, Exp: ship.Exp, Intimacy: ship.Intimacy, Energy: ship.Energy, ID: ship.ID, CommanderID: tx.commanderID,
	}); err != nil {
		return err
	}
	_, err = raiseShipGroup(tx.ctx, tx.Queries, ship)
	return err
}
