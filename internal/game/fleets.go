package game

import (
	"context"
	"slices"

	"google.golang.org/protobuf/proto"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/db"
	"github.com/ggmolly/belfast/internal/protobuf"
)

// fleetRegular is Fleet.REGULAR_FLEET_ID (model/vo/fleet.lua), the first
// surface fleet.
const fleetRegular = 1

// grantFleetsStarter creates the fleets of new_commander.json (the capture's
// SC_12101: 1, 2, 11, 12). 1 and 2 are the surface fleets Fleet:isUnlock
// never locks (3 to 6 wait for chapters 404 to 704), 11 and 12 the first two
// submarine fleets (Fleet.SUBMARINE_FLEET_ID, of SUBMARINE_FLEET_NUMS = 4),
// which no Lua rule picks. The client fills in the other ids
// (FormationUI.MAX_FLEET_NUM, PvP...) itself. It puts the commander's ships
// in fleet 1, the chosen starter first (capture: [Javelin, Long Island]), and
// offers the starter commissions. It has to run after the starter ships are
// created.
func grantFleetsStarter(tx *Tx, commander db.Commander) error {
	ctx, queries := tx.ctx, tx.Queries
	for _, id := range newCommander.Fleets {
		if err := queries.CreateFleet(ctx, db.CreateFleetParams{CommanderID: commander.ID, FleetID: id, Name: ""}); err != nil {
			return err
		}
	}
	ships, err := queries.ListShipsByCommander(ctx, commander.ID)
	if err != nil {
		return err
	}
	// the starter ship leads
	if i := slices.IndexFunc(ships, func(ship db.Ship) bool { return ship.TemplateID == commander.StarterShip }); i > 0 {
		starter := ships[i]
		ships = slices.Insert(slices.Delete(ships, i, i+1), 0, starter)
	}
	ids := make([]uint32, len(ships))
	for i, ship := range ships {
		ids[i] = ship.ID
	}
	if err := setFleetShips(ctx, queries, commander.ID, fleetRegular, ids); err != nil {
		return err
	}
	return grantCommissionsStarter(ctx, queries, commander)
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

// fleetList is SC_12101, FleetProxy's fleets with their meowfficers
// (Fleet.commanderIds, by pos). The capture lists them by descending id.
func fleetList(ctx context.Context, queries *db.Queries, commanderID uint32) (*protobuf.SC_12101, error) {
	fleets, err := queries.ListFleets(ctx, commanderID)
	if err != nil {
		return nil, err
	}
	ships, err := queries.ListFleetShips(ctx, commanderID)
	if err != nil {
		return nil, err
	}
	cats, err := queries.ListFleetMeowfficers(ctx, commanderID)
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
		for _, cat := range cats {
			if cat.FleetID == fleet.FleetID {
				group.Commanders = append(group.Commanders, &protobuf.COMMANDERSINFO{Pos: proto.Uint32(cat.Pos), Id: proto.Uint32(cat.MeowfficerID)})
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

// handleRenameFleet answers FleetRenameCommand (CS_12104,
// controller/command/fleet/renamefleetcommand.lua, the formation screen's
// fleet name): 2 to 24 characters, as the client checks before sending. On result 0 the client renames its fleet, anything else
// shows ERROR_MESSAGE[result]. Built from the Lua alone.
func (game *Game) handleRenameFleet(client *connection.Client, req *protobuf.CS_12104) error {
	err := errRefused
	if name := req.GetName(); req.GetId() != 0 && validLabel(name, minFleetNameLength, maxFleetNameLength) {
		err = game.DB.RenameFleet(context.Background(), db.RenameFleetParams{CommanderID: client.CommanderID, FleetID: req.GetId(), Name: name})
	}
	if err != nil && !refused(err) {
		return err
	}
	return client.Send(&protobuf.SC_12105{Result: result(err)})
}
