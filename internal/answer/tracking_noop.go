package answer

import (
	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/protobuf"
	"google.golang.org/protobuf/proto"
)

func NewTracking(buffer *[]byte, client *connection.Client) (int, int, error) {
	// TODO: Persist new tracking events if analytics support is added.
	return 0, 0, nil
}

func MainSceneTracking(buffer *[]byte, client *connection.Client) (int, int, error) {
	// TODO: Persist main scene tracking events if analytics support is added.
	return 0, 0, nil
}

func TrackCommand(buffer *[]byte, client *connection.Client) (int, int, error) {
	// TODO: Persist track command events if analytics support is added.
	return 0, 0, nil
}

func UrExchangeTracking(buffer *[]byte, client *connection.Client) (int, int, error) {
	// TODO: Persist ur exchange tracking events if analytics support is added.
	return 0, 0, nil
}

func ApartmentTrackEvent(buffer *[]byte, client *connection.Client) (int, int, error) {
	var payload protobuf.CS_28090
	if err := proto.Unmarshal(*buffer, &payload); err != nil {
		return 0, 28090, err
	}
	return 0, 0, nil
}
