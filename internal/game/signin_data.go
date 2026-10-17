package game

import (
	"encoding/json"
	"fmt"

	"github.com/ggmolly/belfast/internal/gamedata"
)

// monthSignDrops are activity_month_sign[month].dayN: the drop of the Nth
// sign-in of each month (index 0 is January).
var monthSignDrops [12][31][]uint32

// loadSignInData reads the monthly sign-in drops and the 7-day login ones.
func loadSignInData() error {
	if err := gamedata.Load("activity_7_day_sign", &sevenDaySignConfigs)(); err != nil {
		return err
	}
	months, err := gamedata.Table[map[string]json.RawMessage]("activity_month_sign")
	if err != nil {
		return err
	}
	for month := range monthSignDrops {
		for day := range monthSignDrops[month] {
			var drops [][]uint32
			field := fmt.Sprintf("day%d", day+1)
			if json.Unmarshal(months[uint32(month+1)][field], &drops) != nil || len(drops) == 0 || len(drops[0]) < 3 {
				return gamedata.Missing("activity_month_sign", "%s in month %d", field, month+1)
			}
			monthSignDrops[month][day] = drops[0]
		}
	}
	return nil
}
