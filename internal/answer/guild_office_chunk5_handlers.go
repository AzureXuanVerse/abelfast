package answer

import (
	"context"
	"encoding/json"
	"errors"
	"sort"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"
	"google.golang.org/protobuf/proto"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/consts"
	"github.com/ggmolly/belfast/internal/db"
	"github.com/ggmolly/belfast/internal/orm"
	"github.com/ggmolly/belfast/internal/protobuf"
)

const (
	guildChunkResultSuccess = uint32(0)
	guildChunkResultFailure = uint32(1)

	guildResultApplicantJoined = uint32(1)
	guildResultApplicantWait   = uint32(4)
	guildResultGuildFrozen     = uint32(4305)

	guildCoinResourceID = uint32(8)
	goldResourceID      = uint32(1)
)

type guildContributionTemplate struct {
	ID                uint32   `json:"id"`
	Consume           []uint32 `json:"consume"`
	AwardContribution uint32   `json:"award_contribution"`
	AwardCapital      uint32   `json:"award_capital"`
	AwardTechExp      uint32   `json:"award_tech_exp"`
	GuildActive       uint32   `json:"guild_active"`
}

type guildMissionTemplate struct {
	ID     uint32 `json:"id"`
	MaxNum uint32 `json:"max_num"`
}

type guildTechnologyTemplate struct {
	ID                   uint32 `json:"id"`
	Group                uint32 `json:"group"`
	NextTech             uint32 `json:"next_tech"`
	GoldConsume          uint32 `json:"gold_consume"`
	ContributionConsume  uint32 `json:"contribution_consume"`
	ContributionMultiple uint32 `json:"contribution_multiple"`
	NeedGuildActive      uint32 `json:"need_guild_active"`
	LevelMax             uint32 `json:"level_max"`
}

func AcceptGuildJoinRequest(buffer *[]byte, client *connection.Client) (int, int, error) {
	request := &protobuf.CS_60020{}
	if err := proto.Unmarshal(*buffer, request); err != nil {
		return 0, 60021, err
	}
	response := &protobuf.SC_60021{Result: proto.Uint32(guildChunkResultFailure)}
	if client.Commander == nil || request.GetPlayerId() == 0 {
		return client.SendMessage(60021, response)
	}

	result, err := orm.AcceptGuildJoinRequest(client.Commander.CommanderID, request.GetPlayerId())
	if err != nil {
		return 0, 60021, err
	}
	response.Result = proto.Uint32(result)
	return client.SendMessage(60021, response)
}

func GuildListRefresh(buffer *[]byte, client *connection.Client) (int, int, error) {
	request := &protobuf.CS_60024{}
	if err := proto.Unmarshal(*buffer, request); err != nil {
		return 0, 60025, err
	}
	response := &protobuf.SC_60025{GuildList: []*protobuf.GUILD_SIMPLE_INFO{}}
	if request.GetType() != 0 {
		return client.SendMessage(60025, response)
	}

	entries, err := orm.ListGuildDirectoryRefresh(30)
	if err != nil {
		return 0, 60025, err
	}
	for _, entry := range entries {
		response.GuildList = append(response.GuildList, buildGuildSimpleInfo(entry))
	}
	return client.SendMessage(60025, response)
}

func GuildCommitDonate(buffer *[]byte, client *connection.Client) (int, int, error) {
	request := &protobuf.CS_62002{}
	if err := proto.Unmarshal(*buffer, request); err != nil {
		return 0, 62003, err
	}
	response := &protobuf.SC_62003{Result: proto.Uint32(guildChunkResultFailure), DonateTasks: []uint32{}}
	if client.Commander == nil || request.GetId() == 0 {
		return client.SendMessage(62003, response)
	}

	template, ok, err := loadGuildContributionTemplate(request.GetId())
	if err != nil {
		return 0, 62003, err
	}
	if !ok {
		return client.SendMessage(62003, response)
	}

	info, err := orm.GetGuildUserInfo(client.Commander.CommanderID)
	if err != nil {
		return 0, 62003, err
	}
	tasks := append([]uint32{}, info.DonateTasks...)
	if len(tasks) == 0 {
		tasks, err = loadDefaultGuildDonateTasks()
		if err != nil {
			return 0, 62003, err
		}
	}
	if !containsUint32(tasks, request.GetId()) {
		response.DonateTasks = tasks
		return client.SendMessage(62003, response)
	}

	guild, member, err := orm.GetGuildForCommander(client.Commander.CommanderID)
	if err != nil {
		if errors.Is(err, db.ErrNotFound) {
			response.DonateTasks = tasks
			return client.SendMessage(62003, response)
		}
		return 0, 62003, err
	}

	ctx := context.Background()
	err = orm.WithPGXTx(ctx, func(tx pgx.Tx) error {
		if len(template.Consume) >= 3 {
			consumeType := template.Consume[0]
			consumeID := template.Consume[1]
			consumeAmount := template.Consume[2]
			switch consumeType {
			case consts.DROP_TYPE_RESOURCE:
				if err := client.Commander.ConsumeResourceTx(ctx, tx, consumeID, consumeAmount); err != nil {
					return err
				}
			default:
				if err := client.Commander.ConsumeItemTx(ctx, tx, consumeID, consumeAmount); err != nil {
					return err
				}
			}
		}

		if _, err := tx.Exec(ctx, `
UPDATE guild_user_infos
SET donate_count = donate_count + 1, donate_tasks = $2
WHERE commander_id = $1
`, int64(client.Commander.CommanderID), mustMarshalJSON(tasks)); err != nil {
			return err
		}
		if err := client.Commander.AddResourceTx(ctx, tx, guildCoinResourceID, template.AwardContribution); err != nil {
			return err
		}
		if guild != nil {
			if _, err := tx.Exec(ctx, `
UPDATE guilds
SET capital = capital + $2,
	updated_at = CURRENT_TIMESTAMP
WHERE id = $1
`, int64(guild.ID), int64(template.AwardCapital)); err != nil {
				return err
			}
			if member != nil {
				if _, err := tx.Exec(ctx, `
UPDATE guild_members
SET liveness = liveness + $3
WHERE guild_id = $1
	AND commander_id = $2
`, int64(guild.ID), int64(member.CommanderID), int64(template.GuildActive)); err != nil {
					return err
				}
			}
		}
		return nil
	})
	if err != nil {
		_ = client.Commander.Load()
		response.DonateTasks = tasks
		return client.SendMessage(62003, response)
	}

	if err := orm.SetGuildUserDonateTasks(client.Commander.CommanderID, tasks); err != nil {
		return 0, 62003, err
	}
	response.Result = proto.Uint32(guildChunkResultSuccess)
	response.DonateTasks = tasks
	_, _, sendErr := client.SendMessage(62003, response)
	if sendErr != nil {
		return 0, 62003, sendErr
	}
	if guild != nil {
		_ = orm.CreateGuildCapitalLog(guild.ID, orm.GuildCapitalLogEntry{
			Category:    orm.GuildCapitalLogCategoryIncrease,
			MemberID:    client.Commander.CommanderID,
			Name:        client.Commander.Name,
			EventType:   1,
			EventTarget: []uint32{request.GetId()},
			Time:        nowUnix(),
		})
		_ = orm.IncrementGuildMemberRank(guild.ID, 1, client.Commander.CommanderID, template.AwardContribution)
		broadcastGuildPacket(client, guild.ID, 62019, &protobuf.SC_62019{
			Id:           proto.Uint32(request.GetId()),
			UserId:       proto.Uint32(client.Commander.CommanderID),
			HasCapital:   proto.Uint32(template.AwardCapital),
			HasTechPoint: proto.Uint32(template.AwardTechExp),
		})
	}
	_, _, _ = client.SendMessage(62031, &protobuf.SC_62031{DonateTasks: tasks})
	return 0, 62003, nil
}

func GuildBuySupply(buffer *[]byte, client *connection.Client) (int, int, error) {
	request := &protobuf.CS_62007{}
	if err := proto.Unmarshal(*buffer, request); err != nil {
		return 0, 62008, err
	}
	response := &protobuf.SC_62008{Result: proto.Uint32(guildChunkResultFailure)}
	if client.Commander == nil || request.GetType() != 0 {
		return client.SendMessage(62008, response)
	}

	guild, member, err := orm.GetGuildForCommander(client.Commander.CommanderID)
	if err != nil {
		return client.SendMessage(62008, response)
	}
	if member.Duty != orm.GuildDutyCommander && member.Duty != orm.GuildDutyDeputy {
		return client.SendMessage(62008, response)
	}

	cost, err := orm.GetGuildSetUint("guild_award_consume")
	if err != nil {
		return 0, 62008, err
	}
	duration, err := orm.GetGuildSetUint("guild_award_duration")
	if err != nil {
		return 0, 62008, err
	}
	finishTime := nowUnix() + duration
	if err := orm.StartGuildSupply(guild.ID, cost, finishTime); err != nil {
		return client.SendMessage(62008, response)
	}

	_ = orm.CreateGuildCapitalLog(guild.ID, orm.GuildCapitalLogEntry{
		Category:    orm.GuildCapitalLogCategoryDecrease,
		MemberID:    client.Commander.CommanderID,
		Name:        client.Commander.Name,
		EventType:   2,
		EventTarget: []uint32{cost},
		Time:        nowUnix(),
	})
	response.Result = proto.Uint32(guildChunkResultSuccess)
	if _, _, err := client.SendMessage(62008, response); err != nil {
		return 0, 62008, err
	}
	broadcastGuildPacket(client, guild.ID, 62005, &protobuf.SC_62005{BenefitFinishTime: proto.Uint32(finishTime)})
	return 0, 62008, nil
}

func GuildGetSupplyAwardCommandResponse(buffer *[]byte, client *connection.Client) (int, int, error) {
	request := &protobuf.CS_62009{}
	if err := proto.Unmarshal(*buffer, request); err != nil {
		return 0, 62010, err
	}
	response := &protobuf.SC_62010{Result: proto.Uint32(guildChunkResultFailure), DropList: []*protobuf.DROPINFO{}}
	if client.Commander == nil || request.GetType() != 0 {
		return client.SendMessage(62010, response)
	}

	guild, member, err := orm.GetGuildForCommander(client.Commander.CommanderID)
	if err != nil {
		return client.SendMessage(62010, response)
	}
	if member.Duty == orm.GuildDutyRecruit {
		return client.SendMessage(62010, response)
	}
	todayStart := uint32(time.Now().UTC().Truncate(24 * time.Hour).Unix())
	if member.JoinTime >= todayStart {
		return client.SendMessage(62010, response)
	}
	officeState, err := orm.GetGuildOfficeState(guild.ID)
	if err != nil {
		return 0, 62010, err
	}
	if officeState.BenefitFinishTime == 0 || officeState.BenefitFinishTime <= nowUnix() {
		return client.SendMessage(62010, response)
	}

	info, err := orm.GetGuildUserInfo(client.Commander.CommanderID)
	if err != nil {
		return 0, 62010, err
	}
	if info.BenefitTime >= todayStart {
		return client.SendMessage(62010, response)
	}

	rewardID, err := orm.GetGuildSetUint("guild_award_drop")
	if err != nil {
		return 0, 62010, err
	}
	ctx := context.Background()
	dropType := consts.DROP_TYPE_ITEM
	err = orm.WithPGXTx(ctx, func(tx pgx.Tx) error {
		if _, resourceErr := orm.GetResourceByID(rewardID); resourceErr == nil {
			if err := client.Commander.AddResourceTx(ctx, tx, rewardID, 1); err != nil {
				return err
			}
			dropType = consts.DROP_TYPE_RESOURCE
		} else if err := client.Commander.AddItemTx(ctx, tx, rewardID, 1); err != nil {
			if err := client.Commander.AddResourceTx(ctx, tx, rewardID, 1); err != nil {
				return err
			}
			dropType = consts.DROP_TYPE_RESOURCE
		}
		if _, err := tx.Exec(ctx, `
UPDATE guild_user_infos
SET benefit_time = $2
WHERE commander_id = $1
`, int64(client.Commander.CommanderID), int64(nowUnix())); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return client.SendMessage(62010, response)
	}

	_ = orm.UpdateGuildUserBenefitTime(client.Commander.CommanderID, nowUnix())
	_ = orm.CreateGuildCapitalLog(guild.ID, orm.GuildCapitalLogEntry{
		Category:    orm.GuildCapitalLogCategoryOther,
		MemberID:    client.Commander.CommanderID,
		Name:        client.Commander.Name,
		EventType:   3,
		EventTarget: []uint32{rewardID},
		Time:        nowUnix(),
	})

	response.Result = proto.Uint32(guildChunkResultSuccess)
	response.DropList = []*protobuf.DROPINFO{{
		Type:   proto.Uint32(dropType),
		Id:     proto.Uint32(rewardID),
		Number: proto.Uint32(1),
	}}
	return client.SendMessage(62010, response)
}

func GuildFetchCapitalLogCommandResponse(buffer *[]byte, client *connection.Client) (int, int, error) {
	request := &protobuf.CS_62011{}
	if err := proto.Unmarshal(*buffer, request); err != nil {
		return 0, 62012, err
	}
	response := &protobuf.SC_62012{
		Result:   proto.Uint32(guildChunkResultFailure),
		Inclog:   []*protobuf.CAPITAL_LOG{},
		Declog:   []*protobuf.CAPITAL_LOG{},
		Otherlog: []*protobuf.CAPITAL_LOG{},
	}
	if client.Commander == nil || request.GetType() != 0 {
		return client.SendMessage(62012, response)
	}

	guild, _, err := orm.GetGuildForCommander(client.Commander.CommanderID)
	if err != nil {
		return client.SendMessage(62012, response)
	}
	logs, err := orm.ListGuildCapitalLogsByCategory(guild.ID, 120)
	if err != nil {
		return 0, 62012, err
	}
	toProto := func(entries []orm.GuildCapitalLogEntry) []*protobuf.CAPITAL_LOG {
		out := make([]*protobuf.CAPITAL_LOG, 0, len(entries))
		for _, entry := range entries {
			out = append(out, &protobuf.CAPITAL_LOG{
				MemberId:    proto.Uint32(entry.MemberID),
				Name:        proto.String(entry.Name),
				EventType:   proto.Uint32(entry.EventType),
				EventTarget: entry.EventTarget,
				Time:        proto.Uint32(entry.Time),
			})
		}
		return out
	}
	response.Result = proto.Uint32(guildChunkResultSuccess)
	response.Inclog = toProto(logs[orm.GuildCapitalLogCategoryIncrease])
	response.Declog = toProto(logs[orm.GuildCapitalLogCategoryDecrease])
	response.Otherlog = toProto(logs[orm.GuildCapitalLogCategoryOther])
	return client.SendMessage(62012, response)
}

func GuildSelectWeeklyTask(buffer *[]byte, client *connection.Client) (int, int, error) {
	request := &protobuf.CS_62013{}
	if err := proto.Unmarshal(*buffer, request); err != nil {
		return 0, 62014, err
	}
	response := &protobuf.SC_62014{Result: proto.Uint32(guildChunkResultFailure)}
	if client.Commander == nil || request.GetId() == 0 {
		return client.SendMessage(62014, response)
	}

	guild, member, err := orm.GetGuildForCommander(client.Commander.CommanderID)
	if err != nil {
		return client.SendMessage(62014, response)
	}
	if member.Duty != orm.GuildDutyCommander && member.Duty != orm.GuildDutyDeputy {
		return client.SendMessage(62014, response)
	}
	if _, ok, err := loadGuildMissionTemplate(request.GetId()); err != nil {
		return 0, 62014, err
	} else if !ok {
		return client.SendMessage(62014, response)
	}

	current, err := orm.GetGuildWeeklyTaskState(guild.ID)
	if err != nil && !errors.Is(err, db.ErrNotFound) {
		return 0, 62014, err
	}
	weekStart := currentWeekMonday0Clock()
	if current != nil && current.TaskID != 0 && current.Monday0Clock == weekStart {
		return client.SendMessage(62014, response)
	}
	if err := orm.UpsertGuildWeeklyTaskState(guild.ID, request.GetId(), 0, weekStart); err != nil {
		return 0, 62014, err
	}
	response.Result = proto.Uint32(guildChunkResultSuccess)
	if _, _, err := client.SendMessage(62014, response); err != nil {
		return 0, 62014, err
	}
	broadcastGuildPacket(client, guild.ID, 62004, &protobuf.SC_62004{ThisWeeklyTasks: &protobuf.WEEKLY_TASK{
		Id:            proto.Uint32(request.GetId()),
		Progress:      proto.Uint32(0),
		Monday_0Clock: proto.Uint32(weekStart),
	}})
	return 0, 62014, nil
}

func GuildUpgradeTechnologyCommandResponse(buffer *[]byte, client *connection.Client) (int, int, error) {
	request := &protobuf.CS_62015{}
	if err := proto.Unmarshal(*buffer, request); err != nil {
		return 0, 62016, err
	}
	response := &protobuf.SC_62016{Result: proto.Uint32(guildChunkResultFailure)}
	if client.Commander == nil || request.GetId() == 0 {
		return client.SendMessage(62016, response)
	}

	if _, _, err := orm.GetGuildForCommander(client.Commander.CommanderID); err != nil {
		if errors.Is(err, db.ErrNotFound) {
			response.Result = proto.Uint32(guildResultGuildFrozen)
			return client.SendMessage(62016, response)
		}
		return 0, 62016, err
	}

	template, ok, err := loadGuildTechnologyTemplate(request.GetId())
	if err != nil {
		return 0, 62016, err
	}
	if !ok || template.Group == 0 || template.NextTech == 0 {
		return client.SendMessage(62016, response)
	}

	currentTechID, err := orm.GetGuildUserTechnologyByGroup(client.Commander.CommanderID, template.Group)
	if err != nil {
		return 0, 62016, err
	}
	if currentTechID != 0 && currentTechID != request.GetId() {
		return client.SendMessage(62016, response)
	}

	coinCost := template.ContributionConsume
	if template.ContributionMultiple > 1 {
		coinCost *= template.ContributionMultiple
	}
	ctx := context.Background()
	err = orm.WithPGXTx(ctx, func(tx pgx.Tx) error {
		if coinCost > 0 {
			if err := client.Commander.ConsumeResourceTx(ctx, tx, guildCoinResourceID, coinCost); err != nil {
				return err
			}
		}
		if template.GoldConsume > 0 {
			if err := client.Commander.ConsumeResourceTx(ctx, tx, goldResourceID, template.GoldConsume); err != nil {
				return err
			}
		}
		if _, err := tx.Exec(ctx, `
INSERT INTO guild_user_technology_states (commander_id, tech_group, tech_id, updated_at)
VALUES ($1, $2, $3, CURRENT_TIMESTAMP)
ON CONFLICT (commander_id, tech_group)
DO UPDATE SET tech_id = EXCLUDED.tech_id, updated_at = CURRENT_TIMESTAMP
`, int64(client.Commander.CommanderID), int64(template.Group), int64(template.NextTech)); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		_ = client.Commander.Load()
		return client.SendMessage(62016, response)
	}

	response.Result = proto.Uint32(guildChunkResultSuccess)
	return client.SendMessage(62016, response)
}

func GuildStartTechGroupCommandResponse(buffer *[]byte, client *connection.Client) (int, int, error) {
	request := &protobuf.CS_62020{}
	if err := proto.Unmarshal(*buffer, request); err != nil {
		return 0, 62021, err
	}
	response := &protobuf.SC_62021{Result: proto.Uint32(guildChunkResultFailure)}
	if client.Commander == nil || request.GetId() == 0 {
		return client.SendMessage(62021, response)
	}

	guild, member, err := orm.GetGuildForCommander(client.Commander.CommanderID)
	if err != nil {
		return client.SendMessage(62021, response)
	}
	if member.Duty != orm.GuildDutyCommander && member.Duty != orm.GuildDutyDeputy {
		return client.SendMessage(62021, response)
	}
	if _, ok, err := loadGuildTechnologyTemplate(request.GetId()); err != nil {
		return 0, 62021, err
	} else if !ok {
		return client.SendMessage(62021, response)
	}
	if err := orm.SetGuildTechnologyActive(guild.ID, request.GetId()); err != nil {
		return client.SendMessage(62021, response)
	}
	_ = orm.CreateGuildCapitalLog(guild.ID, orm.GuildCapitalLogEntry{
		Category:    orm.GuildCapitalLogCategoryOther,
		MemberID:    client.Commander.CommanderID,
		Name:        client.Commander.Name,
		EventType:   4,
		EventTarget: []uint32{request.GetId()},
		Time:        nowUnix(),
	})
	response.Result = proto.Uint32(guildChunkResultSuccess)
	if _, _, err := client.SendMessage(62021, response); err != nil {
		return 0, 62021, err
	}
	broadcastGuildPacket(client, guild.ID, 62018, &protobuf.SC_62018{Id: proto.Uint32(request.GetId())})
	return 0, 62021, nil
}

func GuildFetchWeeklyTaskProgressCommandResponse(buffer *[]byte, client *connection.Client) (int, int, error) {
	request := &protobuf.CS_62022{}
	if err := proto.Unmarshal(*buffer, request); err != nil {
		return 0, 62023, err
	}
	response := &protobuf.SC_62023{Result: proto.Uint32(guildChunkResultFailure), Progress: proto.Uint32(0)}
	if client.Commander == nil || request.GetType() != 0 {
		return client.SendMessage(62023, response)
	}

	guild, _, err := orm.GetGuildForCommander(client.Commander.CommanderID)
	if err != nil {
		return client.SendMessage(62023, response)
	}
	state, err := orm.GetGuildWeeklyTaskState(guild.ID)
	if err != nil {
		return client.SendMessage(62023, response)
	}
	mission, ok, err := loadGuildMissionTemplate(state.TaskID)
	if err != nil {
		return 0, 62023, err
	}
	if !ok {
		return client.SendMessage(62023, response)
	}
	progress := state.Progress
	if progress > mission.MaxNum {
		progress = mission.MaxNum
		_ = orm.UpsertGuildWeeklyTaskState(guild.ID, state.TaskID, progress, state.Monday0Clock)
	}
	response.Result = proto.Uint32(guildChunkResultSuccess)
	response.Progress = proto.Uint32(progress)
	return client.SendMessage(62023, response)
}

func GuildFetchCapitalCommandResponse(buffer *[]byte, client *connection.Client) (int, int, error) {
	request := &protobuf.CS_62024{}
	if err := proto.Unmarshal(*buffer, request); err != nil {
		return 0, 62025, err
	}
	response := &protobuf.SC_62025{Result: proto.Uint32(guildChunkResultFailure), Capital: proto.Uint32(0)}
	if client.Commander == nil || request.GetType() != 0 {
		return client.SendMessage(62025, response)
	}
	guild, _, err := orm.GetGuildForCommander(client.Commander.CommanderID)
	if err != nil {
		return client.SendMessage(62025, response)
	}
	response.Result = proto.Uint32(guildChunkResultSuccess)
	response.Capital = proto.Uint32(guild.Capital)
	return client.SendMessage(62025, response)
}

func GuildGetRankCommandResponse(buffer *[]byte, client *connection.Client) (int, int, error) {
	request := &protobuf.CS_62029{}
	if err := proto.Unmarshal(*buffer, request); err != nil {
		return 0, 62030, err
	}
	response := &protobuf.SC_62030{List: []*protobuf.RANK_INFO_P62{}}
	if client.Commander == nil {
		return client.SendMessage(62030, response)
	}
	typeID := request.GetType()
	if typeID < 1 || typeID > 3 {
		return client.SendMessage(62030, response)
	}
	guild, _, err := orm.GetGuildForCommander(client.Commander.CommanderID)
	if err != nil {
		return client.SendMessage(62030, response)
	}
	ranks, err := orm.ListGuildMemberRanks(guild.ID, typeID)
	if err != nil {
		return 0, 62030, err
	}
	for _, period := range []uint32{1, 2, 3} {
		entries := ranks[period]
		users := make([]*protobuf.RANK_USER_INFO, 0, len(entries))
		for _, entry := range entries {
			users = append(users, &protobuf.RANK_USER_INFO{UserId: proto.Uint32(entry.UserID), Count: proto.Uint32(entry.Count)})
		}
		response.List = append(response.List, &protobuf.RANK_INFO_P62{Period: proto.Uint32(period), Rankuserinfo: users})
	}
	return client.SendMessage(62030, response)
}

func loadGuildContributionTemplate(id uint32) (*guildContributionTemplate, bool, error) {
	entry, err := orm.GetConfigEntry("ShareCfg/guild_contribution_template.json", strconv.FormatUint(uint64(id), 10))
	if err != nil {
		if db.IsNotFound(err) {
			return nil, false, nil
		}
		return nil, false, err
	}
	var tpl guildContributionTemplate
	if err := json.Unmarshal(entry.Data, &tpl); err != nil {
		return nil, false, err
	}
	if tpl.ID == 0 {
		tpl.ID = id
	}
	return &tpl, true, nil
}

func loadDefaultGuildDonateTasks() ([]uint32, error) {
	entries, err := orm.ListConfigEntries("ShareCfg/guild_contribution_template.json")
	if err != nil {
		return nil, err
	}
	ids := make([]uint32, 0, len(entries))
	for _, entry := range entries {
		var tpl guildContributionTemplate
		if err := json.Unmarshal(entry.Data, &tpl); err != nil {
			continue
		}
		if tpl.ID == 0 {
			continue
		}
		ids = append(ids, tpl.ID)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	if len(ids) > 3 {
		ids = ids[:3]
	}
	return ids, nil
}

func loadGuildMissionTemplate(id uint32) (*guildMissionTemplate, bool, error) {
	entry, err := orm.GetConfigEntry("ShareCfg/guild_mission_template.json", strconv.FormatUint(uint64(id), 10))
	if err != nil {
		if db.IsNotFound(err) {
			return nil, false, nil
		}
		return nil, false, err
	}
	var tpl guildMissionTemplate
	if err := json.Unmarshal(entry.Data, &tpl); err != nil {
		return nil, false, err
	}
	if tpl.ID == 0 {
		tpl.ID = id
	}
	return &tpl, true, nil
}

func loadGuildTechnologyTemplate(id uint32) (*guildTechnologyTemplate, bool, error) {
	entry, err := orm.GetConfigEntry("ShareCfg/guild_technology_template.json", strconv.FormatUint(uint64(id), 10))
	if err != nil {
		if db.IsNotFound(err) {
			return nil, false, nil
		}
		return nil, false, err
	}
	var tpl guildTechnologyTemplate
	if err := json.Unmarshal(entry.Data, &tpl); err != nil {
		return nil, false, err
	}
	if tpl.ID == 0 {
		tpl.ID = id
	}
	return &tpl, true, nil
}

func currentWeekMonday0Clock() uint32 {
	now := time.Now().UTC()
	weekday := int(now.Weekday())
	if weekday == 0 {
		weekday = 7
	}
	monday := time.Date(now.Year(), now.Month(), now.Day()-(weekday-1), 0, 0, 0, 0, time.UTC)
	return uint32(monday.Unix())
}

func broadcastGuildPacket(client *connection.Client, guildID uint32, packetID int, message any) {
	if client == nil || client.Server == nil {
		return
	}
	for _, connected := range client.Server.ListClients() {
		if connected == nil || connected.Commander == nil {
			continue
		}
		membership, err := orm.GetCommanderGuildMembership(connected.Commander.CommanderID)
		if err != nil || membership.GuildID != guildID {
			continue
		}
		_, _, _ = connected.SendMessage(packetID, message)
	}
}

func mustMarshalJSON(value any) []byte {
	encoded, err := json.Marshal(value)
	if err != nil {
		return []byte("[]")
	}
	return encoded
}
