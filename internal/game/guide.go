package game

import (
	"context"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/db"
	"github.com/ggmolly/belfast/internal/protobuf"
)

// handleGuideIndex records tutorial progress (UpdateGuideIndexCommand); type 1
// is the new guide (SG002). The client expects no reply.
func (game *Game) handleGuideIndex(client *connection.Client, req *protobuf.CS_11016) error {
	if req.GetType() == 1 {
		return game.DB.SetNewGuideIndex(context.Background(), db.SetNewGuideIndexParams{NewGuideIndex: req.GetGuideIndex(), ID: client.CommanderID})
	}
	return game.DB.SetGuideIndex(context.Background(), db.SetGuideIndexParams{GuideIndex: req.GetGuideIndex(), ID: client.CommanderID})
}

// handleStoryRead marks a story as seen (UpdateStoryCommand) so it doesn't
// play again, it comes back in SC_11003.story_list.
func (game *Game) handleStoryRead(client *connection.Client, req *protobuf.CS_11017) error {
	if err := game.DB.AddStory(context.Background(), db.AddStoryParams{CommanderID: client.CommanderID, StoryID: req.GetStoryId()}); err != nil {
		return err
	}
	return client.Send(&protobuf.SC_11018{Result: result(nil)})
}

// handleCommonFlag records a one-time client flag (UpdateCommonFlagCommand),
// e.g. GAME_RESTOREVIEW_ALREADY once the first Elite or better ship asked for
// a store review; it comes back in SC_11003.flag_list.
func (game *Game) handleCommonFlag(client *connection.Client, req *protobuf.CS_11019) error {
	if err := game.DB.AddCommonFlag(context.Background(), db.AddCommonFlagParams{CommanderID: client.CommanderID, FlagID: req.GetFlagId()}); err != nil {
		return err
	}
	return client.Send(&protobuf.SC_11020{Result: result(nil)})
}
