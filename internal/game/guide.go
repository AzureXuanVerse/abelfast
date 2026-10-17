package game

import (
	"context"

	"google.golang.org/protobuf/proto"

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
	return client.Send(&protobuf.SC_11018{Result: proto.Uint32(0)})
}
