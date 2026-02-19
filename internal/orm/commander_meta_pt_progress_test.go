package orm

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5"
)

func TestCommanderMetaPtProgressCRUD(t *testing.T) {
	initCommanderItemTestDB(t)
	clearTable(t, &CommanderMetaPtProgress{})

	created, err := GetOrCreateCommanderMetaPtProgress(7001, 970108)
	if err != nil {
		t.Fatalf("get or create meta pt progress: %v", err)
	}
	if created.CommanderID != 7001 || created.GroupID != 970108 {
		t.Fatalf("unexpected created state: %+v", created)
	}
	if created.Pt != 0 || len(created.FetchList) != 0 {
		t.Fatalf("expected zeroed progress, got %+v", created)
	}

	created.Pt = 300
	created.FetchList = []uint32{100, 300}
	if err := SaveCommanderMetaPtProgress(created); err != nil {
		t.Fatalf("save meta pt progress: %v", err)
	}

	loaded, err := GetCommanderMetaPtProgress(7001, 970108)
	if err != nil {
		t.Fatalf("load meta pt progress: %v", err)
	}
	if loaded.Pt != 300 {
		t.Fatalf("expected pt=300, got %d", loaded.Pt)
	}
	if len(loaded.FetchList) != 2 || loaded.FetchList[0] != 100 || loaded.FetchList[1] != 300 {
		t.Fatalf("unexpected fetch list: %+v", loaded.FetchList)
	}
}

func TestCommanderMetaPtProgressTxLockAndList(t *testing.T) {
	initCommanderItemTestDB(t)
	clearTable(t, &CommanderMetaPtProgress{})

	err := WithPGXTx(context.Background(), func(tx pgx.Tx) error {
		state, txErr := GetOrCreateCommanderMetaPtProgressTx(context.Background(), tx, 7002, 970109)
		if txErr != nil {
			return txErr
		}
		state.Pt = 500
		state.FetchList = []uint32{100}
		return SaveCommanderMetaPtProgressTx(context.Background(), tx, state)
	})
	if err != nil {
		t.Fatalf("transactional save failed: %v", err)
	}

	state, err := GetCommanderMetaPtProgress(7002, 970109)
	if err != nil {
		t.Fatalf("load saved state failed: %v", err)
	}
	if state.Pt != 500 || len(state.FetchList) != 1 || state.FetchList[0] != 100 {
		t.Fatalf("unexpected saved state: %+v", state)
	}

	list, err := ListCommanderMetaPtProgress(7002)
	if err != nil {
		t.Fatalf("list progress failed: %v", err)
	}
	if len(list) != 1 || list[0].GroupID != 970109 {
		t.Fatalf("unexpected list result: %+v", list)
	}
}
