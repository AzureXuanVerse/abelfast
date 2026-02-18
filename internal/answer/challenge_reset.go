package answer

import (
	"context"

	"github.com/jackc/pgx/v5"
	"google.golang.org/protobuf/proto"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/orm"
	"github.com/ggmolly/belfast/internal/protobuf"
)

func ChallengeReset(buffer *[]byte, client *connection.Client) (int, int, error) {
	var payload protobuf.CS_24011
	if err := proto.Unmarshal(*buffer, &payload); err != nil {
		return 0, 24012, err
	}

	response := protobuf.SC_24012{Result: proto.Uint32(challengeFailureResult)}
	activity, err := loadActivityTemplate(payload.GetActivityId())
	if err != nil || activity.Type != activityTypeChallenge || !isValidChallengeMode(payload.GetMode()) {
		return client.SendMessage(24012, &response)
	}

	err = orm.WithPGXTx(context.Background(), func(tx pgx.Tx) error {
		return orm.DeleteChallengeModeStateTx(context.Background(), tx, client.Commander.CommanderID, payload.GetActivityId(), payload.GetMode())
	})
	if err != nil {
		return client.SendMessage(24012, &response)
	}

	response.Result = proto.Uint32(challengeSuccessResult)
	return client.SendMessage(24012, &response)
}
