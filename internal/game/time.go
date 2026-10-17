package game

import (
	"time"

	"github.com/ggmolly/belfast/internal/gamedata"
)

// mondayReference is the "monday_0oclock_timestamp" the official EN servers
// send (SC_11000, SC_11002): Monday 2020-11-23 00:00 in UTC-7. The client
// derives its weekly resets from it.
const mondayReference = 1606114800

const week = 7 * 24 * time.Hour

// exerciseResetHours are when exercise attempts refill, server time.
var exerciseResetHours = []int{0, 12, 18}

// exerciseSeasonLength is SeasonInfo.ONE_SEASON_TIME. Seasons start every
// two weeks from mondayReference (the captured season started 153 seasons
// after it).
const exerciseSeasonLength = 2 * week

// serverNow is the current time on the EN servers' clock.
func serverNow() time.Time {
	return time.Now().In(gamedata.Zone)
}

// startOfDay returns the midnight, server time, that began now's day.
func startOfDay(now time.Time) time.Time {
	local := now.In(gamedata.Zone)
	return time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, gamedata.Zone)
}

// nextDay returns the next midnight, server time.
func nextDay(now time.Time) time.Time {
	return startOfDay(now).AddDate(0, 0, 1)
}

// nextMonth returns the start of the month after now, server time: when the
// monthly shops refresh.
func nextMonth(now time.Time) time.Time {
	local := now.In(gamedata.Zone)
	return time.Date(local.Year(), local.Month()+1, 1, 0, 0, 0, 0, gamedata.Zone)
}

// sameServerDay tells whether a and b fall on the same day, server time.
func sameServerDay(a, b time.Time) bool {
	a, b = a.In(gamedata.Zone), b.In(gamedata.Zone)
	return a.Year() == b.Year() && a.YearDay() == b.YearDay()
}

// periodStart returns when the period running at now began, periods of the
// given length following each other from mondayReference (the weeks, the
// exercise seasons).
func periodStart(now time.Time, period time.Duration) time.Time {
	reference := time.Unix(mondayReference, 0)
	return reference.Add(now.Sub(reference).Truncate(period))
}

// nextExerciseReset returns the first exercise attempt refill after now.
func nextExerciseReset(now time.Time) time.Time {
	day := startOfDay(now)
	for _, hour := range exerciseResetHours {
		if reset := day.Add(time.Duration(hour) * time.Hour); reset.After(now) {
			return reset
		}
	}
	return nextDay(now)
}
