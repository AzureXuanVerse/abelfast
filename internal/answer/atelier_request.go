package answer

import (
	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/orm"
	"github.com/ggmolly/belfast/internal/protobuf"
	"google.golang.org/protobuf/proto"
)

func AtelierRequest(buffer *[]byte, client *connection.Client) (int, int, error) {
	var payload protobuf.CS_26051
	if err := proto.Unmarshal(*buffer, &payload); err != nil {
		return 0, 26052, err
	}

	result := atelierResultSuccess
	state := &orm.AtelierState{Items: map[uint32]uint32{}, RecipeUses: map[uint32]uint32{}, Slots: map[uint32]orm.AtelierBuffSlotState{}}
	if err := ensureAtelierActivity(payload.GetActId()); err != nil {
		result = atelierResultInvalidActivity
	} else {
		loadedState, err := orm.GetOrCreateAtelierState(client.Commander.CommanderID, payload.GetActId())
		if err != nil {
			result = atelierResultStorageFailure
		} else {
			state = loadedState
		}
	}

	response := protobuf.SC_26052{
		Result:  proto.Uint32(result),
		Items:   sortedAtelierKVDATA(state.Items),
		Recipes: sortedAtelierKVDATA(state.RecipeUses),
		Slots:   sortedAtelierSlots(state.Slots),
	}
	return client.SendMessage(26052, &response)
}
