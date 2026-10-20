package game

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"slices"
	"time"

	"google.golang.org/protobuf/proto"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/db"
	"github.com/ggmolly/belfast/internal/gamedata"
	"github.com/ggmolly/belfast/internal/protobuf"
)

// ship_skin_template skin_type (ShipSkin.SKIN_TYPE_*, model/vo/shipskin.lua)
const (
	skinTypeDefault = -1
	skinTypePropose = 1 // the oath skin, worn by an oathed ship without owning it
	skinTypeRemake  = 2 // the retrofit skin, worn by a retrofitted ship
	skinTypeOld     = 3 // worn by any ship of its group
)

// ship_skin_template row
type skinConfig struct {
	ShipGroup uint32 `json:"ship_group"`
	SkinType  int    `json:"skin_type"`
	// "" or {group, index, next...} for the skins that switch between
	// variants (ShipSkin.IsChangeSkin): owning one owns its group
	ChangeSkin json.RawMessage `json:"change_skin"`
}

// shipSkin is a ship_skin_template row with its change_skin read.
type shipSkin struct {
	group       uint32 // ship_data_template group_type
	skinType    int
	changeGroup uint32 // change_skin.group, 0 for the others
	// words: the skin has a ship_skin_words row. The client reads it to show
	// the skin anywhere (ShipWordHelper.GetWordAndCV, from Drop:InitConfig
	// for a mail or an award), so a skin without one can't be given.
	words bool
}

var shipSkins map[uint32]shipSkin

func loadSkinData() error {
	var rows map[uint32]skinConfig
	var words map[uint32]json.RawMessage
	if err := gamedata.LoadAll(
		gamedata.Load("ship_skin_template", &rows),
		gamedata.Load("ship_skin_words", &words),
	); err != nil {
		return err
	}
	shipSkins = make(map[uint32]shipSkin, len(rows))
	for id, row := range rows {
		_, hasWords := words[id]
		skin := shipSkin{group: row.ShipGroup, skinType: row.SkinType, words: hasWords}
		var change struct {
			Group uint32 `json:"group"`
		}
		if json.Unmarshal(row.ChangeSkin, &change) == nil {
			skin.changeGroup = change.Group
		}
		shipSkins[id] = skin
	}
	return nil
}

// ownedSkins are the commander's skins that haven't run out: their end_time
// (0 permanent) by ship_skin_template id.
func ownedSkins(ctx context.Context, queries *db.Queries, commanderID uint32, now time.Time) (map[uint32]int64, error) {
	rows, err := queries.ListSkins(ctx, db.ListSkinsParams{CommanderID: commanderID, Now: now.Unix()})
	if err != nil {
		return nil, err
	}
	skins := make(map[uint32]int64, len(rows))
	for _, row := range rows {
		skins[row.SkinID] = row.EndTime
	}
	return skins, nil
}

// hasSkin is ShipSkinProxy:hasSkin: owning the skin, or for a change skin
// any skin of its change group.
func hasSkin(owned map[uint32]int64, id uint32) bool {
	if _, ok := owned[id]; ok {
		return true
	}
	group := shipSkins[id].changeGroup
	if group == 0 {
		return false
	}
	for skin := range owned {
		if shipSkins[skin].changeGroup == group {
			return true
		}
	}
	return false
}

// skinList is SC_12201, ShipSkinProxy's skins: each owned skin with its
// end_time (ShipSkin reads IDTIMEINFO.time as it; 0 permanent). Belfast
// forbids no skin. The official capture's account owns none and gets it
// empty.
func skinList(ctx context.Context, queries *db.Queries, commanderID uint32, now time.Time) (*protobuf.SC_12201, error) {
	rows, err := queries.ListSkins(ctx, db.ListSkinsParams{CommanderID: commanderID, Now: now.Unix()})
	if err != nil {
		return nil, err
	}
	msg := &protobuf.SC_12201{SkinList: make([]*protobuf.IDTIMEINFO, len(rows))}
	for i, row := range rows {
		msg.SkinList[i] = &protobuf.IDTIMEINFO{Id: proto.Uint32(row.SkinID), Time: proto.Uint32(uint32(row.EndTime))}
	}
	return msg, nil
}

// expireSkins forgets the commander's time-limited skins that ran out, and
// puts the ships that wore them back in their default skin, as the client
// did when they ran out (ShipSkinProxy:addExpireTimer).
func expireSkins(ctx context.Context, store *db.Store, commanderID uint32, now time.Time) error {
	return store.InTx(ctx, func(queries *db.Queries) error {
		if err := queries.ResetExpiredShipSkins(ctx, db.ResetExpiredShipSkinsParams{CommanderID: commanderID, Now: now.Unix()}); err != nil {
			return err
		}
		return queries.DeleteExpiredSkins(ctx, db.DeleteExpiredSkinsParams{CommanderID: commanderID, Now: now.Unix()})
	})
}

// AddSkin gives the commander a skin for good (DROP_TYPE_SKIN, a skin_shop
// purchase), a time-limited one they have becoming permanent like on the
// client (ShipSkin.New without end_time).
func (tx *Tx) AddSkin(id uint32) error {
	if _, ok := shipSkins[id]; !ok {
		return errRefused
	}
	return tx.SetSkin(tx.ctx, db.SetSkinParams{CommanderID: tx.commanderID, SkinID: id, EndTime: 0})
}

// AddTimedSkin gives the commander a skin for seconds
// (DROP_TYPE_SKIN_TIMELIMIT, a skin_shop_timelimit purchase): added to what
// is left of it when they have it for a time, nothing when they have it for
// good (Drop's DROP_TYPE_SKIN_TIMELIMIT case, SkinShoppingCommand).
func (tx *Tx) AddTimedSkin(id uint32, seconds int64, now time.Time) error {
	if _, ok := shipSkins[id]; !ok || seconds <= 0 {
		return errRefused
	}
	owned, err := ownedSkins(tx.ctx, tx.Queries, tx.commanderID, now)
	if err != nil {
		return err
	}
	end, ok := owned[id]
	switch {
	case ok && end == 0:
		return nil
	case !ok:
		end = now.Unix()
	}
	return tx.SetSkin(tx.ctx, db.SetSkinParams{CommanderID: tx.commanderID, SkinID: id, EndTime: end + seconds})
}

// canBuySkin is whether count of skin shop goods can be bought: a skin, one
// at a time for good (skin_shop) or time_second per count
// (skin_shop_timelimit), that the commander doesn't have for good. The
// client counts the goods bought then (ShipSkinProxy:GetAllSkins) and
// wouldn't add the time (SkinShoppingCommand).
func (tx *Tx) canBuySkin(goods shopGoods, count uint32, now time.Time) error {
	if _, ok := shipSkins[goods.target]; !ok || goods.Type != shopTypeSkin ||
		(goods.Genre == genreSkinShop && count != 1) || (goods.Genre == genreSkinShopTimeLimit && goods.TimeSecond <= 0) {
		return errRefused
	}
	owned, err := ownedSkins(tx.ctx, tx.Queries, tx.commanderID, now)
	if err != nil {
		return err
	}
	if end, ok := owned[goods.target]; ok && end == 0 {
		return errRefused
	}
	return nil
}

// buySkin gives a skin shop purchase of count checked by canBuySkin.
func (tx *Tx) buySkin(goods shopGoods, count uint32, now time.Time) error {
	switch goods.Genre {
	case genreSkinShop:
		return tx.AddSkin(goods.target)
	case genreSkinShopTimeLimit:
		return tx.AddTimedSkin(goods.target, goods.TimeSecond*int64(count), now)
	}
	return errRefused
}

// handleSetShipSkin answers SetShipSkinCommand
// (controller/command/ship/setshipskincommand.lua), a ship putting on a skin,
// skin_id 0 for its default one. The client sends one per ship when a skin is
// put on several ships at once (ChangeShipSkinPage), and the main skin of a
// change skin. On result 0 it updates the ship, and the secretary when it is
// that ship, itself. The shadow skins (skin_shadow, technology phantoms)
// aren't kept and are refused. No official capture has it: the reply is
// built from the Lua alone.
func (game *Game) handleSetShipSkin(client *connection.Client, req *protobuf.CS_12202) error {
	ctx := context.Background()
	err := game.tx(ctx, client, func(tx *Tx) error {
		return tx.setShipSkin(req.GetShipId(), req.GetSkinId(), req.GetSkinShadow(), time.Now())
	})
	if err != nil && !refused(err) {
		return err
	}
	return client.Send(&protobuf.SC_12203{Result: result(err)})
}

// setShipSkin puts a skin on one of the commander's ships if ShipFashionView
// lets it wear it.
func (tx *Tx) setShipSkin(shipID, skinID, shadow uint32, now time.Time) error {
	if shadow != 0 {
		return errRefused
	}
	ship, err := tx.GetShip(tx.ctx, db.GetShipParams{ID: shipID, CommanderID: tx.commanderID})
	if errors.Is(err, sql.ErrNoRows) {
		return errRefused
	} else if err != nil {
		return err
	}
	if skinID != 0 && skinID != ownableShips[ship.TemplateID].defaultSkin {
		if err := tx.canWear(ship, skinID, now); err != nil {
			return err
		}
	} else {
		skinID = 0
	}
	return tx.SetShipSkin(tx.ctx, db.SetShipSkinParams{SkinID: skinID, ID: shipID, CommanderID: tx.commanderID})
}

// canWear is whether a ship can put on a skin other than its default one
// (ShipFashionView:updateFashionDetail): one of its group's or a group it
// shares skins with (ship_data_group share_group_id), that is an old skin,
// the oath skin of an oathed ship or a skin the commander has. A shared one
// must be a skin that can be shared, worn by an oathed ship for an oath skin
// or at 100 affinity for the others (ShipSkin.CanUseShareSkinForShip).
// The retrofit skin isn't owned: a remoulded ship of its group wears it
// (Ship:getRemouldSkinId and isRemoulded).
func (tx *Tx) canWear(ship db.Ship, skinID uint32, now time.Time) error {
	skin, ok := shipSkins[skinID]
	group := shipGroup(ship.TemplateID) // Ship.groupId, ShipSkin's ship_group
	shared := skin.group != group
	if !ok || (shared && !slices.Contains(shareGroups[group], skin.group)) {
		return errRefused
	}
	owned, err := ownedSkins(tx.ctx, tx.Queries, tx.commanderID, now)
	if err != nil {
		return err
	}
	proposed := ship.Propose > 0
	if skin.skinType == skinTypeRemake && !shared {
		rows, err := tx.ListShipTransforms(tx.ctx, ship.ID)
		if err != nil {
			return err
		}
		if remoulded(group, transformLevels(rows)) {
			return nil
		}
	}
	if skin.skinType != skinTypeOld && !(skin.skinType == skinTypePropose && proposed) && !hasSkin(owned, skinID) {
		return errRefused
	}
	if !shared {
		return nil
	}
	switch skin.skinType {
	case skinTypeDefault, skinTypeRemake, skinTypeOld:
		return errRefused
	case skinTypePropose:
		if !proposed {
			return errRefused
		}
	default:
		if ship.Intimacy < maxIntimacy {
			return errRefused
		}
	}
	return nil
}
