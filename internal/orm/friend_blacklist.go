package orm

import (
	"context"

	"github.com/jackc/pgx/v5"

	"github.com/ggmolly/belfast/internal/db"
)

func GetFriendBlacklist(commanderID uint32) ([]uint32, error) {
	state, err := GetOrCreateCommanderIslandSocialState(commanderID)
	if err != nil {
		return nil, err
	}

	return append([]uint32(nil), dedupeUint32(state.BlackList)...), nil
}

func AddFriendBlacklist(commanderID uint32, targetID uint32) (bool, error) {
	ctx := context.Background()
	added := false
	err := db.DefaultStore.WithPGXTx(ctx, func(tx pgx.Tx) error {
		state, err := GetCommanderIslandSocialStateForUpdateTx(ctx, tx, commanderID)
		if err != nil {
			return err
		}

		for _, blacklistedID := range state.BlackList {
			if blacklistedID == targetID {
				return nil
			}
		}

		state.BlackList = append(dedupeUint32(state.BlackList), targetID)
		if err := SaveCommanderIslandSocialStateTx(ctx, tx, state); err != nil {
			return err
		}
		added = true
		return nil
	})

	if err != nil {
		return false, err
	}

	return added, nil
}

func RemoveFriendBlacklist(commanderID uint32, targetID uint32) (bool, error) {
	ctx := context.Background()
	removed := false
	err := db.DefaultStore.WithPGXTx(ctx, func(tx pgx.Tx) error {
		state, err := GetCommanderIslandSocialStateForUpdateTx(ctx, tx, commanderID)
		if err != nil {
			return err
		}

		next := make([]uint32, 0, len(state.BlackList))
		for _, blacklistedID := range state.BlackList {
			if blacklistedID == targetID {
				removed = true
				continue
			}
			next = append(next, blacklistedID)
		}

		if !removed {
			return nil
		}

		state.BlackList = dedupeUint32(next)
		return SaveCommanderIslandSocialStateTx(ctx, tx, state)
	})

	if err != nil {
		return false, err
	}

	return removed, nil
}

func dedupeUint32(values []uint32) []uint32 {
	if len(values) < 2 {
		return append([]uint32(nil), values...)
	}

	seen := make(map[uint32]struct{}, len(values))
	deduped := make([]uint32, 0, len(values))
	for _, value := range values {
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		deduped = append(deduped, value)
	}

	return deduped
}
