package game

import (
	"context"
	"database/sql"
	"errors"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"google.golang.org/protobuf/proto"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/db"
	"github.com/ggmolly/belfast/internal/protobuf"
)

// The guild membership rules the client hard codes (no guildset key holds
// them).
const (
	// guildWaitDuration is how long after quitting or dissolving a guild a
	// commander can't join another (GuildQuitCommand / GuildDissolveCommand
	// set guildWaitTime to now + 86400, guild_join_cd)
	guildWaitDuration = 86400
	// guildImpeachDelay is how long the commander has to log in and overrule
	// an impeachment (GuildImpeachCommand sets kickLeaderTime to now +
	// 86400, guild_member_tip 5)
	guildImpeachDelay = 86400
	// guildImpeachOffline is how long the commander must have been offline
	// to be impeached (GuildMember.isLongOffLine, guild_member_tip 4)
	guildImpeachOffline = 864000
	// guildFactionCooldown is the wait between two faction changes
	// (guild_faction_change_tip: 30 days)
	guildFactionCooldown = 30 * 86400
	// guildListCount is how many guilds the join page lists
	// (JoinGuildLayer, guild_search_list_max_count 30)
	guildListCount = 30
	// guildRequestMax caps the applications a guild keeps waiting (result
	// 6, guild_apply_full). The client doesn't say how many: belfast's choice.
	guildRequestMax = 50
	// guildTextLength caps a manifesto, an announcement or an application
	// message: belfast's choice, the client's inputs are shorter.
	guildTextLength = 200
)

// ModifyGuildInfoCommand types (GuildModifitonPage, GuildThemePage)
const (
	guildModifyName      = 1
	guildModifyFaction   = 2
	guildModifyPolicy    = 3
	guildModifyManifesto = 4
	guildModifyAnnounce  = 5
)

// GuildSearchCommand types: a numeric keyword searches ids
const guildSearchByID = 0

var (
	// errGuildJoinCooldown is guild_join_cd (apply) and
	// guild_player_in_cd_time (accept)
	errGuildJoinCooldown error = refusal{"guild join cooldown", 4}
	// errGuildRequestsFull is guild_apply_full
	errGuildRequestsFull error = refusal{"guild requests full", 6}
	// errGuildFull is guild_member_full
	errGuildFull error = refusal{"guild full", 4306}
	// errGuildPlayerJoined is guild_player_already_join: the applicant
	// joined another guild, the client then rejects its request itself
	errGuildPlayerJoined error = refusal{"player already in a guild", 1}
)

// guildLevelAt is the guild_data_level row of a guild level, the last one
// past the table's end (Guild.getMaxMember).
func guildLevelAt(level uint32) guildLevelConfig {
	if row, ok := guildLevels[level]; ok {
		return row
	}
	last := guildLevels[1]
	for _, row := range guildLevels {
		if row.Level > last.Level && row.Level <= level {
			last = row
		}
	}
	return last
}

// guildMaxMembers is how many members a guild holds: its level's
// member_num, plus the seats its technologies add (Guild.getMaxMember).
func guildMaxMembers(ctx context.Context, queries *db.Queries, guild db.Guild) (uint32, error) {
	seats, err := guildTechSeats(ctx, queries, guild.ID)
	if err != nil {
		return 0, err
	}
	return guildLevelAt(guild.Level).MemberNum + seats, nil
}

// guildWaitTime is when the commander may join a guild again (0 none),
// SC_11003.guild_wait_time.
func guildWaitTime(ctx context.Context, queries *db.Queries, commanderID uint32) (uint32, error) {
	until, err := queries.GetGuildWaitTime(ctx, commanderID)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, nil
	}
	return until, err
}

// noGuild is SC_60000 outside a guild: an id of 0 makes GuildProxy exit
// the guild it holds (the official server still fills policy, faction and
// level with 1).
func noGuild() *protobuf.SC_60000 {
	return &protobuf.SC_60000{Guild: &protobuf.GUILD_INFO{
		Base: &protobuf.GUILD_BASE_INFO{
			Policy:  proto.Uint32(guildPolicyPower),
			Faction: proto.Uint32(guildFactionAzurLane),
			Level:   proto.Uint32(1),
		},
	}}
}

// guildBaseUpdate is SC_60030, a guild's base info for its members.
func guildBaseUpdate(ctx context.Context, queries *db.Queries, guildID uint32) (*protobuf.SC_60030, error) {
	guild, err := queries.GetGuild(ctx, guildID)
	if err != nil {
		return nil, err
	}
	count, err := queries.CountGuildMembers(ctx, guildID)
	if err != nil {
		return nil, err
	}
	return &protobuf.SC_60030{Guild: guildBaseInfo(guild, uint32(count))}, nil
}

// guildMemberUpdate is a member's MEMBER_INFO as it is now, for SC_60031.
func (game *Game) guildMemberUpdate(ctx context.Context, queries *db.Queries, commanderID uint32) (*protobuf.MEMBER_INFO, error) {
	member, err := queries.GetGuildMember(ctx, commanderID)
	if err != nil {
		return nil, err
	}
	commander, err := queries.GetCommander(ctx, commanderID)
	if err != nil {
		return nil, err
	}
	return game.guildMemberInfo(member, commander), nil
}

// guildMemberLeft is the MEMBER_INFO telling the members a member left
// (duty 0): GuildProxy removes it.
func (game *Game) guildMemberLeft(member db.GuildMember, commander db.Commander) *protobuf.MEMBER_INFO {
	member.Duty = 0
	return game.guildMemberInfo(member, commander)
}

// guildSimpleInfo is a guild as the join page lists it (GUILD_SIMPLE_INFO):
// its base info, its commander and the seats its technologies add.
func guildSimpleInfo(ctx context.Context, game *Game, queries *db.Queries, guild db.Guild, memberCount uint32) (*protobuf.GUILD_SIMPLE_INFO, error) {
	leader, err := queries.GetGuildCommander(ctx, guild.ID)
	if err != nil {
		return nil, err
	}
	maxMembers, err := guildMaxMembers(ctx, queries, guild)
	if err != nil {
		return nil, err
	}
	return &protobuf.GUILD_SIMPLE_INFO{
		Base:     guildBaseInfo(guild, memberCount),
		Leader:   game.playerInfoP60(leader.Commander),
		TechSeat: proto.Uint32(maxMembers - guildLevelAt(guild.Level).MemberNum),
	}, nil
}

// guildAdministrators are the members of a guild who see its applications.
func guildAdministrators(ctx context.Context, queries *db.Queries, guildID uint32) ([]uint32, error) {
	members, err := queries.ListGuildMembers(ctx, guildID)
	if err != nil {
		return nil, err
	}
	var ids []uint32
	for _, row := range members {
		if guildAdministrator(row.GuildMember.Duty) {
			ids = append(ids, row.GuildMember.CommanderID)
		}
	}
	return ids, nil
}

// handleGuildRequests answers GetGuildRequestsCommand (CS_60003), sent when
// GuildProxy gets its guild and when the requests page opens or SC_60009
// says new ones came: the applications to the commander's guild, for its
// administrators (nothing for the others). Built from the Lua alone.
func (game *Game) handleGuildRequests(client *connection.Client, req *protobuf.CS_60003) error {
	ctx := context.Background()
	reply := &protobuf.SC_60004{}
	member, ok, err := guildMembership(ctx, game.DB.Queries, client.CommanderID)
	if err != nil {
		return err
	}
	if !ok || member.GuildID != req.GetId() || !guildAdministrator(member.Duty) {
		return client.Send(reply)
	}
	requests, err := game.DB.ListGuildRequests(ctx, member.GuildID)
	if err != nil {
		return err
	}
	for _, row := range requests {
		reply.RequestList = append(reply.RequestList, &protobuf.MSG_INFO_P60{
			Timestamp: proto.Uint32(row.GuildRequest.Time),
			Player:    game.playerInfoP60(row.Commander),
			Content:   proto.String(row.GuildRequest.Content),
		})
	}
	return client.Send(reply)
}

// handleGuildApply answers GuildApplyCommand (CS_60005), the join page's
// apply button: the application waits for the guild's administrators,
// pushed the count of applications waiting (SC_60009: GuildProxy lights the
// requests tip and an open requests page fetches them again). Applying
// again replaces the message. 4 is the cooldown after leaving a guild
// (guild_join_cd), 6 a full list of applications, 4306 a full guild.
// Built from the Lua alone.
func (game *Game) handleGuildApply(client *connection.Client, req *protobuf.CS_60005) error {
	ctx := context.Background()
	content := req.GetContent()
	var admins []uint32
	var count int64
	err := game.tx(ctx, client, func(tx *Tx) error {
		if !utf8.ValidString(content) || utf8.RuneCountInString(content) > guildTextLength {
			return errRefused
		}
		guild, err := tx.GetGuild(ctx, req.GetId())
		if errors.Is(err, sql.ErrNoRows) {
			return errRefused
		}
		if err != nil {
			return err
		}
		_, inGuild, err := guildMembership(ctx, tx.Queries, client.CommanderID)
		if err != nil {
			return err
		}
		if inGuild {
			return errRefused
		}
		wait, err := guildWaitTime(ctx, tx.Queries, client.CommanderID)
		if err != nil {
			return err
		}
		if uint32(time.Now().Unix()) < wait {
			return errGuildJoinCooldown
		}
		members, err := tx.CountGuildMembers(ctx, guild.ID)
		if err != nil {
			return err
		}
		maxMembers, err := guildMaxMembers(ctx, tx.Queries, guild)
		if err != nil {
			return err
		}
		if uint32(members) >= maxMembers {
			return errGuildFull
		}
		requests, err := tx.CountGuildRequests(ctx, guild.ID)
		if err != nil {
			return err
		}
		again, err := tx.GuildRequestExists(ctx, db.GuildRequestExistsParams{GuildID: guild.ID, CommanderID: client.CommanderID})
		if err != nil {
			return err
		}
		if !again && requests >= guildRequestMax {
			return errGuildRequestsFull
		}
		if err := tx.AddGuildRequest(ctx, db.AddGuildRequestParams{
			GuildID: guild.ID, CommanderID: client.CommanderID, Content: content, Time: uint32(time.Now().Unix()),
		}); err != nil {
			return err
		}
		if count, err = tx.CountGuildRequests(ctx, guild.ID); err != nil {
			return err
		}
		admins, err = guildAdministrators(ctx, tx.Queries, guild.ID)
		return err
	})
	if err != nil && !refused(err) {
		return err
	}
	if err == nil {
		game.tellGuild(client, admins, 0, &protobuf.SC_60009{Count: proto.Uint32(uint32(count))})
	}
	return client.Send(&protobuf.SC_60006{Result: result(err)})
}

// handleDissolveGuild answers GuildDissolveCommand (CS_60010), the
// commander's dissolve button on the guild info page: the guild and all it
// holds are deleted. On result 0 the client leaves the guild and starts its
// own day of cooldown (guildWaitTime), kept here for SC_11003; the other
// members online leave it too (SC_60000 with id 0). Built from the Lua alone.
func (game *Game) handleDissolveGuild(client *connection.Client, req *protobuf.CS_60010) error {
	ctx := context.Background()
	var members []uint32
	err := game.tx(ctx, client, func(tx *Tx) error {
		member, ok, err := guildMembership(ctx, tx.Queries, client.CommanderID)
		if err != nil {
			return err
		}
		if !ok || member.GuildID != req.GetId() || member.Duty != guildDutyCommander {
			return errRefused
		}
		if members, err = tx.ListGuildMemberIDs(ctx, member.GuildID); err != nil {
			return err
		}
		if err := tx.DeleteGuild(ctx, member.GuildID); err != nil {
			return err
		}
		return tx.SetGuildWaitTime(ctx, db.SetGuildWaitTimeParams{
			CommanderID: client.CommanderID, Until: uint32(time.Now().Unix()) + guildWaitDuration,
		})
	})
	if err != nil && !refused(err) {
		return err
	}
	if err == nil {
		game.tellGuild(client, members, client.CommanderID, noGuild())
	}
	return client.Send(&protobuf.SC_60011{Result: result(err)})
}

// handleSetGuildDuty answers SetGuildDutyCommand (CS_60012), the duty
// button on the members page. An administrator sets the duty of a member
// below it, not a recruit (Guild.getEnableDuty): the commander to deputy
// (up to the level's assistant_commander), picked or ordinary, or hands its
// own duty to a deputy and becomes ordinary itself; a deputy to picked or
// ordinary. On result 0 the client sets the duties itself; every member
// online gets them and the log lines (SC_60031). Built from the Lua alone.
func (game *Game) handleSetGuildDuty(client *connection.Client, req *protobuf.CS_60012) error {
	ctx := context.Background()
	duty := req.GetDutyId()
	var members []uint32
	update := &protobuf.SC_60031{}
	var base *protobuf.SC_60030
	err := game.tx(ctx, client, func(tx *Tx) error {
		self, err := guildAdministratorTx(tx)
		if err != nil {
			return err
		}
		target, ok, err := guildMembership(ctx, tx.Queries, req.GetPlayerId())
		if err != nil {
			return err
		}
		if !ok || target.GuildID != self.GuildID || target.Duty <= self.Duty || target.Duty == guildDutyRecruit || target.Duty == duty {
			return errRefused
		}
		guild, err := tx.GetGuild(ctx, self.GuildID)
		if err != nil {
			return err
		}
		switch duty {
		case guildDutyCommander:
			if self.Duty != guildDutyCommander || target.Duty != guildDutyDeputy {
				return errRefused
			}
		case guildDutyDeputy:
			if self.Duty != guildDutyCommander {
				return errRefused
			}
			deputies, err := countGuildDuty(ctx, tx.Queries, guild.ID, guildDutyDeputy)
			if err != nil {
				return err
			}
			if deputies >= guildLevelAt(guild.Level).Deputies {
				return errRefused
			}
		case guildDutyPicked, guildDutyOrdinary:
		default:
			return errRefused
		}
		changes := [][2]uint32{{target.CommanderID, duty}}
		if duty == guildDutyCommander {
			changes = append(changes, [2]uint32{self.CommanderID, guildDutyOrdinary})
		}
		if err := game.setGuildDuties(tx, guild.ID, changes, update); err != nil {
			return err
		}
		if duty == guildDutyCommander && guild.KickLeaderCd != 0 {
			if err := tx.SetGuildKickLeaderCd(ctx, db.SetGuildKickLeaderCdParams{ID: guild.ID}); err != nil {
				return err
			}
			if base, err = guildBaseUpdate(ctx, tx.Queries, guild.ID); err != nil {
				return err
			}
		}
		members, err = tx.ListGuildMemberIDs(ctx, guild.ID)
		return err
	})
	if err != nil && !refused(err) {
		return err
	}
	if err == nil {
		game.tellGuild(client, members, 0, update)
		if base != nil {
			game.tellGuild(client, members, 0, base)
		}
	}
	return client.Send(&protobuf.SC_60013{Result: result(err)})
}

// countGuildDuty counts the members of a guild with a duty.
func countGuildDuty(ctx context.Context, queries *db.Queries, guildID, duty uint32) (uint32, error) {
	members, err := queries.ListGuildMembers(ctx, guildID)
	if err != nil {
		return 0, err
	}
	count := uint32(0)
	for _, row := range members {
		if row.GuildMember.Duty == duty {
			count++
		}
	}
	return count, nil
}

// setGuildDuties gives members (commander id, duty) their new duty, with a
// SET_DUTY log line each, and adds both to update.
func (game *Game) setGuildDuties(tx *Tx, guildID uint32, changes [][2]uint32, update *protobuf.SC_60031) error {
	for _, change := range changes {
		if err := tx.SetGuildMemberDuty(tx.ctx, db.SetGuildMemberDutyParams{Duty: change[1], CommanderID: change[0]}); err != nil {
			return err
		}
		info, err := game.guildMemberUpdate(tx.ctx, tx.Queries, change[0])
		if err != nil {
			return err
		}
		commander, err := tx.GetCommander(tx.ctx, change[0])
		if err != nil {
			return err
		}
		line, err := tx.addGuildLog(guildID, guildLogSetDuty, commander, change[1])
		if err != nil {
			return err
		}
		update.MemberList = append(update.MemberList, info)
		update.LogList = append(update.LogList, line)
	}
	return nil
}

// removeGuildMember takes a member out of its guild with a log line (quit
// or fire), and out of its running operation: the update for the members
// (duty 0 and the line), the line alone and the members left.
func (game *Game) removeGuildMember(tx *Tx, member db.GuildMember, cmd uint32) (*protobuf.SC_60031, *protobuf.LOG_INFO, []uint32, error) {
	commander, err := tx.GetCommander(tx.ctx, member.CommanderID)
	if err != nil {
		return nil, nil, nil, err
	}
	now := serverNow()
	state, open, err := openGuildOperation(tx.ctx, tx.Queries, member.GuildID, now)
	if err != nil {
		return nil, nil, nil, err
	}
	if open {
		if err := state.withdraw(tx.ctx, tx.Queries, member.CommanderID, now); err != nil {
			return nil, nil, nil, err
		}
	}
	if err := tx.DeleteGuildMember(tx.ctx, member.CommanderID); err != nil {
		return nil, nil, nil, err
	}
	line, err := tx.addGuildLog(member.GuildID, cmd, commander, 0)
	if err != nil {
		return nil, nil, nil, err
	}
	members, err := tx.ListGuildMemberIDs(tx.ctx, member.GuildID)
	if err != nil {
		return nil, nil, nil, err
	}
	update := &protobuf.SC_60031{
		MemberList: []*protobuf.MEMBER_INFO{game.guildMemberLeft(member, commander)},
		LogList:    []*protobuf.LOG_INFO{line},
	}
	return update, line, members, nil
}

// handleFireGuildMember answers GuildFireCommand (CS_60014), the kick
// button on the members page: an administrator removes a member below it.
// On result 0 the client removes the member itself, so it only gets the log
// line (SC_60031: a member it no longer has would break its update); the
// other members get the member gone and the line, the member fired leaves
// the guild if online (SC_60000 with id 0). Built from the Lua alone.
func (game *Game) handleFireGuildMember(client *connection.Client, req *protobuf.CS_60014) error {
	ctx := context.Background()
	var members []uint32
	var update *protobuf.SC_60031
	var line *protobuf.LOG_INFO
	err := game.tx(ctx, client, func(tx *Tx) error {
		self, err := guildAdministratorTx(tx)
		if err != nil {
			return err
		}
		target, ok, err := guildMembership(ctx, tx.Queries, req.GetPlayerId())
		if err != nil {
			return err
		}
		if !ok || target.GuildID != self.GuildID || target.Duty <= self.Duty {
			return errRefused
		}
		update, line, members, err = game.removeGuildMember(tx, target, guildLogFire)
		return err
	})
	if err != nil && !refused(err) {
		return err
	}
	if err == nil {
		game.tellGuild(client, members, client.CommanderID, update)
		game.deliver(client, client.CommanderID, &protobuf.SC_60031{LogList: []*protobuf.LOG_INFO{line}})
		game.deliver(client, req.GetPlayerId(), noGuild())
	}
	return client.Send(&protobuf.SC_60015{Result: result(err)})
}

// handleImpeachGuildCommander answers GuildImpeachCommand (CS_60016), the
// impeach button a deputy sees on a commander offline for 10 days
// (GuildMember.isLongOffLine): the commander has a day to log in and
// overrule it, after which the deputy with the most liveness takes over
// (guild_member_tip 4 and 5, settleGuildImpeachment). On result 0 the
// client sets kick_leader_cd to now + a day itself; the other members get it
// in SC_60030. Built from the Lua alone.
func (game *Game) handleImpeachGuildCommander(client *connection.Client, req *protobuf.CS_60016) error {
	ctx := context.Background()
	var members []uint32
	var base *protobuf.SC_60030
	now := time.Now().Unix()
	err := game.tx(ctx, client, func(tx *Tx) error {
		self, ok, err := guildMembership(ctx, tx.Queries, client.CommanderID)
		if err != nil {
			return err
		}
		if !ok || self.Duty != guildDutyDeputy {
			return errRefused
		}
		guild, err := tx.GetGuild(ctx, self.GuildID)
		if err != nil {
			return err
		}
		leader, err := tx.GetGuildCommander(ctx, guild.ID)
		if err != nil {
			return err
		}
		if guild.KickLeaderCd != 0 || leader.Commander.ID != req.GetPlayerId() ||
			game.isOnline(leader.Commander.ID) || now-leader.Commander.LastOnline <= guildImpeachOffline {
			return errRefused
		}
		if err := tx.SetGuildKickLeaderCd(ctx, db.SetGuildKickLeaderCdParams{
			KickLeaderCd: uint32(now) + guildImpeachDelay, ID: guild.ID,
		}); err != nil {
			return err
		}
		if base, err = guildBaseUpdate(ctx, tx.Queries, guild.ID); err != nil {
			return err
		}
		members, err = tx.ListGuildMemberIDs(ctx, guild.ID)
		return err
	})
	if err != nil && !refused(err) {
		return err
	}
	if err == nil {
		game.tellGuild(client, members, client.CommanderID, base)
	}
	return client.Send(&protobuf.SC_60017{Result: result(err)})
}

// settleGuildImpeachment ends the impeachment of the guild's commander,
// if any, when the guild info is asked for (CS_60037, sent at every login):
// the commander logged in since (or is the one asking) overrules it; once
// its day is over, the deputy with the most liveness (then the oldest
// member) becomes commander and the commander ordinary, as a commander
// handing its duty over. The other members online get the change (SC_60031,
// SC_60030); the one asking gets the whole guild next.
func (game *Game) settleGuildImpeachment(ctx context.Context, client *connection.Client, guildID uint32) error {
	var members []uint32
	var msgs []proto.Message
	err := game.tx(ctx, client, func(tx *Tx) error {
		guild, err := tx.GetGuild(ctx, guildID)
		if err != nil || guild.KickLeaderCd == 0 {
			return err
		}
		now := time.Now().Unix()
		leader, err := tx.GetGuildCommander(ctx, guildID)
		if err != nil {
			return err
		}
		overruled := leader.Commander.ID == client.CommanderID || game.isOnline(leader.Commander.ID) ||
			leader.Commander.LastOnline >= int64(guild.KickLeaderCd)-guildImpeachDelay
		if !overruled && now < int64(guild.KickLeaderCd) {
			return nil
		}
		if !overruled {
			rows, err := tx.ListGuildMembers(ctx, guildID)
			if err != nil {
				return err
			}
			var heir *db.GuildMember
			for i, row := range rows {
				if row.GuildMember.Duty == guildDutyDeputy && (heir == nil || row.GuildMember.Liveness > heir.Liveness) {
					heir = &rows[i].GuildMember
				}
			}
			if heir != nil {
				update := &protobuf.SC_60031{}
				if err := game.setGuildDuties(tx, guildID, [][2]uint32{
					{heir.CommanderID, guildDutyCommander}, {leader.Commander.ID, guildDutyOrdinary},
				}, update); err != nil {
					return err
				}
				msgs = append(msgs, update)
			}
		}
		if err := tx.SetGuildKickLeaderCd(ctx, db.SetGuildKickLeaderCdParams{ID: guildID}); err != nil {
			return err
		}
		base, err := guildBaseUpdate(ctx, tx.Queries, guildID)
		if err != nil {
			return err
		}
		msgs = append(msgs, base)
		members, err = tx.ListGuildMemberIDs(ctx, guildID)
		return err
	})
	if err != nil {
		return err
	}
	game.tellGuild(client, members, client.CommanderID, msgs...)
	return nil
}

// handleQuitGuild answers GuildQuitCommand (CS_60018), the quit button on
// the guild info page (not the commander's: it dissolves the guild or hands
// its duty over first). On result 0 the client leaves the guild and starts
// its own day of cooldown (guildWaitTime), kept here for SC_11003; the
// members online get the member gone and the log line (SC_60031). Built from
// the Lua alone.
func (game *Game) handleQuitGuild(client *connection.Client, req *protobuf.CS_60018) error {
	ctx := context.Background()
	var members []uint32
	var update *protobuf.SC_60031
	err := game.tx(ctx, client, func(tx *Tx) error {
		self, ok, err := guildMembership(ctx, tx.Queries, client.CommanderID)
		if err != nil {
			return err
		}
		if !ok || self.GuildID != req.GetId() || self.Duty == guildDutyCommander {
			return errRefused
		}
		if update, _, members, err = game.removeGuildMember(tx, self, guildLogQuit); err != nil {
			return err
		}
		return tx.SetGuildWaitTime(ctx, db.SetGuildWaitTimeParams{
			CommanderID: client.CommanderID, Until: uint32(time.Now().Unix()) + guildWaitDuration,
		})
	})
	if err != nil && !refused(err) {
		return err
	}
	if err == nil {
		game.tellGuild(client, members, client.CommanderID, update)
	}
	return client.Send(&protobuf.SC_60019{Result: result(err)})
}

// handleAcceptGuildRequest answers GuildRequestAcceptCommand (CS_60020), the
// accept button on the requests page: the applicant joins as a recruit with
// a JOIN log line. The client drops the request itself, whatever the result,
// and doesn't add the member: every member online gets it (SC_60031), the
// new member its guild if online (SC_60000). 1 is an applicant already in a
// guild (the client then rejects the request), 4 one still in its cooldown
// after leaving a guild, 4306 a full guild; those drop the request here too.
// Built from the Lua alone.
func (game *Game) handleAcceptGuildRequest(client *connection.Client, req *protobuf.CS_60020) error {
	ctx := context.Background()
	applicant := req.GetPlayerId()
	var members []uint32
	var guildID uint32
	update := &protobuf.SC_60031{}
	err := game.tx(ctx, client, func(tx *Tx) error {
		self, err := guildAdministratorTx(tx)
		if err != nil {
			return err
		}
		guildID = self.GuildID
		exists, err := tx.GuildRequestExists(ctx, db.GuildRequestExistsParams{GuildID: guildID, CommanderID: applicant})
		if err != nil {
			return err
		}
		if !exists {
			return errRefused
		}
		if err := tx.DeleteGuildRequest(ctx, db.DeleteGuildRequestParams{GuildID: guildID, CommanderID: applicant}); err != nil {
			return err
		}
		_, inGuild, err := guildMembership(ctx, tx.Queries, applicant)
		if err != nil {
			return err
		}
		if inGuild {
			return errGuildPlayerJoined
		}
		wait, err := guildWaitTime(ctx, tx.Queries, applicant)
		if err != nil {
			return err
		}
		now := uint32(time.Now().Unix())
		if now < wait {
			return errGuildJoinCooldown
		}
		guild, err := tx.GetGuild(ctx, guildID)
		if err != nil {
			return err
		}
		if members, err = tx.ListGuildMemberIDs(ctx, guildID); err != nil {
			return err
		}
		maxMembers, err := guildMaxMembers(ctx, tx.Queries, guild)
		if err != nil {
			return err
		}
		if uint32(len(members)) >= maxMembers {
			return errGuildFull
		}
		if err := tx.AddGuildMember(ctx, db.AddGuildMemberParams{
			CommanderID: applicant, GuildID: guildID, Duty: guildDutyRecruit, JoinTime: now,
		}); err != nil {
			return err
		}
		if err := tx.DeleteCommanderGuildRequests(ctx, applicant); err != nil {
			return err
		}
		commander, err := tx.GetCommander(ctx, applicant)
		if err != nil {
			return err
		}
		line, err := tx.addGuildLog(guildID, guildLogJoin, commander, 0)
		if err != nil {
			return err
		}
		info, err := game.guildMemberUpdate(ctx, tx.Queries, applicant)
		if err != nil {
			return err
		}
		update.MemberList = append(update.MemberList, info)
		update.LogList = append(update.LogList, line)
		return nil
	})
	if errors.Is(err, errGuildPlayerJoined) || errors.Is(err, errGuildJoinCooldown) || errors.Is(err, errGuildFull) {
		// the request is gone either way, as the client drops it
		if delErr := game.DB.DeleteGuildRequest(ctx, db.DeleteGuildRequestParams{GuildID: guildID, CommanderID: applicant}); delErr != nil {
			return delErr
		}
	}
	if err != nil && !refused(err) {
		return err
	}
	if err == nil {
		game.tellGuild(client, members, 0, update)
		if game.isOnline(applicant) {
			info, err := game.guildInfo(ctx, game.DB.Queries, guildID)
			if err != nil {
				return err
			}
			game.deliver(client, applicant, &protobuf.SC_60000{Guild: info})
		}
	}
	return client.Send(&protobuf.SC_60021{Result: result(err)})
}

// handleRejectGuildRequest answers GuildRequestRejectCommand (CS_60022), the
// reject button on the requests page, also sent by the client itself after
// accepting someone already in a guild: the application is dropped. On
// result 0 the client drops it too. Built from the Lua alone.
func (game *Game) handleRejectGuildRequest(client *connection.Client, req *protobuf.CS_60022) error {
	ctx := context.Background()
	err := game.tx(ctx, client, func(tx *Tx) error {
		self, err := guildAdministratorTx(tx)
		if err != nil {
			return err
		}
		return tx.DeleteGuildRequest(ctx, db.DeleteGuildRequestParams{GuildID: self.GuildID, CommanderID: req.GetPlayerId()})
	})
	if err != nil && !refused(err) {
		return err
	}
	return client.Send(&protobuf.SC_60023{Result: result(err)})
}

// handleGuildList answers GuildsRefreshCommand (CS_60024), sent when the
// join page opens and on its refresh button: up to 30 guilds with room, at
// random, with their commander. Built from the Lua alone.
func (game *Game) handleGuildList(client *connection.Client, req *protobuf.CS_60024) error {
	ctx := context.Background()
	exclude := uint32(0)
	if member, ok, err := guildMembership(ctx, game.DB.Queries, client.CommanderID); err != nil {
		return err
	} else if ok {
		exclude = member.GuildID
	}
	rows, err := game.DB.ListGuildsToJoin(ctx, db.ListGuildsToJoinParams{Exclude: exclude, Count: 4 * guildListCount})
	if err != nil {
		return err
	}
	reply := &protobuf.SC_60025{}
	for _, row := range rows {
		if len(reply.GuildList) == guildListCount {
			break
		}
		maxMembers, err := guildMaxMembers(ctx, game.DB.Queries, row.Guild)
		if err != nil {
			return err
		}
		if uint32(row.MemberCount) >= maxMembers {
			continue
		}
		info, err := guildSimpleInfo(ctx, game, game.DB.Queries, row.Guild, uint32(row.MemberCount))
		if err != nil {
			return err
		}
		reply.GuildList = append(reply.GuildList, info)
	}
	return client.Send(reply)
}

// handleSearchGuild answers GuildSearchCommand (CS_60028), the join page's
// search: type 0 (a numeric keyword) the guild with that id, else the
// guilds whose name holds the keyword, up to 30. Nothing found is a
// non-zero result (guild_no_exist). Built from the Lua alone.
func (game *Game) handleSearchGuild(client *connection.Client, req *protobuf.CS_60028) error {
	ctx := context.Background()
	keyword := strings.TrimSpace(req.GetKeyword())
	params := db.SearchGuildsParams{Keyword: keyword, Count: guildListCount}
	if req.GetType() == guildSearchByID {
		id, err := strconv.ParseUint(keyword, 10, 32)
		if err == nil {
			params.ID = uint32(id)
		}
	}
	if keyword == "" {
		return client.Send(&protobuf.SC_60029{Result: result(errRefused)})
	}
	rows, err := game.DB.SearchGuilds(ctx, params)
	if err != nil {
		return err
	}
	if len(rows) == 0 {
		return client.Send(&protobuf.SC_60029{Result: result(errRefused)})
	}
	reply := &protobuf.SC_60029{Result: result(nil)}
	for _, row := range rows {
		info, err := guildSimpleInfo(ctx, game, game.DB.Queries, row.Guild, uint32(row.MemberCount))
		if err != nil {
			return err
		}
		reply.Guild = append(reply.Guild, info)
	}
	return client.Send(reply)
}

// handleModifyGuild answers ModifyGuildInfoCommand (CS_60026): the
// commander's guild info page (1 name for modify_guild_cost gems, 2
// faction once every 30 days, 3 policy, 4 manifesto) and the
// administrators' announcement (5). On result 0 the client only takes the
// gems of a rename itself: every member online, the commander included,
// gets the new base info (SC_60030). 2015 is a name taken. Built from the
// Lua alone.
func (game *Game) handleModifyGuild(client *connection.Client, req *protobuf.CS_60026) error {
	ctx := context.Background()
	var members []uint32
	var base *protobuf.SC_60030
	err := game.tx(ctx, client, func(tx *Tx) error {
		self, ok, err := guildMembership(ctx, tx.Queries, client.CommanderID)
		if err != nil {
			return err
		}
		if !ok || !guildAdministrator(self.Duty) || (req.GetType() != guildModifyAnnounce && self.Duty != guildDutyCommander) {
			return errRefused
		}
		guild, err := tx.GetGuild(ctx, self.GuildID)
		if err != nil {
			return err
		}
		text := req.GetStr()
		switch req.GetType() {
		case guildModifyName:
			if !validNameUpTo(text, guildNameLength) {
				return errGuildNameTaken
			}
			taken, err := tx.GuildNameTaken(ctx, db.GuildNameTakenParams{Name: text, ID: guild.ID})
			if err != nil {
				return err
			}
			if taken {
				return errGuildNameTaken
			}
			if err := tx.SpendShopResource(ResourceGem, guildRenameCost); err != nil {
				return err
			}
			err = tx.SetGuildName(ctx, db.SetGuildNameParams{Name: text, ID: guild.ID})
		case guildModifyFaction:
			faction := req.GetInt()
			now := uint32(time.Now().Unix())
			if (faction != guildFactionAzurLane && faction != guildFactionCrimson) || faction == guild.Faction || now < guild.ChangeFactionCd {
				return errRefused
			}
			err = tx.SetGuildFaction(ctx, db.SetGuildFactionParams{Faction: faction, ChangeFactionCd: now + guildFactionCooldown, ID: guild.ID})
		case guildModifyPolicy:
			policy := req.GetInt()
			if policy != guildPolicyPower && policy != guildPolicyRelaxation {
				return errRefused
			}
			err = tx.SetGuildPolicy(ctx, db.SetGuildPolicyParams{Policy: policy, ID: guild.ID})
		case guildModifyManifesto, guildModifyAnnounce:
			if !utf8.ValidString(text) || utf8.RuneCountInString(text) > guildTextLength {
				return errRefused
			}
			if req.GetType() == guildModifyManifesto {
				err = tx.SetGuildManifesto(ctx, db.SetGuildManifestoParams{Manifesto: text, ID: guild.ID})
			} else {
				err = tx.SetGuildAnnounce(ctx, db.SetGuildAnnounceParams{Announce: text, ID: guild.ID})
			}
		default:
			return errRefused
		}
		if err != nil {
			return err
		}
		if base, err = guildBaseUpdate(ctx, tx.Queries, guild.ID); err != nil {
			return err
		}
		members, err = tx.ListGuildMemberIDs(ctx, guild.ID)
		return err
	})
	if err != nil && !refused(err) {
		return err
	}
	if err == nil {
		game.tellGuild(client, members, 0, base)
	}
	return client.Send(&protobuf.SC_60027{Result: result(err)})
}
