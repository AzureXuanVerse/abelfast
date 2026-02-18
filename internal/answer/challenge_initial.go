package answer

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"google.golang.org/protobuf/proto"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/orm"
	"github.com/ggmolly/belfast/internal/protobuf"
)

const (
	challengeModeCasual   uint32 = 0
	challengeModeInfinite uint32 = 1

	challengeSuccessResult uint32 = 0
	challengeFailureResult uint32 = 1

	challengeShipFullHPRatio uint32 = 10000
)

func ChallengeInitial(buffer *[]byte, client *connection.Client) (int, int, error) {
	var payload protobuf.CS_24002
	if err := proto.Unmarshal(*buffer, &payload); err != nil {
		return 0, 24003, err
	}

	state, err := buildChallengeInitialState(client, &payload)
	if err != nil {
		response := protobuf.SC_24003{Result: proto.Uint32(challengeFailureResult)}
		return client.SendMessage(24003, &response)
	}

	err = orm.WithPGXTx(context.Background(), func(tx pgx.Tx) error {
		return orm.UpsertChallengeModeStateTx(context.Background(), tx, state)
	})
	if err != nil {
		response := protobuf.SC_24003{Result: proto.Uint32(challengeFailureResult)}
		return client.SendMessage(24003, &response)
	}

	response := protobuf.SC_24003{Result: proto.Uint32(challengeSuccessResult)}
	return client.SendMessage(24003, &response)
}

func buildChallengeInitialState(client *connection.Client, payload *protobuf.CS_24002) (*orm.ChallengeModeState, error) {
	if !isValidChallengeMode(payload.GetMode()) {
		return nil, errChallengeValidation
	}
	activity, err := loadActivityTemplate(payload.GetActivityId())
	if err != nil {
		return nil, err
	}
	if activity.Type != activityTypeChallenge {
		return nil, errChallengeValidation
	}
	challengeConfig, err := loadActivityEventChallenge(activity)
	if err != nil {
		return nil, err
	}
	seasonID := uint32(1)
	if challengeConfig.ID > 0 {
		seasonID = challengeConfig.ID
	}
	if client.Commander.OwnedShipsMap == nil {
		if err := client.Commander.Load(); err != nil {
			return nil, err
		}
	}

	regularID := payload.GetMode() + 1
	submarineID := payload.GetMode() + 11
	groups := make(map[uint32]*protobuf.GROUPINFO_P24, len(payload.GetGroupList()))
	for _, group := range payload.GetGroupList() {
		if group == nil {
			return nil, errChallengeValidation
		}
		groupID := group.GetId()
		if groupID != regularID && groupID != submarineID {
			return nil, errChallengeValidation
		}
		groups[groupID] = group
	}
	regularGroup := groups[regularID]
	submarineGroup := groups[submarineID]
	if regularGroup == nil || submarineGroup == nil {
		return nil, errChallengeValidation
	}

	seenShip := map[uint32]struct{}{}
	if err := validateChallengeGroup(client, regularGroup, seenShip); err != nil {
		return nil, err
	}
	if err := validateChallengeGroup(client, submarineGroup, seenShip); err != nil {
		return nil, err
	}

	return &orm.ChallengeModeState{
		CommanderID:         client.Commander.CommanderID,
		ActivityID:          payload.GetActivityId(),
		Mode:                payload.GetMode(),
		SeasonID:            seasonID,
		Level:               1,
		CurrentScore:        0,
		Issl:                0,
		RegularGroupID:      regularGroup.GetId(),
		SubmarineGroupID:    submarineGroup.GetId(),
		RegularShipIDs:      cloneUint32Slice(regularGroup.GetShipList()),
		SubmarineShipIDs:    cloneUint32Slice(submarineGroup.GetShipList()),
		RegularCommanders:   toChallengeCommanderSlots(regularGroup.GetCommanders()),
		SubmarineCommanders: toChallengeCommanderSlots(submarineGroup.GetCommanders()),
	}, nil
}

var errChallengeValidation = errors.New("challenge validation failed")

func validateChallengeGroup(client *connection.Client, group *protobuf.GROUPINFO_P24, seenShip map[uint32]struct{}) error {
	for _, shipID := range group.GetShipList() {
		if shipID == 0 {
			return errChallengeValidation
		}
		if _, ok := client.Commander.OwnedShipsMap[shipID]; !ok {
			return errChallengeValidation
		}
		if _, exists := seenShip[shipID]; exists {
			return errChallengeValidation
		}
		seenShip[shipID] = struct{}{}
	}
	for _, commander := range group.GetCommanders() {
		if commander == nil {
			return errChallengeValidation
		}
		if commander.GetId() == 0 {
			continue
		}
		if _, err := orm.GetCommanderMeow(client.Commander.CommanderID, commander.GetId()); err != nil {
			return errChallengeValidation
		}
	}
	return nil
}

func toChallengeCommanderSlots(commanders []*protobuf.COMMANDERSINFO) []orm.ChallengeCommanderSlot {
	if len(commanders) == 0 {
		return []orm.ChallengeCommanderSlot{}
	}
	result := make([]orm.ChallengeCommanderSlot, 0, len(commanders))
	for _, commander := range commanders {
		if commander == nil {
			continue
		}
		result = append(result, orm.ChallengeCommanderSlot{Pos: commander.GetPos(), CommanderID: commander.GetId()})
	}
	return result
}

func cloneUint32Slice(values []uint32) []uint32 {
	if len(values) == 0 {
		return []uint32{}
	}
	result := make([]uint32, len(values))
	copy(result, values)
	return result
}

func isValidChallengeMode(mode uint32) bool {
	return mode == challengeModeCasual || mode == challengeModeInfinite
}
