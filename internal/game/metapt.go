package game

import (
	"context"
	"slices"

	"google.golang.org/protobuf/proto"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/db"
	"github.com/ggmolly/belfast/internal/protobuf"
)

// dropTypeMetaPt is DROP_TYPE_META_PT (const.lua): Sync Points, id a META
// ship group (ship_strengthen_meta), added to its MetaPTData by the client
// (Drop's DROP_TYPE_META_PT case, model/vo/drop.lua).
const dropTypeMetaPt = 22

// metaTypePt is MetaCharacterConst.Meta_Type_Act_PT: a META ship recruited
// with Sync Points from the META Showdown (MetaProgress:isPtType).
const metaTypePt = 1

// ship_strengthen_meta row: target is the Sync Point ladder (MetaPTData
// targets), award_display the reward of each step ([type, id, count]).
type metaShipConfig struct {
	Type         uint32     `json:"type"`
	Target       []uint32   `json:"target"`
	AwardDisplay [][]uint32 `json:"award_display"`
}

var metaShips map[uint32]metaShipConfig

// addMetaPt gives count Sync Points of a META ship group.
func (tx *Tx) addMetaPt(group, count uint32) error {
	return tx.AddMetaPt(tx.ctx, db.AddMetaPtParams{CommanderID: tx.commanderID, GroupID: group, Pt: count})
}

// handleMetaPTData answers MetaPTDataRequestCommand (CS_34001, sent after
// login with the Sync Point groups of the current META Showdown and the
// Dossier Analysis): each group's Sync Points and the ladder targets whose
// reward was taken (MetaPTData:initFromServerData reads the claimed steps as
// the sorted fetch_list matching the targets in order). As captured, a
// group without any has pt 0 and no fetch_list.
func (game *Game) handleMetaPTData(client *connection.Client, req *protobuf.CS_34001) error {
	ctx := context.Background()
	pts, err := game.DB.ListMetaPts(ctx, client.CommanderID)
	if err != nil {
		return err
	}
	awards, err := game.DB.ListMetaPtAwards(ctx, client.CommanderID)
	if err != nil {
		return err
	}
	reply := &protobuf.SC_34002{}
	for _, group := range req.GetGroupId() {
		// setAllProgressPTData asserts every group is a META ship
		if _, ok := metaShips[group]; !ok {
			continue
		}
		info := &protobuf.META_SHIP_INFO{GroupId: proto.Uint32(group), Pt: proto.Uint32(0)}
		if i := slices.IndexFunc(pts, func(row db.MetaPt) bool { return row.GroupID == group }); i >= 0 {
			info.Pt = proto.Uint32(pts[i].Pt)
		}
		for _, award := range awards {
			if award.GroupID == group {
				info.FetchList = append(info.FetchList, award.Target)
			}
		}
		reply.MetaShipList = append(reply.MetaShipList, info)
	}
	return client.Send(reply)
}

// handleMetaPTAward answers MetaPTAwardGetCommand (CS_34003, the META Lab's
// Sync Point ladder "Collect All"): target_pt is the highest step the
// commander reached (MetaCharacterSynLayer:getOneStepPTAwardLevelAndCount,
// the client then sets the level to its index), and every step not taken
// yet up to it gives its award_display, added by the client through
// PlayerConst.addTranDrop (the step that recruits the META ship gives the
// ship, pushed in SC_12042). Any other result shows "34004 : <result>".
// Built from the Lua alone (never captured).
func (game *Game) handleMetaPTAward(client *connection.Client, req *protobuf.CS_34003) error {
	ctx := context.Background()
	group, target := req.GetGroupId(), req.GetTargetPt()
	config, ok := metaShips[group]
	reply := &protobuf.SC_34004{Result: result(nil)}
	err := game.tx(ctx, client, func(tx *Tx) error {
		if !ok || config.Type != metaTypePt {
			return errRefused
		}
		awards, err := tx.ListMetaPtAwards(ctx, client.CommanderID)
		if err != nil {
			return err
		}
		level := 0
		for _, award := range awards {
			if award.GroupID == group {
				level++
			}
		}
		last := slices.Index(config.Target, target)
		if last < level || last >= len(config.AwardDisplay) {
			return errRefused
		}
		pts, err := tx.ListMetaPts(ctx, client.CommanderID)
		if err != nil {
			return err
		}
		i := slices.IndexFunc(pts, func(row db.MetaPt) bool { return row.GroupID == group })
		if i < 0 || pts[i].Pt < target {
			return ErrNotEnough
		}
		for step := level; step <= last; step++ {
			if err := tx.AddMetaPtAward(ctx, db.AddMetaPtAwardParams{CommanderID: client.CommanderID, GroupID: group, Target: config.Target[step]}); err != nil {
				return err
			}
		}
		reply.DropList = configDrops(config.AwardDisplay[level : last+1])
		return tx.grantDrops(reply.DropList)
	})
	if refused(err) {
		return client.Send(&protobuf.SC_34004{Result: result(err)})
	}
	if err != nil {
		return err
	}
	return client.Send(reply)
}
