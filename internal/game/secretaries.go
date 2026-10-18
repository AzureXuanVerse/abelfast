package game

import (
	"context"
	"database/sql"
	"errors"
	"sync"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/db"
	"github.com/ggmolly/belfast/internal/gamedata"
	"github.com/ggmolly/belfast/internal/protobuf"
)

// maxSecretaries is the most secretaries a commander can set: the last
// gameset secretary_group_unlock [chapter, count] entry
// (PlayerVitaeShipsPage.GetSlotMaxCnt, the slots unlock with chapter clears).
var maxSecretaries = sync.OnceValues(func() (int, error) {
	var unlocks [][2]int
	if err := gamedata.GamesetDescription("secretary_group_unlock", &unlocks); err != nil {
		return 0, err
	}
	if len(unlocks) == 0 {
		return 1, nil
	}
	return unlocks[len(unlocks)-1][1], nil
})

// handleChangeSecretaries replaces the commander's secretaries
// (ChangePlayerIconCommand, CS_11011): character is the new list in order,
// key the ship id and value its phantom (ShipPhantom.UnpackMark). On result 0
// the client updates its own player and flag ships; SC_11003 character
// sends them back at login.
func (game *Game) handleChangeSecretaries(client *connection.Client, req *protobuf.CS_11011) error {
	ctx := context.Background()
	limit, err := maxSecretaries()
	if err != nil {
		return err
	}
	characters := req.GetCharacter()
	err = game.tx(ctx, client, func(tx *Tx) error {
		if len(characters) == 0 || len(characters) > limit {
			return errRefused
		}
		seen := map[uint32]bool{}
		for _, character := range characters {
			// belfast keeps no ship phantoms: only the ship itself
			if seen[character.GetKey()] || character.GetValue() != 0 {
				return errRefused
			}
			seen[character.GetKey()] = true
			if _, err := tx.GetShip(ctx, db.GetShipParams{ID: character.GetKey(), CommanderID: client.CommanderID}); errors.Is(err, sql.ErrNoRows) {
				return errRefused
			} else if err != nil {
				return err
			}
		}
		if err := tx.DeleteSecretaries(ctx, client.CommanderID); err != nil {
			return err
		}
		for i, character := range characters {
			if err := tx.SetSecretary(ctx, db.SetSecretaryParams{CommanderID: client.CommanderID, Position: uint32(i + 1), ShipID: character.GetKey(), PhantomID: 0}); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil && !refused(err) {
		return err
	}
	return client.Send(&protobuf.SC_11012{Result: result(err)})
}
