package orm

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5"

	"github.com/ggmolly/belfast/internal/db"
)

func TestAddIslandSeasonPTTxAccumulatesPoints(t *testing.T) {
	initCommanderItemTestDB(t)
	clearTable(t, &IslandSeason{})
	clearTable(t, &Commander{})

	const commanderID = 9301
	if err := CreateCommanderRoot(commanderID, commanderID, "Island Season", 0, 0); err != nil {
		t.Fatalf("seed commander: %v", err)
	}

	err := db.DefaultStore.WithPGXTx(context.Background(), func(tx pgx.Tx) error {
		if err := AddIslandSeasonPTTx(context.Background(), tx, commanderID, 7); err != nil {
			return err
		}
		if err := AddIslandSeasonPTTx(context.Background(), tx, commanderID, 5); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		t.Fatalf("add island season pt: %v", err)
	}

	season, err := GetIslandSeason(commanderID)
	if err != nil {
		t.Fatalf("get season: %v", err)
	}
	if season.PT != 12 {
		t.Fatalf("expected pt 12, got %d", season.PT)
	}
}
