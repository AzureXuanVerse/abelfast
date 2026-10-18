package game

import (
	"context"
	"slices"
	"sync"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/gamedata"
	"github.com/ggmolly/belfast/internal/protobuf"
)

// generalBlueprints are the general blueprints (42000 General Blueprint -
// Series 1...) the Dev. Dock's quick exchange offers: gameset
// general_blueprint_list (BlueprintQuickExchangeView.UpdateBlueprint,
// view/technology/blueprintquickexchangeview.lua). They are
// usage_drop_appointed selection boxes of ship blueprints.
var generalBlueprints = sync.OnceValues(func() ([]uint32, error) {
	var ids []uint32
	err := gamedata.GamesetDescription("general_blueprint_list", &ids)
	return ids, err
})

// handleQuickExchangeBlueprint answers QuickExchangeBlueprintCommand
// (controller/command/item/quickexchangeblueprintcommand.lua, the Dev.
// Dock's exchange button): use_list opens general blueprints, each entry
// count of one with arg its picked usage_arg entry [2, blueprint, 1], as
// CS_15002 does. ret_list has one result per entry, in order; on result 0
// the client removes that entry's items and adds its drop_list
// (PlayerConst.addTranDrop). Each entry is its own transaction, so one
// turned down leaves the others. The view caps the count at what the ship's
// research still needs (getUseageMaxItem); belfast keeps no research
// progress, so that cap is the client's alone. No official capture has it:
// the reply is built from the Lua alone.
func (game *Game) handleQuickExchangeBlueprint(client *connection.Client, req *protobuf.CS_15012) error {
	ctx := context.Background()
	general, err := generalBlueprints()
	if err != nil {
		return err
	}
	rets := make([]*protobuf.SC_15003, 0, len(req.GetUseList()))
	for _, use := range req.GetUseList() {
		var drops []*protobuf.DROPINFO
		err := errRefused
		if slices.Contains(general, use.GetId()) {
			err = game.tx(ctx, client, func(tx *Tx) (err error) {
				drops, err = tx.useItem(use.GetId(), use.GetCount(), use.GetArg())
				return err
			})
		}
		if err != nil && !refused(err) {
			return err
		}
		if err != nil {
			drops = nil
		}
		rets = append(rets, &protobuf.SC_15003{Result: result(err), DropList: drops})
	}
	return client.Send(&protobuf.SC_15013{RetList: rets})
}
