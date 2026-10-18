package game

import (
	"context"
	"sync"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/gamedata"
	"github.com/ggmolly/belfast/internal/protobuf"
)

// ringConversion is gameset vow_prop_conversion: [ring, tiara], the Promise
// Ring (15006) and the Unity Tiara (15011) the collab ships take instead.
var ringConversion = sync.OnceValues(func() ([]uint32, error) {
	var ids []uint32
	err := gamedata.GamesetDescription("vow_prop_conversion", &ids)
	return ids, err
})

// handleExchangeRing answers ProposeExchangeRingCommand
// (controller/command/ship/proposeexchangeringcommand.lua, the propose
// screen's EXCHANGE_TIARA button): one ring for one tiara, id is always 0.
// On result 0 the client removes the ring and adds the tiara itself. No
// official capture has it: the reply is built from the Lua alone.
func (game *Game) handleExchangeRing(client *connection.Client, req *protobuf.CS_15010) error {
	ids, err := ringConversion()
	if err != nil {
		return err
	}
	err = errRefused
	if len(ids) == 2 {
		err = game.tx(context.Background(), client, func(tx *Tx) error {
			if err := tx.SpendItem(ids[0], 1); err != nil {
				return err
			}
			return tx.AddItem(ids[1], 1)
		})
	}
	if err != nil && !refused(err) {
		return err
	}
	return client.Send(&protobuf.SC_15011{Result: result(err)})
}
