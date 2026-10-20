package game

import (
	"context"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/db"
	"github.com/ggmolly/belfast/internal/protobuf"
)

// recordChapterDrops keeps the ships among a chapter's drops
// (Chapter:writeDrops, after a battle and an Operational Handover): the
// ships CS_13109 reports.
func (tx *Tx) recordChapterDrops(chapterID uint32, drops []*protobuf.DROPINFO) error {
	for _, drop := range drops {
		if drop.GetType() != dropTypeShip {
			continue
		}
		err := tx.AddChapterDropShip(tx.ctx, db.AddChapterDropShipParams{CommanderID: tx.commanderID, ChapterID: chapterID, ShipID: drop.GetId()})
		if err != nil {
			return err
		}
	}
	return nil
}

// handleChapterDropShips answers GetChapterDropShipListCommand (CS_13109),
// sent once a session per chapter when its drop list is opened: the ships
// the chapter has dropped, which the client adds to the ones it saw drop
// itself (Chapter:UpdateDropShipList); the display_icon ships missing from
// it show as unknown. Nothing was captured: built from the Lua alone.
func (game *Game) handleChapterDropShips(client *connection.Client, req *protobuf.CS_13109) error {
	ships, err := game.DB.ListChapterDropShips(context.Background(), db.ListChapterDropShipsParams{CommanderID: client.CommanderID, ChapterID: req.GetId()})
	if err != nil {
		return err
	}
	return client.Send(&protobuf.SC_13110{DropShipList: ships})
}
