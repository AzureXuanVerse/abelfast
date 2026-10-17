package game

import (
	"encoding/json"
	"fmt"
)

// monthSignDrops are activity_month_sign[month].dayN: the drop of the Nth
// sign-in of each month (index 0 is January).
var monthSignDrops [12][31]drop

// loadSignInData reads the monthly sign-in drops.
func loadSignInData() error {
	months, err := readConfigTable[map[string]json.RawMessage]("activity_month_sign")
	if err != nil {
		return err
	}
	for month := range monthSignDrops {
		for day := range monthSignDrops[month] {
			var drops [][3]uint32
			field := fmt.Sprintf("day%d", day+1)
			if json.Unmarshal(months[uint32(month+1)][field], &drops) != nil || len(drops) == 0 {
				return fmt.Errorf("config table activity_month_sign in %s: no %s in month %d", regionDataDir(), field, month+1)
			}
			monthSignDrops[month][day] = drop{drops[0][0], drops[0][1], drops[0][2]}
		}
	}
	return nil
}
