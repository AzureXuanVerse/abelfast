package game

import (
	"context"
	"database/sql"
	"errors"
	"time"
	"unicode/utf8"

	"github.com/ggmolly/belfast/internal/db"
	"github.com/ggmolly/belfast/internal/protobuf"
)

// The back-office's moderation writes: shared backyard themes and guilds.
// A commander playing keeps its own copy of what these change (its
// templates, its guild's members, funds, texts and operation), so the
// commanders concerned are kicked first and see the change at their next
// login, as AdminWrite does.

// guildWrite runs fn in a transaction once the guild's members playing,
// and the extra commanders, are kicked. A row fn doesn't find (the guild
// gone meanwhile...) is an AdminError.
func (game *Game) guildWrite(ctx context.Context, guildID uint32, extra []uint32, fn func(queries *db.Queries) error) error {
	members, err := game.DB.ListGuildMemberIDs(ctx, guildID)
	if err != nil {
		return err
	}
	for _, id := range append(members, extra...) {
		if _, err := game.Kick(id); err != nil {
			return err
		}
	}
	err = game.DB.InTx(ctx, fn)
	if errors.Is(err, sql.ErrNoRows) {
		return AdminError{Msg: "not found", Missing: true}
	}
	return err
}

// UnshareDormTheme takes a commander's template (slot pos) out of the theme
// shop as BackYardUnloadThemeTemplateCommand does: its likes and the
// collections holding it go too. The owner, whose template list says it is
// shared, is kicked first.
func (game *Game) UnshareDormTheme(ctx context.Context, commanderID, pos uint32) (bool, error) {
	return game.EditCommanderRows(ctx, commanderID, func(queries *db.Queries) error {
		theme, err := queries.GetDormTheme(ctx, db.GetDormThemeParams{CommanderID: commanderID, Pos: pos})
		if errors.Is(err, sql.ErrNoRows) || err == nil && theme.UploadTime == 0 {
			return AdminError{Msg: "no shared template there"}
		}
		if err != nil {
			return err
		}
		if _, err := queries.SetDormThemeUploadTime(ctx, db.SetDormThemeUploadTimeParams{UploadTime: 0, CommanderID: commanderID, Pos: pos}); err != nil {
			return err
		}
		return unshareTheme(ctx, queries, theme.ThemeID)
	})
}

// GuildRankPeriods are the starts of the week and the month the guild
// ranks count in (guild_rank_scores week_start, month_start).
func GuildRankPeriods(now time.Time) (week, month uint32) {
	return guildWeek(now), uint32(startOfMonth(now).Unix())
}

// GuildMaxMembers is how many members a guild holds (its level and
// technologies).
func (game *Game) GuildMaxMembers(ctx context.Context, guild db.Guild) (uint32, error) {
	return guildMaxMembers(ctx, game.DB.Queries, guild)
}

// guildMember is a guild's member, an AdminError when it isn't one.
func guildMember(ctx context.Context, queries *db.Queries, guildID, commanderID uint32) (db.GuildMember, error) {
	member, ok, err := guildMembership(ctx, queries, commanderID)
	if err != nil {
		return member, err
	}
	if !ok || member.GuildID != guildID {
		return member, AdminError{Msg: "not a member of this guild"}
	}
	return member, nil
}

// GuildEdit is what the back-office changes of a guild; nil and 0 leave a
// field as it is.
type GuildEdit struct {
	Name      *string
	Announce  *string
	Manifesto *string
	// funds added to the guild's
	GiveFunds uint32
	// the member made the guild's commander, the current one becoming an
	// ordinary member as when it hands its duty over (SetGuildDutyCommand)
	Leader uint32
}

// EditGuild changes a guild with the checks the client's pages make (a name
// of 1 to 20 characters without spaces nobody else has, texts up to
// guildTextLength), without their costs.
func (game *Game) EditGuild(ctx context.Context, guildID uint32, edit GuildEdit) error {
	if edit.Name != nil && !validNameUpTo(*edit.Name, guildNameLength) {
		return AdminError{Msg: "a name is 1 to 20 characters, without spaces"}
	}
	for _, text := range []*string{edit.Announce, edit.Manifesto} {
		if text != nil && (!utf8.ValidString(*text) || utf8.RuneCountInString(*text) > guildTextLength) {
			return AdminError{Msg: "texts are up to 200 characters"}
		}
	}
	return game.guildWrite(ctx, guildID, nil, func(queries *db.Queries) error {
		guild, err := queries.GetGuild(ctx, guildID)
		if err != nil {
			return err
		}
		if edit.Name != nil && *edit.Name != guild.Name {
			taken, err := queries.GuildNameTaken(ctx, db.GuildNameTakenParams{Name: *edit.Name, ID: guild.ID})
			if err != nil {
				return err
			}
			if taken {
				return AdminError{Msg: "another guild has that name"}
			}
			if err := queries.SetGuildName(ctx, db.SetGuildNameParams{Name: *edit.Name, ID: guild.ID}); err != nil {
				return err
			}
		}
		if edit.Announce != nil {
			if err := queries.SetGuildAnnounce(ctx, db.SetGuildAnnounceParams{Announce: *edit.Announce, ID: guild.ID}); err != nil {
				return err
			}
		}
		if edit.Manifesto != nil {
			if err := queries.SetGuildManifesto(ctx, db.SetGuildManifestoParams{Manifesto: *edit.Manifesto, ID: guild.ID}); err != nil {
				return err
			}
		}
		if edit.GiveFunds != 0 {
			if uint64(guild.Capital)+uint64(edit.GiveFunds) > 1<<32-1 {
				return AdminError{Msg: "too many funds"}
			}
			if err := queries.AddGuildCapital(ctx, db.AddGuildCapitalParams{Capital: edit.GiveFunds, ID: guild.ID}); err != nil {
				return err
			}
		}
		if edit.Leader != 0 {
			return setGuildLeader(ctx, newTx(ctx, queries, 0), game, guild, edit.Leader)
		}
		return nil
	})
}

// setGuildLeader makes a member the guild's commander and the commander an
// ordinary member, with their SET_DUTY log lines; a pending impeachment is
// over, as when the commander hands its duty over.
func setGuildLeader(ctx context.Context, tx *Tx, game *Game, guild db.Guild, leader uint32) error {
	target, err := guildMember(ctx, tx.Queries, guild.ID, leader)
	if err != nil {
		return err
	}
	if target.Duty == guildDutyCommander {
		return nil
	}
	changes := [][2]uint32{{leader, guildDutyCommander}}
	current, err := tx.GetGuildCommander(ctx, guild.ID)
	switch {
	case err == nil:
		changes = append(changes, [2]uint32{current.GuildMember.CommanderID, guildDutyOrdinary})
	case !errors.Is(err, sql.ErrNoRows):
		return err
	}
	if err := game.setGuildDuties(tx, guild.ID, changes, &protobuf.SC_60031{}); err != nil {
		return err
	}
	if guild.KickLeaderCd != 0 {
		return tx.SetGuildKickLeaderCd(ctx, db.SetGuildKickLeaderCdParams{ID: guild.ID})
	}
	return nil
}

// SetGuildMemberDuty gives a member below the commander a duty below it
// (deputy up to the level's assistant_commander, picked, ordinary), as
// SetGuildDutyCommand does; the commander changes with EditGuild's Leader.
func (game *Game) SetGuildMemberDuty(ctx context.Context, guildID, commanderID, duty uint32) error {
	switch duty {
	case guildDutyDeputy, guildDutyPicked, guildDutyOrdinary:
	default:
		return AdminError{Msg: "a duty is deputy (2), picked (3) or ordinary (4)"}
	}
	return game.guildWrite(ctx, guildID, nil, func(queries *db.Queries) error {
		member, err := guildMember(ctx, queries, guildID, commanderID)
		if err != nil {
			return err
		}
		if member.Duty == guildDutyCommander {
			return AdminError{Msg: "change the commander with a new leader"}
		}
		if member.Duty == duty {
			return nil
		}
		if duty == guildDutyDeputy {
			guild, err := queries.GetGuild(ctx, guildID)
			if err != nil {
				return err
			}
			deputies, err := countGuildDuty(ctx, queries, guildID, guildDutyDeputy)
			if err != nil {
				return err
			}
			if deputies >= guildLevelAt(guild.Level).Deputies {
				return AdminError{Msg: "the guild has all the deputies its level allows"}
			}
		}
		return game.setGuildDuties(newTx(ctx, queries, 0), guildID, [][2]uint32{{commanderID, duty}}, &protobuf.SC_60031{})
	})
}

// RemoveGuildMember takes a member out of its guild as GuildFireCommand
// does (a FIRE log line, out of the running operation). The commander has
// to be replaced first.
func (game *Game) RemoveGuildMember(ctx context.Context, guildID, commanderID uint32) error {
	return game.guildWrite(ctx, guildID, nil, func(queries *db.Queries) error {
		member, err := guildMember(ctx, queries, guildID, commanderID)
		if err != nil {
			return err
		}
		if member.Duty == guildDutyCommander {
			return AdminError{Msg: "give the guild a new leader first"}
		}
		_, _, _, err = game.removeGuildMember(newTx(ctx, queries, 0), member, guildLogFire)
		return err
	})
}

// DisbandGuild deletes a guild and all it holds (members, applications,
// logs, chat, technologies, logistics, operations...: the guild tables
// cascade from guilds), as GuildDissolveCommand does. Its members keep what
// is theirs (guild_users, the technologies they bought).
func (game *Game) DisbandGuild(ctx context.Context, guildID uint32) error {
	return game.guildWrite(ctx, guildID, nil, func(queries *db.Queries) error {
		if _, err := queries.GetGuild(ctx, guildID); err != nil {
			return err
		}
		return queries.DeleteGuild(ctx, guildID)
	})
}

// AcceptGuildRequest takes an applicant in as GuildAcceptCommand does: a
// recruit, its other applications dropped, a JOIN log line. An applicant
// already in a guild, in its cooldown or a full guild turn it down (the
// application stays).
func (game *Game) AcceptGuildRequest(ctx context.Context, guildID, commanderID uint32) error {
	return game.guildWrite(ctx, guildID, []uint32{commanderID}, func(queries *db.Queries) error {
		exists, err := queries.GuildRequestExists(ctx, db.GuildRequestExistsParams{GuildID: guildID, CommanderID: commanderID})
		if err != nil {
			return err
		}
		if !exists {
			return AdminError{Msg: "no such application"}
		}
		if _, inGuild, err := guildMembership(ctx, queries, commanderID); err != nil {
			return err
		} else if inGuild {
			return AdminError{Msg: "the applicant is in a guild"}
		}
		wait, err := guildWaitTime(ctx, queries, commanderID)
		if err != nil {
			return err
		}
		now := uint32(time.Now().Unix())
		if now < wait {
			return AdminError{Msg: "the applicant left a guild less than a day ago"}
		}
		guild, err := queries.GetGuild(ctx, guildID)
		if err != nil {
			return err
		}
		count, err := queries.CountGuildMembers(ctx, guildID)
		if err != nil {
			return err
		}
		maxMembers, err := guildMaxMembers(ctx, queries, guild)
		if err != nil {
			return err
		}
		if uint32(count) >= maxMembers {
			return AdminError{Msg: "the guild is full"}
		}
		if err := queries.AddGuildMember(ctx, db.AddGuildMemberParams{
			CommanderID: commanderID, GuildID: guildID, Duty: guildDutyRecruit, JoinTime: now,
		}); err != nil {
			return err
		}
		if err := queries.DeleteCommanderGuildRequests(ctx, commanderID); err != nil {
			return err
		}
		commander, err := queries.GetCommander(ctx, commanderID)
		if err != nil {
			return err
		}
		_, err = newTx(ctx, queries, 0).addGuildLog(guildID, guildLogJoin, commander, 0)
		return err
	})
}

// RejectGuildRequest drops an application. The guild's administrators see
// the applications count, so the guild's members playing are kicked.
func (game *Game) RejectGuildRequest(ctx context.Context, guildID, commanderID uint32) error {
	return game.guildWrite(ctx, guildID, nil, func(queries *db.Queries) error {
		exists, err := queries.GuildRequestExists(ctx, db.GuildRequestExistsParams{GuildID: guildID, CommanderID: commanderID})
		if err != nil {
			return err
		}
		if !exists {
			return AdminError{Msg: "no such application"}
		}
		return queries.DeleteGuildRequest(ctx, db.DeleteGuildRequestParams{GuildID: guildID, CommanderID: commanderID})
	})
}

// GuildMissionView is a mission of an operation as the back-office shows
// it: its progress (0 to 1) brought up to now.
type GuildMissionView struct {
	EventID  uint32
	Position uint32
	// guild_base_event type (1 common, 2 elite) and sub_type (1 main, 2 side)
	Type, SubType uint32
	Progress      float64
	Started       bool
	Finished      bool
	Score         uint32
	Ships         int
	// the nodes' positions (percent) and GuildMissionNode.STATE_*
	Nodes [][2]uint32
}

// GuildOperationView is a guild's latest operation.
type GuildOperationView struct {
	db.GuildOperation
	// running: neither ended nor past its end time
	Open     bool
	Missions []GuildMissionView
	Members  []db.GuildOperationMember
	Ships    int
	Reports  int64
}

// GuildOperation is a guild's latest operation, settled up to now when it
// runs (as any member's request does), nil when the guild never started one.
func (game *Game) GuildOperation(ctx context.Context, guildID uint32) (*GuildOperationView, error) {
	now := serverNow()
	var view *GuildOperationView
	err := game.DB.InTx(ctx, func(queries *db.Queries) error {
		op, err := queries.ModerationLatestGuildOperation(ctx, guildID)
		if errors.Is(err, sql.ErrNoRows) {
			return nil
		}
		if err != nil {
			return err
		}
		state := &guildOperationState{op: op}
		open := !op.Ended && op.EndTime > uint32(now.Unix())
		if state.missions, err = queries.ListGuildMissions(ctx, op.ID); err != nil {
			return err
		}
		if state.nodes, err = queries.ListGuildMissionNodes(ctx, op.ID); err != nil {
			return err
		}
		if state.ships, err = queries.ListGuildMissionShips(ctx, op.ID); err != nil {
			return err
		}
		if open {
			if err := state.settle(ctx, queries, now); err != nil {
				return err
			}
		}
		view = &GuildOperationView{GuildOperation: state.op, Open: open, Ships: len(state.ships)}
		for _, mission := range state.missions {
			event := guildBaseEvents[mission.EventID]
			row := GuildMissionView{
				EventID: mission.EventID, Position: mission.Position, Type: event.Type, SubType: event.SubType,
				Started: mission.StartTime != 0, Finished: mission.FinishedAt != 0, Score: mission.Score,
				Ships: len(state.missionShips(mission.EventID)),
			}
			if event.Workload > 0 {
				row.Progress = min(1, mission.Work/float64(event.Workload))
			}
			if row.Finished {
				row.Progress = 1
			}
			for _, node := range state.nodes {
				if node.EventID == mission.EventID {
					row.Nodes = append(row.Nodes, [2]uint32{node.Position, node.Status})
				}
			}
			view.Missions = append(view.Missions, row)
		}
		if view.Members, err = queries.ListGuildOperationMembers(ctx, op.ID); err != nil {
			return err
		}
		view.Reports, err = queries.ModerationCountGuildReports(ctx, op.ID)
		return err
	})
	return view, err
}

// EndGuildOperation ends a guild's running operation, as its boss's defeat
// does: nothing progresses any more, the reports already written stay
// claimable, and the administrators can start another.
func (game *Game) EndGuildOperation(ctx context.Context, guildID uint32) error {
	return game.guildWrite(ctx, guildID, nil, func(queries *db.Queries) error {
		state, open, err := openGuildOperation(ctx, queries, guildID, serverNow())
		if err != nil {
			return err
		}
		if !open {
			return AdminError{Msg: "no operation running"}
		}
		if err := state.settle(ctx, queries, serverNow()); err != nil {
			return err
		}
		_, err = queries.ModerationEndGuildOperation(ctx, state.op.ID)
		return err
	})
}

// ResetGuildOperation starts a guild's running operation over: it ends
// (its reports stay claimable, they need its nodes) and the same operation
// is drawn again (createGuildOperation) from now, without its funds being
// taken again, its members still taking part (their joins this month were
// counted already) without their fleets or fights. It counts as one more
// operation started this month (active_event_cnt).
func (game *Game) ResetGuildOperation(ctx context.Context, guildID uint32) error {
	return game.guildWrite(ctx, guildID, nil, func(queries *db.Queries) error {
		now := serverNow()
		state, open, err := openGuildOperation(ctx, queries, guildID, now)
		if err != nil {
			return err
		}
		if !open {
			return AdminError{Msg: "no operation running"}
		}
		if _, ok := guildOperationPlans[state.op.OperationID]; !ok {
			return AdminError{Msg: "the operation's missions aren't in the tables"}
		}
		members, err := queries.ListGuildOperationMembers(ctx, state.op.ID)
		if err != nil {
			return err
		}
		if err := state.settle(ctx, queries, now); err != nil {
			return err
		}
		if _, err := queries.ModerationEndGuildOperation(ctx, state.op.ID); err != nil {
			return err
		}
		if err := createGuildOperation(ctx, queries, guildID, state.op.OperationID, now); err != nil {
			return err
		}
		op, err := queries.ModerationLatestGuildOperation(ctx, guildID)
		if err != nil {
			return err
		}
		for _, member := range members {
			if err := queries.ModerationRestoreGuildOperationMember(ctx, db.ModerationRestoreGuildOperationMemberParams{
				OperationID: op.ID, CommanderID: member.CommanderID, JoinedAt: member.JoinedAt, Extra: member.Extra,
			}); err != nil {
				return err
			}
		}
		return nil
	})
}
