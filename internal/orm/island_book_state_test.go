package orm

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5"
)

func TestIslandBookCondAddExistsAndList(t *testing.T) {
	initCommanderItemTestDB(t)
	clearTable(t, &IslandBookCond{})
	clearTable(t, &Commander{})

	if err := CreateCommanderRoot(9801, 9801, "Book Cond Tester", 0, 0); err != nil {
		t.Fatalf("create commander: %v", err)
	}

	err := WithPGXTx(context.Background(), func(tx pgx.Tx) error {
		if err := AddIslandBookCondTx(context.Background(), tx, 9801, 2, 100100); err != nil {
			return err
		}
		if err := AddIslandBookCondTx(context.Background(), tx, 9801, 2, 100100); err != nil {
			return err
		}
		exists, err := IslandBookCondExistsTx(context.Background(), tx, 9801, 2, 100100)
		if err != nil {
			return err
		}
		if !exists {
			t.Fatalf("expected condition to exist")
		}
		return AddIslandBookCondTx(context.Background(), tx, 9801, 1, 10517)
	})
	if err != nil {
		t.Fatalf("upsert/list setup failed: %v", err)
	}

	conds, err := ListIslandBookConds(9801)
	if err != nil {
		t.Fatalf("list island book conds: %v", err)
	}
	if len(conds) != 2 {
		t.Fatalf("expected two unique conds, got %+v", conds)
	}
	if conds[0].Type != 1 || conds[0].UnlockID != 10517 {
		t.Fatalf("expected sorted first cond, got %+v", conds[0])
	}
}
