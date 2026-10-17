package game

import (
	"context"
	"math"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/protobuf"
)

// itemCompose is how a bag item combines into another: number of it make one
// target (item_data_statistics compose_number / target_id, the T1/T2 tech
// packs, skill books, retrofit blueprints...).
type itemCompose struct {
	number, target uint32
}

var itemComposes map[uint32]itemCompose

// readItemCompose keeps an item_data_statistics row's combining, if it has
// one that makes another bag item.
func readItemCompose(id uint32, row itemUseConfig) {
	if row.ComposeNumber > 0 && row.TargetID != 0 {
		itemComposes[id] = itemCompose{row.ComposeNumber, row.TargetID}
	}
}

// handleComposeItem combines bag items (ComposeItemCommand, CS_15006): num
// target_id for num * compose_number of id, no other cost. On result 0 the
// client removes the items and adds the targets itself. No official capture
// has it: built from the Lua alone.
func (game *Game) handleComposeItem(client *connection.Client, req *protobuf.CS_15006) error {
	ctx := context.Background()
	compose, ok := itemComposes[req.GetId()]
	_, target := itemUses[compose.target]
	num := req.GetNum()
	cost := uint64(compose.number) * uint64(num)
	err := errRefused
	if ok && target && num > 0 && cost <= math.MaxUint32 {
		err = game.tx(ctx, client, func(tx *Tx) error {
			if err := tx.SpendItem(req.GetId(), uint32(cost)); err != nil {
				return err
			}
			return tx.AddItem(compose.target, num)
		})
	}
	if err != nil && !refused(err) {
		return err
	}
	return client.Send(&protobuf.SC_15007{Result: result(err)})
}
