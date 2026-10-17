package game

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"slices"

	"github.com/ggmolly/belfast/internal/gamedata"
)

// newCommanderContent is what a new commander starts with that no client
// table holds, the official server's own choice: server content
// new_commander.json, e.g. (the capture's values)
//
//	{
//	  "resources": [{"id": 1, "amount": 3000}, {"id": 2, "amount": 500}, {"id": 5, "amount": 5},
//	                {"id": 7, "amount": 5}, {"id": 8, "amount": 4000}],
//	  "items": [{"id": 20001, "count": 5}, {"id": 15003, "count": 10},
//	            {"id": 50002, "count": 10}, {"id": 50001, "count": 10}],
//	  "ships": [106011],
//	  "fleets": [1, 2, 11, 12],
//	  "commissions": 5
//	}
type newCommanderContent struct {
	// player_resource ids and amounts (SC_11003.resource_list)
	Resources []struct {
		ID     uint32 `json:"id"`
		Amount uint32 `json:"amount"`
	} `json:"resources"`
	// item_data_statistics ids and counts, the bag (SC_15001)
	Items []struct {
		ID    uint32 `json:"id"`
		Count uint32 `json:"count"`
	} `json:"items"`
	// ship_data_template ids given next to the chosen starter (Long Island,
	// created with her at CS_10024)
	Ships []uint32 `json:"ships"`
	// the fleet ids that exist (SC_12101), see grantFleetsStarter
	Fleets []uint32 `json:"fleets"`
	// how many commissions are drawn (SC_13002), see grantCommissionsStarter
	Commissions int `json:"commissions"`
}

var newCommander newCommanderContent

// loadNewCommander reads new_commander.json. Without it the server can't
// create commanders (no fleet for the starter ships, no gold or oil to play
// with), so it is a startup error.
func loadNewCommander() error {
	raw, err := gamedata.ServerFile("new_commander.json")
	if err != nil {
		return err
	}
	if raw == nil {
		return fmt.Errorf("server content %s: no new_commander.json, needed to create commanders", filepath.Join(gamedata.RegionDir(), "server"))
	}
	newCommander = newCommanderContent{}
	if err := json.Unmarshal(raw, &newCommander); err != nil {
		return fmt.Errorf("new_commander.json: %w", err)
	}
	if !slices.Contains(newCommander.Fleets, fleetRegular) {
		return fmt.Errorf("new_commander.json: fleets lack fleet %d, where the starter ships go", fleetRegular)
	}
	return nil
}
