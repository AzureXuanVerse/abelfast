package island

import (
	"context"

	"github.com/jackc/pgx/v5"
	"google.golang.org/protobuf/proto"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/db"
	"github.com/ggmolly/belfast/internal/orm"
	"github.com/ggmolly/belfast/internal/protobuf"
)

func IslandBookCollectPoint(buffer *[]byte, client *connection.Client) (int, int, error) {
	var payload protobuf.CS_21345
	if err := proto.Unmarshal(*buffer, &payload); err != nil {
		return 0, 21346, err
	}

	response := &protobuf.SC_21346{Result: proto.Uint32(1), CollectList: []*protobuf.PB_BOOK_COLLECT{}}
	bookIDs := dedupeTaskIDs(payload.GetBookIds())
	if len(bookIDs) == 0 {
		return client.SendMessage(21346, response)
	}

	guideByID, err := loadIslandIllustratedGuideConfig()
	if err != nil {
		return client.SendMessage(21346, response)
	}

	err = db.DefaultStore.WithPGXTx(context.Background(), func(tx pgx.Tx) error {
		state, err := orm.GetIslandBookStateForUpdateTx(context.Background(), tx, client.Commander.CommanderID)
		if err != nil {
			return err
		}

		unlocked := make(map[uint32]struct{}, len(state.BookList))
		for _, id := range state.BookList {
			unlocked[id] = struct{}{}
		}

		collectByID := make(map[uint32]orm.IslandBookCollectEntry, len(state.BookCollects))
		for _, collect := range state.BookCollects {
			collectByID[collect.ID] = collect
		}

		pendingIDs := make([]uint32, 0, len(bookIDs))
		for _, bookID := range bookIDs {
			_, ok := guideByID[bookID]
			if !ok {
				return nil
			}
			if _, ok := unlocked[bookID]; !ok {
				return nil
			}
			if _, exists := collectByID[bookID]; exists {
				return nil
			}
			pendingIDs = append(pendingIDs, bookID)
		}

		updated := make([]orm.IslandBookCollectEntry, 0, len(pendingIDs))
		for _, bookID := range pendingIDs {
			cfg := guideByID[bookID]
			collect := orm.IslandBookCollectEntry{
				ID:       bookID,
				Base:     cfg.CollectAdd,
				LvList:   toIslandBookCollectLevels(cfg.CollectUpgrade),
				StarList: toIslandBookCollectLevels(cfg.CollectStar),
			}
			updated = append(updated, collect)
			collectByID[bookID] = collect
			state.BookCollects = append(state.BookCollects, collect)
		}

		if err := orm.SaveIslandBookStateTx(context.Background(), tx, state); err != nil {
			return err
		}

		response.Result = proto.Uint32(0)
		response.CollectList = buildIslandBookCollectProto(updated)
		return nil
	})
	if err != nil {
		return client.SendMessage(21346, response)
	}

	return client.SendMessage(21346, response)
}

func toIslandBookCollectLevels(values [][]uint32) []orm.IslandBookCollectLevel {
	out := make([]orm.IslandBookCollectLevel, 0, len(values))
	for _, value := range values {
		if len(value) < 2 {
			continue
		}
		out = append(out, orm.IslandBookCollectLevel{Lv: value[0], Value: value[1]})
	}
	return out
}
