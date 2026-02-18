package answer

import (
	"context"

	"github.com/jackc/pgx/v5"
	"google.golang.org/protobuf/proto"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/orm"
	"github.com/ggmolly/belfast/internal/protobuf"
)

func LimitChallengeAward(buffer *[]byte, client *connection.Client) (int, int, error) {
	var payload protobuf.CS_24022
	if err := proto.Unmarshal(*buffer, &payload); err != nil {
		return 0, 24023, err
	}

	response := protobuf.SC_24023{Result: proto.Uint32(limitChallengeFailureResult), DropList: []*protobuf.DROPINFO{}}
	requestedIDs := normalizedChallengeIDs(payload.GetChallengeids())
	if len(requestedIDs) == 0 {
		return client.SendMessage(24023, &response)
	}

	monthConfig, err := loadCurrentConstellationChallengeMonth(nowUTC())
	if err != nil {
		return client.SendMessage(24023, &response)
	}
	allowed := make(map[uint32]struct{}, len(monthConfig.Stage))
	for _, challengeID := range monthConfig.Stage {
		allowed[challengeID] = struct{}{}
	}

	drops := map[string]*protobuf.DROPINFO{}
	err = orm.WithPGXTx(context.Background(), func(tx pgx.Tx) error {
		state, err := orm.LoadLimitChallengeStateForUpdateTx(context.Background(), tx, client.Commander.CommanderID, nowUTC())
		if err != nil {
			return err
		}
		passed := make(map[uint32]struct{}, len(state.PassIDs))
		for _, challengeID := range state.PassIDs {
			passed[challengeID] = struct{}{}
		}

		for _, challengeID := range requestedIDs {
			if _, ok := allowed[challengeID]; !ok {
				return errChallengeValidation
			}
			if state.Awarded[challengeID] {
				return errChallengeValidation
			}
			if _, ok := passed[challengeID]; !ok {
				return errChallengeValidation
			}
			template, err := loadConstellationChallengeTemplate(challengeID)
			if err != nil {
				return err
			}
			for _, drop := range template.AwardDisplay {
				if len(drop) < 3 {
					continue
				}
				accumulateDrop(drops, drop[0], drop[1], drop[2])
			}
			state.Awarded[challengeID] = true
		}

		if err := applyLoveLetterDropsTx(context.Background(), tx, client, drops); err != nil {
			return err
		}
		return orm.SaveLimitChallengeStateTx(context.Background(), tx, state)
	})
	if err != nil {
		return client.SendMessage(24023, &response)
	}

	response.Result = proto.Uint32(limitChallengeSuccessResult)
	response.DropList = dropMapToSortedList(drops)
	return client.SendMessage(24023, &response)
}
