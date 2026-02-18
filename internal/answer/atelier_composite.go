package answer

import (
	"context"
	"math"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/db"
	"github.com/ggmolly/belfast/internal/orm"
	"github.com/ggmolly/belfast/internal/protobuf"
	"github.com/jackc/pgx/v5"
	"google.golang.org/protobuf/proto"
)

func AtelierComposite(buffer *[]byte, client *connection.Client) (int, int, error) {
	var payload protobuf.CS_26053
	if err := proto.Unmarshal(*buffer, &payload); err != nil {
		return 0, 26054, err
	}

	response := &protobuf.SC_26054{Result: proto.Uint32(atelierResultMalformedRequest), AwardList: []*protobuf.DROPINFO{}}
	if err := ensureAtelierActivity(payload.GetActId()); err != nil {
		response.Result = proto.Uint32(atelierResultInvalidActivity)
		return client.SendMessage(26054, response)
	}
	recipe, err := parseAtelierRecipeConfig(payload.GetRecipeId())
	if err != nil {
		response.Result = proto.Uint32(atelierResultInvalidRecipeOrItem)
		return client.SendMessage(26054, response)
	}
	if payload.GetTimes() == 0 {
		return client.SendMessage(26054, response)
	}

	requestItems, ok := parseAtelierRequestItems(payload.GetItems(), payload.GetTimes())
	if !ok {
		return client.SendMessage(26054, response)
	}
	if len(requestItems) == 0 {
		return client.SendMessage(26054, response)
	}
	allowedItems, err := parseAtelierRecipeAllowedItems(recipe)
	if err != nil {
		response.Result = proto.Uint32(atelierResultInvalidRecipeOrItem)
		return client.SendMessage(26054, response)
	}
	if len(allowedItems) > 0 {
		for itemID := range requestItems {
			if _, exists := allowedItems[itemID]; !exists {
				response.Result = proto.Uint32(atelierResultInvalidRecipeOrItem)
				return client.SendMessage(26054, response)
			}
		}
	}

	rewards, ok := buildAtelierRecipeRewards(recipe, payload.GetTimes())
	if !ok {
		response.Result = proto.Uint32(atelierResultInvalidRecipeOrItem)
		return client.SendMessage(26054, response)
	}

	ctx := context.Background()
	err = db.DefaultStore.WithPGXTx(ctx, func(tx pgx.Tx) error {
		if err := orm.LockAtelierStateTx(ctx, tx, client.Commander.CommanderID, payload.GetActId()); err != nil {
			return err
		}
		state, err := orm.GetOrCreateAtelierStateTx(ctx, tx, client.Commander.CommanderID, payload.GetActId())
		if err != nil {
			return err
		}

		if recipe.ItemNum > 0 {
			if math.MaxUint32-state.RecipeUses[recipe.ID] < payload.GetTimes() {
				response.Result = proto.Uint32(atelierResultRecipeLimitReached)
				return nil
			}
			if state.RecipeUses[recipe.ID]+payload.GetTimes() > recipe.ItemNum {
				response.Result = proto.Uint32(atelierResultRecipeLimitReached)
				return nil
			}
		}

		for itemID, count := range requestItems {
			if state.Items[itemID] < count {
				response.Result = proto.Uint32(atelierResultInsufficientItems)
				return nil
			}
		}
		for itemID, count := range requestItems {
			state.Items[itemID] -= count
			if state.Items[itemID] == 0 {
				delete(state.Items, itemID)
			}
		}
		state.RecipeUses[recipe.ID] += payload.GetTimes()

		awardList, err := applyAtelierRewardsTx(ctx, tx, client, state, rewards)
		if err != nil {
			return err
		}
		response.AwardList = awardList

		if err := orm.SaveAtelierStateTx(ctx, tx, state); err != nil {
			return err
		}
		response.Result = proto.Uint32(atelierResultSuccess)
		return nil
	})
	if err != nil {
		response.Result = proto.Uint32(atelierResultStorageFailure)
		response.AwardList = []*protobuf.DROPINFO{}
	}
	return client.SendMessage(26054, response)
}

func parseAtelierRequestItems(items []*protobuf.KVDATA, times uint32) (map[uint32]uint32, bool) {
	if times == 0 {
		return nil, false
	}
	itemMap := make(map[uint32]uint32, len(items))
	for _, item := range items {
		key := item.GetKey()
		value := item.GetValue()
		if key == 0 || value == 0 {
			return nil, false
		}
		if math.MaxUint32/value < times {
			return nil, false
		}
		scaled := value * times
		if math.MaxUint32-itemMap[key] < scaled {
			return nil, false
		}
		itemMap[key] += scaled
	}
	return itemMap, true
}

func buildAtelierRecipeRewards(recipe *atelierRecipeConfig, times uint32) ([]atelierReward, bool) {
	if len(recipe.ItemID) < 2 || times == 0 {
		return nil, false
	}
	if math.MaxUint32 < times {
		return nil, false
	}
	count := times
	if count == 0 {
		return nil, false
	}
	return []atelierReward{{DropType: recipe.ItemID[0], DropID: recipe.ItemID[1], Count: count}}, true
}
