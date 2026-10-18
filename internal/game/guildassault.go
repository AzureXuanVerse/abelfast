package game

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"google.golang.org/protobuf/proto"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/db"
	"github.com/ggmolly/belfast/internal/protobuf"
)

const (
	// guildAssaultPositions is how many ships a member lends to the guild
	// (GuildAssaultFleet.AnyShipChanged checks positions 1 and 2).
	guildAssaultPositions = 2
	// guildRecommendMax is how many assault ships the administrators can
	// recommend (GuildShowAssultShipPage, guild_recommend_limit); no config
	// table has it.
	guildRecommendMax = 9
	// GuildConst.RECOMMAND_SHIP and CANCEL_RECOMMAND_SHIP, CS_61033.cmd
	guildRecommendShip       = 0
	guildCancelRecommendShip = 1
)

// guildAssaultCooldown is how long a position of the assault fleet stays
// locked after it is set, in seconds (guildset operation_assault_team_cd,
// GuildProxy.GetNextCanFormationTime).
var guildAssaultCooldown uint32

func loadGuildAssaultData() error {
	return guildSetting("operation_assault_team_cd", &guildAssaultCooldown)
}

// guildAssaultShipInfos is the full SHIPINFO (slots, skills, enhancement)
// of the owner's ships of ids, in their order: a member's assault ships as
// the others see them ("check gear") and the boss battle borrows them.
func guildAssaultShipInfos(ctx context.Context, queries *db.Queries, ownerID uint32, ids []uint32) ([]*protobuf.SHIPINFO, error) {
	byID, err := shipsByID(ctx, queries, ownerID, ids)
	if err != nil {
		return nil, err
	}
	ships := make([]db.Ship, 0, len(ids))
	for _, id := range ids {
		if ship, ok := byID[id]; ok {
			ships = append(ships, ship)
		}
	}
	list, err := shipList(ctx, queries, ownerID, ships)
	if err != nil {
		return nil, err
	}
	return list.Shiplist, nil
}

// guildSupportShip is one of a member's assault ships as the guild's boss
// battle borrows it, ok false when the ship isn't in the owner's assault
// fleet or the owner isn't in the guild.
func (game *Game) guildSupportShip(ctx context.Context, queries *db.Queries, guildID, ownerID, shipID uint32) (*protobuf.SHIPINFO, bool, error) {
	_, err := queries.GetGuildAssaultShip(ctx, db.GetGuildAssaultShipParams{GuildID: guildID, CommanderID: ownerID, ShipID: shipID})
	if errors.Is(err, sql.ErrNoRows) {
		return nil, false, nil
	} else if err != nil {
		return nil, false, err
	}
	infos, err := guildAssaultShipInfos(ctx, queries, ownerID, []uint32{shipID})
	if err != nil || len(infos) == 0 {
		return nil, false, err
	}
	return infos[0], true, nil
}

// guildRecommendedShips is the guild's recommended assault ships as
// GuildAssaultFleet.SetRecommendList reads them.
func guildRecommendedShips(ctx context.Context, queries *db.Queries, guildID uint32) ([]*protobuf.TEAM_CELL, error) {
	rows, err := queries.ListGuildRecommendedShips(ctx, guildID)
	if err != nil {
		return nil, err
	}
	cells := make([]*protobuf.TEAM_CELL, len(rows))
	for i, row := range rows {
		cells[i] = &protobuf.TEAM_CELL{UserId: proto.Uint32(row.CommanderID), ShipId: proto.Uint32(row.ShipID)}
	}
	return cells, nil
}

// handleGuildUpdateAssaultFleet answers GuildUpdateAssaultFleetCommand
// (CS_61003): the positions of the commander's assault fleet that changed,
// each with its ship (the same ship again when only its gear changed). On
// result 0 the client starts each position's cooldown at its own clock and
// takes the fleet it sent; a position still cooling down, a ship that isn't
// the commander's or the same ship at both positions is refused. The client
// refuses while the commander fights the guild boss, as belfast does
// (guild_formation_erro_in_boss_battle). Built from the Lua alone.
func (game *Game) handleGuildUpdateAssaultFleet(client *connection.Client, req *protobuf.CS_61003) error {
	ctx := context.Background()
	err := game.tx(ctx, client, func(tx *Tx) error {
		member, ok, err := guildMembership(ctx, tx.Queries, client.CommanderID)
		if err != nil {
			return err
		}
		if !ok || len(req.GetShipIds()) == 0 {
			return errRefused
		}
		fighting, err := guildBossBattleOngoing(ctx, tx.Queries, member.GuildID, client.CommanderID)
		if err != nil {
			return err
		}
		if fighting {
			return errRefused
		}
		rows, err := tx.ListGuildAssaultShips(ctx, client.CommanderID)
		if err != nil {
			return err
		}
		fleet := map[uint32]db.GuildAssaultShip{}
		for _, row := range rows {
			fleet[row.Pos] = row
		}
		now := uint32(time.Now().Unix())
		changed := map[uint32]uint32{}
		var ids []uint32
		for _, cell := range req.GetShipIds() {
			pos := cell.GetPos()
			if pos < 1 || pos > guildAssaultPositions {
				return errRefused
			}
			if _, dup := changed[pos]; dup {
				return errRefused
			}
			if row, set := fleet[pos]; set && now < row.ChangedAt+guildAssaultCooldown {
				return errRefused
			}
			changed[pos] = cell.GetShipId()
			ids = append(ids, cell.GetShipId())
		}
		if owned, err := ownsShips(ctx, tx.Queries, client.CommanderID, ids); err != nil {
			return err
		} else if !owned {
			return errRefused
		}
		// GuildEventMediator.OnSelectShips: guild_fleet_exist_same_kind_ship
		seen := map[uint32]bool{}
		for pos := uint32(1); pos <= guildAssaultPositions; pos++ {
			shipID, set := changed[pos]
			if !set {
				row, ok := fleet[pos]
				if !ok {
					continue
				}
				shipID = row.ShipID
			}
			if seen[shipID] {
				return errRefused
			}
			seen[shipID] = true
		}
		for pos, shipID := range changed {
			if err := tx.SetGuildAssaultShip(ctx, db.SetGuildAssaultShipParams{
				CommanderID: client.CommanderID, Pos: pos, ShipID: shipID, ChangedAt: now,
			}); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil && !refused(err) {
		return err
	}
	return client.Send(&protobuf.SC_61004{Result: result(err)})
}

// handleMyGuildAssaultFleet answers GetMyAssaultFleetCommand (CS_61009,
// asked as soon as the commander is in a guild): the commander's assault
// ships by position, each with the time it was set, which restarts the
// position's cooldown on the client. An empty fleet is no ships. Built from
// the Lua alone.
func (game *Game) handleMyGuildAssaultFleet(client *connection.Client, req *protobuf.CS_61009) error {
	ctx := context.Background()
	queries := game.DB.Queries
	_, ok, err := guildMembership(ctx, queries, client.CommanderID)
	if err != nil {
		return err
	}
	if !ok {
		return client.Send(&protobuf.SC_61010{Result: result(errNotInGuild)})
	}
	rows, err := queries.ListGuildAssaultShips(ctx, client.CommanderID)
	if err != nil {
		return err
	}
	ids := make([]uint32, len(rows))
	for i, row := range rows {
		ids[i] = row.ShipID
	}
	infos, err := guildAssaultShipInfos(ctx, queries, client.CommanderID, ids)
	if err != nil {
		return err
	}
	byID := make(map[uint32]*protobuf.SHIPINFO, len(infos))
	for _, info := range infos {
		byID[info.GetId()] = info
	}
	reply := &protobuf.SC_61010{Result: result(nil)}
	for _, row := range rows {
		if info, ok := byID[row.ShipID]; ok {
			reply.PersonShips = append(reply.PersonShips, &protobuf.SHIPID_POS_INFO{
				Pos: proto.Uint32(row.Pos), Ship: info, LastTime: proto.Uint32(row.ChangedAt),
			})
		}
	}
	return client.Send(reply)
}

// handleGuildAssaultFleets answers GuildGetAssaultFleetCommand (CS_61011,
// at most every GuildConst.REQUEST_FORMATION_CD seconds): every member's
// assault ships with their gear (GuildAssaultFleet.New keys them by their
// index, so in position order) and the recommended ones. Built from the Lua
// alone.
func (game *Game) handleGuildAssaultFleets(client *connection.Client, req *protobuf.CS_61011) error {
	ctx := context.Background()
	queries := game.DB.Queries
	member, ok, err := guildMembership(ctx, queries, client.CommanderID)
	if err != nil {
		return err
	}
	if !ok {
		return client.Send(&protobuf.SC_61012{Result: result(errNotInGuild)})
	}
	rows, err := queries.ListGuildAssaultShipsByGuild(ctx, member.GuildID)
	if err != nil {
		return err
	}
	reply := &protobuf.SC_61012{Result: result(nil)}
	for start := 0; start < len(rows); {
		ownerID := rows[start].GuildAssaultShip.CommanderID
		var ids []uint32
		end := start
		for ; end < len(rows) && rows[end].GuildAssaultShip.CommanderID == ownerID; end++ {
			ids = append(ids, rows[end].GuildAssaultShip.ShipID)
		}
		start = end
		infos, err := guildAssaultShipInfos(ctx, queries, ownerID, ids)
		if err != nil {
			return err
		}
		reply.Ships = append(reply.Ships, &protobuf.TEAM_CHUNK{UserId: proto.Uint32(ownerID), Ships: infos})
	}
	if reply.Recommends, err = guildRecommendedShips(ctx, queries, member.GuildID); err != nil {
		return err
	}
	return client.Send(reply)
}

// handleGuildRecommendAssaultShip answers MarkAssultShipRecommandCommand
// (CS_61033): an administrator recommends (cmd 0) or stops recommending
// (cmd 1) a member's assault ship, up to guildRecommendMax for the guild.
// The client checks the duty, the limit and whether the ship already is in
// that state after refreshing the recommendations (CS_61035). Built from the
// Lua alone.
func (game *Game) handleGuildRecommendAssaultShip(client *connection.Client, req *protobuf.CS_61033) error {
	ctx := context.Background()
	err := game.tx(ctx, client, func(tx *Tx) error {
		member, ok, err := guildMembership(ctx, tx.Queries, client.CommanderID)
		if err != nil {
			return err
		}
		if !ok || !guildAdministrator(member.Duty) {
			return errRefused
		}
		cmd := req.GetCmd()
		if cmd != guildRecommendShip && cmd != guildCancelRecommendShip {
			return errRefused
		}
		row, err := tx.GetGuildAssaultShip(ctx, db.GetGuildAssaultShipParams{
			GuildID: member.GuildID, CommanderID: req.GetRecommendUid(), ShipID: req.GetRecommendShipid(),
		})
		if errors.Is(err, sql.ErrNoRows) {
			return errRefused
		} else if err != nil {
			return err
		}
		recommend := cmd == guildRecommendShip
		if row.GuildAssaultShip.Recommended == recommend {
			return nil
		}
		if recommend {
			count, err := tx.CountGuildRecommendedShips(ctx, member.GuildID)
			if err != nil {
				return err
			}
			if count >= guildRecommendMax {
				return errRefused
			}
		}
		return tx.SetGuildAssaultShipRecommended(ctx, db.SetGuildAssaultShipRecommendedParams{
			Recommended: recommend, CommanderID: req.GetRecommendUid(), ShipID: req.GetRecommendShipid(),
		})
	})
	if err != nil && !refused(err) {
		return err
	}
	return client.Send(&protobuf.SC_61034{Result: result(err)})
}

// handleGuildRecommendedShips answers GuildRefreshRecommandShipCommand
// (CS_61035, at most every 3 s): the guild's recommended assault ships, which
// replace every member's (none outside a guild). Built from the Lua alone.
func (game *Game) handleGuildRecommendedShips(client *connection.Client, req *protobuf.CS_61035) error {
	ctx := context.Background()
	queries := game.DB.Queries
	member, ok, err := guildMembership(ctx, queries, client.CommanderID)
	if err != nil {
		return err
	}
	reply := &protobuf.SC_61036{}
	if ok {
		if reply.Recommends, err = guildRecommendedShips(ctx, queries, member.GuildID); err != nil {
			return err
		}
	}
	return client.Send(reply)
}
