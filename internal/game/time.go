package game

import (
	"time"

	"github.com/ggmolly/belfast/internal/gamedata"
)

// mondayReference is the "monday_0oclock_timestamp" the official EN servers
// send (SC_11000, SC_11002): Monday 2020-11-23 00:00 in UTC-7. The client
// derives its weekly resets from it.
const mondayReference = 1606114800

const (
	day  = 24 * time.Hour
	week = 7 * day
)

// Exercise attempts refill from midnight, server time:
// ExerciseCountRecoverUpCommand (controller/command/militaryexercise) expects
// the next refill SeasonInfo.RECOVER_UP_TWELVE_HOUR after one at hour 0,
// SeasonInfo.RECOVER_UP_SIX_HOUR after any other (model/vo/seasoninfo.lua),
// so at 0:00, 12:00 and 18:00.
const (
	exerciseRefillAfterMidnight = 12 * time.Hour
	exerciseRefillInterval      = 6 * time.Hour
)

// exerciseSeasonLength is SeasonInfo.ONE_SEASON_TIME (1209600 s,
// model/vo/seasoninfo.lua). Seasons start every two weeks from
// mondayReference (the captured season started 153 seasons after it).
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

// startOfMonth returns the midnight, server time, that began now's month:
// when the monthly counts start again.
func startOfMonth(now time.Time) time.Time {
	local := now.In(gamedata.Zone)
	return time.Date(local.Year(), local.Month(), 1, 0, 0, 0, 0, gamedata.Zone)
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

// serverWeekday is TimeMgr:GetServerWeek: the day of the week, server
// time, counted from 1 on Monday as the client counts it from the
// monday_0oclock_timestamp it is sent (mondayReference).
func serverWeekday(now time.Time) uint32 {
	return uint32(now.Sub(periodStart(now, week))/(24*time.Hour)) + 1
}

// nextExerciseReset returns the first exercise attempt refill after now.
func nextExerciseReset(now time.Time) time.Time {
	reset := startOfDay(now)
	for !reset.After(now) {
		if reset.Equal(startOfDay(reset)) {
			reset = reset.Add(exerciseRefillAfterMidnight)
		} else {
			reset = reset.Add(exerciseRefillInterval)
		}
	}
	return reset
}
