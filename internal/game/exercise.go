package game

import (
	"time"

	"google.golang.org/protobuf/proto"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/protobuf"
)

// serverZone is the EN servers' clock, the one mondayReference is midnight in.
var serverZone = time.FixedZone("UTC-7", -7*60*60)

// exerciseResetHours are when exercise attempts refill, server time.
var exerciseResetHours = []int{0, 12, 18}

// nextExerciseReset returns the first attempt refill after now.
func nextExerciseReset(now time.Time) time.Time {
	local := now.In(serverZone)
	day := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, serverZone)
	for _, hour := range exerciseResetHours {
		if reset := day.Add(time.Duration(hour) * time.Hour); reset.After(now) {
			return reset
		}
	}
	return day.AddDate(0, 0, 1)
}

// handleSeasonInfo answers the exercise (PvP) season (GetSeasonInfoCommand):
// unranked, no opponents yet.
func (game *Game) handleSeasonInfo(client *connection.Client, req *protobuf.CS_18001) error {
	return client.Send(&protobuf.SC_18002{
		Score:               proto.Uint32(0),
		Rank:                proto.Uint32(0),
		FightCount:          proto.Uint32(0),
		FightCountResetTime: proto.Uint32(uint32(nextExerciseReset(time.Now()).Unix())),
		FlashTargetCount:    proto.Uint32(0),
	})
}
