package game

import (
	"context"
	"database/sql"
	"errors"

	"google.golang.org/protobuf/proto"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/db"
	"github.com/ggmolly/belfast/internal/gamedata"
	"github.com/ggmolly/belfast/internal/protobuf"
)

// const.lua DROP_TYPE_*: the frames AttireProxy keeps (model/vo/attire),
// the port wallpapers (LivingAreaCoverProxy) and the combat UIs
const (
	dropTypeIconFrame = 14 // item_data_frame
	dropTypeChatFrame = 15 // item_data_chat
	dropTypeCover     = 29 // DROP_TYPE_LIVINGAREA_COVER, livingarea_cover
	dropTypeCombatUI  = 31 // DROP_TYPE_COMBAT_UI_STYLE, item_data_battleui
)

// AttireConst.TYPE_*: what CS_11005 applies
const (
	attireTypeIconFrame = 2
	attireTypeChatFrame = 3
	attireTypeCombatUI  = 4
)

// defaultAppearance is the frame, wallpaper and combat UI everyone has (id
// 0 of item_data_frame, item_data_chat, livingarea_cover, item_data_battleui).
const defaultAppearance = 0

// combatUIConfig is an item_data_battleui row: is_unlock 0 is owned by
// everyone (CombatUIStyle.isOwned).
type combatUIConfig struct {
	IsUnlock uint32 `json:"is_unlock"`
}

var (
	iconFrames map[uint32]struct{}       // item_data_frame
	chatFrames map[uint32]struct{}       // item_data_chat
	combatUIs  map[uint32]combatUIConfig // item_data_battleui
	covers     map[uint32]struct{}       // livingarea_cover
)

func loadAttireData() error {
	return gamedata.LoadAll(
		gamedata.Load("item_data_frame", &iconFrames),
		gamedata.Load("item_data_chat", &chatFrames),
		gamedata.Load("item_data_battleui", &combatUIs),
		gamedata.Load("livingarea_cover", &covers),
	)
}

// ownedAttire is what SC_11003 lists of the commander's cosmetics:
// icon_frame_list and chat_frame_list (AttireProxy: time is a timed frame's
// end, AttireFrame.updateData, 0 for one owned for good, as every stored
// frame is), battle_ui_list (combat UIs got, the free ones aren't listed)
// and cover.covers (the wallpapers unlocked, LivingAreaCoverProxy).
type ownedAttire struct {
	icons, chats      []*protobuf.IDTIMEINFO
	combatUIs, covers []uint32
}

func attireFrames(ctx context.Context, queries *db.Queries, commanderID uint32) (ownedAttire, error) {
	var owned ownedAttire
	frames, err := queries.ListAttireFrames(ctx, commanderID)
	if err != nil {
		return owned, err
	}
	for _, frame := range frames {
		switch frame.DropType {
		case dropTypeIconFrame:
			owned.icons = append(owned.icons, &protobuf.IDTIMEINFO{Id: proto.Uint32(frame.FrameID), Time: proto.Uint32(0)})
		case dropTypeChatFrame:
			owned.chats = append(owned.chats, &protobuf.IDTIMEINFO{Id: proto.Uint32(frame.FrameID), Time: proto.Uint32(0)})
		case dropTypeCombatUI:
			owned.combatUIs = append(owned.combatUIs, frame.FrameID)
		case dropTypeCover:
			owned.covers = append(owned.covers, frame.FrameID)
		}
	}
	return owned, nil
}

// addAttireFrame stores a frame, wallpaper or combat UI drop for good; the
// client adds it itself (AttireProxy.addAttireFrame,
// LivingAreaCoverProxy.UpdateCover).
func (tx *Tx) addAttireFrame(kind, id uint32) error {
	return tx.AddAttireFrame(tx.ctx, db.AddAttireFrameParams{CommanderID: tx.commanderID, DropType: kind, FrameID: id})
}

// appearance is the commander's commander_appearance row, the defaults when
// it has none.
func appearance(ctx context.Context, queries *db.Queries, commanderID uint32) (db.CommanderAppearance, error) {
	row, err := queries.GetAppearance(ctx, commanderID)
	if errors.Is(err, sql.ErrNoRows) {
		return db.CommanderAppearance{CommanderID: commanderID}, nil
	}
	return row, err
}

// ownsAttire tells whether the commander may use the cosmetic id of drop
// type kind: the default (0) always, a combat UI everyone has
// (is_unlock 0), else one it got (attire_frames).
func ownsAttire(ctx context.Context, queries *db.Queries, commanderID, kind, id uint32) (bool, error) {
	if id == defaultAppearance {
		return true, nil
	}
	if kind == dropTypeCombatUI && combatUIs[id].IsUnlock == 0 {
		return true, nil
	}
	return queries.HasAttireFrame(ctx, db.HasAttireFrameParams{CommanderID: commanderID, DropType: kind, FrameID: id})
}

// handleAttireApply answers AttireApplyCommand (Attire screen, Apply): the
// icon frame, chat frame or combat UI the commander now uses. It comes back
// in SC_11003 (display.icon_frame / chat_frame, battle_ui) and in every
// DISPLAYINFO other players see. The client only sends ids AttireProxy
// knows; one the commander doesn't own is refused (the client only prints
// the result). Never captured: built from the Lua alone.
func (game *Game) handleAttireApply(client *connection.Client, req *protobuf.CS_11005) error {
	ctx := context.Background()
	id := req.GetId()
	var kind uint32
	var known bool
	switch req.GetType() {
	case attireTypeIconFrame:
		kind = dropTypeIconFrame
		_, known = iconFrames[id]
	case attireTypeChatFrame:
		kind = dropTypeChatFrame
		_, known = chatFrames[id]
	case attireTypeCombatUI:
		kind = dropTypeCombatUI
		_, known = combatUIs[id]
	}
	if !known {
		return client.Send(&protobuf.SC_11006{Result: result(errRefused)})
	}
	owned, err := ownsAttire(ctx, game.DB.Queries, client.CommanderID, kind, id)
	if err != nil {
		return err
	}
	if !owned {
		return client.Send(&protobuf.SC_11006{Result: result(errRefused)})
	}
	switch kind {
	case dropTypeIconFrame:
		err = game.DB.SetIconFrame(ctx, db.SetIconFrameParams{CommanderID: client.CommanderID, IconFrame: id})
	case dropTypeChatFrame:
		err = game.DB.SetChatFrame(ctx, db.SetChatFrameParams{CommanderID: client.CommanderID, ChatFrame: id})
	default:
		err = game.DB.SetBattleUI(ctx, db.SetBattleUIParams{CommanderID: client.CommanderID, BattleUi: id})
	}
	if err != nil {
		return err
	}
	return client.Send(&protobuf.SC_11006{Result: result(nil)})
}
