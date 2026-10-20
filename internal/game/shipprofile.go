package game

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"
	"unicode/utf8"

	"google.golang.org/protobuf/proto"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/db"
	"github.com/ggmolly/belfast/internal/protobuf"
)

const (
	// Nation.IDOL_LINK: those ships take the Unity Tiara instead of the
	// Promise Ring (Ship:getProposeType "imas", ITEM_ID_FOR_PROPOSE_IMAS)
	nationIdolLink = 107
	// shipRenameCooldown is how long an oathed ship keeps its name
	// (ShipMainScene: renameTime + 2592000, word_rename_time_warning)
	shipRenameCooldown = 30 * 24 * 60 * 60
	// maxShipNameLength is RenameShipCommand's nameValidityCheck bound
	maxShipNameLength = 40
	// FleetRenameCommand's nameValidityCheck bounds
	minFleetNameLength = 2
	maxFleetNameLength = 24
	// Ship.PREFERENCE_TAG_NONE / PREFERENCE_TAG_COMMON, SHIPINFO.common_flag
	preferenceTagCommon = 1
	// ship_renameShip_error_4 "You can't do this at the current time!"
	resultTooSoon = 4
)

// validLabel is what belfast checks of a ship or fleet name the client
// let through nameValidityCheck (a native helper, not in the Lua): valid
// UTF-8 of minLength to maxLength characters, not blank. Unlike commander
// names they may hold spaces (the default "Fleet 1", "Prinz Eugen").
func validLabel(name string, minLength, maxLength int) bool {
	n := utf8.RuneCountInString(name)
	return utf8.ValidString(name) && n >= minLength && n <= maxLength && strings.TrimSpace(name) != ""
}

// commanderShip is one of the commander's ships, errRefused when it isn't
// theirs.
func (tx *Tx) commanderShip(id uint32) (db.Ship, error) {
	ship, err := tx.GetShip(tx.ctx, db.GetShipParams{ID: id, CommanderID: tx.commanderID})
	if errors.Is(err, sql.ErrNoRows) {
		return db.Ship{}, errRefused
	}
	return ship, err
}

// handlePropose answers ProposeCommand and ProposeRegiesterShipCommand
// (CS_12032, controller/command/ship/propose*.lua, the oath screen). A ship
// at 100 affinity not yet oathed takes the ring of its kind (Promise Ring,
// or the Unity Tiara for an IDOL_LINK ship, gameset vow_prop_conversion)
// and is oathed now, and locked: on result 0 the client removes the ring,
// sets propose with time as Ship.proposeTime and locks the ship. An oathed
// ship is only registered again (ProposeRegiesterShipCommand). Either way it
// becomes the commander's oath ship (Player.proposeShipId, SC_11003
// marry_ship). The archive entry gets its marry flag. No official capture
// has it: built from the Lua alone.
func (game *Game) handlePropose(client *connection.Client, req *protobuf.CS_12032) error {
	rings, err := ringConversion()
	if err != nil {
		return err
	}
	var proposed uint32
	err = game.tx(context.Background(), client, func(tx *Tx) error {
		ship, err := tx.commanderShip(req.GetShipId())
		if err != nil {
			return err
		}
		if ship.Propose == 0 {
			if ship.Intimacy < maxIntimacy || len(rings) != 2 {
				return errRefused
			}
			ring := rings[0]
			if ownableShips[ship.TemplateID].Nationality == nationIdolLink {
				ring = rings[1]
			}
			if err := tx.SpendItem(ring, 1); err != nil {
				return err
			}
			ship.Propose = uint32(time.Now().Unix())
			ship.IsLocked = true
			if err := tx.ProposeShip(tx.ctx, db.ProposeShipParams{Propose: ship.Propose, ID: ship.ID, CommanderID: tx.commanderID}); err != nil {
				return err
			}
			if raised, err := raiseShipGroup(tx.ctx, tx.Queries, ship); err != nil {
				return err
			} else if raised {
				tx.techPointsChanged()
			}
		}
		proposed = ship.Propose
		return tx.SetMarryShip(tx.ctx, db.SetMarryShipParams{MarryShip: ship.ID, ID: tx.commanderID})
	})
	if err != nil && !refused(err) {
		return err
	}
	return client.Send(&protobuf.SC_12033{Result: result(err), Time: proto.Uint32(proposed)})
}

// handleRenameShip answers RenameShipCommand (CS_12034): an oathed ship
// (other than an IDOL_LINK one) takes a name of its own, an empty name
// meaning its default one, once every 30 days (ShipMainScene). On result 0
// the client sets the name and Ship.renameTime to now. No official capture
// has it: built from the Lua alone.
func (game *Game) handleRenameShip(client *connection.Client, req *protobuf.CS_12034) error {
	name := req.GetName()
	err := errRefused
	if name == "" || validLabel(name, 1, maxShipNameLength) {
		err = game.tx(context.Background(), client, func(tx *Tx) error {
			ship, err := tx.commanderShip(req.GetShipId())
			if err != nil {
				return err
			}
			if ship.Propose == 0 || ownableShips[ship.TemplateID].Nationality == nationIdolLink {
				return errRefused
			}
			now := time.Now().Unix()
			if ship.ChangeNameTimestamp != 0 && now < int64(ship.ChangeNameTimestamp)+shipRenameCooldown {
				return refusal{"rename cooldown", resultTooSoon}
			}
			return tx.RenameShip(tx.ctx, db.RenameShipParams{Name: name, ChangeNameTimestamp: uint32(now), ID: ship.ID, CommanderID: tx.commanderID})
		})
	}
	if err != nil && !refused(err) {
		return err
	}
	return client.Send(&protobuf.SC_12035{Result: result(err)})
}

// handleShipPreference answers UpdateShipPreferenceTagCommand (CS_12040,
// the dock's "Preferred" mark): flag 1 marks the ship preferred, 0 clears
// it, kept as SHIPINFO.common_flag (Ship.preferenceTag). Built from the Lua
// alone.
func (game *Game) handleShipPreference(client *connection.Client, req *protobuf.CS_12040) error {
	err := errRefused
	if flag := req.GetFlag(); flag <= preferenceTagCommon {
		err = game.tx(context.Background(), client, func(tx *Tx) error {
			ship, err := tx.commanderShip(req.GetShipId())
			if err != nil {
				return err
			}
			return tx.SetShipCommonFlag(tx.ctx, db.SetShipCommonFlagParams{CommonFlag: flag, ID: ship.ID, CommanderID: tx.commanderID})
		})
	}
	if err != nil && !refused(err) {
		return err
	}
	return client.Send(&protobuf.SC_12041{Result: result(err)})
}

// handleReqPlayerAssist answers ReqPlayerAssistCommand (CS_12301): the
// ship each player of id_list lends for a sham battle (type 0) or the guild
// (type 1), one SHIPINFO per player in their order, a blank one (template
// 0) for no ship. Nothing in any region's client sends it any more
// (GAME.REQ_PLAYER_ASSIST_SHIP has no sender, PlayerProxy.playerAssists no
// reader) and neither feature exists in the game today, so no player lends
// a ship: every entry is blank.
func (game *Game) handleReqPlayerAssist(client *connection.Client, req *protobuf.CS_12301) error {
	reply := &protobuf.SC_12302{ShipList: make([]*protobuf.SHIPINFO, len(req.GetIdList()))}
	for i := range reply.ShipList {
		reply.ShipList[i] = &protobuf.SHIPINFO{}
	}
	return client.Send(reply)
}
