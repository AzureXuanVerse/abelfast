package game

import (
	"context"
	"maps"
	"slices"
	"time"

	"google.golang.org/protobuf/proto"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/db"
	"github.com/ggmolly/belfast/internal/gamedata"
	"github.com/ggmolly/belfast/internal/protobuf"
)

// The island map of the sixth anniversary (SixthAnniversaryIslandProxy,
// SixthAnniversaryIslandScene, ACTIVITY_TYPE_ISLAND): nodes
// (activity_map_event_list) each at an event (activity_map_event_data),
// completed once at event 0. The client works out itself which nodes are
// unlocked or shown (IslandNode:GetUnlock, IsVisual) from every node's
// state, so the server lists all of them. The events are played through
// CS_11202 (IslandEventTriggerCommand cmd 1, IslandNodeMarkCommand cmd 2,
// IslandFlowerFieldGetAwardCommand cmd 1 on the flower field activity),
// which belfast doesn't run yet.

// ActivityConst.ACTIVITY_TYPE_ISLAND
const activityTypeIsland = 95

// activity_map_event_list row: a node, the event it starts at and whether
// it starts over every day (refresh)
type islandNodeConfig struct {
	EventID uint32 `json:"event_id"`
	Refresh uint32 `json:"refresh"`
}

var islandNodeConfigs map[uint32]islandNodeConfig

func loadIslandData() error {
	return gamedata.Load("activity_map_event_list", &islandNodeConfigs)()
}

// handleIslandNodes answers IslandRequestCommand (CS_26108, sent when the
// island opens and again once the server day changed,
// SixthAnniversaryIslandProxy:CheckValid): every node with its event and
// whether it is new (IslandNode). Built from the Lua alone (no capture);
// GUESSES: a node the commander never reached is at its event_id and new,
// and the refresh nodes go back to their event_id, new, at the first
// request of each server day. An island activity that isn't running gets
// ret 1 (the reply's result field is ret).
func (game *Game) handleIslandNodes(client *connection.Client, req *protobuf.CS_26108) error {
	ctx := context.Background()
	now := time.Now()
	config, open, err := game.commanderActivity(ctx, client.CommanderID, req.GetActId(), now)
	if err != nil {
		return err
	}
	if !open || config.Type != activityTypeIsland {
		return client.Send(&protobuf.SC_26109{Ret: result(errRefused)})
	}
	reply := &protobuf.SC_26109{Ret: result(nil)}
	err = game.tx(ctx, client, func(tx *Tx) error {
		rows, err := tx.ListIslandNodes(ctx, db.ListIslandNodesParams{CommanderID: client.CommanderID, ActivityID: config.ID})
		if err != nil {
			return err
		}
		stored := make(map[uint32]db.CommanderIslandNode, len(rows))
		for _, row := range rows {
			stored[row.NodeID] = row
		}
		for _, id := range slices.Sorted(maps.Keys(islandNodeConfigs)) {
			node, ok := stored[id]
			nodeConfig := islandNodeConfigs[id]
			if !ok || (nodeConfig.Refresh == 1 && !sameServerDay(time.Unix(node.RefreshedAt, 0), now)) {
				node = db.CommanderIslandNode{
					CommanderID: client.CommanderID, ActivityID: config.ID, NodeID: id,
					EventID: nodeConfig.EventID, IsNew: true, RefreshedAt: now.Unix(),
				}
				if err := tx.SaveIslandNode(ctx, db.SaveIslandNodeParams(node)); err != nil {
					return err
				}
			}
			isNew := uint32(0)
			if node.IsNew {
				isNew = 1
			}
			reply.NodeList = append(reply.NodeList, &protobuf.PB_ISLAND_NODE{
				Id: proto.Uint32(id), EventId: proto.Uint32(node.EventID), IsNew: proto.Uint32(isNew),
			})
		}
		return nil
	})
	if err != nil {
		return err
	}
	return client.Send(reply)
}
