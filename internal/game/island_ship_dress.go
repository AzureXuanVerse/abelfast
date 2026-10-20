package game

import (
	"slices"
	"time"

	"google.golang.org/protobuf/proto"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/protobuf"
)

// The outfits of the commander (IslandDressUpAgency, island_dress_template
// belongto 1: worn by part type in user_dress.cur_dress, owned with their
// colors in had_dress) and of the characters (IslandCharacterAgency: their
// accessories, belongto 2, counted in ship_sys.had_dress and worn in
// ship_sys.wear_list; their skins and skin colors in ship_sys.skin_list).

// IslandShipDressHelperNew.DressType
const (
	islandDressBody = 6
	islandDressHat  = 7
)

// island_dress_template belongto
const (
	islandDressCommander = 1
	islandDressCharacter = 2
)

// handleIslandShipDress answers IslandChangeDressupCommand (CS_21617): a
// character takes off accessories (unload_dress, back to the stock), puts
// on others (dress_List: from the stock when ship_id is 0, else taken off
// that character), and wears skin_id (0 its own look) in color color_id (0
// the skin's base color), as the client does it from the reply.
func (game *Game) handleIslandShipDress(client *connection.Client, req *protobuf.CS_21617) error {
	ret, err := game.islandShipTx(client, func(tx *Tx, state *islandState, now time.Time) error {
		id := req.GetShipId()
		ships := state.public().ShipSys
		if state.islandShip(id) == nil && id != islandNpcShipID {
			return errRefused
		}
		skinID, colorID := req.GetSkinId(), req.GetColorId()
		var skin *protobuf.PB_ISLAND_SKIN
		if skinID != 0 {
			skin = islandShipSkin(ships, id, skinID)
			if skin == nil {
				return errRefused
			}
			if colorID != 0 && !slices.Contains(skin.ColorList, colorID) {
				return errRefused
			}
		}
		wears := func(ship, dress uint32) int {
			return slices.IndexFunc(ships.WearList, func(wear *protobuf.PB_ISLAND_SHIP_WEAR) bool {
				return wear.GetShipId() == ship && wear.GetDressId() == dress
			})
		}
		stock := map[uint32]*protobuf.PB_ISLAND_DRESS_NUM{}
		for _, owned := range ships.HadDress {
			stock[owned.GetId()] = owned
		}
		for _, dress := range req.GetUnloadDress() {
			if wears(id, dress) < 0 {
				return errRefused
			}
		}
		taken := map[uint32]uint32{}
		for _, wear := range req.GetDress_List() {
			dress := wear.GetDressId()
			if islandDresses[dress].BelongTo != islandDressCharacter {
				return errRefused
			}
			if wear.GetShipId() == 0 {
				taken[dress]++
				if stock[dress] == nil || stock[dress].GetNum() < taken[dress] {
					return ErrNotEnough
				}
			} else if wears(wear.GetShipId(), dress) < 0 {
				return errRefused
			}
		}
		for _, dress := range req.GetUnloadDress() {
			i := wears(id, dress)
			if i < 0 {
				return errRefused // listed twice
			}
			ships.WearList = slices.Delete(ships.WearList, i, i+1)
			if stock[dress] == nil {
				stock[dress] = &protobuf.PB_ISLAND_DRESS_NUM{Id: proto.Uint32(dress), Num: proto.Uint32(0), Read: proto.Uint32(1), Time: proto.Uint32(uint32(now.Unix()))}
				ships.HadDress = append(ships.HadDress, stock[dress])
			}
			stock[dress].Num = proto.Uint32(stock[dress].GetNum() + 1)
		}
		for _, wear := range req.GetDress_List() {
			dress := wear.GetDressId()
			if wear.GetShipId() == 0 {
				stock[dress].Num = proto.Uint32(stock[dress].GetNum() - 1)
			} else {
				i := wears(wear.GetShipId(), dress)
				if i < 0 {
					return errRefused // listed twice
				}
				ships.WearList = slices.Delete(ships.WearList, i, i+1)
			}
			ships.WearList = append(ships.WearList, &protobuf.PB_ISLAND_SHIP_WEAR{ShipId: proto.Uint32(id), DressId: proto.Uint32(dress)})
		}
		if ship := state.islandShip(id); ship != nil {
			ship.CurSkinId = proto.Uint32(skinID)
		}
		if skin != nil {
			skin.ColorId = proto.Uint32(colorID)
		}
		return nil
	})
	if err != nil {
		return err
	}
	return client.Send(&protobuf.SC_21618{Result: ret})
}

// islandShipSkin is a skin a character owns, nil if it doesn't.
func islandShipSkin(ships *protobuf.PB_ISLAND_SHIP_SYS, shipID, skinID uint32) *protobuf.PB_ISLAND_SKIN {
	for _, entry := range ships.GetSkinList() {
		if entry.GetShipId() != shipID {
			continue
		}
		for _, skin := range entry.SkinList {
			if skin.GetId() == skinID {
				return skin
			}
		}
	}
	return nil
}

// handleIslandShipSkinColor answers IslandSendRoleSkinColorCommand
// (CS_21619): a character's skin gets a color
// (island_skin_colordiff_template, of skin_group) for its cost.
func (game *Game) handleIslandShipSkinColor(client *connection.Client, req *protobuf.CS_21619) error {
	ret, err := game.islandShipTx(client, func(tx *Tx, state *islandState, now time.Time) error {
		color, ok := islandSkinColors[req.GetColorId()]
		if !ok {
			return errRefused
		}
		skin := islandShipSkin(state.public().ShipSys, req.GetShipId(), color.SkinGroup)
		if skin == nil || slices.Contains(skin.ColorList, req.GetColorId()) {
			return errRefused
		}
		if err := state.takeIslandItems(color.Cost); err != nil {
			return err
		}
		skin.ColorList = append(skin.ColorList, req.GetColorId())
		return nil
	})
	if err != nil {
		return err
	}
	return client.Send(&protobuf.SC_21620{Result: ret})
}

// handleIslandCommanderDressRead answers IslandSetCommanderDressupReadCommand
// (CS_21621): the commander's new outfit parts are seen (state 1).
func (game *Game) handleIslandCommanderDressRead(client *connection.Client, req *protobuf.CS_21621) error {
	ret, err := game.islandShipTx(client, func(tx *Tx, state *islandState, now time.Time) error {
		for _, owned := range state.private().GetUserDress().GetHadDress() {
			if slices.Contains(req.GetDressId(), owned.GetId()) {
				owned.State = proto.Uint32(1)
			}
		}
		return nil
	})
	if err != nil {
		return err
	}
	return client.Send(&protobuf.SC_21622{Result: ret})
}
