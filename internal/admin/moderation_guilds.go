package admin

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"sort"
	"time"

	"github.com/ggmolly/belfast/internal/db"
	"github.com/ggmolly/belfast/internal/game"
	"github.com/ggmolly/belfast/internal/gamedata"
)

func (server *Server) registerGuilds(api apiFunc) {
	api("GET /api/guilds", server.handleGuilds)
	api("GET /api/guilds/{id}", server.handleGuild)
	api("POST /api/guilds/{id}", server.handleEditGuild)
	api("DELETE /api/guilds/{id}", server.handleDisbandGuild)
	api("POST /api/guilds/{id}/members/{member}", server.handleSetGuildDuty)
	api("DELETE /api/guilds/{id}/members/{member}", server.handleRemoveGuildMember)
	api("GET /api/guilds/{id}/operations", server.handleGuildOps)
	api("POST /api/guilds/{id}/requests/{member}/{action}", server.handleGuildRequest)
	api("POST /api/guilds/{id}/operation/{action}", server.handleGuildOperation)
}

// GuildConst.CMD_TYPE_* and the log line's words (GuildLogInfo:getConent)
var guildLogWords = map[uint32]string{1: "joined", 2: "duty", 3: "left", 4: "removed"}

type guildRow struct {
	ID         uint32 `json:"id"`
	Name       string `json:"name"`
	Faction    uint32 `json:"faction"`
	Policy     uint32 `json:"policy"`
	Level      uint32 `json:"level"`
	Exp        uint32 `json:"exp"`
	ExpNext    uint32 `json:"exp_next"`
	Capital    uint32 `json:"capital"`
	Members    int64  `json:"members"`
	MaxMembers uint32 `json:"max_members"`
	Requests   int64  `json:"requests"`
	Leader     string `json:"leader"`
	CreatedAt  uint32 `json:"created_at"`
}

func (server *Server) guildRow(ctx context.Context, tables *moderationTables, guild db.Guild, members, requests int64, leader string) (guildRow, error) {
	maxMembers, err := server.Game.GuildMaxMembers(ctx, guild)
	if err != nil {
		return guildRow{}, err
	}
	return guildRow{
		ID: guild.ID, Name: guild.Name, Faction: guild.Faction, Policy: guild.Policy, Level: guild.Level, Exp: guild.Exp,
		ExpNext: tables.levels[guild.Level].Exp, Capital: guild.Capital, Members: members, MaxMembers: maxMembers,
		Requests: requests, Leader: leader, CreatedAt: guild.CreatedAt,
	}, nil
}

// handleGuilds lists the guilds, the highest level first.
func (server *Server) handleGuilds(r *http.Request) (any, error) {
	tables, err := server.tables()
	if err != nil {
		return nil, err
	}
	guilds, err := server.Store.ModerationListGuilds(r.Context())
	if err != nil {
		return nil, err
	}
	rows := make([]guildRow, 0, len(guilds))
	for _, row := range guilds {
		guild, err := server.guildRow(r.Context(), tables, row.Guild, row.MemberCount, row.RequestCount, row.Leader)
		if err != nil {
			return nil, err
		}
		rows = append(rows, guild)
	}
	return rows, nil
}

// guild is the {id} guild, 404 when there is none.
func (server *Server) guild(r *http.Request) (db.Guild, error) {
	id, err := pathID(r)
	if err != nil {
		return db.Guild{}, err
	}
	guild, err := server.Store.GetGuild(r.Context(), id)
	if errors.Is(err, sql.ErrNoRows) {
		return guild, notFound("no guild " + itoa(id))
	}
	return guild, err
}

type guildMemberRow struct {
	ID       uint32 `json:"id"`
	Name     string `json:"name"`
	Level    uint32 `json:"level"`
	Duty     uint32 `json:"duty"`
	Liveness uint32 `json:"liveness"`
	JoinTime uint32 `json:"join_time"`
	// its avatar, its flagship's icon
	Icon       string `json:"icon,omitempty"`
	Rarity     uint32 `json:"rarity,omitempty"`
	Online     bool   `json:"online"`
	LastOnline int64  `json:"last_online"`
}

type guildLogRow struct {
	Time uint32 `json:"time"`
	Kind string `json:"kind"`
	Name string `json:"name"`
	Duty uint32 `json:"duty,omitempty"`
}

type operationSummary struct {
	Name      string `json:"name"`
	Open      bool   `json:"open"`
	Ended     bool   `json:"ended"`
	StartTime uint32 `json:"start_time"`
	EndTime   uint32 `json:"end_time"`
	MainDone  int    `json:"main_done"`
	MainCount int    `json:"main_count"`
	Joined    int    `json:"joined"`
}

type guildDetail struct {
	guildRow
	Announce  string            `json:"announce"`
	Manifesto string            `json:"manifesto"`
	TechLevel uint32            `json:"tech_level"`
	TechMax   uint32            `json:"tech_max"`
	Operation *operationSummary `json:"operation"`
	Roster    []guildMemberRow  `json:"roster"`
	Logs      []guildLogRow     `json:"logs"`
}

// guildTechRow is a technology group of a guild.
type guildTechRow struct {
	Group    uint32 `json:"group"`
	Name     string `json:"name"`
	Level    uint32 `json:"level"`
	LevelMax uint32 `json:"level_max"`
	Progress uint32 `json:"progress"`
	Exp      uint32 `json:"exp"`
	Active   bool   `json:"active"`
}

// guildTechs lists every technology group, at the level the guild reached
// (its level 0 template without a row).
func (server *Server) guildTechs(ctx context.Context, tables *moderationTables, guildID uint32) ([]guildTechRow, error) {
	rows, err := server.Store.ListGuildTechnologies(ctx, guildID)
	if err != nil {
		return nil, err
	}
	reached := map[uint32]db.GuildTechnology{}
	for _, row := range rows {
		reached[row.GroupID] = row
	}
	start := map[uint32]uint32{}
	for id, tech := range tables.techs {
		if at, ok := start[tech.Group]; !ok || tech.Level < tables.techs[at].Level {
			start[tech.Group] = id
		}
	}
	list := make([]guildTechRow, 0, len(start))
	for group, id := range start {
		row, ok := reached[group]
		if ok {
			id = row.TechID
		}
		tech := tables.techs[id]
		list = append(list, guildTechRow{
			Group: group, Name: tech.Name, Level: tech.Level, LevelMax: tech.LevelMax,
			Progress: row.Progress, Exp: tech.Exp, Active: row.Active,
		})
	}
	sort.Slice(list, func(i, j int) bool { return list[i].Group < list[j].Group })
	return list, nil
}

func (tables *moderationTables) summary(view *game.GuildOperationView) *operationSummary {
	if view == nil {
		return nil
	}
	summary := &operationSummary{
		Name: tables.operations[view.OperationID].Name, Open: view.Open, Ended: view.Ended,
		StartTime: view.StartTime, EndTime: view.EndTime, Joined: len(view.Members),
	}
	for _, mission := range view.Missions {
		if mission.SubType == 1 {
			summary.MainCount++
			if mission.Finished {
				summary.MainDone++
			}
		}
	}
	return summary
}

// handleGuild is a guild with its members, log, research and operation.
func (server *Server) handleGuild(r *http.Request) (any, error) {
	tables, err := server.tables()
	if err != nil {
		return nil, err
	}
	guild, err := server.guild(r)
	if err != nil {
		return nil, err
	}
	ctx := r.Context()
	members, err := server.Store.ListGuildMembers(ctx, guild.ID)
	if err != nil {
		return nil, err
	}
	requests, err := server.Store.CountGuildRequests(ctx, guild.ID)
	if err != nil {
		return nil, err
	}
	icons, err := server.avatars(ctx)
	if err != nil {
		return nil, err
	}
	detail := guildDetail{Announce: guild.Announce, Manifesto: guild.Manifesto, Roster: []guildMemberRow{}, Logs: []guildLogRow{}}
	leader := ""
	for _, row := range members {
		member, commander := row.GuildMember, row.Commander
		if member.Duty == 1 {
			leader = commander.Name
		}
		detail.Roster = append(detail.Roster, guildMemberRow{
			ID: commander.ID, Name: commander.Name, Level: commander.Level, Duty: member.Duty, Liveness: member.Liveness,
			JoinTime: member.JoinTime, Icon: icons[commander.ID].Icon, Rarity: icons[commander.ID].Rarity, Online: server.Game.IsOnline(commander.ID), LastOnline: commander.LastOnline,
		})
	}
	if detail.guildRow, err = server.guildRow(ctx, tables, guild, int64(len(members)), requests, leader); err != nil {
		return nil, err
	}
	logs, err := server.Store.ModerationListGuildLogs(ctx, db.ModerationListGuildLogsParams{GuildID: guild.ID, Count: 50})
	if err != nil {
		return nil, err
	}
	for _, line := range logs {
		row := guildLogRow{Time: line.Time, Kind: guildLogWords[line.Cmd], Name: line.Name}
		if line.Cmd == 2 {
			row.Duty = line.Arg1
		}
		detail.Logs = append(detail.Logs, row)
	}
	techs, err := server.guildTechs(ctx, tables, guild.ID)
	if err != nil {
		return nil, err
	}
	for _, tech := range techs {
		detail.TechLevel += tech.Level
		detail.TechMax += tech.LevelMax
	}
	view, err := server.Game.GuildOperation(ctx, guild.ID)
	if err != nil {
		return nil, err
	}
	detail.Operation = tables.summary(view)
	return detail, nil
}

// handleEditGuild changes a guild's texts, funds or leader.
func (server *Server) handleEditGuild(r *http.Request) (any, error) {
	guild, err := server.guild(r)
	if err != nil {
		return nil, err
	}
	var body struct {
		Name      *string `json:"name"`
		Announce  *string `json:"announce"`
		Manifesto *string `json:"manifesto"`
		GiveFunds uint32  `json:"give_funds"`
		Leader    uint32  `json:"leader"`
	}
	if err := readJSON(r, &body); err != nil {
		return nil, err
	}
	edit := game.GuildEdit{Name: body.Name, Announce: body.Announce, Manifesto: body.Manifesto, GiveFunds: body.GiveFunds, Leader: body.Leader}
	if err := server.Game.EditGuild(r.Context(), guild.ID, edit); err != nil {
		return nil, err
	}
	return struct{}{}, nil
}

// handleDisbandGuild deletes a guild.
func (server *Server) handleDisbandGuild(r *http.Request) (any, error) {
	guild, err := server.guild(r)
	if err != nil {
		return nil, err
	}
	if err := server.Game.DisbandGuild(r.Context(), guild.ID); err != nil {
		return nil, err
	}
	return struct{}{}, nil
}

// handleSetGuildDuty changes a member's duty.
func (server *Server) handleSetGuildDuty(r *http.Request) (any, error) {
	guild, err := server.guild(r)
	if err != nil {
		return nil, err
	}
	member, err := pathUint(r, "member")
	if err != nil {
		return nil, err
	}
	var body struct {
		Duty uint32 `json:"duty"`
	}
	if err := readJSON(r, &body); err != nil {
		return nil, err
	}
	if err := server.Game.SetGuildMemberDuty(r.Context(), guild.ID, member, body.Duty); err != nil {
		return nil, err
	}
	return struct{}{}, nil
}

// handleRemoveGuildMember takes a member out of a guild.
func (server *Server) handleRemoveGuildMember(r *http.Request) (any, error) {
	guild, err := server.guild(r)
	if err != nil {
		return nil, err
	}
	member, err := pathUint(r, "member")
	if err != nil {
		return nil, err
	}
	if err := server.Game.RemoveGuildMember(r.Context(), guild.ID, member); err != nil {
		return nil, err
	}
	return struct{}{}, nil
}

// handleGuildRequest accepts or rejects an application.
func (server *Server) handleGuildRequest(r *http.Request) (any, error) {
	guild, err := server.guild(r)
	if err != nil {
		return nil, err
	}
	member, err := pathUint(r, "member")
	if err != nil {
		return nil, err
	}
	switch r.PathValue("action") {
	case "accept":
		err = server.Game.AcceptGuildRequest(r.Context(), guild.ID, member)
	case "reject":
		err = server.Game.RejectGuildRequest(r.Context(), guild.ID, member)
	default:
		return nil, badRequest("action: accept or reject")
	}
	if err != nil {
		return nil, err
	}
	return struct{}{}, nil
}

// handleGuildOperation ends or starts over the guild's running operation.
func (server *Server) handleGuildOperation(r *http.Request) (any, error) {
	guild, err := server.guild(r)
	if err != nil {
		return nil, err
	}
	switch r.PathValue("action") {
	case "end":
		err = server.Game.EndGuildOperation(r.Context(), guild.ID)
	case "reset":
		err = server.Game.ResetGuildOperation(r.Context(), guild.ID)
	default:
		return nil, badRequest("action: end or reset")
	}
	if err != nil {
		return nil, err
	}
	return struct{}{}, nil
}

type guildRequestRow struct {
	ID      uint32 `json:"id"`
	Name    string `json:"name"`
	Level   uint32 `json:"level"`
	Content string `json:"content"`
	Time    uint32 `json:"time"`
	Icon    string `json:"icon,omitempty"`
	Rarity  uint32 `json:"rarity,omitempty"`
}

type missionRow struct {
	Name     string      `json:"name"`
	Position uint32      `json:"position"`
	Elite    bool        `json:"elite"`
	Side     bool        `json:"side"`
	Progress float64     `json:"progress"`
	Started  bool        `json:"started"`
	Finished bool        `json:"finished"`
	Ships    int         `json:"ships"`
	Nodes    [][2]uint32 `json:"nodes"`
}

type operationDetail struct {
	operationSummary
	Boss struct {
		Name   string `json:"name"`
		HP     uint32 `json:"hp"`
		Damage uint32 `json:"damage"`
		Active bool   `json:"active"`
	} `json:"boss"`
	Missions []missionRow `json:"missions"`
	Ships    int          `json:"ships"`
	Reports  int64        `json:"reports"`
}

type capitalLogRow struct {
	Time   uint32 `json:"time"`
	Name   string `json:"name"`
	Text   string `json:"text"`
	Amount int64  `json:"amount"`
}

type rankRow struct {
	ID    uint32 `json:"id"`
	Name  string `json:"name"`
	Week  uint32 `json:"week"`
	Month uint32 `json:"month"`
	Total uint32 `json:"total"`
}

// capitalLog says a funds log line as GuildCapitalLog:buildText does, with
// the funds it brought or took when the line knows them.
func (tables *moderationTables) capitalLog(row db.GuildCapitalLog) capitalLogRow {
	line := capitalLogRow{Time: row.Time, Name: row.Name}
	target := uint32(0)
	if len(row.EventTarget) > 0 {
		target = row.EventTarget[0]
	}
	switch row.EventType {
	case 1:
		contribution := tables.contributions[target]
		line.Text, line.Amount = contribution.Name, int64(contribution.AwardCapital)
	case 2:
		line.Text = "bought supplies"
	case 3:
		line.Text, line.Amount = "Weekly mission reward", int64(target)
	case 4:
		line.Text, line.Amount = "started an operation", -int64(target)
	case 5:
		line.Text = "switched research to " + tables.techs[target].Name
	case 6:
		line.Text = "researched " + tables.techs[target].Name
	case 7:
		line.Text = "finished " + tables.techs[target].Name
	default:
		line.Text = "event " + itoa(row.EventType)
	}
	return line
}

// handleGuildOps is a guild's operations side: applications, research, the
// latest operation, logistics, the funds log and the member ranks.
func (server *Server) handleGuildOps(r *http.Request) (any, error) {
	tables, err := server.tables()
	if err != nil {
		return nil, err
	}
	guild, err := server.guild(r)
	if err != nil {
		return nil, err
	}
	ctx := r.Context()
	now := time.Now().In(gamedata.Zone())
	reply := struct {
		ID        uint32            `json:"id"`
		Name      string            `json:"name"`
		Capital   uint32            `json:"capital"`
		Members   int64             `json:"members"`
		Requests  []guildRequestRow `json:"requests"`
		Techs     []guildTechRow    `json:"techs"`
		Operation *operationDetail  `json:"operation"`
		Logistics struct {
			Task           string `json:"task"`
			TaskProgress   uint32 `json:"task_progress"`
			TaskMax        uint32 `json:"task_max"`
			SupplyUntil    uint32 `json:"supply_until"`
			DonatedCapital uint32 `json:"donated_capital"`
			DonatedTech    uint32 `json:"donated_tech"`
		} `json:"logistics"`
		CapitalLog []capitalLogRow      `json:"capital_log"`
		Ranks      map[string][]rankRow `json:"ranks"`
	}{ID: guild.ID, Name: guild.Name, Capital: guild.Capital, Requests: []guildRequestRow{}, CapitalLog: []capitalLogRow{}}
	if reply.Members, err = server.Store.CountGuildMembers(ctx, guild.ID); err != nil {
		return nil, err
	}
	requests, err := server.Store.ListGuildRequests(ctx, guild.ID)
	if err != nil {
		return nil, err
	}
	icons, err := server.avatars(ctx)
	if err != nil {
		return nil, err
	}
	for _, row := range requests {
		reply.Requests = append(reply.Requests, guildRequestRow{
			ID: row.Commander.ID, Name: row.Commander.Name, Level: row.Commander.Level, Content: row.GuildRequest.Content, Time: row.GuildRequest.Time, Icon: icons[row.Commander.ID].Icon, Rarity: icons[row.Commander.ID].Rarity,
		})
	}
	if reply.Techs, err = server.guildTechs(ctx, tables, guild.ID); err != nil {
		return nil, err
	}
	view, err := server.Game.GuildOperation(ctx, guild.ID)
	if err != nil {
		return nil, err
	}
	if view != nil {
		op := &operationDetail{operationSummary: *tables.summary(view), Missions: []missionRow{}, Ships: view.Ships, Reports: view.Reports}
		op.Boss.Name, op.Boss.HP, op.Boss.Damage, op.Boss.Active = tables.bosses[view.BossID].Name, view.BossHp, view.BossDamage, view.BossActive
		for _, mission := range view.Missions {
			nodes := mission.Nodes
			if nodes == nil {
				nodes = [][2]uint32{}
			}
			op.Missions = append(op.Missions, missionRow{
				Name: tables.events[mission.EventID].Name, Position: mission.Position, Elite: mission.Type == 2, Side: mission.SubType == 2,
				Progress: mission.Progress, Started: mission.Started, Finished: mission.Finished, Ships: mission.Ships, Nodes: nodes,
			})
		}
		reply.Operation = op
	}
	logistics, err := server.Store.GetGuildLogistics(ctx, guild.ID)
	switch {
	case err == nil:
		week, _ := game.GuildRankPeriods(now)
		if mission, ok := tables.missions[logistics.WeeklyTaskID]; ok && logistics.WeeklyTaskWeek == week {
			reply.Logistics.Task, reply.Logistics.TaskProgress, reply.Logistics.TaskMax = mission.Name, logistics.WeeklyTaskProgress, mission.MaxNum
		}
		reply.Logistics.SupplyUntil = logistics.BenefitFinishTime
		today := uint32(time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location()).Unix())
		if logistics.DonateDay == today {
			reply.Logistics.DonatedCapital, reply.Logistics.DonatedTech = logistics.DonatedCapital, logistics.DonatedTech
		}
	case !errors.Is(err, sql.ErrNoRows):
		return nil, err
	}
	logs, err := server.Store.ModerationListGuildCapitalLogs(ctx, db.ModerationListGuildCapitalLogsParams{GuildID: guild.ID, Count: 50})
	if err != nil {
		return nil, err
	}
	for _, row := range logs {
		reply.CapitalLog = append(reply.CapitalLog, tables.capitalLog(row))
	}
	scores, err := server.Store.ModerationListGuildRankScores(ctx, guild.ID)
	if err != nil {
		return nil, err
	}
	week, month := game.GuildRankPeriods(now)
	kinds := map[uint32]string{1: "donations", 2: "missions", 3: "operations"}
	reply.Ranks = map[string][]rankRow{"donations": {}, "missions": {}, "operations": {}}
	for _, score := range scores {
		kind, ok := kinds[score.Kind]
		if !ok {
			continue
		}
		row := rankRow{ID: score.CommanderID, Name: score.Name, Total: score.Total}
		if score.WeekStart == week {
			row.Week = score.Week
		}
		if score.MonthStart == month {
			row.Month = score.Month
		}
		reply.Ranks[kind] = append(reply.Ranks[kind], row)
	}
	return reply, nil
}
