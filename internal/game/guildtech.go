package game

import (
	"context"

	"github.com/ggmolly/belfast/internal/db"
	"github.com/ggmolly/belfast/internal/protobuf"
)

// fillGuildTechnologies adds the guild's technologies to its
// GUILD_EXPANSION_INFO.
func (game *Game) fillGuildTechnologies(ctx context.Context, queries *db.Queries, guild db.Guild, expansion *protobuf.GUILD_EXPANSION_INFO) error {
	return nil
}

// guildTechnologiesCreated starts a new guild's default technology.
func (game *Game) guildTechnologiesCreated(tx *Tx, guildID uint32) error {
	return nil
}
