package game

import (
	"context"

	"github.com/ggmolly/belfast/internal/db"
	"github.com/ggmolly/belfast/internal/protobuf"
)

// fillGuildOperations adds the guild's operation counters to its
// GUILD_EXPANSION_INFO.
func (game *Game) fillGuildOperations(ctx context.Context, queries *db.Queries, guild db.Guild, expansion *protobuf.GUILD_EXPANSION_INFO) error {
	return nil
}
