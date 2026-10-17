package game

import (
	"context"
	"slices"

	"google.golang.org/protobuf/proto"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/db"
	"github.com/ggmolly/belfast/internal/packets"
	"github.com/ggmolly/belfast/internal/protobuf"
)

// starterFleets are the fleets a new commander has (SC_12101 in the
// capture): the first two surface fleets and both submarine fleets. The
// client fills in the other ids (FormationUI.MAX_FLEET_NUM, PvP...) itself.
var starterFleets = []uint32{1, 2, 11, 12}

// registerFleets installs the formation, campaign and battle handlers.
func (game *Game) registerFleets() {
	packets.Handle(game.handleUpdateFleet)
	packets.Handle(game.handleTracking13101)
	packets.Handle(game.handleChapterOp)
	packets.Handle(game.handleChapterBattleResult)
	packets.Handle(game.handleRemasterInfo)
	packets.Handle(game.handleBeginStage)
	packets.Handle(game.handleFinishStage)
}

// grantFleetsStarter creates the starter fleets, puts the commander's ships
// in fleet 1, the chosen starter first (capture: [Javelin, Long Island]), and
// offers the starter commissions. It has to run after the starter ships are
// created.
func grantFleetsStarter(tx *Tx, commander db.Commander) error {
	ctx, queries := tx.ctx, tx.Queries
	for _, id := range starterFleets {
		if err := queries.CreateFleet(ctx, db.CreateFleetParams{CommanderID: commander.ID, FleetID: id, Name: ""}); err != nil {
			return err
		}
	}
	ships, err := queries.ListShipsByCommander(ctx, commander.ID)
	if err != nil {
		return err
	}
	slices.SortStableFunc(ships, func(a, b db.Ship) int {
		return boolOrder(b.TemplateID == commander.StarterShip) - boolOrder(a.TemplateID == commander.StarterShip)
	})
	ids := make([]uint32, len(ships))
	for i, ship := range ships {
		ids[i] = ship.ID
	}
	if err := setFleetShips(ctx, queries, commander.ID, 1, ids); err != nil {
		return err
	}
	return grantCommissionsStarter(ctx, queries, commander.ID)
}

func boolOrder(b bool) int {
	if b {
		return 1
	}
	return 0
}

func setFleetShips(ctx context.Context, queries *db.Queries, commanderID uint32, fleetID uint32, shipIDs []uint32) error {
	if err := queries.ClearFleetShips(ctx, db.ClearFleetShipsParams{CommanderID: commanderID, FleetID: fleetID}); err != nil {
		return err
	}
	for i, id := range shipIDs {
		if err := queries.AddFleetShip(ctx, db.AddFleetShipParams{CommanderID: commanderID, FleetID: fleetID, Position: uint32(i), ShipID: id}); err != nil {
			return err
		}
	}
	return nil
}

// fleetList is SC_12101, FleetProxy's fleets. The capture lists them by
// descending id.
func fleetList(ctx context.Context, queries *db.Queries, commanderID uint32) (*protobuf.SC_12101, error) {
	fleets, err := queries.ListFleets(ctx, commanderID)
	if err != nil {
		return nil, err
	}
	ships, err := queries.ListFleetShips(ctx, commanderID)
	if err != nil {
		return nil, err
	}
	msg := &protobuf.SC_12101{GroupList: make([]*protobuf.GROUPINFO_P12, len(fleets))}
	for i, fleet := range fleets {
		group := &protobuf.GROUPINFO_P12{Id: proto.Uint32(fleet.FleetID), Name: proto.String(fleet.Name)}
		for _, ship := range ships {
			if ship.FleetID == fleet.FleetID {
				group.ShipList = append(group.ShipList, ship.ShipID)
			}
		}
		msg.GroupList[i] = group
	}
	return msg, nil
}

// handleUpdateFleet saves a fleet's ships (UpdateFleetCommand, CS_12102). The
// list comes vanguard, main, submarine; result 0 makes the client apply its
// edit, anything else shows fleet_updateFleet's error tip.
func (game *Game) handleUpdateFleet(client *connection.Client, req *protobuf.CS_12102) error {
	ctx := context.Background()
	owned, err := ownsShips(ctx, game.DB.Queries, client.CommanderID, req.GetShipList())
	if err != nil {
		return err
	}
	if !owned {
		return client.Send(&protobuf.SC_12103{Result: result(errRefused)})
	}
	err = game.DB.InTx(ctx, func(queries *db.Queries) error {
		if err := queries.CreateFleet(ctx, db.CreateFleetParams{CommanderID: client.CommanderID, FleetID: req.GetId(), Name: ""}); err != nil {
			return err
		}
		return setFleetShips(ctx, queries, client.CommanderID, req.GetId(), req.GetShipList())
	})
	if err != nil {
		return err
	}
	return client.Send(&protobuf.SC_12103{Result: result(nil)})
}
