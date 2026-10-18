package game

import (
	"context"
	"database/sql"
	"errors"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/db"
	"github.com/ggmolly/belfast/internal/protobuf"
)

// The random secretary modes, SettingsRandomFlagShipAndSkinPanel
// SHIP_FREQUENTLYUSED, SHIP_LOCKED and COUSTOM (the ships flagged with
// CS_12208).
const (
	randomShipFrequentlyUsed = 1
	randomShipCustom         = 3
)

// handleRandomFlagship records the random secretary switch of the profile
// page (RandomFlagshipCommand, CS_12204): flag 1 on, 0 off. The client draws
// and keeps the ships itself (SettingsProxy random list in PlayerPrefs) and
// whatever the result only runs its callback; nothing sends the switch back.
func (game *Game) handleRandomFlagship(client *connection.Client, req *protobuf.CS_12204) error {
	var err error
	if req.GetFlag() > 1 {
		err = errRefused
	} else if err = game.DB.SetRandomFlagshipEnabled(context.Background(), db.SetRandomFlagshipEnabledParams{
		CommanderID: client.CommanderID,
		Enabled:     req.GetFlag() == 1,
	}); err != nil {
		return err
	}
	return client.Send(&protobuf.SC_12205{Result: result(err)})
}

// handleChangeRandomFlagshipMode sets which ships the random secretary draws
// from (ChangeRandomFlagShipModeCommand, CS_12206): flag is the mode. On
// result 0 the client updates its player (UpdateRandomFlagShipMode); SC_11003
// random_ship_mode sends it back at login.
func (game *Game) handleChangeRandomFlagshipMode(client *connection.Client, req *protobuf.CS_12206) error {
	var err error
	if mode := req.GetFlag(); mode < randomShipFrequentlyUsed || mode > randomShipCustom {
		err = errRefused
	} else if err = game.DB.SetRandomFlagshipMode(context.Background(), db.SetRandomFlagshipModeParams{
		CommanderID: client.CommanderID,
		Mode:        mode,
	}); err != nil {
		return err
	}
	return client.Send(&protobuf.SC_12207{Result: result(err)})
}

// handleChangeRandomFlagships adds ships to or removes them from the custom
// random pool (ChangeRandomFlagShipsCommand, CS_12208, sent in batches of 300):
// key is the ship id, value1 its phantom (ShipPhantom.UnpackMark) and value2
// whether it is now in the pool. On result 0 the client updates its ships
// (BayProxy.updateRandomFlagShips); SHIPINFO.char_random_flag sends them back.
func (game *Game) handleChangeRandomFlagships(client *connection.Client, req *protobuf.CS_12208) error {
	ctx := context.Background()
	err := game.tx(ctx, client, func(tx *Tx) error {
		seen := map[uint32]bool{}
		for _, ship := range req.GetShipShadowList() {
			// belfast keeps no ship phantoms: only the ship itself
			if seen[ship.GetKey()] || ship.GetValue1() != 0 || ship.GetValue2() > 1 {
				return errRefused
			}
			seen[ship.GetKey()] = true
			if _, err := tx.GetShip(ctx, db.GetShipParams{ID: ship.GetKey(), CommanderID: client.CommanderID}); errors.Is(err, sql.ErrNoRows) {
				return errRefused
			} else if err != nil {
				return err
			}
		}
		for _, ship := range req.GetShipShadowList() {
			if err := tx.SetShipRandomFlag(ctx, db.SetShipRandomFlagParams{
				RandomFlag:  ship.GetValue2() == 1,
				ID:          ship.GetKey(),
				CommanderID: client.CommanderID,
			}); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil && !refused(err) {
		return err
	}
	return client.Send(&protobuf.SC_12209{Result: result(err)})
}

// randomFlagshipMode is SC_11003 random_ship_mode: 0 until the commander picks
// one, the client then falls back to common flag RANDOM_FLAG_SHIP_MODE
// (Player.GetRandomFlagShipMode).
func randomFlagshipMode(ctx context.Context, queries *db.Queries, commanderID uint32) (uint32, error) {
	mode, err := queries.GetRandomFlagshipMode(ctx, commanderID)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, nil
	}
	return mode, err
}
