package answer

import (
	"context"
	"encoding/json"

	"github.com/jackc/pgx/v5"
	"google.golang.org/protobuf/proto"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/orm"
	"github.com/ggmolly/belfast/internal/protobuf"
)

type islandIllustratedGuideEntry struct {
	Type     uint32 `json:"type"`
	UnlockID uint32 `json:"unlock_id"`
}

func IslandUpdateIllustration(buffer *[]byte, client *connection.Client) (int, int, error) {
	var payload protobuf.CS_21340
	if err := proto.Unmarshal(*buffer, &payload); err != nil {
		return 0, 21341, err
	}

	response := &protobuf.SC_21341{Result: proto.Uint32(1)}
	unlockByType, err := loadIslandIllustratedGuideUnlockSet()
	if err != nil {
		return client.SendMessage(21341, response)
	}
	unlockSet := unlockByType[payload.GetType()]
	if len(unlockSet) == 0 {
		return client.SendMessage(21341, response)
	}
	if _, ok := unlockSet[payload.GetCondId()]; !ok {
		return client.SendMessage(21341, response)
	}

	err = orm.WithPGXTx(context.Background(), func(tx pgx.Tx) error {
		exists, err := orm.IslandBookCondExistsTx(context.Background(), tx, client.Commander.CommanderID, payload.GetType(), payload.GetCondId())
		if err != nil {
			return err
		}
		if exists {
			return nil
		}
		if err := orm.AddIslandBookCondTx(context.Background(), tx, client.Commander.CommanderID, payload.GetType(), payload.GetCondId()); err != nil {
			return err
		}
		response.Result = proto.Uint32(0)
		return nil
	})
	if err != nil {
		response.Result = proto.Uint32(1)
	}

	return client.SendMessage(21341, response)
}

func loadIslandIllustratedGuideUnlockSet() (map[uint32]map[uint32]struct{}, error) {
	entries, err := listConfigEntriesWithFallback(islandIllustratedGuideCategory, islandIllustratedGuideCategoryLC, orm.ListConfigEntries)
	if err != nil {
		return nil, err
	}

	result := make(map[uint32]map[uint32]struct{})
	for i := range entries {
		rows, err := parseIslandIllustratedGuideEntries(entries[i].Data)
		if err != nil {
			return nil, err
		}
		for _, row := range rows {
			if row.Type == 0 || row.UnlockID == 0 {
				continue
			}
			if result[row.Type] == nil {
				result[row.Type] = make(map[uint32]struct{})
			}
			result[row.Type][row.UnlockID] = struct{}{}
		}
	}
	return result, nil
}

func parseIslandIllustratedGuideEntries(raw json.RawMessage) ([]islandIllustratedGuideEntry, error) {
	rows := make([]islandIllustratedGuideEntry, 0)

	var single islandIllustratedGuideEntry
	if err := json.Unmarshal(raw, &single); err == nil && (single.Type != 0 || single.UnlockID != 0) {
		return []islandIllustratedGuideEntry{single}, nil
	}

	if err := json.Unmarshal(raw, &rows); err == nil {
		return rows, nil
	}

	var wrapper map[string]json.RawMessage
	if err := json.Unmarshal(raw, &wrapper); err != nil {
		return nil, err
	}
	for _, entryRaw := range wrapper {
		var row islandIllustratedGuideEntry
		if err := json.Unmarshal(entryRaw, &row); err == nil {
			rows = append(rows, row)
		}
	}
	return rows, nil
}
