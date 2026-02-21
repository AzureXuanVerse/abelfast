package answer

import (
	"errors"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/orm"
	"github.com/ggmolly/belfast/internal/protobuf"
)

func pushFleetSync(client *connection.Client, fleet *orm.Fleet) error {
	if fleet == nil {
		return errors.New("fleet is required")
	}

	push := protobuf.SC_12106{Group: commanderFleetGroupInfo(fleet)}
	_, _, err := client.SendMessage(12106, &push)
	return err
}
