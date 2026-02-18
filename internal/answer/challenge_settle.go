package answer

import (
	"context"

	"github.com/jackc/pgx/v5"
	"google.golang.org/protobuf/encoding/protowire"
	"google.golang.org/protobuf/proto"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/db"
	"github.com/ggmolly/belfast/internal/orm"
	"github.com/ggmolly/belfast/internal/protobuf"
)

func ChallengeSettle(buffer *[]byte, client *connection.Client) (int, int, error) {
	activityID, mode, score := parseChallengeSettlePayload(*buffer)

	if activityID > 0 && isValidChallengeMode(mode) && score > 0 {
		err := orm.WithPGXTx(context.Background(), func(tx pgx.Tx) error {
			state, err := orm.GetChallengeModeStateForUpdateTx(context.Background(), tx, client.Commander.CommanderID, activityID, mode)
			if err != nil {
				if db.IsNotFound(err) {
					return nil
				}
				return err
			}
			state.CurrentScore += score
			return orm.UpsertChallengeModeStateTx(context.Background(), tx, state)
		})
		if err != nil {
			return 0, 24010, err
		}
	}

	response := protobuf.SC_24010{Score: proto.Uint32(score)}
	return client.SendMessage(24010, &response)
}

func parseChallengeSettlePayload(data []byte) (uint32, uint32, uint32) {
	rest := data
	var field1 uint32
	var field2 uint32
	var field3 uint32
	for len(rest) > 0 {
		num, wireType, tagLen := protowire.ConsumeTag(rest)
		if tagLen < 0 {
			break
		}
		rest = rest[tagLen:]

		if wireType != protowire.VarintType {
			skipLen := protowire.ConsumeFieldValue(num, wireType, rest)
			if skipLen < 0 {
				break
			}
			rest = rest[skipLen:]
			continue
		}

		value, valueLen := protowire.ConsumeVarint(rest)
		if valueLen < 0 {
			break
		}
		rest = rest[valueLen:]
		switch num {
		case 1:
			field1 = uint32(value)
		case 2:
			field2 = uint32(value)
		case 3:
			field3 = uint32(value)
		}
	}
	if field3 > 0 {
		return field1, field2, field3
	}
	if field2 > 0 {
		return 0, 0, field2
	}
	return 0, 0, field1
}
