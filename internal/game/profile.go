package game

import (
	"context"
	"database/sql"
	"errors"
	"slices"
	"strings"
	"sync"
	"time"
	"unicode"
	"unicode/utf8"

	"google.golang.org/protobuf/proto"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/db"
	"github.com/ggmolly/belfast/internal/gamedata"
	"github.com/ggmolly/belfast/internal/protobuf"
)

// SC_11003.cd_list keys (Player.CHANGE_NAME_KEY, model/vo/player.lua)
const cooldownChangeName = 1

// flagIllegalName is the common flag the server sets on a commander whose
// name it rejected (ILLEGALITY_PLAYER_NAME, const.lua): the client then forces
// a free rename (ForcePlayerNameModificationPage, CS_11007 type 2).
const flagIllegalName = 1000010

// CS_11007.type
const (
	renameByItem = 1 // ModifyNameByItem: paid, rate limited
	renameForced = 2 // ForceModifyName: after flagIllegalName
)

// maxNameLength is nameValidityCheck's upper bound for a commander name
// (ChangePlayerNameCommand, NewPlayerScene: 4..14). The check itself is a
// client global the Lua dump lacks, so the lower bound (which may weigh
// characters) is left to the client.
const maxNameLength = 14

// errNameTaken is player_changePlayerName_error_2015 "Name is being used by
// another player."
var errNameTaken error = refusal{"name taken", 2015}

// renameRules are the gameset settings ChangePlayerNameCommand and
// Player:canModifyName check.
type renameRules struct {
	minLevel uint32    // player_name_change_lv_limit
	cooldown int64     // player_name_cold_time, seconds
	cost     [3]uint32 // player_name_change_cost description: drop type, id, count
}

var loadRenameRules = sync.OnceValues(func() (renameRules, error) {
	gameset, err := gamedata.Gameset()
	if err != nil {
		return renameRules{}, err
	}
	rules := renameRules{
		minLevel: gameset["player_name_change_lv_limit"],
		cooldown: int64(gameset["player_name_cold_time"]),
	}
	if err := gamedata.GamesetDescription("player_name_change_cost", &rules.cost); err != nil {
		return renameRules{}, err
	}
	return rules, nil
})

// cooldowns is SC_11003.cd_list.
func cooldowns(ctx context.Context, queries *db.Queries, commanderID uint32) ([]*protobuf.COOLDOWN, error) {
	rows, err := queries.ListCooldowns(ctx, commanderID)
	if err != nil {
		return nil, err
	}
	list := make([]*protobuf.COOLDOWN, len(rows))
	for i, row := range rows {
		list[i] = &protobuf.COOLDOWN{Key: proto.Uint32(row.Key), Timestamp: proto.Uint32(uint32(row.Until))}
	}
	return list, nil
}

// validName mirrors what the client checks before sending a commander name:
// not empty, no spaces (spece_illegal_tip), at most maxNameLength characters.
func validName(name string) bool {
	return validNameUpTo(name, maxNameLength)
}

// validNameUpTo is validName with an upper bound of maxLength characters.
func validNameUpTo(name string, maxLength int) bool {
	return name != "" && utf8.ValidString(name) && utf8.RuneCountInString(name) <= maxLength &&
		!strings.ContainsFunc(name, unicode.IsSpace)
}

// handleChangeName renames the commander (ChangePlayerNameCommand, CS_11007).
// Type 1 (PlayerVitaeRenamePage) needs player_name_change_lv_limit, waits
// player_name_cold_time between renames and costs player_name_change_cost
// (a Name Change Form, item 15009); type 2 is the free rename forced by
// flagIllegalName. On result 0 the client sets the name, the type 1 cooldown
// (now + player_name_cold_time, in its cdList, back in SC_11003.cd_list) and
// takes the cost from its bag itself; type 2 clears the flag. 2015 is "name
// taken". Built from the Lua alone (no capture).
func (game *Game) handleChangeName(client *connection.Client, req *protobuf.CS_11007) error {
	ctx := context.Background()
	rules, err := loadRenameRules()
	if err != nil {
		return err
	}
	name := req.GetName()
	now := time.Now().Unix()
	err = game.tx(ctx, client, func(tx *Tx) error {
		commander, err := tx.GetCommander(ctx, client.CommanderID)
		if err != nil {
			return err
		}
		if !validName(name) || name == commander.Name {
			return errRefused
		}
		switch req.GetType() {
		case renameByItem:
			if commander.Level < rules.minLevel {
				return errRefused
			}
			until, err := tx.GetCooldown(ctx, db.GetCooldownParams{CommanderID: client.CommanderID, Key: cooldownChangeName})
			if err != nil && !errors.Is(err, sql.ErrNoRows) {
				return err
			}
			if now < until {
				return errRefused
			}
			if err := tx.payRename(rules.cost); err != nil {
				return err
			}
			if err := tx.SetCooldown(ctx, db.SetCooldownParams{CommanderID: client.CommanderID, Key: cooldownChangeName, Until: now + rules.cooldown}); err != nil {
				return err
			}
		case renameForced:
			flags, err := tx.ListCommonFlags(ctx, client.CommanderID)
			if err != nil {
				return err
			}
			if !slices.Contains(flags, flagIllegalName) {
				return errRefused
			}
			if err := tx.DeleteCommonFlag(ctx, db.DeleteCommonFlagParams{CommanderID: client.CommanderID, FlagID: flagIllegalName}); err != nil {
				return err
			}
		default:
			return errRefused
		}
		taken, err := tx.CommanderNameTaken(ctx, db.CommanderNameTakenParams{Name: name, ID: client.CommanderID})
		if err != nil {
			return err
		}
		if taken {
			return errNameTaken
		}
		return tx.SetCommanderName(ctx, db.SetCommanderNameParams{Name: name, ID: client.CommanderID})
	})
	if err != nil && !refused(err) {
		return err
	}
	return client.Send(&protobuf.SC_11008{Result: result(err)})
}

// payRename takes player_name_change_cost, a [drop type, id, count] the
// client only knows as a resource (1) or an item (2).
func (tx *Tx) payRename(cost [3]uint32) error {
	switch cost[0] {
	case dropTypeResource:
		return tx.SpendShopResource(cost[1], cost[2])
	case dropTypeItem:
		return tx.SpendItem(cost[1], cost[2])
	}
	return errRefused
}

// handleChangeManifesto sets the commander's profile message
// (ChangeManifestoCommand, CS_11009), SC_11003.adv. The client filters it
// (wordVer) and sets it itself on result 0. Built from the Lua alone.
func (game *Game) handleChangeManifesto(client *connection.Client, req *protobuf.CS_11009) error {
	var err error
	if !utf8.ValidString(req.GetAdv()) {
		err = errRefused
	} else if err = game.DB.SetManifesto(context.Background(), db.SetManifestoParams{Manifesto: req.GetAdv(), ID: client.CommanderID}); err != nil {
		return err
	}
	return client.Send(&protobuf.SC_11010{Result: result(err)})
}
