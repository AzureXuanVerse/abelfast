package exercise

import (
	"fmt"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/protobuf"
	"google.golang.org/protobuf/proto"
)

func ExerciseReplaceRivals(buffer *[]byte, client *connection.Client) (int, int, error) {
	var payload protobuf.CS_18003
	if err := proto.Unmarshal(*buffer, &payload); err != nil {
		return 0, 18004, err
	}
	if payload.Type == nil {
		return 0, 18004, fmt.Errorf("CS_18003 missing required field: type")
	}

	targets := buildExerciseRivalTargetList()

	response := protobuf.SC_18004{
		Result:     proto.Uint32(0),
		TargetList: targets,
	}
	if _, _, err := client.SendMessage(18004, &response); err != nil {
		return 0, 18004, err
	}

	return client.SendMessage(18005, buildExerciseSeasonPushUpdate(targets))
}
