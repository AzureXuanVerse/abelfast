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

func IslandBookUnlock(buffer *[]byte, client *connection.Client) (int, int, error) {
	var payload protobuf.CS_21343
	if err := proto.Unmarshal(*buffer, &payload); err != nil {
		return 0, 21344, err
	}

	response := &protobuf.SC_21344{Result: proto.Uint32(1), DropList: []*protobuf.DROPINFO{}}
	if err := ensureCommanderLoaded(client, "Island/BookUnlock"); err != nil {
		return client.SendMessage(21344, response)
	}

	bookIDs := dedupeTaskIDs(payload.GetBookIds())
	if len(bookIDs) == 0 {
		return client.SendMessage(21344, response)
	}

	guideByID, err := loadIslandIllustratedGuideConfig()
	if err != nil {
		return client.SendMessage(21344, response)
	}

	err = db.DefaultStore.WithPGXTx(context.Background(), func(tx pgx.Tx) error {
		state, err := orm.GetIslandBookStateForUpdateTx(context.Background(), tx, client.Commander.CommanderID)
		if err != nil {
			return err
		}

		unlockedSet := make(map[uint32]struct{}, len(state.BookList))
		for _, unlocked := range state.BookList {
			unlockedSet[unlocked] = struct{}{}
		}

		pendingUnlocks := make([]uint32, 0, len(bookIDs))
		for _, bookID := range bookIDs {
			_, ok := guideByID[bookID]
			if !ok {
				return nil
			}
			if _, exists := unlockedSet[bookID]; exists {
				return nil
			}
			pendingUnlocks = append(pendingUnlocks, bookID)
		}

		pendingDrops := make([]*protobuf.DROPINFO, 0)
		for _, bookID := range pendingUnlocks {
			cfg := guideByID[bookID]
			if len(cfg.AwardUnlock) > 0 {
				drops, dropErr := buildAwardDrops(cfg.AwardUnlock)
				if dropErr != nil {
					return nil
				}
				pendingDrops = append(pendingDrops, drops...)
			}
			state.BookList = append(state.BookList, bookID)
		}

		if err := applyIslandDropsTx(context.Background(), tx, client, pendingDrops); err != nil {
			return err
		}
		if err := orm.SaveIslandBookStateTx(context.Background(), tx, state); err != nil {
			return err
		}

		response.Result = proto.Uint32(0)
		response.DropList = mergeDropList(pendingDrops)
		return nil
	})
	if err != nil {
		return client.SendMessage(21344, response)
	}

	return client.SendMessage(21344, response)
}
