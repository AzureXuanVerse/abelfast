package answer

import (
	"context"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/db"
	"github.com/ggmolly/belfast/internal/orm"
	"github.com/ggmolly/belfast/internal/protobuf"
	"github.com/jackc/pgx/v5"
	"google.golang.org/protobuf/proto"
)

func AtelierRefreshBuff(buffer *[]byte, client *connection.Client) (int, int, error) {
	var payload protobuf.CS_26055
	if err := proto.Unmarshal(*buffer, &payload); err != nil {
		return 0, 26056, err
	}

	response := &protobuf.SC_26056{Result: proto.Uint32(atelierResultMalformedRequest)}
	if err := ensureAtelierActivity(payload.GetActId()); err != nil {
		response.Result = proto.Uint32(atelierResultInvalidActivity)
		return client.SendMessage(26056, response)
	}

	nextSlots, result := validateAtelierBuffSlots(payload.GetSlots())
	if result != atelierResultSuccess {
		response.Result = proto.Uint32(result)
		return client.SendMessage(26056, response)
	}

	ctx := context.Background()
	err := db.DefaultStore.WithPGXTx(ctx, func(tx pgx.Tx) error {
		if err := orm.LockAtelierStateTx(ctx, tx, client.Commander.CommanderID, payload.GetActId()); err != nil {
			return err
		}
		state, err := orm.GetOrCreateAtelierStateTx(ctx, tx, client.Commander.CommanderID, payload.GetActId())
		if err != nil {
			return err
		}
		if !atelierHasBuffSlotItems(state.Items, nextSlots) {
			response.Result = proto.Uint32(atelierResultInvalidRecipeOrItem)
			return nil
		}
		state.Slots = nextSlots
		if err := orm.SaveAtelierStateTx(ctx, tx, state); err != nil {
			return err
		}
		response.Result = proto.Uint32(atelierResultSuccess)
		return nil
	})
	if err != nil {
		response.Result = proto.Uint32(atelierResultStorageFailure)
	}
	return client.SendMessage(26056, response)
}

func validateAtelierBuffSlots(slots []*protobuf.BUFF_SLOT) (map[uint32]orm.AtelierBuffSlotState, uint32) {
	nextSlots := make(map[uint32]orm.AtelierBuffSlotState, 5)
	for pos := uint32(1); pos <= 5; pos++ {
		nextSlots[pos] = orm.AtelierBuffSlotState{Pos: pos}
	}
	seenPos := make(map[uint32]struct{}, len(slots))
	seenItem := make(map[uint32]struct{}, len(slots))
	for _, slot := range slots {
		pos := slot.GetPos()
		if pos < 1 || pos > 5 {
			return nil, atelierResultMalformedRequest
		}
		if _, exists := seenPos[pos]; exists {
			return nil, atelierResultMalformedRequest
		}
		seenPos[pos] = struct{}{}

		itemID := slot.GetItemid()
		itemNum := slot.GetItemnum()
		if itemID == 0 {
			if itemNum != 0 {
				return nil, atelierResultMalformedRequest
			}
			nextSlots[pos] = orm.AtelierBuffSlotState{Pos: pos}
			continue
		}
		if _, exists := seenItem[itemID]; exists {
			return nil, atelierResultMalformedRequest
		}
		itemConfig, err := parseAtelierItemConfig(itemID)
		if err != nil {
			return nil, atelierResultInvalidRecipeOrItem
		}
		tierCount := itemConfig.buffTierCount()
		if tierCount == 0 {
			return nil, atelierResultInvalidRecipeOrItem
		}
		if itemNum == 0 || itemNum > uint32(tierCount) {
			return nil, atelierResultMalformedRequest
		}
		seenItem[itemID] = struct{}{}
		nextSlots[pos] = orm.AtelierBuffSlotState{Pos: pos, ItemID: itemID, ItemNum: itemNum}
	}
	return nextSlots, atelierResultSuccess
}

func atelierHasBuffSlotItems(items map[uint32]uint32, slots map[uint32]orm.AtelierBuffSlotState) bool {
	for _, slot := range slots {
		if slot.ItemID == 0 {
			continue
		}
		if items[slot.ItemID] == 0 {
			return false
		}
	}
	return true
}
