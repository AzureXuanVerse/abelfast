package game

import (
	"context"
	"database/sql"
	"errors"
	"time"
	"unicode/utf8"

	"google.golang.org/protobuf/proto"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/db"
	"github.com/ggmolly/belfast/internal/protobuf"
)

// GuildConst.DUTY_*: a member's rank, 1 the guild's only commander
const (
	guildDutyCommander = 1
	guildDutyDeputy    = 2
	guildDutyPicked    = 3
	guildDutyOrdinary  = 4
	guildDutyRecruit   = 5
)

// GuildConst.CMD_TYPE_*: what a guild log line says (GuildLogInfo:getConent)
const (
	guildLogJoin    = 1
	guildLogSetDuty = 2 // arg1 the new duty
	guildLogQuit    = 3
	guildLogFire    = 4
)

// GuildConst.POLICY_TYPE_* and FACTION_TYPE_*
const (
	guildPolicyPower      = 1
	guildPolicyRelaxation = 2
	guildFactionAzurLane  = 1
	guildFactionCrimson   = 2
)

// guildNameLength is what the create and modify pages allow
// (nameValidityCheck(name, 0, 20)).
const guildNameLength = 20

// guildLogCount is how many log lines GUILD_INFO carries.
const guildLogCount = 50

// errGuildNameTaken is guild_create_error 2015 / guild_name_invaild.
var errGuildNameTaken error = refusal{"guild name taken", 2015}

// errNotInGuild is a guild request from a commander outside any guild.
var errNotInGuild = errRefused

// guildMembership is the commander's guild membership, ok false outside a
// guild.
func guildMembership(ctx context.Context, queries *db.Queries, commanderID uint32) (member db.GuildMember, ok bool, err error) {
	member, err = queries.GetGuildMember(ctx, commanderID)
	if errors.Is(err, sql.ErrNoRows) {
		return member, false, nil
	}
	return member, err == nil, err
}

// guildAdministrator tells whether a duty can manage the guild
// (GuildMember.IsAdministrator).
func guildAdministrator(duty uint32) bool {
	return duty == guildDutyCommander || duty == guildDutyDeputy
}

// guildBaseInfo is a guild's GUILD_BASE_INFO, with its member count.
func guildBaseInfo(guild db.Guild, memberCount uint32) *protobuf.GUILD_BASE_INFO {
	return &protobuf.GUILD_BASE_INFO{
		Id:              proto.Uint32(guild.ID),
		Policy:          proto.Uint32(guild.Policy),
		Faction:         proto.Uint32(guild.Faction),
		Name:            proto.String(guild.Name),
		Level:           proto.Uint32(guild.Level),
		Announce:        proto.String(guild.Announce),
		Manifesto:       proto.String(guild.Manifesto),
		Exp:             proto.Uint32(guild.Exp),
		MemberCount:     proto.Uint32(memberCount),
		ChangeFactionCd: proto.Uint32(guild.ChangeFactionCd),
		KickLeaderCd:    proto.Uint32(guild.KickLeaderCd),
	}
}

// guildMemberInfo is a member as GuildMember.New reads it (a Friend plus
// its duty, liveness and join time). A duty of 0 tells the members' push
// (SC_60031) the member left.
func (game *Game) guildMemberInfo(member db.GuildMember, commander db.Commander) *protobuf.MEMBER_INFO {
	friend := game.friendInfo(commander)
	return &protobuf.MEMBER_INFO{
		Liveness:      proto.Uint32(member.Liveness),
		Duty:          proto.Uint32(member.Duty),
		Id:            friend.Id,
		Name:          friend.Name,
		Lv:            friend.Lv,
		Adv:           friend.Adv,
		Online:        friend.Online,
		PreOnlineTime: friend.PreOnlineTime,
		Display:       friend.Display,
		JoinTime:      proto.Uint32(member.JoinTime),
	}
}

// guildLogInfo is a guild log line.
func guildLogInfo(row db.GuildLog) *protobuf.LOG_INFO {
	return &protobuf.LOG_INFO{
		Cmd:    proto.Uint32(row.Cmd),
		Time:   proto.Uint32(row.Time),
		UserId: proto.Uint32(row.UserID),
		Name:   proto.String(row.Name),
		Arg1:   proto.Uint32(row.Arg1),
	}
}

// guildInfo is the whole guild as SC_60000 carries it: base info, members,
// the latest log lines and the expansion info.
func (game *Game) guildInfo(ctx context.Context, queries *db.Queries, guildID uint32) (*protobuf.GUILD_INFO, error) {
	guild, err := queries.GetGuild(ctx, guildID)
	if err != nil {
		return nil, err
	}
	members, err := queries.ListGuildMembers(ctx, guildID)
	if err != nil {
		return nil, err
	}
	logs, err := queries.ListGuildLogs(ctx, db.ListGuildLogsParams{GuildID: guildID, Count: guildLogCount})
	if err != nil {
		return nil, err
	}
	expansion, err := game.guildExpansion(ctx, queries, guild)
	if err != nil {
		return nil, err
	}
	info := &protobuf.GUILD_INFO{
		Base:    guildBaseInfo(guild, uint32(len(members))),
		GuildEx: expansion,
	}
	for _, row := range members {
		info.Member = append(info.Member, game.guildMemberInfo(row.GuildMember, row.Commander))
	}
	for _, row := range logs {
		info.Log = append(info.Log, guildLogInfo(row))
	}
	return info, nil
}

// guildExpansion is the guild's GUILD_EXPANSION_INFO: its funds, then what
// logistics (weekly task, supplies), technologies and operations add.
func (game *Game) guildExpansion(ctx context.Context, queries *db.Queries, guild db.Guild) (*protobuf.GUILD_EXPANSION_INFO, error) {
	expansion := &protobuf.GUILD_EXPANSION_INFO{
		Capital: proto.Uint32(guild.Capital),
	}
	for _, fill := range []func(context.Context, *db.Queries, db.Guild, *protobuf.GUILD_EXPANSION_INFO) error{
		game.fillGuildLogistics,
		game.fillGuildTechnologies,
		game.fillGuildOperations,
	} {
		if err := fill(ctx, queries, guild, expansion); err != nil {
			return nil, err
		}
	}
	return expansion, nil
}

// addGuildLog writes a log line, returned for the members' push.
func (tx *Tx) addGuildLog(guildID, cmd uint32, commander db.Commander, arg1 uint32) (*protobuf.LOG_INFO, error) {
	row, err := tx.AddGuildLog(tx.ctx, db.AddGuildLogParams{
		GuildID: guildID,
		Cmd:     cmd,
		Time:    uint32(time.Now().Unix()),
		UserID:  commander.ID,
		Name:    commander.Name,
		Arg1:    arg1,
	})
	if err != nil {
		return nil, err
	}
	return guildLogInfo(row), nil
}

// tellGuild pushes msgs to the members of a guild online, except the
// commander except (0 for all of them): to call once the change is
// committed. members is the guild's member ids (ListGuildMemberIDs).
func (game *Game) tellGuild(client *connection.Client, members []uint32, except uint32, msgs ...proto.Message) {
	for _, id := range members {
		if id != except {
			game.deliver(client, id, msgs...)
		}
	}
}

// handleGuildInfo answers GetGuildInfoCommand (CS_60037): the commander's
// guild, or a guild id of 0 outside one (GuildProxy then exits it; the
// official server still fills policy, faction and level with 1). A new
// guild makes GuildProxy ask for the chat, the user info, the assault fleets,
// the operation and the requests. A pending impeachment is settled first.
func (game *Game) handleGuildInfo(client *connection.Client, req *protobuf.CS_60037) error {
	ctx := context.Background()
	member, ok, err := guildMembership(ctx, game.DB.Queries, client.CommanderID)
	if err != nil {
		return err
	}
	if !ok {
		return client.Send(noGuild())
	}
	if err := game.settleGuildImpeachment(ctx, client, member.GuildID); err != nil {
		return err
	}
	info, err := game.guildInfo(ctx, game.DB.Queries, member.GuildID)
	if err != nil {
		return err
	}
	return client.Send(&protobuf.SC_60000{Guild: info})
}

// handleCreateGuild founds a guild (CreateGuildCommand, CS_60001) for
// create_guild_cost gems, the commander its commander. On result 0 the
// client builds the guild from what it sent and the reply's id, takes the
// gems itself and starts the default technology (guild_tech_default) on its
// side; 2015 is "name invalid". Built from the Lua alone.
func (game *Game) handleCreateGuild(client *connection.Client, req *protobuf.CS_60001) error {
	ctx := context.Background()
	name, manifesto := req.GetName(), req.GetManifesto()
	var guildID uint32
	err := game.tx(ctx, client, func(tx *Tx) error {
		if !validNameUpTo(name, guildNameLength) || !utf8.ValidString(manifesto) {
			return errGuildNameTaken
		}
		if policy := req.GetPolicy(); policy != guildPolicyPower && policy != guildPolicyRelaxation {
			return errRefused
		}
		if faction := req.GetFaction(); faction != guildFactionAzurLane && faction != guildFactionCrimson {
			return errRefused
		}
		_, inGuild, err := guildMembership(ctx, tx.Queries, client.CommanderID)
		if err != nil {
			return err
		}
		if inGuild {
			return errRefused
		}
		taken, err := tx.GuildNameTaken(ctx, db.GuildNameTakenParams{Name: name})
		if err != nil {
			return err
		}
		if taken {
			return errGuildNameTaken
		}
		if err := tx.SpendShopResource(ResourceGem, guildCreateCost); err != nil {
			return err
		}
		now := uint32(time.Now().Unix())
		if guildID, err = tx.CreateGuild(ctx, db.CreateGuildParams{
			Name: name, Policy: req.GetPolicy(), Faction: req.GetFaction(), Manifesto: manifesto, CreatedAt: now,
		}); err != nil {
			return err
		}
		if err := tx.AddGuildMember(ctx, db.AddGuildMemberParams{
			CommanderID: client.CommanderID, GuildID: guildID, Duty: guildDutyCommander, JoinTime: now,
		}); err != nil {
			return err
		}
		return game.guildCreated(tx, guildID)
	})
	if err != nil && !refused(err) {
		return err
	}
	return client.Send(&protobuf.SC_60002{Result: result(err), Id: proto.Uint32(guildID)})
}

// guildCreated sets up what a new guild starts with besides its row and its
// commander (the default technology...).
func (game *Game) guildCreated(tx *Tx, guildID uint32) error {
	return game.guildTechnologiesCreated(tx, guildID)
}

// publicGuildTechs are the public guild technologies a commander outside a
// guild sees (GetPublicGuildUserDataCommand), with the progress the official
// server reports for a new commander (captured).
var publicGuildTechs = [][2]uint32{{6012, 0}, {5009, 20}, {4009, 20}, {3030, 0}, {2030, 0}}

// handlePublicGuildTech answers GetPublicGuildUserDataCommand: the public
// guild technologies, none started.
func (game *Game) handlePublicGuildTech(client *connection.Client, req *protobuf.CS_62100) error {
	reply := &protobuf.SC_62101{}
	for _, tech := range publicGuildTechs {
		reply.Technologys = append(reply.Technologys, &protobuf.GUILD_TECHNOLOGY{
			Id: proto.Uint32(tech[0]), Progress: proto.Uint32(tech[1]),
		})
	}
	return client.Send(reply)
}

// handleGuildUserInfo answers GuildGetUserInfoCommand: no donations or
// benefits yet, today's donation tasks (guild_donate_task ids, captured).
func (game *Game) handleGuildUserInfo(client *connection.Client, req *protobuf.CS_60102) error {
	return client.Send(&protobuf.SC_60103{UserInfo: &protobuf.USER_GUILD_INFO{
		DonateTasks: []uint32{1, 2, 20},
	}})
}

// handleGuildShop answers GetGuildShopCommand: outside a guild the shop is
// empty and never refreshes.
func (game *Game) handleGuildShop(client *connection.Client, req *protobuf.CS_60033) error {
	return client.Send(&protobuf.SC_60034{Result: result(nil)})
}
