package game

import (
	"context"
	"errors"

	"github.com/ggmolly/belfast/internal/db"
)

// ErrUnlinkedTaken is UnlinkCommander's error when another commander is
// already unlinked.
var ErrUnlinkedTaken = errors.New("another commander is already unlinked (account 0)")

// UnlinkCommander detaches a commander from its account: commanders.account_id
// is unique and not null, it is moved to account 0, which no login hands out
// (the gateway's account ids are never 0), so the account gets a new
// commander at its next login. A commander playing is kicked first.
func (game *Game) UnlinkCommander(ctx context.Context, id uint32) (bool, error) {
	return game.EditCommanderRows(ctx, id, func(queries *db.Queries) error {
		taken, err := queries.AdminCountCommandersOfAccount(ctx, 0)
		if err != nil {
			return err
		}
		if taken != 0 {
			return ErrUnlinkedTaken
		}
		return queries.AdminSetCommanderAccount(ctx, db.AdminSetCommanderAccountParams{AccountID: 0, ID: id})
	})
}
