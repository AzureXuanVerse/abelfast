package game

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"math/rand/v2"
	"slices"
	"sync"
	"time"

	"google.golang.org/protobuf/proto"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/db"
	"github.com/ggmolly/belfast/internal/gamedata"
	"github.com/ggmolly/belfast/internal/protobuf"
)

// ActivityConst.ACTIVITY_TYPE_FEAST: the 5th anniversary's Castle of
// Celebrations, a backyard (FeastDorm) where the day's party ships walk
// around and the event's invited ships can be sent invitations and gifts.
const activityTypeFeast = 104

// FeastDorm.OP_*: the CS_11202 cmds of a feast (FeastOpCommand), and
// OP_RANDOM_SHIPS, sent as CS_26158.
const (
	feastOpEnter       = 1
	feastOpMakeTicket  = 2
	feastOpGiveTicket  = 3
	feastOpGiveGift    = 4
	feastOpInteraction = 5
)

// FeastShip.BUBBLE_TYPE_* (greet, drink, eat, dance, sleep): what the party
// ship asks for; CHAT_BUBBLE_TYPE_1/2: which of the invited ship's lines for
// it she says (InvitedFeastShip:GetSpeechContent).
const (
	feastBubbles       = 5
	feastSpeechBubbles = 2
)

// InvitedFeastShip.STATE_* and GIFT_STATE_GOT
const (
	feastInvitationMade  = 1
	feastInvitationGiven = 2
	feastGiftGiven       = 1
)

// task_data_template sub types of the feast's tasks (FeastMediator's
// HandleTaskGo): entering the castle, interacting with a party ship,
// sending an invitation, giving a gift.
const (
	TaskSubFeastEnter       = 430
	TaskSubFeastInteraction = 431
	TaskSubFeastInvitation  = 432
	TaskSubFeastGift        = 433
)

// activity_partyinvitation_template row: an invited ship (by ship group)
// and the virtual bag items ([drop type, id, count]) her invitation and her
// gift cost
type partyInvitationConfig struct {
	GroupID      uint32   `json:"groupid"`
	InvitationID []uint32 `json:"invitationID"`
	GiftID       []uint32 `json:"giftID"`
}

var partyInvitations map[uint32]partyInvitationConfig

func loadFeastData() error {
	return gamedata.LoadAll(
		gamedata.Load("activity_partyinvitation_template", &partyInvitations),
		gamedata.Load("activity_drop_type", &activityDropTypes),
	)
}

// feastSettings is a feast activity's config_data: how many ships the day's
// party draws (FeastProxy, 5 when missing), the server drops of an
// interaction, an invitation and a gift, and its invited ships
// (activity_partyinvitation_template ids).
type feastSettings struct {
	partySize uint32
	drops     []uint32
	invited   []partyInvitationConfig
}

func feastConfig(config activityConfig) feastSettings {
	settings := feastSettings{partySize: 5}
	var raw []json.RawMessage
	if json.Unmarshal(config.ConfigData, &raw) != nil {
		return settings
	}
	if len(raw) > 0 {
		var size uint32
		if json.Unmarshal(raw[0], &size) == nil && size > 0 {
			settings.partySize = size
		}
	}
	if len(raw) > 1 {
		settings.drops = gamedata.IDs(raw[1])
	}
	if len(raw) > 2 {
		for _, id := range gamedata.IDs(raw[2]) {
			if row, ok := partyInvitations[id]; ok {
				settings.invited = append(settings.invited, row)
			}
		}
	}
	return settings
}

// invitation is the invited ship of group, if the feast has one.
func (settings feastSettings) invitation(group uint32) (partyInvitationConfig, bool) {
	for _, row := range settings.invited {
		if row.GroupID == group {
			return row, true
		}
	}
	return partyInvitationConfig{}, false
}

// drop is what the feast's index-th server drop (0 interaction, 1
// invitation, 2 gift) gives.
func (settings feastSettings) drop(index int) []*protobuf.DROPINFO {
	if index >= len(settings.drops) {
		return nil
	}
	awards, err := serverDropAwards()
	if err != nil {
		return nil
	}
	return configDrops(awards[settings.drops[index]])
}

// serverDropAwards are the server-only drop ids (task_data_template award)
// and the awards the client shows for them (the same row's award_display):
// a feast's config_data names its drops by these ids, the same ones its
// tasks use.
var serverDropAwards = sync.OnceValues(func() (map[uint32][][]uint32, error) {
	awards := map[uint32][][]uint32{}
	for _, config := range taskConfigs {
		if _, ok := awards[config.Award]; !ok && config.Award != 0 && len(config.awards) > 0 {
			awards[config.Award] = config.awards
		}
	}
	return awards, nil
})

// openFeast is the feast activity id, if it runs for the commander.
func (game *Game) openFeast(ctx context.Context, commanderID, id uint32) (activityConfig, bool, error) {
	config, open, err := game.commanderActivity(ctx, commanderID, id, time.Now())
	return config, open && config.Type == activityTypeFeast, err
}

// feastShips are the feast's party ships as party_roles.
func feastShips(ctx context.Context, queries *db.Queries, commanderID, activityID uint32) ([]*protobuf.P_PARTY_ROLE, error) {
	rows, err := queries.ListFeastShips(ctx, db.ListFeastShipsParams{CommanderID: commanderID, ActivityID: activityID})
	if err != nil {
		return nil, err
	}
	roles := make([]*protobuf.P_PARTY_ROLE, len(rows))
	for i, row := range rows {
		roles[i] = &protobuf.P_PARTY_ROLE{Tid: proto.Uint32(row.GroupID), Bubble: proto.Uint32(row.Bubble), SpeechBubble: proto.Uint32(row.SpeechBubble)}
	}
	return roles, nil
}

// feastInvitation is the stored state of the invited ship of group.
func feastInvitation(ctx context.Context, queries *db.Queries, commanderID, activityID, group uint32) (db.CommanderFeastInvitation, error) {
	rows, err := queries.ListFeastInvitations(ctx, db.ListFeastInvitationsParams{CommanderID: commanderID, ActivityID: activityID})
	if err != nil {
		return db.CommanderFeastInvitation{}, err
	}
	for _, row := range rows {
		if row.GroupID == group {
			return row, nil
		}
	}
	return db.CommanderFeastInvitation{CommanderID: commanderID, ActivityID: activityID, GroupID: group}, nil
}

// handleFeastData answers GetFeastDataCommand (controller/command/feast),
// sent by FeastProxy once the main menu shows while a feast runs: FeastDorm
// takes the party ships (party_roles), every invited ship with her
// invitation and gift states (special_roles: InvitedFeastShip asserts each
// is one of the activity's, and the invitation page lists only these) and
// when the party was last drawn (refresh_time, 0 never: FeastProxy then
// draws one, CS_26158). No official capture has it: built from the Lua
// alone.
func (game *Game) handleFeastData(client *connection.Client, req *protobuf.CS_26156) error {
	ctx := context.Background()
	config, open, err := game.openFeast(ctx, client.CommanderID, req.GetActId())
	if err != nil {
		return err
	}
	if !open {
		return client.Send(&protobuf.SC_26157{Ret: result(errRefused)})
	}
	queries := game.DB.Queries
	refresh, err := queries.GetFeastRefreshTime(ctx, db.GetFeastRefreshTimeParams{CommanderID: client.CommanderID, ActivityID: config.ID})
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	ships, err := feastShips(ctx, queries, client.CommanderID, config.ID)
	if err != nil {
		return err
	}
	rows, err := queries.ListFeastInvitations(ctx, db.ListFeastInvitationsParams{CommanderID: client.CommanderID, ActivityID: config.ID})
	if err != nil {
		return err
	}
	reply := &protobuf.SC_26157{Ret: result(nil), PartyRoles: ships, RefreshTime: proto.Uint32(uint32(refresh))}
	for _, invited := range feastConfig(config).invited {
		role := &protobuf.P_SPECIAL_ROLE{Tid: proto.Uint32(invited.GroupID), State: proto.Uint32(0), Gift: proto.Uint32(0)}
		for _, row := range rows {
			if row.GroupID == invited.GroupID {
				role.State, role.Gift = proto.Uint32(row.State), proto.Uint32(row.Gift)
			}
		}
		reply.SpecialRoles = append(reply.SpecialRoles, role)
	}
	return client.Send(reply)
}

// handleFeastRandomShips answers FeastOpCommand's OP_RANDOM_SHIPS
// (CS_26158), sent when the party was last drawn before today
// (FeastDorm:ShouldRandomShips) and at midnight: the client picks the new
// ships itself (FeastProxy:RandomShips: config_data[1] ship groups, owned
// ones in their party skin first, none of the invited ones nor of the
// current party). The reply's party_roles replace the party, so the invited
// ships that came keep their place; refresh_time is when it was drawn.
// Each ship gets a random bubble and line (a guess: the Lua only shows
// them, the wiki says every party ship has a bubble). Refused when the
// party was already drawn today, or for ships that aren't ship groups or
// are invited ones. No official capture has it: built from the Lua alone.
func (game *Game) handleFeastRandomShips(client *connection.Client, req *protobuf.CS_26158) error {
	ctx := context.Background()
	config, open, err := game.openFeast(ctx, client.CommanderID, req.GetActId())
	if err != nil {
		return err
	}
	settings := feastConfig(config)
	groups := req.GetShipGroupId()
	if !open || uint32(len(groups)) > settings.partySize {
		return client.Send(&protobuf.SC_26159{Ret: result(errRefused)})
	}
	for i, group := range groups {
		_, invited := settings.invitation(group)
		if invited || !archiveGroups[group] || slices.Contains(groups[:i], group) {
			return client.Send(&protobuf.SC_26159{Ret: result(errRefused)})
		}
	}
	now := time.Now()
	reply := &protobuf.SC_26159{Ret: result(nil), RefreshTime: proto.Uint32(uint32(now.Unix()))}
	err = game.tx(ctx, client, func(tx *Tx) error {
		key := db.GetFeastRefreshTimeParams{CommanderID: tx.commanderID, ActivityID: config.ID}
		refresh, err := tx.GetFeastRefreshTime(tx.ctx, key)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		if refresh != 0 && sameServerDay(time.Unix(refresh, 0), now) {
			return errRefused
		}
		party, err := tx.ListFeastShips(tx.ctx, db.ListFeastShipsParams(key))
		if err != nil {
			return err
		}
		if err := tx.DeleteFeastShips(tx.ctx, db.DeleteFeastShipsParams(key)); err != nil {
			return err
		}
		var drawn []uint32
		for _, ship := range party {
			if _, invited := settings.invitation(ship.GroupID); invited {
				drawn = append(drawn, ship.GroupID)
			}
		}
		for _, group := range append(drawn, groups...) {
			ship := db.SaveFeastShipParams{
				CommanderID: tx.commanderID, ActivityID: config.ID, GroupID: group,
				Bubble: rand.Uint32N(feastBubbles) + 1, SpeechBubble: rand.Uint32N(feastSpeechBubbles) + 1,
			}
			if err := tx.SaveFeastShip(tx.ctx, ship); err != nil {
				return err
			}
			reply.PartyRoles = append(reply.PartyRoles, &protobuf.P_PARTY_ROLE{
				Tid: proto.Uint32(group), Bubble: proto.Uint32(ship.Bubble), SpeechBubble: proto.Uint32(ship.SpeechBubble),
			})
		}
		return tx.SetFeastRefreshTime(tx.ctx, db.SetFeastRefreshTimeParams{CommanderID: tx.commanderID, ActivityID: config.ID, RefreshTime: now.Unix()})
	})
	if refused(err) {
		return client.Send(&protobuf.SC_26159{Ret: result(err)})
	}
	if err != nil {
		return err
	}
	return client.Send(reply)
}

// feastOperation answers FeastOpCommand's CS_11202 cmds on a running feast
// (arg1 a ship group). On result 0 the client updates FeastDorm and the
// virtual bag itself:
//   - OP_ENTER, sent when the castle opens: counts for the daily task.
//   - OP_INTERACTION, once a party ship's bubble was answered: her bubble
//     goes, number[1] is the line she says (an invited ship's
//     GetSpeechContent), award_list config_data[2][1]'s drop.
//   - OP_MAKE_TICKET: pays the invited ship's invitationID out of the
//     virtual bag, her invitation is made.
//   - OP_GIVE_TICKET: a made invitation is given, she joins the party with
//     the bubble in number[1]; award_list config_data[2][2]'s drop.
//   - OP_GIVE_GIFT: once she has her invitation, pays giftID out of the
//     virtual bag; award_list config_data[2][3]'s drop.
//
// config_data[2]'s ids are server drops; their awards are those the tasks
// with the same award id show (100 / 200 Ambiance, as the wiki's Castle of
// Celebrations page gives for an interaction and an invitation; the gift's
// isn't on the wiki). No official capture has it: built from the Lua alone.
func (game *Game) feastOperation(client *connection.Client, config activityConfig, req *protobuf.CS_11202) error {
	ctx := context.Background()
	settings := feastConfig(config)
	group := req.GetArg1()
	reply := &protobuf.SC_11203{Result: result(nil)}
	err := game.tx(ctx, client, func(tx *Tx) error {
		switch req.GetCmd() {
		case feastOpEnter:
			tx.event(TaskEvent{SubType: TaskSubFeastEnter, Count: 1})
			return nil
		case feastOpInteraction:
			return tx.feastInteraction(config.ID, group, settings, reply)
		}
		invited, ok := settings.invitation(group)
		if !ok {
			return errRefused
		}
		state, err := feastInvitation(tx.ctx, tx.Queries, tx.commanderID, config.ID, group)
		if err != nil {
			return err
		}
		switch {
		case req.GetCmd() == feastOpMakeTicket && state.State == 0:
			if err := tx.spendFeastItem(invited.InvitationID); err != nil {
				return err
			}
			state.State = feastInvitationMade
		case req.GetCmd() == feastOpGiveTicket && state.State == feastInvitationMade:
			ship := db.SaveFeastShipParams{
				CommanderID: tx.commanderID, ActivityID: config.ID, GroupID: group,
				Bubble: rand.Uint32N(feastBubbles) + 1, SpeechBubble: rand.Uint32N(feastSpeechBubbles) + 1,
			}
			if err := tx.SaveFeastShip(tx.ctx, ship); err != nil {
				return err
			}
			state.State = feastInvitationGiven
			reply.Number = []uint32{ship.Bubble}
			reply.AwardList = settings.drop(1)
			tx.event(TaskEvent{SubType: TaskSubFeastInvitation, Count: 1})
		case req.GetCmd() == feastOpGiveGift && state.State == feastInvitationGiven && state.Gift == 0:
			if err := tx.spendFeastItem(invited.GiftID); err != nil {
				return err
			}
			state.Gift = feastGiftGiven
			reply.AwardList = settings.drop(2)
			tx.event(TaskEvent{SubType: TaskSubFeastGift, Count: 1})
		default:
			return errRefused
		}
		if err := tx.grantDrops(reply.AwardList); err != nil {
			return err
		}
		return tx.SaveFeastInvitation(tx.ctx, db.SaveFeastInvitationParams(state))
	})
	if refused(err) {
		return client.Send(&protobuf.SC_11203{Result: result(err)})
	}
	if err != nil {
		return err
	}
	return client.Send(reply)
}

// feastInteraction is OP_INTERACTION on the party ship of group, refused
// when she has no bubble left.
func (tx *Tx) feastInteraction(activityID, group uint32, settings feastSettings, reply *protobuf.SC_11203) error {
	rows, err := tx.ListFeastShips(tx.ctx, db.ListFeastShipsParams{CommanderID: tx.commanderID, ActivityID: activityID})
	if err != nil {
		return err
	}
	i := slices.IndexFunc(rows, func(row db.CommanderFeastShip) bool { return row.GroupID == group })
	if i < 0 || rows[i].Bubble == 0 {
		return errRefused
	}
	ship := rows[i]
	ship.Bubble = 0
	if err := tx.SaveFeastShip(tx.ctx, db.SaveFeastShipParams(ship)); err != nil {
		return err
	}
	reply.Number = []uint32{ship.SpeechBubble}
	reply.AwardList = settings.drop(0)
	tx.event(TaskEvent{SubType: TaskSubFeastInteraction, Count: 1})
	return tx.grantDrops(reply.AwardList)
}

// spendFeastItem pays an activity_partyinvitation_template cost, [drop
// type, item, count], out of the drop type's virtual bag
// (FeastOpCommand:CheckRes).
func (tx *Tx) spendFeastItem(cost []uint32) error {
	if len(cost) < 3 {
		return errRefused
	}
	activityID, ok := activityItemDrop(cost[0])
	if !ok {
		return errRefused
	}
	return tx.spendActivityItem(activityID, cost[1], cost[2])
}
