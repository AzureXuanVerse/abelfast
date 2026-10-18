package game

import (
	"context"
	"database/sql"
	"errors"
	"math/rand/v2"
	"slices"
	"time"

	"google.golang.org/protobuf/proto"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/db"
	"github.com/ggmolly/belfast/internal/gamedata"
	"github.com/ggmolly/belfast/internal/logger"
	"github.com/ggmolly/belfast/internal/protobuf"
)

// The guild office: donations (guild_contribution_template), the weekly
// guild mission (guild_mission_template, with its personal task), the guild
// supplies and the funds log, and the member ranks. Built from the client's
// Lua alone (controller/command/guild/*, model/vo/guild/guild.lua,
// guildproxy.lua): there is no capture of a commander in a guild.

// guild_contribution_template row: a donation, its cost (a DROP_TYPE_*
// resource or item) and what it brings the commander (guild coins,
// liveness) and the guild (funds, tech points)
type guildContributionConfig struct {
	ID                uint32 `json:"id"`
	Type              uint32 `json:"type"`
	TypeID            uint32 `json:"type_id"`
	Consume           uint32 `json:"consume"`
	AwardCapital      uint32 `json:"award_capital"`
	AwardContribution uint32 `json:"award_contribution"`
	AwardTechExp      uint32 `json:"award_tech_exp"`
	GuildActive       uint32 `json:"guild_active"`
}

// guild_mission_template row: a weekly guild mission, its members' personal
// task (task_data_template, type 12), how many completions finish it, the
// guild coins one completion gives a member once a week (award_display)
// and the funds each completion is worth (award_capital_display, paid when
// the mission is finished: GuildTask.GetCaptailAward)
type guildMissionConfig struct {
	ID                  uint32 `json:"id"`
	TaskID              uint32 `json:"task_id"`
	MaxNum              uint32 `json:"max_num"`
	AwardDisplay        uint32 `json:"award_display"`
	AwardCapitalDisplay uint32 `json:"award_capital_display"`
	GuildActive         uint32 `json:"guild_active"`
}

// GuildConst capital log event types (GuildCapitalLog.buildText)
const (
	guildCapitalDonate     = 1 // event_target: the contribution id
	guildCapitalSupply     = 2
	guildCapitalWeeklyTask = 3 // event_target: the funds the mission brought
	guildCapitalBattle     = 4 // event_target: the funds spent
	guildCapitalTechSwitch = 5 // event_target: the technology
	guildCapitalTech       = 6
	guildCapitalTechOver   = 7
)

// the member ranks (GuildRankPage tabs commit, task, battle: CS_62029 type)
// and their periods (GuildRank.SetScore)
const (
	guildRankDonate    = 1
	guildRankMission   = 2
	guildRankOperation = 3

	guildRankWeek  = 1
	guildRankMonth = 2
	guildRankTotal = 3
)

// TaskSubGuildDonate counts the donations (the "Materials Contribution"
// personal tasks, task_data_template sub_type 402).
const TaskSubGuildDonate = 402

// Task.TYPE_* that give guild liveness when claimed (Task.IsGuildAddLivnessType)
const (
	taskTypeRoutine     = 3
	taskTypeWeekly      = 4
	taskTypeGuildWeekly = 12
)

// guildCapitalLogCount is how many funds log lines CS_62011 returns.
const guildCapitalLogCount = 100

var (
	guildContributions   map[uint32]guildContributionConfig
	guildContributionIDs []uint32 // sorted
	guildMissions        map[uint32]guildMissionConfig

	// guildset values: the donations a day (contribution_task_num), the
	// liveness that makes a recruit a regular member
	// (guild_active_become_regular), the supplies' price, duration and drop
	// (guild_award_consume, guild_award_duration, guild_award_drop) and how
	// many days of them can pile up (guild_award_max, GuildConst
	// MAX_SUPPLY_CNT), the liveness daily and weekly tasks give
	// (new_daily_task_guild_active, new_weekly_task_guild_active)
	guildDonateTaskNum, guildBecomeRegular                      uint32
	guildSupplyCost, guildSupplyDuration, guildSupplyDrop       uint32
	guildSupplyMax, guildDailyTaskLiveness, guildWeeklyLiveness uint32
	// guildDailyCapital and guildDailyTech cap the funds and tech points a
	// guild gets from donations in a day (guildset base_capital,
	// base_contribution: no Lua reads them; what the "unable to contribute
	// any more today" tips of SC_62019 point at)
	guildDailyCapital, guildDailyTech uint32
)

// loadGuildLogisticsData reads the donations, the weekly missions and the
// guildset values the office uses; guildSet must be loaded.
func loadGuildLogisticsData() error {
	var err error
	if guildContributions, err = gamedata.Table[guildContributionConfig]("guild_contribution_template"); err != nil {
		return err
	}
	guildContributionIDs = guildContributionIDs[:0]
	for id := range guildContributions {
		guildContributionIDs = append(guildContributionIDs, id)
	}
	slices.Sort(guildContributionIDs)
	if guildMissions, err = gamedata.Table[guildMissionConfig]("guild_mission_template"); err != nil {
		return err
	}
	for key, value := range map[string]*uint32{
		"contribution_task_num":        &guildDonateTaskNum,
		"guild_active_become_regular":  &guildBecomeRegular,
		"guild_award_consume":          &guildSupplyCost,
		"guild_award_duration":         &guildSupplyDuration,
		"guild_award_drop":             &guildSupplyDrop,
		"guild_award_max":              &guildSupplyMax,
		"new_daily_task_guild_active":  &guildDailyTaskLiveness,
		"new_weekly_task_guild_active": &guildWeeklyLiveness,
		"base_capital":                 &guildDailyCapital,
		"base_contribution":            &guildDailyTech,
	} {
		if err := guildSetting(key, value); err != nil {
			return err
		}
	}
	return nil
}

// guildWeek is the monday_0clock of now's week, when weekly missions start.
func guildWeek(now time.Time) uint32 {
	return uint32(periodStart(now, week).Unix())
}

// drawDonateTasks picks a day's donations: contribution_task_num distinct
// guild_contribution_template ids, the gold one always first (the official
// server gave a new commander [1 gold, 2 oil, 20 merit], and the wiki says
// gold is always offered), the others at random.
func drawDonateTasks() db.IDList {
	ids := slices.Clone(guildContributionIDs)
	rand.Shuffle(len(ids), func(i, j int) { ids[i], ids[j] = ids[j], ids[i] })
	gold := slices.IndexFunc(ids, func(id uint32) bool {
		config := guildContributions[id]
		return config.Type == dropTypeResource && config.TypeID == ResourceGold
	})
	if gold > 0 {
		ids[0], ids[gold] = ids[gold], ids[0]
	}
	return db.IDList(ids[:min(int(guildDonateTaskNum), len(ids))])
}

// guildUser is the commander's guild state, its donations drawn again when
// a server day began since they were: then the client gets the new ones
// (SC_62031: a guild starts its count again on it, the client asks for
// nothing at midnight in a guild).
func (tx *Tx) guildUser(now time.Time) (user db.GuildUser, err error) {
	if err := tx.CreateGuildUser(tx.ctx, tx.commanderID); err != nil {
		return user, err
	}
	if user, err = tx.GetGuildUser(tx.ctx, tx.commanderID); err != nil {
		return user, err
	}
	today := uint32(startOfDay(now).Unix())
	if user.DonateDay == today {
		return user, nil
	}
	user.DonateDay, user.DonateCount, user.DonateTasks = today, 0, drawDonateTasks()
	tx.push(&protobuf.SC_62031{DonateTasks: user.DonateTasks})
	return user, tx.SetGuildUserDonations(tx.ctx, db.SetGuildUserDonationsParams{
		DonateDay: user.DonateDay, DonateCount: 0, DonateTasks: user.DonateTasks, ExtraDonate: user.ExtraDonate, CommanderID: tx.commanderID,
	})
}

// guildLogistics is the guild's logistics row, zero before anything
// happened.
func guildLogistics(ctx context.Context, queries *db.Queries, guildID uint32) (db.GuildLogistic, error) {
	row, err := queries.GetGuildLogistics(ctx, guildID)
	if errors.Is(err, sql.ErrNoRows) {
		return db.GuildLogistic{GuildID: guildID}, nil
	}
	return row, err
}

// guildLogisticsForUpdate is guildLogistics with its row created, to update.
func (tx *Tx) guildLogisticsForUpdate(guildID uint32) (db.GuildLogistic, error) {
	if err := tx.CreateGuildLogistics(tx.ctx, guildID); err != nil {
		return db.GuildLogistic{}, err
	}
	return tx.GetGuildLogistics(tx.ctx, guildID)
}

// activeGuildMission is the guild's weekly mission, ok false when none was
// picked this week.
func activeGuildMission(logistics db.GuildLogistic, now time.Time) (guildMissionConfig, bool) {
	mission, ok := guildMissions[logistics.WeeklyTaskID]
	return mission, ok && logistics.WeeklyTaskWeek == guildWeek(now)
}

// weeklyTaskInfo is the guild's weekly mission as WEEKLY_TASK: id 0 when
// there is none this week (GuildTask STATE_EMPTY, the administrators pick
// one).
func weeklyTaskInfo(logistics db.GuildLogistic, now time.Time) *protobuf.WEEKLY_TASK {
	if _, ok := activeGuildMission(logistics, now); !ok {
		return &protobuf.WEEKLY_TASK{Id: proto.Uint32(0), Progress: proto.Uint32(0), Monday_0Clock: proto.Uint32(0)}
	}
	return &protobuf.WEEKLY_TASK{
		Id:            proto.Uint32(logistics.WeeklyTaskID),
		Progress:      proto.Uint32(logistics.WeeklyTaskProgress),
		Monday_0Clock: proto.Uint32(logistics.WeeklyTaskWeek),
	}
}

// fillGuildLogistics adds the guild's weekly task and supplies to its
// GUILD_EXPANSION_INFO (Guild.updateExtraInfo).
func (game *Game) fillGuildLogistics(ctx context.Context, queries *db.Queries, guild db.Guild, expansion *protobuf.GUILD_EXPANSION_INFO) error {
	logistics, err := guildLogistics(ctx, queries, guild.ID)
	if err != nil {
		return err
	}
	expansion.ThisWeeklyTasks = weeklyTaskInfo(logistics, serverNow())
	expansion.BenefitFinishTime = proto.Uint32(logistics.BenefitFinishTime)
	expansion.LastBenefitFinishTime = proto.Uint32(logistics.LastBenefitFinishTime)
	return nil
}

// guildNotices are the pushes a guild request makes to the guild's members
// online, sent once it is committed (tellGuildNotices).
type guildNotices struct {
	members  []uint32
	everyone []proto.Message // the commander included, ahead of its reply
	others   []proto.Message
}

// guildNoticesFor starts the notices to a guild's members.
func (tx *Tx) guildNoticesFor(guildID uint32) (*guildNotices, error) {
	members, err := tx.ListGuildMemberIDs(tx.ctx, guildID)
	if err != nil {
		return nil, err
	}
	return &guildNotices{members: members}, nil
}

// tellGuildNotices sends notices once the change is committed.
func (game *Game) tellGuildNotices(client *connection.Client, notices *guildNotices) {
	if notices == nil {
		return
	}
	game.tellGuild(client, notices.members, 0, notices.everyone...)
	game.tellGuild(client, notices.members, client.CommanderID, notices.others...)
}

// addGuildLiveness adds liveness to a member (GuildMember.AddLiveness): a
// recruit reaching guild_active_become_regular becomes an ordinary member,
// which the others see (SC_60031; the member's own client changes the duty
// itself). The guild gains as much exp, which levels it up
// (guild_data_level, SC_60032 to everyone): no table or Lua says what guild
// exp comes from, the members' liveness is belfast's choice.
func (tx *Tx) addGuildLiveness(game *Game, member db.GuildMember, amount uint32, notices *guildNotices) error {
	if amount == 0 {
		return nil
	}
	liveness, duty := member.Liveness+amount, member.Duty
	if duty == guildDutyRecruit && liveness >= guildBecomeRegular {
		duty = guildDutyOrdinary
	}
	if err := tx.SetGuildMemberLiveness(tx.ctx, db.SetGuildMemberLivenessParams{Liveness: liveness, Duty: duty, CommanderID: member.CommanderID}); err != nil {
		return err
	}
	if duty != member.Duty {
		commander, err := tx.GetCommander(tx.ctx, member.CommanderID)
		if err != nil {
			return err
		}
		member.Liveness, member.Duty = liveness, duty
		notices.others = append(notices.others, &protobuf.SC_60031{MemberList: []*protobuf.MEMBER_INFO{game.guildMemberInfo(member, commander)}})
	}
	return tx.addGuildExp(member.GuildID, amount, notices)
}

// addGuildExp adds exp to a guild, levelling it up while it has the exp its
// level needs (exp counts from 0 at each level: Guild.getLevelMaxExp); at
// the last level the exp stops at that level's.
func (tx *Tx) addGuildExp(guildID, amount uint32, notices *guildNotices) error {
	guild, err := tx.GetGuild(tx.ctx, guildID)
	if err != nil {
		return err
	}
	level, exp := guild.Level, guild.Exp+amount
	for {
		config, ok := guildLevels[level]
		if !ok || exp < config.Exp {
			break
		}
		if _, next := guildLevels[level+1]; !next {
			exp = config.Exp
			break
		}
		exp -= config.Exp
		level++
	}
	if level == guild.Level && exp == guild.Exp {
		return nil
	}
	if err := tx.SetGuildLevel(tx.ctx, db.SetGuildLevelParams{Level: level, Exp: exp, ID: guildID}); err != nil {
		return err
	}
	notices.everyone = append(notices.everyone, &protobuf.SC_60032{Lv: proto.Uint32(level), Exp: proto.Uint32(exp)})
	return nil
}

// addGuildCapitalLog writes a line of the guild's funds log.
func (tx *Tx) addGuildCapitalLog(guildID uint32, commander db.Commander, eventType uint32, targets ...uint32) error {
	return tx.AddGuildCapitalLog(tx.ctx, db.AddGuildCapitalLogParams{
		GuildID: guildID, MemberID: commander.ID, Name: commander.Name, EventType: eventType,
		EventTarget: db.IDList(targets), Time: uint32(time.Now().Unix()),
	})
}

// addGuildRankScore counts n for a member in one of the ranks
// (guildRankDonate...), its week and month counts starting again with each
// week and month.
func (tx *Tx) addGuildRankScore(guildID, commanderID, kind, n uint32, now time.Time) error {
	score, err := tx.GetGuildRankScore(tx.ctx, db.GetGuildRankScoreParams{GuildID: guildID, CommanderID: commanderID, Kind: kind})
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	weekStart, monthStart := guildWeek(now), uint32(startOfMonth(now).Unix())
	if score.WeekStart != weekStart {
		score.Week = 0
	}
	if score.MonthStart != monthStart {
		score.Month = 0
	}
	return tx.SetGuildRankScore(tx.ctx, db.SetGuildRankScoreParams{
		GuildID: guildID, CommanderID: commanderID, Kind: kind,
		WeekStart: weekStart, Week: score.Week + n,
		MonthStart: monthStart, Month: score.Month + n,
		Total: score.Total + n,
	})
}

// handleGuildUserInfo answers GuildGetUserInfoCommand (CS_60102), which
// GetPublicGuildUserDataCommand also sends outside a guild (again at each
// server day): the commander's donations today and today's donation tasks,
// the extra donations and operations, when it last collected the supplies,
// whether it took this week's mission reward and its guild technologies.
// The personal guild mission tasks of past weeks are dropped (the client
// drops them at midnight, ZeroHourCommand). The client asks again at each
// day (for the public guild), which sends a guild's new donations
// (guildUser).
func (game *Game) handleGuildUserInfo(client *connection.Client, req *protobuf.CS_60102) error {
	ctx := context.Background()
	now := serverNow()
	info := &protobuf.USER_GUILD_INFO{}
	err := game.tx(ctx, client, func(tx *Tx) error {
		user, err := tx.guildUser(now)
		if err != nil {
			return err
		}
		if err := tx.dropPastGuildTasks(now); err != nil {
			return err
		}
		techIDs, err := game.guildUserTechIDs(ctx, tx.Queries, client.CommanderID)
		if err != nil {
			return err
		}
		var flag uint32
		if user.WeeklyTaskWeek == guildWeek(now) {
			flag = 1
		}
		info = &protobuf.USER_GUILD_INFO{
			DonateCount:    proto.Uint32(user.DonateCount),
			DonateTasks:    user.DonateTasks,
			BenefitTime:    proto.Uint32(user.BenefitTime),
			TechId:         techIDs,
			WeeklyTaskFlag: proto.Uint32(flag),
			ExtraDonate:    proto.Uint32(user.ExtraDonate),
			ExtraOperation: proto.Uint32(user.ExtraOperation),
		}
		return nil
	})
	if err != nil {
		return err
	}
	return client.Send(&protobuf.SC_60103{UserInfo: info})
}

// dropPastGuildTasks deletes the commander's guild mission personal tasks
// accepted before this week.
func (tx *Tx) dropPastGuildTasks(now time.Time) error {
	tasks, err := tx.ListGuildTasks(tx.ctx, tx.commanderID)
	if err != nil {
		return err
	}
	for _, task := range tasks {
		if taskConfigs[task.TaskID].Type == taskTypeGuildWeekly && task.AcceptTime < guildWeek(now) {
			if err := tx.DeleteCommanderTask(tx.ctx, db.DeleteCommanderTaskParams{CommanderID: tx.commanderID, TaskID: task.TaskID}); err != nil {
				return err
			}
		}
	}
	return nil
}

// handleGuildDonate answers GuildCommitDonateCommand and, outside a guild,
// PublicGuildCommitDonateCommand (CS_62002, told apart by membership): one
// of today's donations, paid with its resource or item, for its guild coins
// (the client adds them and takes the cost itself, then replaces its tasks
// with the reply's). A donation beyond contribution_task_num uses an extra
// one (Guild.updateDonateCount). In a guild it also brings the member its
// liveness and the guild its funds and tech points within the daily caps,
// told to every member online (SC_62019, has_capital / has_tech_point 0 show
// "no more today"). A day that began since the tasks were drawn sends the
// new ones first (SC_62031, the client starts its count again). Result 1
// refuses ("Failed to dissolve guild." errorTip).
func (game *Game) handleGuildDonate(client *connection.Client, req *protobuf.CS_62002) error {
	ctx := context.Background()
	now := serverNow()
	// the day's new donations reach the client even when this one is
	// refused (it may be one of yesterday's)
	err := game.tx(ctx, client, func(tx *Tx) error {
		_, err := tx.guildUser(now)
		return err
	})
	if err != nil {
		return err
	}
	reply := &protobuf.SC_62003{}
	var notices *guildNotices
	err = game.tx(ctx, client, func(tx *Tx) error {
		user, err := tx.guildUser(now)
		if err != nil {
			return err
		}
		config, ok := guildContributions[req.GetId()]
		if !ok || !slices.Contains(user.DonateTasks, req.GetId()) {
			return errRefused
		}
		switch {
		case user.DonateCount < guildDonateTaskNum:
			user.DonateCount++
		case user.ExtraDonate > 0:
			user.ExtraDonate--
		default:
			return errRefused
		}
		switch config.Type {
		case dropTypeResource:
			err = tx.SpendResource(config.TypeID, config.Consume)
		case dropTypeItem:
			err = tx.SpendItem(config.TypeID, config.Consume)
		default:
			err = errRefused
		}
		if err != nil {
			return err
		}
		if err := tx.AddResource(ResourceGuildCoin, config.AwardContribution); err != nil {
			return err
		}
		if err := tx.SetGuildUserDonations(ctx, db.SetGuildUserDonationsParams{
			DonateDay: user.DonateDay, DonateCount: user.DonateCount, DonateTasks: user.DonateTasks, ExtraDonate: user.ExtraDonate, CommanderID: client.CommanderID,
		}); err != nil {
			return err
		}
		tx.event(TaskEvent{SubType: TaskSubGuildDonate, Count: 1})
		reply.DonateTasks = user.DonateTasks
		member, inGuild, err := guildMembership(ctx, tx.Queries, client.CommanderID)
		if err != nil || !inGuild {
			return err
		}
		notices, err = game.guildDonated(tx, member, config, now)
		return err
	})
	if err != nil && !refused(err) {
		return err
	}
	if err == nil {
		game.tellGuildNotices(client, notices)
	}
	reply.Result = result(err)
	return client.Send(reply)
}

// guildDonated is what a member's donation brings its guild: funds and tech
// points (to the running technology) while under the day's caps, a funds
// log line, the member's liveness and rank.
func (game *Game) guildDonated(tx *Tx, member db.GuildMember, config guildContributionConfig, now time.Time) (*guildNotices, error) {
	guildID := member.GuildID
	notices, err := tx.guildNoticesFor(guildID)
	if err != nil {
		return nil, err
	}
	logistics, err := tx.guildLogisticsForUpdate(guildID)
	if err != nil {
		return nil, err
	}
	if today := uint32(startOfDay(now).Unix()); logistics.DonateDay != today {
		logistics.DonateDay, logistics.DonatedCapital, logistics.DonatedTech = today, 0, 0
	}
	var hasCapital, hasTech uint32
	if logistics.DonatedCapital+config.AwardCapital <= guildDailyCapital {
		if err := tx.AddGuildCapital(tx.ctx, db.AddGuildCapitalParams{Capital: config.AwardCapital, ID: guildID}); err != nil {
			return nil, err
		}
		logistics.DonatedCapital += config.AwardCapital
		hasCapital = 1
	}
	if logistics.DonatedTech+config.AwardTechExp <= guildDailyTech {
		taken, err := tx.addGuildTechProgress(guildID, config.AwardTechExp)
		if err != nil {
			return nil, err
		}
		if taken {
			logistics.DonatedTech += config.AwardTechExp
			hasTech = 1
		}
	}
	if err := tx.SetGuildDonated(tx.ctx, db.SetGuildDonatedParams{
		DonateDay: logistics.DonateDay, DonatedCapital: logistics.DonatedCapital, DonatedTech: logistics.DonatedTech, GuildID: guildID,
	}); err != nil {
		return nil, err
	}
	commander, err := tx.GetCommander(tx.ctx, member.CommanderID)
	if err != nil {
		return nil, err
	}
	if err := tx.addGuildCapitalLog(guildID, commander, guildCapitalDonate, config.ID); err != nil {
		return nil, err
	}
	if err := tx.addGuildRankScore(guildID, member.CommanderID, guildRankDonate, 1, now); err != nil {
		return nil, err
	}
	notices.everyone = append(notices.everyone, &protobuf.SC_62019{
		Id: proto.Uint32(config.ID), UserId: proto.Uint32(member.CommanderID),
		HasCapital: proto.Uint32(hasCapital), HasTechPoint: proto.Uint32(hasTech),
	})
	return notices, tx.addGuildLiveness(game, member, config.GuildActive, notices)
}

// useGuildOrders spends count Guild Contribution Directives (an extra
// donation each) or Guild Operation Orders (an extra operation each), in a
// guild only (UseItemCommand: not_exist_guild_use_item); on result 0 the
// client adds them itself (Guild:AddExtraDonateCnt / AddExtraBattleCnt).
func (tx *Tx) useGuildOrders(id, count uint32, usage string) error {
	_, inGuild, err := guildMembership(tx.ctx, tx.Queries, tx.commanderID)
	if err != nil {
		return err
	}
	if !inGuild {
		return errRefused
	}
	if err := tx.SpendItem(id, count); err != nil {
		return err
	}
	if err := tx.CreateGuildUser(tx.ctx, tx.commanderID); err != nil {
		return err
	}
	extras := db.AddGuildUserExtrasParams{CommanderID: tx.commanderID}
	if usage == usageGuildDonate {
		extras.ExtraDonate = count
	} else {
		extras.ExtraOperation = count
	}
	return tx.AddGuildUserExtras(tx.ctx, extras)
}

// handleGuildBuySupply answers GuildBuySupplyCommand (CS_62007): an
// administrator spends guild_award_consume funds on guild_award_duration of
// supplies, when none are running. Every member online is told when they
// end (SC_62005: GuildProxy takes the funds off and opens the supplies).
func (game *Game) handleGuildBuySupply(client *connection.Client, req *protobuf.CS_62007) error {
	ctx := context.Background()
	var notices *guildNotices
	err := game.tx(ctx, client, func(tx *Tx) error {
		member, err := guildAdministratorTx(tx)
		if err != nil {
			return err
		}
		logistics, err := tx.guildLogisticsForUpdate(member.GuildID)
		if err != nil {
			return err
		}
		now := uint32(time.Now().Unix())
		if logistics.BenefitFinishTime > now {
			return errRefused
		}
		spent, err := tx.SpendGuildCapital(ctx, db.SpendGuildCapitalParams{Amount: guildSupplyCost, ID: member.GuildID})
		if err != nil {
			return err
		}
		if spent == 0 {
			return ErrNotEnough
		}
		finish := now + guildSupplyDuration
		if err := tx.SetGuildBenefit(ctx, db.SetGuildBenefitParams{
			BenefitFinishTime: finish, LastBenefitFinishTime: logistics.BenefitFinishTime, GuildID: member.GuildID,
		}); err != nil {
			return err
		}
		commander, err := tx.GetCommander(ctx, client.CommanderID)
		if err != nil {
			return err
		}
		if err := tx.addGuildCapitalLog(member.GuildID, commander, guildCapitalSupply); err != nil {
			return err
		}
		if notices, err = tx.guildNoticesFor(member.GuildID); err != nil {
			return err
		}
		notices.everyone = append(notices.everyone, &protobuf.SC_62005{BenefitFinishTime: proto.Uint32(finish)})
		return nil
	})
	if err != nil && !refused(err) {
		return err
	}
	if err == nil {
		game.tellGuildNotices(client, notices)
	}
	return client.Send(&protobuf.SC_62008{Result: result(err)})
}

// guildSupplyCount is the days of supplies the member can collect (at most
// guild_award_max): one per server day a purchase ran, the running one's and
// the previous one's, from the member's join day, after the day it last
// collected (benefitTime, that day's midnight). Guild.getSupplyCnt counts
// the same spans in seconds and, as the client stores the collection day's
// midnight, offers the day a purchase began twice; counting days doesn't.
func guildSupplyCount(logistics db.GuildLogistic, member db.GuildMember, benefitTime uint32, now time.Time) uint32 {
	today := startOfDay(now).Unix()
	from := startOfDay(time.Unix(int64(member.JoinTime), 0)).Unix()
	if benefitTime > 0 {
		from = max(from, startOfDay(time.Unix(int64(benefitTime), 0)).AddDate(0, 0, 1).Unix())
	}
	// days of a purchase that ended at finish (or runs until it)
	days := func(finish uint32) int64 {
		if finish == 0 {
			return 0
		}
		first := max(from, startOfDay(time.Unix(int64(finish)-int64(guildSupplyDuration), 0)).Unix())
		last := min(today, startOfDay(time.Unix(int64(finish)-1, 0)).Unix())
		if last < first {
			return 0
		}
		return int64(time.Unix(last, 0).Sub(time.Unix(first, 0)).Round(time.Hour)/(24*time.Hour)) + 1
	}
	count := days(logistics.BenefitFinishTime) + days(logistics.LastBenefitFinishTime)
	return uint32(min(count, int64(guildSupplyMax)))
}

// handleGuildSupplyAward answers GuildGetSupplyAwardCommand (CS_62009): the
// days of supplies the member can collect, one draw of the supplies' drop
// (guildset guild_award_drop) each. Members who joined today and recruits
// can't. The client then sets its benefitTime to today's midnight, or to the
// supplies' end once they ran out, as stored here. guild_award_drop names a
// server-side drop table: it is read from server/drop_packs.json, without
// an entry there the supplies give nothing.
func (game *Game) handleGuildSupplyAward(client *connection.Client, req *protobuf.CS_62009) error {
	ctx := context.Background()
	now := serverNow()
	reply := &protobuf.SC_62010{}
	err := game.tx(ctx, client, func(tx *Tx) error {
		member, ok, err := guildMembership(ctx, tx.Queries, client.CommanderID)
		if err != nil {
			return err
		}
		if !ok || member.Duty == guildDutyRecruit || sameServerDay(time.Unix(int64(member.JoinTime), 0), now) {
			return errRefused
		}
		user, err := tx.guildUser(now)
		if err != nil {
			return err
		}
		logistics, err := guildLogistics(ctx, tx.Queries, member.GuildID)
		if err != nil {
			return err
		}
		count := guildSupplyCount(logistics, member, user.BenefitTime, now)
		if logistics.BenefitFinishTime == 0 || count == 0 {
			return errRefused
		}
		pack, ok := dropPacks[guildSupplyDrop]
		if ok {
			reply.DropList = pack.draw(count)
		} else {
			logger.WithFields("Guild", logger.FieldValue("drop", guildSupplyDrop)).Warn("guild supplies: no drop table")
		}
		if err := tx.grantDrops(reply.DropList); err != nil {
			return err
		}
		benefit := uint32(startOfDay(now).Unix())
		if now.Unix() >= int64(logistics.BenefitFinishTime) {
			benefit = logistics.BenefitFinishTime
		}
		return tx.SetGuildUserBenefitTime(ctx, db.SetGuildUserBenefitTimeParams{BenefitTime: benefit, CommanderID: client.CommanderID})
	})
	if err != nil && !refused(err) {
		return err
	}
	reply.Result = result(err)
	return client.Send(reply)
}

// handleGuildCapitalLog answers GuildFetchCapitalLogCommand (CS_62011): the
// latest lines of the funds log. The client shows the three lists one after
// the other: gains, spendings, the rest.
func (game *Game) handleGuildCapitalLog(client *connection.Client, req *protobuf.CS_62011) error {
	ctx := context.Background()
	member, ok, err := guildMembership(ctx, game.DB.Queries, client.CommanderID)
	if err != nil {
		return err
	}
	if !ok {
		return client.Send(&protobuf.SC_62012{Result: result(errNotInGuild)})
	}
	rows, err := game.DB.ListGuildCapitalLogs(ctx, db.ListGuildCapitalLogsParams{GuildID: member.GuildID, Count: guildCapitalLogCount})
	if err != nil {
		return err
	}
	reply := &protobuf.SC_62012{Result: result(nil)}
	for _, row := range rows {
		line := &protobuf.CAPITAL_LOG{
			MemberId: proto.Uint32(row.MemberID), Name: proto.String(row.Name), EventType: proto.Uint32(row.EventType),
			EventTarget: row.EventTarget, Time: proto.Uint32(row.Time),
		}
		switch row.EventType {
		case guildCapitalDonate, guildCapitalWeeklyTask:
			reply.Inclog = append(reply.Inclog, line)
		case guildCapitalSupply, guildCapitalBattle, guildCapitalTech:
			reply.Declog = append(reply.Declog, line)
		default:
			reply.Otherlog = append(reply.Otherlog, line)
		}
	}
	return client.Send(reply)
}

// handleGuildCapital answers GuildFetchCapitalCommand (CS_62024): the
// guild's funds.
func (game *Game) handleGuildCapital(client *connection.Client, req *protobuf.CS_62024) error {
	ctx := context.Background()
	member, ok, err := guildMembership(ctx, game.DB.Queries, client.CommanderID)
	if err != nil {
		return err
	}
	if !ok {
		return client.Send(&protobuf.SC_62025{Result: result(errNotInGuild)})
	}
	guild, err := game.DB.GetGuild(ctx, member.GuildID)
	if err != nil {
		return err
	}
	return client.Send(&protobuf.SC_62025{Result: result(nil), Capital: proto.Uint32(guild.Capital)})
}

// handleGuildSelectWeeklyTask answers GuildSelectWeeklyTaskCommand
// (CS_62013): an administrator picks the week's guild mission when there is
// none; every member online gets it (SC_62004, the client also clears its
// weekly task flag).
func (game *Game) handleGuildSelectWeeklyTask(client *connection.Client, req *protobuf.CS_62013) error {
	ctx := context.Background()
	now := serverNow()
	var notices *guildNotices
	err := game.tx(ctx, client, func(tx *Tx) error {
		member, err := guildAdministratorTx(tx)
		if err != nil {
			return err
		}
		if _, ok := guildMissions[req.GetId()]; !ok {
			return errRefused
		}
		logistics, err := tx.guildLogisticsForUpdate(member.GuildID)
		if err != nil {
			return err
		}
		if _, active := activeGuildMission(logistics, now); active {
			return errRefused
		}
		logistics.WeeklyTaskID, logistics.WeeklyTaskProgress, logistics.WeeklyTaskWeek = req.GetId(), 0, guildWeek(now)
		if err := tx.SetGuildWeeklyTask(ctx, db.SetGuildWeeklyTaskParams{
			WeeklyTaskID: logistics.WeeklyTaskID, WeeklyTaskProgress: 0, WeeklyTaskWeek: logistics.WeeklyTaskWeek, GuildID: member.GuildID,
		}); err != nil {
			return err
		}
		if notices, err = tx.guildNoticesFor(member.GuildID); err != nil {
			return err
		}
		notices.everyone = append(notices.everyone, &protobuf.SC_62004{ThisWeeklyTasks: weeklyTaskInfo(logistics, now)})
		return nil
	})
	if err != nil && !refused(err) {
		return err
	}
	if err == nil {
		game.tellGuildNotices(client, notices)
	}
	return client.Send(&protobuf.SC_62014{Result: result(err)})
}

// handleGuildWeeklyTaskProgress answers GuildFetchWeeklyTaskProgreeCommand
// (CS_62022, polled every minute on the office page): the guild mission's
// progress, its members' completions of the personal task.
func (game *Game) handleGuildWeeklyTaskProgress(client *connection.Client, req *protobuf.CS_62022) error {
	ctx := context.Background()
	member, ok, err := guildMembership(ctx, game.DB.Queries, client.CommanderID)
	if err != nil {
		return err
	}
	if !ok {
		return client.Send(&protobuf.SC_62023{Result: result(errNotInGuild)})
	}
	logistics, err := guildLogistics(ctx, game.DB.Queries, member.GuildID)
	if err != nil {
		return err
	}
	return client.Send(&protobuf.SC_62023{Result: result(nil), Progress: weeklyTaskInfo(logistics, serverNow()).Progress})
}

// handleGuildRank answers GetGuildRankCommand (CS_62029, type the rank:
// donations, missions, operations): each member's count this week, this
// month and in all.
func (game *Game) handleGuildRank(client *connection.Client, req *protobuf.CS_62029) error {
	ctx := context.Background()
	member, ok, err := guildMembership(ctx, game.DB.Queries, client.CommanderID)
	if err != nil {
		return err
	}
	reply := &protobuf.SC_62030{}
	if !ok {
		return client.Send(reply)
	}
	scores, err := game.DB.ListGuildRankScores(ctx, db.ListGuildRankScoresParams{GuildID: member.GuildID, Kind: req.GetType()})
	if err != nil {
		return err
	}
	now := serverNow()
	weekStart, monthStart := guildWeek(now), uint32(startOfMonth(now).Unix())
	periods := map[uint32]*protobuf.RANK_INFO_P62{}
	for _, period := range []uint32{guildRankWeek, guildRankMonth, guildRankTotal} {
		periods[period] = &protobuf.RANK_INFO_P62{Period: proto.Uint32(period)}
		reply.List = append(reply.List, periods[period])
	}
	add := func(period, commanderID, count uint32) {
		periods[period].Rankuserinfo = append(periods[period].Rankuserinfo, &protobuf.RANK_USER_INFO{UserId: proto.Uint32(commanderID), Count: proto.Uint32(count)})
	}
	for _, score := range scores {
		week, month := score.Week, score.Month
		if score.WeekStart != weekStart {
			week = 0
		}
		if score.MonthStart != monthStart {
			month = 0
		}
		add(guildRankWeek, score.CommanderID, week)
		add(guildRankMonth, score.CommanderID, month)
		add(guildRankTotal, score.CommanderID, score.Total)
	}
	return client.Send(reply)
}

// handleTriggerTask answers TriggerTaskCommand (CS_20007) for the guild
// mission's personal task (GuildOfficeMediator.ON_ACCEPT_TASK): it is given
// again once claimed, the week's mission counting each completion. Other
// tasks the client triggers (technology, blueprints...) are refused for now.
// The client adds the task at progress 0 itself.
func (game *Game) handleTriggerTask(client *connection.Client, req *protobuf.CS_20007) error {
	ctx := context.Background()
	now := serverNow()
	reply := &protobuf.SC_20008{}
	err := game.tx(ctx, client, func(tx *Tx) error {
		member, ok, err := guildMembership(ctx, tx.Queries, client.CommanderID)
		if err != nil {
			return err
		}
		if !ok {
			return errRefused
		}
		logistics, err := guildLogistics(ctx, tx.Queries, member.GuildID)
		if err != nil {
			return err
		}
		mission, active := activeGuildMission(logistics, now)
		if !active || mission.TaskID != req.GetId() {
			return errRefused
		}
		task, err := tx.GetTask(ctx, db.GetTaskParams{CommanderID: client.CommanderID, TaskID: req.GetId()})
		switch {
		case err == nil && task.SubmitTime == 0 && task.AcceptTime >= logistics.WeeklyTaskWeek:
			reply.Task = &protobuf.TASK_ADD{Id: proto.Uint32(task.TaskID), Progress: proto.Uint32(task.Progress), AcceptTime: proto.Uint32(task.AcceptTime), SubmitTime: proto.Uint32(0)}
			return nil
		case err != nil && !errors.Is(err, sql.ErrNoRows):
			return err
		}
		if err := tx.DeleteCommanderTask(ctx, db.DeleteCommanderTaskParams{CommanderID: client.CommanderID, TaskID: req.GetId()}); err != nil {
			return err
		}
		added, err := acceptTasks(ctx, tx.Queries, client.CommanderID, []uint32{req.GetId()}, uint32(time.Now().Unix()))
		if err != nil {
			return err
		}
		if added == nil {
			return errRefused
		}
		reply.Task = added.Info[0]
		return nil
	})
	if err != nil && !refused(err) {
		return err
	}
	reply.Result = result(err)
	return client.Send(reply)
}

// guildTaskClaimed is what claiming a task brings a guild member
// (SubmitTaskCommand.AddGuildLivness): liveness for daily, weekly and guild
// mission tasks; for the guild mission's personal task also a completion of
// the mission (its funds paid to the guild when it is finished, a funds log
// line), a rank point, and its guild coins once a week (the drops returned,
// part of the claim's awards). The client sets its weekly task flag itself.
func (game *Game) guildTaskClaimed(tx *Tx, taskID uint32, config taskConfig) ([]*protobuf.DROPINFO, *guildNotices, error) {
	var liveness uint32
	switch config.Type {
	case taskTypeRoutine:
		liveness = guildDailyTaskLiveness
	case taskTypeWeekly, taskTypeNewWeekly:
		liveness = guildWeeklyLiveness
	case taskTypeGuildWeekly:
	default:
		return nil, nil, nil
	}
	member, ok, err := guildMembership(tx.ctx, tx.Queries, tx.commanderID)
	if err != nil || !ok {
		return nil, nil, err
	}
	notices, err := tx.guildNoticesFor(member.GuildID)
	if err != nil {
		return nil, nil, err
	}
	var drops []*protobuf.DROPINFO
	if config.Type == taskTypeGuildWeekly {
		now := serverNow()
		logistics, err := tx.guildLogisticsForUpdate(member.GuildID)
		if err != nil {
			return nil, nil, err
		}
		mission, active := activeGuildMission(logistics, now)
		if !active || mission.TaskID != taskID {
			return nil, nil, nil
		}
		liveness = mission.GuildActive
		if logistics.WeeklyTaskProgress < mission.MaxNum {
			logistics.WeeklyTaskProgress++
			if err := tx.SetGuildWeeklyTask(tx.ctx, db.SetGuildWeeklyTaskParams{
				WeeklyTaskID: logistics.WeeklyTaskID, WeeklyTaskProgress: logistics.WeeklyTaskProgress, WeeklyTaskWeek: logistics.WeeklyTaskWeek, GuildID: member.GuildID,
			}); err != nil {
				return nil, nil, err
			}
			if logistics.WeeklyTaskProgress == mission.MaxNum {
				capital := mission.AwardCapitalDisplay * mission.MaxNum
				if err := tx.AddGuildCapital(tx.ctx, db.AddGuildCapitalParams{Capital: capital, ID: member.GuildID}); err != nil {
					return nil, nil, err
				}
				if err := tx.addGuildCapitalLog(member.GuildID, db.Commander{}, guildCapitalWeeklyTask, capital); err != nil {
					return nil, nil, err
				}
			}
		}
		if err := tx.addGuildRankScore(member.GuildID, member.CommanderID, guildRankMission, 1, now); err != nil {
			return nil, nil, err
		}
		user, err := tx.guildUser(now)
		if err != nil {
			return nil, nil, err
		}
		if user.WeeklyTaskWeek != logistics.WeeklyTaskWeek {
			if err := tx.SetGuildUserWeeklyTaskWeek(tx.ctx, db.SetGuildUserWeeklyTaskWeekParams{WeeklyTaskWeek: logistics.WeeklyTaskWeek, CommanderID: tx.commanderID}); err != nil {
				return nil, nil, err
			}
			drops = append(drops, dropInfo(dropTypeResource, ResourceGuildCoin, mission.AwardDisplay))
		}
	}
	return drops, notices, tx.addGuildLiveness(game, member, liveness, notices)
}
