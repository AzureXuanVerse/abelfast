package game

import (
	"context"
	"time"
	"unicode/utf8"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/db"
	"github.com/ggmolly/belfast/internal/protobuf"
)

// ActivityConst.ACTIVITY_TYPE_STORE_DATE: an activity that only keeps what
// the client stores in it (CS_26160).
const activityTypeStoreData = 113

// maxStoredTextLength is the longest str_value kept: the longest name the
// client lets through (MonopolyCar2026SetNamePage, nameValidityCheck 0-20;
// LinerSignPage asks for 4-14).
const maxStoredTextLength = 20

// handleActivityStoreData answers ActivityStoreDataCommand
// (controller/command/activity/activitystoredatacommand.lua): a running
// ACTIVITY_TYPE_STORE_DATE activity keeps int_value in data1 and str_value
// in str_data1, which the client sets itself on result 0 and gets back in
// SC_11200. The client stores the name given to the car of the 2026
// monopoly event (MonopolyCar2026SetNamePage, the story's {car2026},
// activity 51114 linked from 51111) and the liner's name (LinerSignPage,
// LINER_NAMED_ID 5322). The length cap is a guess from the client's
// checks; no official capture has it.
func (game *Game) handleActivityStoreData(client *connection.Client, req *protobuf.CS_26160) error {
	ctx := context.Background()
	config, open, err := game.commanderActivity(ctx, client.CommanderID, req.GetActId(), time.Now())
	if err != nil {
		return err
	}
	text := req.GetStrValue()
	if !open || config.Type != activityTypeStoreData || !utf8.ValidString(text) || utf8.RuneCountInString(text) > maxStoredTextLength {
		return client.Send(&protobuf.SC_26161{Result: result(errRefused)})
	}
	err = game.tx(ctx, client, func(tx *Tx) error {
		state, err := activityState(ctx, tx.Queries, client.CommanderID, config)
		if err != nil {
			return err
		}
		state.Data1, state.StrData1 = req.GetIntValue(), text
		return tx.SaveActivity(ctx, db.SaveActivityParams(state))
	})
	if err != nil {
		return err
	}
	return client.Send(&protobuf.SC_26161{Result: result(nil)})
}
