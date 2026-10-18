package game

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"math/rand/v2"
	"slices"

	"google.golang.org/protobuf/proto"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/db"
	"github.com/ggmolly/belfast/internal/gamedata"
	"github.com/ggmolly/belfast/internal/protobuf"
)

// spweapon_data_statistics row: an augment module level (SpWeapon,
// model/vo/spweapon.lua). A level past the first only lists what differs
// from its base row, the rest is read from it (the config tables' base
// lookup in include.lua).
type spweaponConfig struct {
	Type      uint32   `json:"type"`
	Rarity    uint32   `json:"rarity"`
	Next      uint32   `json:"next"`
	Prev      uint32   `json:"prev"`
	UpgradeID uint32   `json:"upgrade_id"`
	Unique    uint32   `json:"unique"`
	Usability []uint32 `json:"usability"`
	// the most a roll adds to value_1 / value_2 (GetAttributesRange)
	Random1     uint32 `json:"value_1_random"`
	Random2     uint32 `json:"value_2_random"`
	Uncraftable uint32 `json:"uncraftable"`
}

// spweapon_upgrade row: what crafting, upgrading and reforging a level
// costs (SpWeapon:GetUpgradeConfig)
type spweaponUpgradeConfig struct {
	CreateGold  uint32     `json:"create_use_gold"`
	CreateItems [][]uint32 `json:"create_use_item"`
	CreatePt    uint32     `json:"create_use_pt"`
	ResetItems  [][]uint32 `json:"reset_use_item"`
	// the points an augment fed to another gives (CalculateHistoryPt)
	SupplyPt     uint32     `json:"upgrade_supply_pt"`
	UpgradeGold  uint32     `json:"upgrade_use_gold"`
	UpgradeItems [][]uint32 `json:"upgrade_use_item"`
	UpgradePt    uint32     `json:"upgrade_use_pt"`
}

// spweapon_type row: the ship types an augment type fits
type spweaponTypeConfig struct {
	ShipTypes []uint32 `json:"ship_type"`
}

var (
	spweapons        map[uint32]spweaponConfig
	spweaponUpgrades map[uint32]spweaponUpgradeConfig
	spweaponTypes    map[uint32]spweaponTypeConfig
	// the upgrade points one of the usage_spweapon items gives (its
	// item_data_statistics usage_arg[1], CalculateHistoryPt)
	spweaponPtItems map[uint32]uint32
)

const usageSpWeapon = "usage_spweapon"

func loadSpWeaponData() error {
	var rows map[uint32]json.RawMessage
	var items map[uint32]itemUseConfig
	err := gamedata.LoadAll(
		gamedata.Load("spweapon_data_statistics", &rows),
		gamedata.Load("spweapon_upgrade", &spweaponUpgrades),
		gamedata.Load("spweapon_type", &spweaponTypes),
		gamedata.Load("item_data_statistics", &items),
	)
	if err != nil {
		return err
	}
	spweapons = make(map[uint32]spweaponConfig, len(rows))
	for id, raw := range rows {
		var base struct {
			Base uint32 `json:"base"`
		}
		var config spweaponConfig
		if err := json.Unmarshal(raw, &base); err != nil {
			return fmt.Errorf("spweapon_data_statistics %d: %w", id, err)
		}
		if base.Base != 0 {
			if err := json.Unmarshal(rows[base.Base], &config); err != nil {
				return fmt.Errorf("spweapon_data_statistics %d base %d: %w", id, base.Base, err)
			}
		}
		if err := json.Unmarshal(raw, &config); err != nil {
			return fmt.Errorf("spweapon_data_statistics %d: %w", id, err)
		}
		spweapons[id] = config
	}
	spweaponPtItems = map[uint32]uint32{}
	for id, item := range items {
		if pt := gamedata.IDs(item.UsageArg); item.Usage == usageSpWeapon && len(pt) > 0 {
			spweaponPtItems[id] = pt[0]
		}
	}
	return nil
}

func (config spweaponConfig) upgrade() spweaponUpgradeConfig {
	return spweaponUpgrades[config.UpgradeID]
}

// craftable is SpWeapon:IsCraftable.
func (config spweaponConfig) craftable() bool {
	return config.Uncraftable != 1 && config.upgrade().CreateGold > 0
}

// fits is Ship:CanEquipSpWeapon: the ship's type is one the augment is
// wearable by (usability, else its spweapon_type's), and a unique augment
// only goes to its ship group.
func (config spweaponConfig) fits(ship db.Ship) bool {
	types := config.Usability
	if len(types) == 0 {
		types = spweaponTypes[config.Type].ShipTypes
	}
	template, ok := ownableShips[ship.TemplateID]
	if !ok {
		template = shipTemplates[ship.TemplateID/10*10+1]
	}
	if !slices.Contains(types, template.Type) {
		return false
	}
	return config.Unique == 0 || config.Unique == shipGroup(ship.TemplateID)
}

func spweaponInfo(row db.Spweapon) *protobuf.SPWEAPONINFO {
	return &protobuf.SPWEAPONINFO{
		Id:         proto.Uint32(row.ID),
		TemplateId: proto.Uint32(row.TemplateID),
		Attr_1:     proto.Uint32(row.Attr1),
		Attr_2:     proto.Uint32(row.Attr2),
		AttrTemp_1: proto.Uint32(row.AttrTemp1),
		AttrTemp_2: proto.Uint32(row.AttrTemp2),
		Pt:         proto.Uint32(row.Pt),
	}
}

// bagSpWeapons is SC_14001.spweapon_list, the augments no ship wears.
func bagSpWeapons(ctx context.Context, queries *db.Queries, commanderID uint32) ([]*protobuf.SPWEAPONINFO, error) {
	rows, err := queries.ListBagSpWeapons(ctx, commanderID)
	if err != nil {
		return nil, err
	}
	list := make([]*protobuf.SPWEAPONINFO, len(rows))
	for i, row := range rows {
		list[i] = spweaponInfo(row)
	}
	return list, nil
}

// setShipSpWeapons fills SHIPINFO.spweapon of the ships of a ship list that
// wear one.
func setShipSpWeapons(ctx context.Context, queries *db.Queries, commanderID uint32, ships []*protobuf.SHIPINFO) error {
	rows, err := queries.ListShipSpWeapons(ctx, commanderID)
	if err != nil {
		return err
	}
	byShip := make(map[uint32]db.Spweapon, len(rows))
	for _, row := range rows {
		byShip[row.ShipID] = row.Spweapon
	}
	for _, ship := range ships {
		if row, ok := byShip[ship.GetId()]; ok {
			ship.Spweapon = spweaponInfo(row)
		}
	}
	return nil
}

// spweaponTarget is the augment a request names (EquipmentProxy.StaticGetSpWeapon):
// the one the commander's ship wears when shipID isn't 0, else the bag's
// uid.
func (tx *Tx) spweaponTarget(shipID, uid uint32) (db.Spweapon, error) {
	if shipID != 0 {
		if _, err := tx.GetShip(tx.ctx, db.GetShipParams{ID: shipID, CommanderID: tx.commanderID}); err != nil {
			return db.Spweapon{}, notFound(err)
		}
		id, err := tx.GetShipSpWeapon(tx.ctx, shipID)
		if err != nil {
			return db.Spweapon{}, notFound(err)
		}
		uid = id
	} else if _, err := tx.GetSpWeaponShip(tx.ctx, uid); err == nil {
		return db.Spweapon{}, errRefused
	} else if !errors.Is(err, sql.ErrNoRows) {
		return db.Spweapon{}, err
	}
	row, err := tx.GetSpWeapon(tx.ctx, db.GetSpWeaponParams{CommanderID: tx.commanderID, ID: uid})
	return row, notFound(err)
}

// notFound turns a missing row into a refusal.
func notFound(err error) error {
	if errors.Is(err, sql.ErrNoRows) {
		return errRefused
	}
	return err
}

// spweaponFull is CountBagSpWeapons at the bag's size: no augment can go
// back to it (GetSpWeaponCapacity <= GetSpWeaponCount).
func (tx *Tx) spweaponFull() (bool, error) {
	n, err := tx.CountBagSpWeapons(tx.ctx, tx.commanderID)
	return n >= spweaponBagSize, err
}

// feedSpWeapon spends what an upgrade or a craft is fed and returns its
// points (SpWeapon.CalculateHistoryPt): each usage_spweapon item of
// itemIDs (one entry per item), and each augment of uids, whether in the bag
// or worn, which is destroyed for its spweapon_upgrade upgrade_supply_pt
// (its own pt isn't counted). target can't feed itself.
func (tx *Tx) feedSpWeapon(itemIDs, uids []uint32, target uint32) (uint32, error) {
	var pt uint32
	counts := map[uint32]uint32{}
	for _, id := range itemIDs {
		itemPt, ok := spweaponPtItems[id]
		if !ok {
			return 0, errRefused
		}
		pt += itemPt
		counts[id]++
	}
	for _, id := range slices.Sorted(maps.Keys(counts)) {
		if err := tx.SpendItem(id, counts[id]); err != nil {
			return 0, err
		}
	}
	for i, uid := range uids {
		if uid == target || slices.Contains(uids[:i], uid) {
			return 0, errRefused
		}
		row, err := tx.GetSpWeapon(tx.ctx, db.GetSpWeaponParams{CommanderID: tx.commanderID, ID: uid})
		if err != nil {
			return 0, notFound(err)
		}
		config, ok := spweapons[row.TemplateID]
		if !ok {
			return 0, errRefused
		}
		pt += config.upgrade().SupplyPt
		if err := tx.DeleteSpWeapon(tx.ctx, db.DeleteSpWeaponParams{CommanderID: tx.commanderID, ID: uid}); err != nil {
			return 0, err
		}
	}
	return pt, nil
}

// spweaponLevels walks the upgrades points pay for from templateID, as
// UpgradeSpWeaponCommand and CompositeSpWeaponCommand do: each level's
// upgrade_use_pt (from spent on) adds to the threshold and moves to next
// while the points reach it, paying its upgrade_use_gold and
// upgrade_use_item; a level of higher rarity (a breakthrough) is only taken
// as the first step, and stops there. It returns the level reached, the
// points left past it (what's over the next threshold is lost), how many
// levels were taken and their gold and items.
func spweaponLevels(templateID, points, spent uint32) (id, pt, levels, gold uint32, items [][]uint32) {
	id = templateID
	threshold, reached := spent, uint32(0)
	for {
		config := spweapons[id]
		next, ok := spweapons[config.Next]
		if config.Next == 0 || !ok {
			break
		}
		reached = threshold
		upgrade := config.upgrade()
		threshold += upgrade.UpgradePt
		breakthrough := config.Rarity < next.Rarity
		if levels > 0 && breakthrough {
			break
		}
		if points < threshold {
			break
		}
		items = append(items, upgrade.UpgradeItems...)
		gold += upgrade.UpgradeGold
		levels++
		id = config.Next
		if breakthrough {
			reached = threshold
			break
		}
	}
	return id, min(points, threshold) - reached, levels, gold, items
}

// paySpWeapon pays gold and [item, count] entries.
func (tx *Tx) paySpWeapon(gold uint32, items [][]uint32) error {
	if err := tx.SpendResource(ResourceGold, gold); err != nil {
		return err
	}
	for _, item := range items {
		if len(item) < 2 {
			continue
		}
		if err := tx.SpendItem(item[0], item[1]); err != nil {
			return err
		}
	}
	return nil
}

// handleShipSpWeapon puts the bag augment spweapon_id on a ship, or takes
// the ship's off with 0 (UpdateShipSpWeaponCommand,
// EquipSpWeaponFromShipCommand, ShipEquipAllChangeCommand, CS_14201). The
// one the ship wore goes back to the bag; taking one off needs room there.
// On result 0 the client moves them itself.
func (game *Game) handleShipSpWeapon(client *connection.Client, req *protobuf.CS_14201) error {
	ctx := context.Background()
	err := game.tx(ctx, client, func(tx *Tx) error {
		ship, err := tx.GetShip(ctx, db.GetShipParams{ID: req.GetShipId(), CommanderID: client.CommanderID})
		if err != nil {
			return notFound(err)
		}
		_, err = tx.GetShipSpWeapon(ctx, ship.ID)
		wearing := err == nil
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		if req.GetSpweaponId() == 0 {
			if !wearing {
				return errRefused
			}
			if full, err := tx.spweaponFull(); err != nil {
				return err
			} else if full {
				return errRefused
			}
			return tx.UnequipShipSpWeapon(ctx, ship.ID)
		}
		row, err := tx.spweaponTarget(0, req.GetSpweaponId())
		if err != nil {
			return err
		}
		if config, ok := spweapons[row.TemplateID]; !ok || !config.fits(ship) {
			return errRefused
		}
		if err := tx.UnequipShipSpWeapon(ctx, ship.ID); err != nil {
			return err
		}
		return tx.EquipSpWeapon(ctx, db.EquipSpWeaponParams{ShipID: ship.ID, SpweaponID: row.ID})
	})
	if err != nil && !refused(err) {
		return err
	}
	return client.Send(&protobuf.SC_14202{Result: result(err)})
}

// handleUpgradeSpWeapon feeds an augment (UpgradeSpWeaponCommand, CS_14203):
// usage_spweapon items and other augments add to its pt, which levels it
// up as far as it pays for (spweaponLevels) with those levels' gold and
// items. Nothing fed and no level reached is refused. On result 0 the
// client migrates it to the level reached with the points left and takes
// the same costs off, so the server keeps the same.
func (game *Game) handleUpgradeSpWeapon(client *connection.Client, req *protobuf.CS_14203) error {
	ctx := context.Background()
	err := game.tx(ctx, client, func(tx *Tx) error {
		row, err := tx.spweaponTarget(req.GetShipId(), req.GetSpweaponId())
		if err != nil {
			return err
		}
		if _, ok := spweapons[row.TemplateID]; !ok {
			return errRefused
		}
		fed, err := tx.feedSpWeapon(req.GetItemIdList(), req.GetSpweaponIdList(), row.ID)
		if err != nil {
			return err
		}
		id, pt, levels, gold, items := spweaponLevels(row.TemplateID, row.Pt+fed, 0)
		if fed == 0 && levels == 0 {
			return errRefused
		}
		if err := tx.paySpWeapon(gold, items); err != nil {
			return err
		}
		row.TemplateID, row.Pt = id, pt
		return tx.saveSpWeapon(row)
	})
	if err != nil && !refused(err) {
		return err
	}
	return client.Send(&protobuf.SC_14204{Result: result(err)})
}

func (tx *Tx) saveSpWeapon(row db.Spweapon) error {
	return tx.UpdateSpWeapon(tx.ctx, db.UpdateSpWeaponParams{
		TemplateID: row.TemplateID, Attr1: row.Attr1, Attr2: row.Attr2,
		AttrTemp1: row.AttrTemp1, AttrTemp2: row.AttrTemp2, Pt: row.Pt,
		CommanderID: tx.commanderID, ID: row.ID,
	})
}

// rollSpWeapon is a roll of an augment's two attributes, each from least
// (0 or 1) to its value_N_random: a crafted one's from 0 (the "+ 0~N" of
// SpWeaponHelper), a reroll's from 1 (SpWeaponModifyLayer's min(1, N)), so
// that 0 stands for no reroll waiting.
func rollSpWeapon(config spweaponConfig, least uint32) (uint32, uint32) {
	return rollSpWeaponAttr(config.Random1, least), rollSpWeaponAttr(config.Random2, least)
}

func rollSpWeaponAttr(max, least uint32) uint32 {
	least = min(least, max)
	return least + rand.Uint32N(max-least+1)
}

// handleReforgeSpWeapon rerolls an augment's attributes for its
// reset_use_item (ReforgeSpWeaponCommand, CS_14205): the roll waits in
// attr_temp until it is kept or discarded (CS_14207). Refused while one
// waits, or when the attributes are already at their maximum. On result 0
// the client shows attr_temp_1/2 and takes the items off.
func (game *Game) handleReforgeSpWeapon(client *connection.Client, req *protobuf.CS_14205) error {
	ctx := context.Background()
	reply := &protobuf.SC_14206{}
	err := game.tx(ctx, client, func(tx *Tx) error {
		row, err := tx.spweaponTarget(req.GetShipId(), req.GetSpweaponId())
		if err != nil {
			return err
		}
		config, ok := spweapons[row.TemplateID]
		if !ok || row.AttrTemp1 != 0 || row.AttrTemp2 != 0 || (row.Attr1 == config.Random1 && row.Attr2 == config.Random2) {
			return errRefused
		}
		if err := tx.paySpWeapon(0, config.upgrade().ResetItems); err != nil {
			return err
		}
		row.AttrTemp1, row.AttrTemp2 = rollSpWeapon(config, 1)
		reply.AttrTemp_1, reply.AttrTemp_2 = proto.Uint32(row.AttrTemp1), proto.Uint32(row.AttrTemp2)
		return tx.saveSpWeapon(row)
	})
	if err != nil && !refused(err) {
		return err
	}
	reply.Result = result(err)
	return client.Send(reply)
}

// spweaponConfirmExchange is SpWeapon.CONFIRM_OP_EXCHANGE: keep the reroll
// (CONFIRM_OP_DISCARD 0 drops it).
const spweaponConfirmExchange = 1

// handleConfirmReforgeSpWeapon keeps (cmd 1) or discards (0) an augment's
// reroll (ConfirmReforgeSpWeaponCommand, CS_14207); either way attr_temp
// goes back to 0, as the client sets it on result 0.
func (game *Game) handleConfirmReforgeSpWeapon(client *connection.Client, req *protobuf.CS_14207) error {
	ctx := context.Background()
	err := game.tx(ctx, client, func(tx *Tx) error {
		row, err := tx.spweaponTarget(req.GetShipId(), req.GetSpweaponId())
		if err != nil {
			return err
		}
		if req.GetCmd() == spweaponConfirmExchange {
			row.Attr1, row.Attr2 = row.AttrTemp1, row.AttrTemp2
		}
		row.AttrTemp1, row.AttrTemp2 = 0, 0
		return tx.saveSpWeapon(row)
	})
	if err != nil && !refused(err) {
		return err
	}
	return client.Send(&protobuf.SC_14208{Result: result(err)})
}

// handleCompositeSpWeapon crafts an augment (CompositeSpWeaponCommand,
// CS_14209): fed points (feedSpWeapon) must reach the craftable level's
// create_use_pt, paid with its create_use_gold and create_use_item; the
// points past it level the new augment up as an upgrade does
// (spweaponLevels). The client sends the level that reaches, or the
// craftable one; both are taken. Needs room in the bag. Its attributes are
// rolled; the client adds the reply's spweapon to the bag.
func (game *Game) handleCompositeSpWeapon(client *connection.Client, req *protobuf.CS_14209) error {
	ctx := context.Background()
	reply := &protobuf.SC_14210{}
	err := game.tx(ctx, client, func(tx *Tx) error {
		base := craftableSpWeapon(req.GetTemplateId())
		if base == 0 {
			return errRefused
		}
		if full, err := tx.spweaponFull(); err != nil {
			return err
		} else if full {
			return errRefused
		}
		fed, err := tx.feedSpWeapon(req.GetItemIdList(), req.GetSpweaponIdList(), 0)
		if err != nil {
			return err
		}
		config := spweapons[base]
		create := config.upgrade()
		if fed < create.CreatePt {
			return errRefused
		}
		id, pt, _, gold, items := spweaponLevels(base, fed, create.CreatePt)
		if id != req.GetTemplateId() && base != req.GetTemplateId() {
			return errRefused
		}
		if err := tx.paySpWeapon(create.CreateGold+gold, append(slices.Clone(create.CreateItems), items...)); err != nil {
			return err
		}
		attr1, attr2 := rollSpWeapon(spweapons[id], 0)
		row, err := tx.CreateSpWeapon(ctx, db.CreateSpWeaponParams{CommanderID: client.CommanderID, TemplateID: id, Attr1: attr1, Attr2: attr2, Pt: pt})
		if err != nil {
			return err
		}
		reply.Spweapon = spweaponInfo(row)
		return nil
	})
	if err != nil && !refused(err) {
		return err
	}
	reply.Result = result(err)
	return client.Send(reply)
}

// craftableSpWeapon is the craftable augment level templateID is, or
// comes from through prev, 0 for none.
func craftableSpWeapon(templateID uint32) uint32 {
	for id, steps := templateID, 0; id != 0 && steps < len(spweapons); steps++ {
		config, ok := spweapons[id]
		if !ok {
			return 0
		}
		if config.craftable() {
			return id
		}
		id = config.Prev
	}
	return 0
}
