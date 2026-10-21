package region

import (
	"fmt"
	"slices"
	"strings"
	"time"
)

const Default = "EN"

// All are the client regions, as AzurLaneLuaScripts names them: the game
// data dir has one directory per region (<data dir>/<region>).
var All = []string{"EN", "CN", "JP", "KR", "TW"}

var current = Default

// zones are the regions' server clocks, fixed offsets: no server observes
// daylight saving (SERVER_DAYLIGHT_SAVEING_TIME = false in every const.lua).
// From TIME_ZONE_DATA in TW's const.lua (an older client's SwitchTimeZone,
// unused since the clock comes from monday_0oclock_timestamp): KR and JP
// 32400, CHT (TW) and CH (CN) 28800, US -25200 since 2019-03-10
// (SERVER_TIME_ZONE_SWITCH_TIME). The wiki agrees for EN, CN and JP
// (Missions, Commissions: resets follow the server's local time).
var zones = map[string]*time.Location{
	"EN": time.FixedZone("UTC-7", -7*60*60),
	"CN": time.FixedZone("UTC+8", 8*60*60),
	"JP": time.FixedZone("UTC+9", 9*60*60),
	"KR": time.FixedZone("UTC+9", 9*60*60),
	"TW": time.FixedZone("UTC+8", 8*60*60),
}

func Current() string {
	return current
}

// Zone is the current region's server clock.
func Zone() *time.Location {
	return zones[current]
}

// Set selects the region, case-insensitively, before anything reads the game
// data.
func Set(name string) error {
	name = strings.ToUpper(name)
	if !slices.Contains(All, name) {
		return fmt.Errorf("unknown region %q, want one of %s", name, strings.Join(All, ", "))
	}
	current = name
	return nil
}
