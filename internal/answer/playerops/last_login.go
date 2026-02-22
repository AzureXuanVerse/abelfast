package playerops

import (
	"fmt"
	"time"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/logger"

	"github.com/ggmolly/belfast/internal/protobuf"
	"google.golang.org/protobuf/proto"
)

// Reimplementation of SC_11000
func LastLogin(buffer *[]byte, client *connection.Client) (int, int, error) {
	now := time.Now().UTC()
	sc11000 := protobuf.SC_11000{
		Timestamp:               proto.Uint32(uint32(now.Unix())),
		Monday_0OclockTimestamp: proto.Uint32(1606114800), // 23/11/2020 08:00:00
	}
	client.PreviousLoginAt = client.Commander.LastLogin
	if err := applyNavalAcademyLoginCatchup(client, now); err != nil {
		return 0, 11000, err
	}
	client.Commander.BumpLastLogin()
	logger.LogEvent("Server", "SC_11000", "Updated last login of uid="+fmt.Sprint(client.Commander.CommanderID), logger.LOG_LEVEL_INFO)
	return client.SendMessage(11000, &sc11000)
}
