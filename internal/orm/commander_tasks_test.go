package orm

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
)

func TestCommanderTaskProgressUpsertAndSubmit(t *testing.T) {
	initCommanderItemTestDB(t)
	clearTable(t, &CommanderTask{})
	clearTable(t, &Commander{})

	commanderID := uint32(88001)
	if err := CreateCommanderRoot(commanderID, commanderID, fmt.Sprintf("TaskTester%d", commanderID), 0, 0); err != nil {
		t.Fatalf("create commander: %v", err)
	}

	err := WithPGXTx(context.Background(), func(tx pgx.Tx) error {
		now := uint32(time.Now().Unix())
		if err := UpsertCommanderTaskProgressTx(context.Background(), tx, commanderID, 1001, TaskProgressUpdate, 5, 10, now); err != nil {
			return err
		}
		return UpsertCommanderTaskProgressTx(context.Background(), tx, commanderID, 1001, TaskProgressAppend, 3, 10, now)
	})
	if err != nil {
		t.Fatalf("upsert progress: %v", err)
	}

	err = WithPGXTx(context.Background(), func(tx pgx.Tx) error {
		task, err := GetCommanderTaskTx(context.Background(), tx, commanderID, 1001)
		if err != nil {
			return err
		}
		if task.Progress != 8 {
			return fmt.Errorf("expected progress 8, got %d", task.Progress)
		}
		submitted, err := MarkCommanderTaskSubmittedTx(context.Background(), tx, commanderID, 1001, uint32(time.Now().Unix()))
		if err != nil {
			return err
		}
		if !submitted {
			return fmt.Errorf("expected task submission")
		}
		return nil
	})
	if err != nil {
		t.Fatalf("verify/submit: %v", err)
	}
}
