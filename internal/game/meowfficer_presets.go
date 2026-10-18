package game

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/db"
	"github.com/ggmolly/belfast/internal/protobuf"
)

// Meowfficer presets (CommanderProxy prefabFleet, CommnaderFleet): saved
// command/support pairs, recorded from a fleet and applied back to one from
// the formation's meowfficer panel (LevelUIConst.COMMANDER_OP_*). Applying
// one has no packet of its own: CommanderUsePrefabCommand empties the
// fleet's two slots and fills them through CS_25006.
//
// The client never gets a preset's name back: its SC_25001 listener reads
// the name off the message rather than the preset (slot0.name), so after a
// relogin every preset shows commander_prefab_name ("Team $1") again. The
// name is still kept.

const (
	// CommanderProxy.MAX_PREFAB_FLEET
	meowfficerPresetCount = 3
	// SetCommanderPrefabFleetNameCommand: nameValidityCheck(name, 0, 12)
	maxMeowfficerPresetName = 12
	// CommnaderFleet.RENAME_CODE_TIME, seconds between two renames
	meowfficerPresetRenameCooldown = 60
)

// handleSetMeowfficerPreset records a fleet's cats as a preset
// (SetComanderPrefabFleetCommand, CS_25022): commandersid is the fleet's
// cats by pos (1 command, 2 support; a slot left empty is absent), the
// client sends nothing when they're all empty. The preset's cats are
// replaced; on result 0 the client takes the cats it sent. Built from the
// Lua alone.
func (game *Game) handleSetMeowfficerPreset(client *connection.Client, req *protobuf.CS_25022) error {
	presetID := req.GetId()
	err := errRefused
	if presetID >= 1 && presetID <= meowfficerPresetCount {
		err = game.tx(context.Background(), client, func(tx *Tx) error {
			var cats [meowfficerFleetSlots + 1]uint32
			filled := false
			for _, slot := range req.GetCommandersid() {
				pos, id := slot.GetPos(), slot.GetId()
				if pos < 1 || pos > meowfficerFleetSlots || cats[pos] != 0 {
					return errRefused
				}
				if id == 0 {
					continue
				}
				for _, other := range cats {
					if other == id {
						return errRefused
					}
				}
				if _, err := tx.GetMeowfficer(tx.ctx, db.GetMeowfficerParams{CommanderID: tx.commanderID, ID: id}); errors.Is(err, sql.ErrNoRows) {
					return errRefused
				} else if err != nil {
					return err
				}
				cats[pos] = id
				filled = true
			}
			if !filled {
				return errRefused
			}
			if err := tx.CreateMeowfficerPreset(tx.ctx, db.CreateMeowfficerPresetParams{CommanderID: tx.commanderID, PresetID: presetID}); err != nil {
				return err
			}
			if err := tx.ClearMeowfficerPresetSlots(tx.ctx, db.ClearMeowfficerPresetSlotsParams{CommanderID: tx.commanderID, PresetID: presetID}); err != nil {
				return err
			}
			for pos, id := range cats {
				if id == 0 {
					continue
				}
				if err := tx.SetMeowfficerPresetSlot(tx.ctx, db.SetMeowfficerPresetSlotParams{
					CommanderID: tx.commanderID, PresetID: presetID, Pos: uint32(pos), MeowfficerID: id,
				}); err != nil {
					return err
				}
			}
			return nil
		})
	}
	if err != nil && !refused(err) {
		return err
	}
	return client.Send(&protobuf.SC_25023{Result: result(err)})
}

// validMeowfficerPresetName mirrors what the client checks before renaming a
// preset: not empty, no spaces (spece_illegal_tip), at most
// maxMeowfficerPresetName characters.
func validMeowfficerPresetName(name string) bool {
	return name != "" && utf8.ValidString(name) && utf8.RuneCountInString(name) <= maxMeowfficerPresetName &&
		!strings.ContainsFunc(name, unicode.IsSpace)
}

// handleRenameMeowfficerPreset renames a preset
// (SetCommanderPrefabFleetNameCommand, CS_25024), an empty one too. The
// client refuses the name it already has and a rename less than
// RENAME_CODE_TIME after the last one; on result 0 it takes the name. Built
// from the Lua alone.
func (game *Game) handleRenameMeowfficerPreset(client *connection.Client, req *protobuf.CS_25024) error {
	now := time.Now().Unix()
	presetID, name := req.GetId(), req.GetName()
	err := errRefused
	if presetID >= 1 && presetID <= meowfficerPresetCount && validMeowfficerPresetName(name) {
		err = game.tx(context.Background(), client, func(tx *Tx) error {
			key := db.GetMeowfficerPresetParams{CommanderID: tx.commanderID, PresetID: presetID}
			preset, err := tx.GetMeowfficerPreset(tx.ctx, key)
			if err != nil && !errors.Is(err, sql.ErrNoRows) {
				return err
			}
			if preset.Name == name || now-preset.RenameTime < meowfficerPresetRenameCooldown {
				return errRefused
			}
			if err := tx.CreateMeowfficerPreset(tx.ctx, db.CreateMeowfficerPresetParams(key)); err != nil {
				return err
			}
			return tx.RenameMeowfficerPreset(tx.ctx, db.RenameMeowfficerPresetParams{
				Name: name, RenameTime: now, CommanderID: tx.commanderID, PresetID: presetID,
			})
		})
	}
	if err != nil && !refused(err) {
		return err
	}
	return client.Send(&protobuf.SC_25025{Result: result(err)})
}
