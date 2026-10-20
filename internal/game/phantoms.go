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

// technology_shadow_unlock types, what a phantom quest's progress is
// (ShipBluePrint:getPhantomQuestProgress, model/vo/shipblueprint.lua)
const (
	phantomQuestShipLevel      = 1 // the ship's level
	phantomQuestBlueprintLevel = 2 // the research's level, its fate level once maxed
	phantomQuestVictories      = 3 // the ship's victories, counted by the server (CS_12212)
	phantomQuestOath           = 4 // 1 once the ship is oathed
	phantomQuestGems           = 5 // the gems owned, target_num of them is the cost
)

// technology_shadow_unlock row: the quest that unlocks the skin shadow of
// the same id on a research ship.
type phantomQuestConfig struct {
	Type      uint32 `json:"type"`
	TargetNum uint32 `json:"target_num"`
}

var (
	phantomQuests map[uint32]phantomQuestConfig
	// gameset technology_shadow_unlock_lv: the ship level the phantom
	// quests open at (ShipBluePrint:isUnlockShipPhantom)
	phantomUnlockLevel uint32
)

func loadPhantomQuestData() error {
	quests, err := gamedata.Table[phantomQuestConfig]("technology_shadow_unlock")
	if err != nil {
		return err
	}
	gameset, err := gamedata.Gameset()
	if err != nil {
		return err
	}
	level, ok := gameset["technology_shadow_unlock_lv"]
	if !ok {
		return gamedata.Missing("gameset", "technology_shadow_unlock_lv")
	}
	phantomQuests, phantomUnlockLevel = quests, level
	return nil
}

// handleGetPhantomQuestProgress answers GetPhantomQuestProgressCommand
// (controller/command/technology/getphantomquestprogresscommand.lua), sent
// with the fetched research ships when the research ship page opens:
// ship_count_list is, per ship id, the progress of the type 3 quest
// ("obtain 200 victories with the selected shipgirl"), the one counter only
// the server keeps (TechnologyProxy.updatePhantomQuestProgress). belfast
// doesn't count a ship's victories, so it honestly answers 0 for each of the
// commander's research ships asked for. No official capture has it: the
// reply is built from the Lua alone.
func (game *Game) handleGetPhantomQuestProgress(client *connection.Client, req *protobuf.CS_12212) error {
	ships, err := shipsByID(context.Background(), game.DB.Queries, client.CommanderID, req.GetShipIdList())
	if err != nil {
		return err
	}
	reply := &protobuf.SC_12213{}
	for _, id := range req.GetShipIdList() {
		ship, ok := ships[id]
		if !ok || !ownableShips[ship.TemplateID].blueprint {
			continue
		}
		reply.ShipCountList = append(reply.ShipCountList, &protobuf.KVDATA{Key: proto.Uint32(id), Value: proto.Uint32(0)})
		delete(ships, id) // once per ship
	}
	return client.Send(reply)
}

// handleFinishPhantomQuest answers FinishPhantomQuestCommand
// (controller/command/technology/finishphantomquestcommand.lua): finishing
// phantom quest skin_shadow_id of a research ship unlocks its skin shadow of
// that id. On result 0 the client takes the cost (gems for type 5,
// ShipBluePrint.getPhantomQuestCostDrop) and gives the ship the shadow,
// wearing its default skin (BayProxy.updateShipSkin); SHIPINFO
// skin_shadow_list sends it back. It is refused unless the ship is the
// commander's research ship at technology_shadow_unlock_lv, the shadow isn't
// unlocked yet and the quest's target is reached. belfast keeps no research
// progress nor victory counts: the type 2 and 3 quests can't be verified and
// are refused. No official capture has it: the reply is built from the Lua
// alone.
func (game *Game) handleFinishPhantomQuest(client *connection.Client, req *protobuf.CS_12210) error {
	ctx := context.Background()
	err := game.tx(ctx, client, func(tx *Tx) error {
		quest, ok := phantomQuests[req.GetSkinShadowId()]
		if !ok {
			return errRefused
		}
		ship, err := tx.GetShip(ctx, db.GetShipParams{ID: req.GetShipId(), CommanderID: client.CommanderID})
		if errors.Is(err, sql.ErrNoRows) {
			return errRefused
		} else if err != nil {
			return err
		}
		if !ownableShips[ship.TemplateID].blueprint || ship.Level < phantomUnlockLevel {
			return errRefused
		}
		switch quest.Type {
		case phantomQuestShipLevel:
			if ship.Level < quest.TargetNum {
				return errRefused
			}
		case phantomQuestOath:
			if ship.Propose == 0 && quest.TargetNum > 0 {
				return errRefused
			}
		case phantomQuestGems:
			// Player:consume, free gems first
			if err := tx.SpendShopResource(ResourceGem, quest.TargetNum); err != nil {
				return err
			}
		default:
			return errRefused
		}
		added, err := tx.AddShipSkinShadow(ctx, db.AddShipSkinShadowParams{ShipID: ship.ID, ShadowID: req.GetSkinShadowId()})
		if err != nil {
			return err
		}
		if added == 0 {
			return errRefused // already unlocked
		}
		return nil
	})
	if err != nil && !refused(err) {
		return err
	}
	return client.Send(&protobuf.SC_12211{Result: result(err)})
}

// skinShadowList is SHIPINFO.skin_shadow_list of a ship's unlocked shadows:
// key the shadow id, value the skin it wears, 0 for the default one
// (Ship:updateSkinId).
func skinShadowList(shadows []db.ShipSkinShadow) []*protobuf.KVDATA {
	list := make([]*protobuf.KVDATA, 0, len(shadows))
	for _, shadow := range shadows {
		list = append(list, &protobuf.KVDATA{Key: proto.Uint32(shadow.ShadowID), Value: proto.Uint32(shadow.SkinID)})
	}
	return list
}

// hasSkinShadow is whether shadow is 0 (the ship itself) or one of the
// ship's unlocked skin shadows.
func hasSkinShadow(ctx context.Context, queries *db.Queries, shipID, shadow uint32) (bool, error) {
	if shadow == 0 {
		return true, nil
	}
	return queries.HasShipSkinShadow(ctx, db.HasShipSkinShadowParams{ShipID: shipID, ShadowID: shadow})
}
