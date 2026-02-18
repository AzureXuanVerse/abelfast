package answer

import (
	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/orm"
	"google.golang.org/protobuf/proto"

	"github.com/ggmolly/belfast/internal/protobuf"
)

const maxMiniGameTelemetryTime = uint32(86400)

func MiniGameTimeSubmit(buffer *[]byte, client *connection.Client) (int, int, error) {
	req := &protobuf.CS_26110{}
	if err := proto.Unmarshal(*buffer, req); err != nil {
		return 0, 0, err
	}
	if req.GetGameid() == 0 || req.GetTime() == 0 || req.GetTime() > maxMiniGameTelemetryTime {
		return 0, 0, nil
	}
	if _, err := orm.GetMiniGameConfig(req.GetGameid()); err != nil {
		return 0, 0, nil
	}
	telemetry, err := orm.GetOrCreateMiniGameTelemetryState(client.Commander.CommanderID)
	if err != nil {
		return 0, 0, nil
	}
	telemetry.GameTimes[req.GetGameid()] = req.GetTime()
	if err := orm.SaveMiniGameTelemetryState(telemetry); err != nil {
		return 0, 0, nil
	}
	return 0, 0, nil
}
