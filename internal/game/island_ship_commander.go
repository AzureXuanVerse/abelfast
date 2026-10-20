package game

import (
	"slices"
	"time"

	"google.golang.org/protobuf/proto"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/protobuf"
)

// IslandShipDressHelperNew.DressType of the parts the commander can't go
// without (GUESS)
const (
	islandDressHair = 4
	islandDressFace = 5
)

// handleIslandShipDressRead answers IslandSetRoleDressupReadCommand
// (CS_21624): the characters' new accessories are seen (read 1).
func (game *Game) handleIslandShipDressRead(client *connection.Client, req *protobuf.CS_21624) error {
	ret, err := game.islandShipTx(client, func(tx *Tx, state *islandState, now time.Time) error {
		for _, owned := range state.public().GetShipSys().GetHadDress() {
			if slices.Contains(req.GetDressId(), owned.GetId()) {
				owned.Read = proto.Uint32(1)
			}
		}
		return nil
	})
	if err != nil {
		return err
	}
	return client.Send(&protobuf.SC_21625{Result: ret})
}

// islandDressLinkConfig is what this file reads of island_dress_template:
// the outfit a hat goes with (related_dress) and a body's other form
// (cloth_related, IslandDressUpAgency:GetMorphTargetId).
type islandDressLinkConfig struct {
	RelatedDress uint32 `json:"related_dress"`
	ClothRelated uint32 `json:"cloth_related"`
}

var islandDressLinks map[uint32]islandDressLinkConfig

// ownedCommanderDress is an outfit part the commander owns, nil if it
// doesn't.
func ownedCommanderDress(dress *protobuf.PB_ISLAND_USER_DRESS_SYS, id uint32) *protobuf.PB_ISLAND_DRESS_USER {
	for _, owned := range dress.GetHadDress() {
		if owned.GetId() == id {
			return owned
		}
	}
	return nil
}

// islandBodyHat is the hat that goes with a body (the first
// island_dress_template whose related_dress it is), 0 if none.
func islandBodyHat(body uint32) uint32 {
	hat := uint32(0)
	for id, link := range islandDressLinks {
		if link.RelatedDress == body && (hat == 0 || id < hat) {
			hat = id
		}
	}
	return hat
}

// handleIslandCommanderDress answers IslandChangeCommanderDressupCommand
// (CS_21626, also IslandMorphFormChangeCommand): the commander wears the
// parts of dress_list (by type) and recolors owned parts (color_list, 0
// the base color, else a color it owns). A body's other form becomes the
// current one (twin_cur_list, IslandDressUpAgency:SetTwinCurId). The reply's
// cap_list says, for each body worn that has a hat (related_dress), whether
// its hat is on (the hat's id) or off (0), which the client keeps
// (ChangeCapState). The players on the same island see the new outfit
// (SC_21224). GUESSES: a hat counts as owned with its body; hair, face and
// body can't be left empty; a body's hat state only changes when
// dress_list has a hat.
func (game *Game) handleIslandCommanderDress(client *connection.Client, req *protobuf.CS_21626) error {
	var caps []*protobuf.PB_CAP_STATE
	var worn []*protobuf.PB_ISLAND_CUR_DRESS
	var colors []*protobuf.PB_DRESS_COLOR
	ret, err := game.islandShipTx(client, func(tx *Tx, state *islandState, now time.Time) error {
		dress := state.private().UserDress
		owns := func(id uint32) bool {
			if ownedCommanderDress(dress, id) != nil {
				return true
			}
			body := islandDressLinks[id].RelatedDress
			return body != 0 && ownedCommanderDress(dress, body) != nil
		}
		hat, hasHat := uint32(0), false
		for _, entry := range req.GetDressList() {
			kind, id := entry.GetType(), entry.GetId()
			if id == 0 {
				if kind == islandDressHair || kind == islandDressFace || kind == islandDressBody {
					return errRefused
				}
			} else if config, ok := islandDresses[id]; !ok || config.BelongTo != islandDressCommander || config.Type != kind || !owns(id) {
				return errRefused
			}
			if kind == islandDressHat {
				hat, hasHat = id, true
			}
		}
		for _, color := range req.GetColorList() {
			owned := ownedCommanderDress(dress, color.GetId())
			if owned == nil || color.GetColor() != 0 && !slices.Contains(owned.ColorList, color.GetColor()) {
				return errRefused
			}
		}
		for _, color := range req.GetColorList() {
			ownedCommanderDress(dress, color.GetId()).Color = proto.Uint32(color.GetColor())
		}
		for _, entry := range req.GetDressList() {
			kind, id := entry.GetType(), entry.GetId()
			i := slices.IndexFunc(dress.CurDress, func(cur *protobuf.PB_ISLAND_CUR_DRESS) bool { return cur.GetType() == kind })
			if i < 0 {
				dress.CurDress = append(dress.CurDress, &protobuf.PB_ISLAND_CUR_DRESS{Type: proto.Uint32(kind), Id: proto.Uint32(id)})
			} else {
				dress.CurDress[i].Id = proto.Uint32(id)
			}
			if kind != islandDressBody || id == 0 {
				continue
			}
			if twin := islandDressLinks[id].ClothRelated; twin != 0 {
				other := islandDressLinks[twin].ClothRelated
				dress.TwinCurList = slices.DeleteFunc(dress.TwinCurList, func(cur uint32) bool {
					return cur == twin || other != 0 && cur == other || cur == id
				})
				dress.TwinCurList = append(dress.TwinCurList, id)
			}
			if !hasHat || islandBodyHat(id) == 0 {
				continue
			}
			capID := uint32(0)
			if hat != 0 && islandDressLinks[hat].RelatedDress == id {
				capID = hat
			}
			caps = append(caps, &protobuf.PB_CAP_STATE{DressId: proto.Uint32(id), CapId: proto.Uint32(capID)})
			if j := slices.IndexFunc(dress.CapList, func(entry *protobuf.PB_CAP_STATE) bool { return entry.GetDressId() == id }); j >= 0 {
				dress.CapList[j].CapId = proto.Uint32(capID)
			} else {
				dress.CapList = append(dress.CapList, &protobuf.PB_CAP_STATE{DressId: proto.Uint32(id), CapId: proto.Uint32(capID)})
			}
		}
		worn = dress.CurDress
		for _, cur := range worn {
			if owned := ownedCommanderDress(dress, cur.GetId()); owned != nil && owned.GetColor() != 0 {
				colors = append(colors, &protobuf.PB_DRESS_COLOR{Id: proto.Uint32(cur.GetId()), Color: proto.Uint32(owned.GetColor())})
			}
		}
		return nil
	})
	if err != nil {
		return err
	}
	if *ret == 0 {
		if islandID, here := game.islandOf(client); here {
			game.tellIsland(client, islandID, &protobuf.SC_21224{
				IslandId: proto.Uint32(islandID), UserId: proto.Uint32(client.CommanderID), DressList: worn, DressColor: colors,
			})
		}
	}
	return client.Send(&protobuf.SC_21627{Result: ret, CapList: caps})
}

// handleIslandDressColor answers IslandSendDressColorCommand (CS_21628):
// the commander (id 0; the client sends nothing else) buys a color of an
// outfit part it owns (island_dress_colordiff_template belongto_dress) for
// its cost. The client adds the color but doesn't take the cost off itself
// (GUESS: the official server charges what the page shows; the next island
// data corrects the client's count).
func (game *Game) handleIslandDressColor(client *connection.Client, req *protobuf.CS_21628) error {
	ret, err := game.islandShipTx(client, func(tx *Tx, state *islandState, now time.Time) error {
		color, ok := islandDressCosts[req.GetColorId()]
		if req.GetId() != 0 || !ok {
			return errRefused
		}
		owned := ownedCommanderDress(state.private().UserDress, color.BelongToDress)
		if owned == nil || slices.Contains(owned.ColorList, req.GetColorId()) {
			return errRefused
		}
		if err := state.takeIslandItems(color.Cost); err != nil {
			return err
		}
		owned.ColorList = append(owned.ColorList, req.GetColorId())
		return nil
	})
	if err != nil {
		return err
	}
	return client.Send(&protobuf.SC_21629{Result: ret})
}
