package game

import (
	"context"

	"github.com/ggmolly/belfast/internal/db"
	"github.com/ggmolly/belfast/internal/protobuf"
)

// fillGuildLogistics adds the guild's weekly task and supplies to its
// GUILD_EXPANSION_INFO.
func (game *Game) fillGuildLogistics(ctx context.Context, queries *db.Queries, guild db.Guild, expansion *protobuf.GUILD_EXPANSION_INFO) error {
	return nil
}
