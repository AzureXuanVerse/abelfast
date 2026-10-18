package game

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"math"
	"time"

	"google.golang.org/protobuf/proto"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/db"
	"github.com/ggmolly/belfast/internal/gamedata"
	"github.com/ggmolly/belfast/internal/protobuf"
)

// GuildConst.TYPE_*: what a technology group improves (effect_args[1]), the
// commander's own level of it adding num (BaseGuild.getAddition), except
// the seats: a guild's members follow the guild's level
// (GuildTechnology.Update, GuildMemberCntType)
const (
	guildTechMemberCount = "bigfleet_seats"
	guildTechGoldMax     = "gold_max"
	guildTechOilMax      = "oil_max"
	guildTechShipBag     = "ship_bag_size"
	guildTechEquipBag    = "equip_bag_size"
	guildTechCatBoxGold  = "catbox_gold_cost"
)

// GuildTechnologyGroup.STATE_*: whether the guild researches a group
const (
	guildTechStopped = 0
	guildTechStarted = 1
)

// guild_technology_template row: one level of a technology group. The
// level's costs (contribution in guild coins, gold) buy the next level;
// exp is the guild's donation points that research it, need_guild_active
// the member liveness buying this level takes.
type guildTechConfig struct {
	ID           uint32            `json:"id"`
	Group        uint32            `json:"group"`
	Level        uint32            `json:"level"`
	NextTech     uint32            `json:"next_tech"`
	Exp          uint32            `json:"exp"`
	Contribution uint32            `json:"contribution_consume"`
	Gold         uint32            `json:"gold_consume"`
	Multiple     float64           `json:"contribution_multiple"`
	NeedActive   uint32            `json:"need_guild_active"`
	EffectArgs   []json.RawMessage `json:"effect_args"`
	Num          uint32            `json:"num"`

	effect string // effect_args[1], a GuildConst.TYPE_*
}

var (
	// guildTechs are the guild_technology_template rows, by id
	guildTechs map[uint32]guildTechConfig
	// guildTechGroupStart is each group's level 0 template id
	// (get_id_list_by_group[group][1]), what a group nobody bought or
	// researched is at
	guildTechGroupStart map[uint32]uint32
	// guildTechSeatGroup is the bigfleet_seats group
	guildTechSeatGroup uint32
	// guildTechDefault is the template id a new guild starts researching
	// (guildset guild_tech_default)
	guildTechDefault uint32
	// publicGuildTechs are the public guild's groups (guildset
	// public_level_priority) and the template id each reached
	publicGuildTechs [][2]uint32
)

func loadGuildTechData() error {
	var err error
	if guildTechs, err = gamedata.Table[guildTechConfig]("guild_technology_template"); err != nil {
		return err
	}
	guildTechGroupStart = map[uint32]uint32{}
	for id, tech := range guildTechs {
		if len(tech.EffectArgs) > 0 {
			// ship attribute groups name their attribute differently: no
			// server effect
			_ = json.Unmarshal(tech.EffectArgs[0], &tech.effect)
		}
		guildTechs[id] = tech
		if start, ok := guildTechGroupStart[tech.Group]; !ok || tech.Level < guildTechs[start].Level {
			guildTechGroupStart[tech.Group] = id
		}
		if tech.effect == guildTechMemberCount {
			guildTechSeatGroup = tech.Group
		}
	}
	if err := guildSetting("guild_tech_default", &guildTechDefault); err != nil {
		return err
	}
	if _, ok := guildTechs[guildTechDefault]; !ok {
		return gamedata.Missing("guild_technology_template", "guild_tech_default %d", guildTechDefault)
	}
	// the public guild: the groups of public_level_priority at their
	// public_level_limit, where the official public guild is (captured)
	var groups, limits []uint32
	for key, list := range map[string]*[]uint32{"public_level_priority": &groups, "public_level_limit": &limits} {
		setting, ok := guildSet[key]
		if !ok {
			return gamedata.Missing("guildset", "%s", key)
		}
		if err := json.Unmarshal(setting.KeyArgs, list); err != nil {
			return gamedata.Missing("guildset", "%s key_args", key)
		}
	}
	if len(groups) != len(limits) {
		return gamedata.Missing("guildset", "public_level_limit for every public_level_priority group")
	}
	publicGuildTechs = nil
	for i, group := range groups {
		id, ok := guildTechAtLevel(group, limits[i])
		if !ok {
			return gamedata.Missing("guild_technology_template", "group %d level %d", group, limits[i])
		}
		publicGuildTechs = append(publicGuildTechs, [2]uint32{group, id})
	}
	return nil
}

// guildTechAtLevel is the template id of a group's level.
func guildTechAtLevel(group, level uint32) (uint32, bool) {
	id, ok := guildTechGroupStart[group]
	for ok && guildTechs[id].Level < level {
		id = guildTechs[id].NextTech
		_, ok = guildTechs[id]
	}
	return id, ok && guildTechs[id].Level == level
}

// publicGuildTechLevel is the level the public guild reached in a group (0
// outside its groups): what anybody can buy up to at the risen price
// (GuildTechnology.UPGRADE_TYPE_PUBLIC, the group's fake level).
func publicGuildTechLevel(group uint32) uint32 {
	for _, tech := range publicGuildTechs {
		if tech[0] == group {
			return guildTechs[tech[1]].Level
		}
	}
	return 0
}

// guildTechGroups are the groups a guild researched, by group.
func guildTechGroups(ctx context.Context, queries *db.Queries, guildID uint32) (map[uint32]db.GuildTechnology, error) {
	rows, err := queries.ListGuildTechnologies(ctx, guildID)
	if err != nil {
		return nil, err
	}
	groups := make(map[uint32]db.GuildTechnology, len(rows))
	for _, row := range rows {
		groups[row.GroupID] = row
	}
	return groups, nil
}

// guildTechChangedToday is GUILD_EXPANSION_INFO.tech_cancel_cnt: whether an
// administrator already chose the group to research today (the client
// allows it once a day, Guild:CanCancelTech, and resets it at midnight).
func guildTechChangedToday(groups map[uint32]db.GuildTechnology, now time.Time) bool {
	for _, row := range groups {
		if row.Active && row.StartedAt != 0 && sameServerDay(time.Unix(row.StartedAt, 0), now) {
			return true
		}
	}
	return false
}

// fillGuildTechnologies adds the guild's technologies to its
// GUILD_EXPANSION_INFO: the groups it researched (Guild.New starts the
// others at level 0, stopped) and whether the group to research was chosen
// today.
func (game *Game) fillGuildTechnologies(ctx context.Context, queries *db.Queries, guild db.Guild, expansion *protobuf.GUILD_EXPANSION_INFO) error {
	groups, err := guildTechGroups(ctx, queries, guild.ID)
	if err != nil {
		return err
	}
	for _, row := range groups {
		state := uint32(guildTechStopped)
		if row.Active {
			state = guildTechStarted
		}
		expansion.Technologys = append(expansion.Technologys, &protobuf.GUILD_TECHNOLOGY{
			Id: proto.Uint32(row.TechID), State: proto.Uint32(state), Progress: proto.Uint32(row.Progress),
		})
	}
	if guildTechChangedToday(groups, time.Now()) {
		expansion.TechCancelCnt = proto.Uint32(1)
	}
	return nil
}

// guildTechnologiesCreated starts a new guild's default technology
// (guild_tech_default), as CreateGuildCommand does on the client's side;
// it doesn't count as the day's choice.
func (game *Game) guildTechnologiesCreated(tx *Tx, guildID uint32) error {
	return tx.StartGuildTechnology(tx.ctx, db.StartGuildTechnologyParams{
		GuildID: guildID, GroupID: guildTechs[guildTechDefault].Group, TechID: guildTechDefault,
	})
}

// guildTechSeats is the members a guild's bigfleet_seats research adds to
// its level's member_num (Guild:getMaxMember).
func guildTechSeats(ctx context.Context, queries *db.Queries, guildID uint32) (uint32, error) {
	row, err := queries.GetGuildTechnology(ctx, db.GetGuildTechnologyParams{GuildID: guildID, GroupID: guildTechSeatGroup})
	if errors.Is(err, sql.ErrNoRows) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	return guildTechs[row.TechID].Num, nil
}

// guildUserTechIDs is USER_GUILD_INFO.tech_id: the template id of the level
// the commander bought of each group (Guild:updateUserInfo,
// PublicGuild:InitUser), in a guild or not.
func (game *Game) guildUserTechIDs(ctx context.Context, queries *db.Queries, commanderID uint32) ([]uint32, error) {
	rows, err := queries.ListCommanderGuildTechnologies(ctx, commanderID)
	if err != nil {
		return nil, err
	}
	ids := make([]uint32, 0, len(rows))
	for _, row := range rows {
		ids = append(ids, row.TechID)
	}
	return ids, nil
}

// addGuildTechProgress adds a donation's tech points to the group the guild
// researches, which goes up a level once they reach its exp, starting over
// from 0 (GuildTechnologyGroup:AddProgress); false when nothing is
// researched or the group is at its last level (SC_62019 has_tech_point 0).
func (tx *Tx) addGuildTechProgress(guildID, points uint32) (bool, error) {
	row, err := tx.GetActiveGuildTechnology(tx.ctx, guildID)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	tech := guildTechs[row.TechID]
	if tech.NextTech == 0 {
		return false, nil
	}
	techID, progress := row.TechID, row.Progress+points
	if progress >= tech.Exp {
		techID, progress = tech.NextTech, 0
	}
	return true, tx.SetGuildTechnologyProgress(tx.ctx, db.SetGuildTechnologyProgressParams{
		TechID: techID, Progress: progress, GuildID: guildID, GroupID: row.GroupID,
	})
}

// guildTechBonus is what the commander's bought levels of a GuildConst.TYPE_*
// add (BaseGuild.getAddition): more gold and oil the academy fills up to,
// dock and depot slots, less gold per cat box.
func (tx *Tx) guildTechBonus(effect string) (uint32, error) {
	rows, err := tx.ListCommanderGuildTechnologies(tx.ctx, tx.commanderID)
	if err != nil {
		return 0, err
	}
	bonus := uint32(0)
	for _, row := range rows {
		if tech := guildTechs[row.TechID]; tech.effect == effect && tech.Level > 0 {
			bonus += tech.Num
		}
	}
	return bonus, nil
}

// guildTechPrice is what buying the level after tech costs, in guild coins
// and gold: contribution_multiple times as much bought up to the public
// guild's level (GuildTechnology:GetConsume, PublicGuildTechnology).
func guildTechPrice(tech guildTechConfig, risen bool) (coins, gold uint32) {
	if !risen {
		return tech.Contribution, tech.Gold
	}
	return uint32(math.Round(float64(tech.Contribution) * tech.Multiple)), uint32(math.Round(float64(tech.Gold) * tech.Multiple))
}

// handleUpgradeGuildTech answers GuildStartTechCommand and
// PublicGuildUpgradeTechCommand (CS_62015): the commander buys the next
// level of a technology group, id the template id of the level they have
// (level 0's for a group never bought). Both send the same packet; the
// server tells them apart by the commander's guild. In a guild, the next
// level costs its row's price when the guild researched it and the
// member's liveness reaches its need_guild_active, else contribution_multiple
// times as much up to the public guild's level; outside one, the public
// guild's level at the risen price. The guild's seats aren't for sale. On
// result 0 the client takes the gold and guild coins and levels the
// technology up itself. Built from the Lua alone.
func (game *Game) handleUpgradeGuildTech(client *connection.Client, req *protobuf.CS_62015) error {
	ctx := context.Background()
	err := game.tx(ctx, client, func(tx *Tx) error {
		tech, ok := guildTechs[req.GetId()]
		if !ok || tech.NextTech == 0 || tech.Group == guildTechSeatGroup {
			return errRefused
		}
		next := guildTechs[tech.NextTech]
		rows, err := tx.ListCommanderGuildTechnologies(ctx, client.CommanderID)
		if err != nil {
			return err
		}
		current := guildTechGroupStart[tech.Group]
		for _, row := range rows {
			if row.GroupID == tech.Group {
				current = row.TechID
			}
		}
		if current != tech.ID {
			return errRefused
		}
		public := next.Level <= publicGuildTechLevel(tech.Group)
		member, inGuild, err := guildMembership(ctx, tx.Queries, client.CommanderID)
		if err != nil {
			return err
		}
		risen := true
		if inGuild {
			groups, err := guildTechGroups(ctx, tx.Queries, member.GuildID)
			if err != nil {
				return err
			}
			researched := uint32(0)
			if row, ok := groups[tech.Group]; ok {
				researched = guildTechs[row.TechID].Level
			}
			risen = next.Level > researched || next.NeedActive > member.Liveness
		}
		if risen && !public {
			return errRefused
		}
		coins, gold := guildTechPrice(tech, risen)
		if err := tx.SpendResource(ResourceGold, gold); err != nil {
			return err
		}
		if err := tx.SpendResource(ResourceGuildCoin, coins); err != nil {
			return err
		}
		return tx.SetCommanderGuildTechnology(ctx, db.SetCommanderGuildTechnologyParams{
			CommanderID: client.CommanderID, GroupID: tech.Group, TechID: next.ID,
		})
	})
	if err != nil && !refused(err) {
		return err
	}
	return client.Send(&protobuf.SC_62016{Result: result(err)})
}

// handleStartGuildTechGroup answers GuildStartTechGroupCommand (CS_62020):
// an administrator chooses the group the guild researches, id the template
// id of the level the guild is at, once a day (tech_cancel_cnt), a group
// short of its last level. The reply only closes the page: every member
// online, the administrator too, gets SC_62018 with the group's id, on which
// GuildProxy stops the group researched until then, starts this one and
// counts the day's choice. Built from the Lua alone.
func (game *Game) handleStartGuildTechGroup(client *connection.Client, req *protobuf.CS_62020) error {
	ctx := context.Background()
	var members []uint32
	err := game.tx(ctx, client, func(tx *Tx) error {
		member, ok, err := guildMembership(ctx, tx.Queries, client.CommanderID)
		if err != nil {
			return err
		}
		if !ok || !guildAdministrator(member.Duty) {
			return errRefused
		}
		tech, ok := guildTechs[req.GetId()]
		if !ok || tech.NextTech == 0 {
			return errRefused
		}
		groups, err := guildTechGroups(ctx, tx.Queries, member.GuildID)
		if err != nil {
			return err
		}
		now := time.Now()
		current, ok := groups[tech.Group]
		if !ok {
			current.TechID = guildTechGroupStart[tech.Group]
		}
		if current.TechID != tech.ID || current.Active || guildTechChangedToday(groups, now) {
			return errRefused
		}
		if err := tx.StopGuildTechnologies(ctx, member.GuildID); err != nil {
			return err
		}
		if err := tx.StartGuildTechnology(ctx, db.StartGuildTechnologyParams{
			GuildID: member.GuildID, GroupID: tech.Group, TechID: tech.ID, StartedAt: now.Unix(),
		}); err != nil {
			return err
		}
		members, err = tx.ListGuildMemberIDs(ctx, member.GuildID)
		return err
	})
	if err != nil && !refused(err) {
		return err
	}
	if err == nil {
		game.tellGuild(client, members, 0, &protobuf.SC_62018{Id: proto.Uint32(req.GetId())})
	}
	return client.Send(&protobuf.SC_62021{Result: result(err)})
}

// handlePublicGuildTech answers GetPublicGuildUserDataCommand (CS_62100,
// sent at login and every midnight, in a guild or not): the public guild's
// groups at the level each reached, none researched. PublicGuild starts the
// other groups at level 0; a guild member's technologies take these levels
// as the ones buyable at the risen price
// (HandleGuildAndPublicGuildTechCommand). The commander's own levels and
// donations come next, in SC_60103. The levels are guildset
// public_level_limit, where the official public guild is (captured).
func (game *Game) handlePublicGuildTech(client *connection.Client, req *protobuf.CS_62100) error {
	reply := &protobuf.SC_62101{}
	for _, tech := range publicGuildTechs {
		reply.Technologys = append(reply.Technologys, &protobuf.GUILD_TECHNOLOGY{
			Id: proto.Uint32(tech[1]), State: proto.Uint32(guildTechStopped), Progress: proto.Uint32(0),
		})
	}
	return client.Send(reply)
}
