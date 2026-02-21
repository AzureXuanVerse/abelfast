package fleettech

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/orm"
	"github.com/ggmolly/belfast/internal/protobuf"
	"google.golang.org/protobuf/proto"
)

func StartCampTech(buffer *[]byte, client *connection.Client) (int, int, error) {
	payload := &protobuf.CS_64001{}
	if err := proto.Unmarshal(*buffer, payload); err != nil {
		return 0, 64002, err
	}

	response := &protobuf.SC_64002{Result: proto.Uint32(ResultFailure)}
	groupID := payload.GetTechGroupId()
	techID := payload.GetTechId()
	if groupID == 0 || techID == 0 {
		return client.SendMessage(64002, response)
	}

	groups, templates, err := loadFleetTechConfigs()
	if err != nil {
		return 0, 64002, err
	}
	group, ok := groups[groupID]
	if !ok || fleetTechIndexOfTech(group, techID) < 0 {
		return client.SendMessage(64002, response)
	}
	template, ok := templates[techID]
	if !ok || (template.GroupID != 0 && template.GroupID != groupID) {
		return client.SendMessage(64002, response)
	}

	if client.Commander.OwnedResourcesMap == nil {
		if err := client.Commander.Load(); err != nil {
			return 0, 64002, err
		}
	}
	if !client.Commander.HasEnoughResource(1, template.Cost) {
		return client.SendMessage(64002, response)
	}

	now := uint32(time.Now().UTC().Unix())
	err = orm.WithPGXTx(context.Background(), func(tx pgx.Tx) error {
		return withFleetTechStateTx(context.Background(), tx, client.Commander.CommanderID, func(state *orm.CommanderFleetTechState) error {
			if fleetTechHasActiveStudy(state) {
				return errFleetTechValidation
			}
			groupState := state.UpsertGroup(groupID)
			expectedTechID, ok := fleetTechExpectedNextTech(group, groupState.EffectTechID)
			if !ok || expectedTechID != techID {
				return errFleetTechValidation
			}
			if err := client.Commander.ConsumeResourceTx(context.Background(), tx, 1, template.Cost); err != nil {
				if err.Error() == "not enough resources" {
					return errFleetTechValidation
				}
				return err
			}
			groupState.StudyTechID = techID
			groupState.StudyFinishTime = now + template.Time
			return nil
		})
	})
	if err != nil {
		if errors.Is(err, errFleetTechValidation) {
			return client.SendMessage(64002, response)
		}
		return 0, 64002, err
	}

	response.Result = proto.Uint32(ResultSuccess)
	return client.SendMessage(64002, response)
}

func FinishCampTechnology(buffer *[]byte, client *connection.Client) (int, int, error) {
	payload := &protobuf.CS_64003{}
	if err := proto.Unmarshal(*buffer, payload); err != nil {
		return 0, 64004, err
	}

	response := &protobuf.SC_64004{Result: proto.Uint32(ResultFailure)}
	groupID := payload.GetTechGroupId()
	if groupID == 0 {
		return client.SendMessage(64004, response)
	}

	groups, _, err := loadFleetTechConfigs()
	if err != nil {
		return 0, 64004, err
	}
	group, ok := groups[groupID]
	if !ok {
		return client.SendMessage(64004, response)
	}

	now := uint32(time.Now().UTC().Unix())
	err = orm.WithPGXTx(context.Background(), func(tx pgx.Tx) error {
		return withFleetTechStateTx(context.Background(), tx, client.Commander.CommanderID, func(state *orm.CommanderFleetTechState) error {
			groupState, ok := state.GetGroup(groupID)
			if !ok || groupState.StudyTechID == 0 || groupState.StudyFinishTime > now {
				return errFleetTechValidation
			}
			expectedTechID, ok := fleetTechExpectedNextTech(group, groupState.EffectTechID)
			if !ok || expectedTechID != groupState.StudyTechID {
				return errFleetTechValidation
			}
			groupState.EffectTechID = groupState.StudyTechID
			groupState.StudyTechID = 0
			groupState.StudyFinishTime = 0
			return nil
		})
	})
	if err != nil {
		if errors.Is(err, errFleetTechValidation) {
			return client.SendMessage(64004, response)
		}
		return 0, 64004, err
	}

	response.Result = proto.Uint32(ResultSuccess)
	return client.SendMessage(64004, response)
}

type ApplyDropsTxFunc func(ctx context.Context, tx pgx.Tx, client *connection.Client, drops []*protobuf.DROPINFO) error

func ClaimFleetTechCampAward(buffer *[]byte, client *connection.Client, applyDropsTx ApplyDropsTxFunc) (int, int, error) {
	payload := &protobuf.CS_64005{}
	if err := proto.Unmarshal(*buffer, payload); err != nil {
		return 0, 64006, err
	}

	response := &protobuf.SC_64006{Result: proto.Uint32(ResultFailure), Rewards: []*protobuf.DROPINFO{}}
	groupID := payload.GetGroupId()
	techID := payload.GetTechId()
	if groupID == 0 || techID == 0 {
		return client.SendMessage(64006, response)
	}

	groups, templates, err := loadFleetTechConfigs()
	if err != nil {
		return 0, 64006, err
	}
	group, ok := groups[groupID]
	if !ok {
		return client.SendMessage(64006, response)
	}
	technologyIndex := fleetTechIndexOfTech(group, techID)
	if technologyIndex < 0 {
		return client.SendMessage(64006, response)
	}
	template, ok := templates[techID]
	if !ok {
		return client.SendMessage(64006, response)
	}
	rewards := fleetTechClaimDrops(template)

	if err := client.Commander.Load(); err != nil {
		return 0, 64006, err
	}
	err = orm.WithPGXTx(context.Background(), func(tx pgx.Tx) error {
		return withFleetTechStateTx(context.Background(), tx, client.Commander.CommanderID, func(state *orm.CommanderFleetTechState) error {
			groupState, ok := state.GetGroup(groupID)
			if !ok {
				return errFleetTechValidation
			}
			completeIndex := fleetTechIndexOfTech(group, groupState.EffectTechID)
			if completeIndex < 0 || technologyIndex > completeIndex {
				return errFleetTechValidation
			}
			rewardedIndex := fleetTechIndexOfTech(group, groupState.RewardedTechID)
			if technologyIndex <= rewardedIndex || technologyIndex != rewardedIndex+1 {
				return errFleetTechValidation
			}
			if err := applyDropsTx(context.Background(), tx, client, rewards); err != nil {
				return err
			}
			groupState.RewardedTechID = techID
			return nil
		})
	})
	if err != nil {
		if errors.Is(err, errFleetTechValidation) {
			return client.SendMessage(64006, response)
		}
		return 0, 64006, err
	}

	response.Result = proto.Uint32(ResultSuccess)
	response.Rewards = rewards
	return client.SendMessage(64006, response)
}

func ClaimTechnologyCampAwardsOneStep(buffer *[]byte, client *connection.Client, applyDropsTx ApplyDropsTxFunc) (int, int, error) {
	payload := &protobuf.CS_64007{}
	if err := proto.Unmarshal(*buffer, payload); err != nil {
		return 0, 64008, err
	}

	response := &protobuf.SC_64008{Result: proto.Uint32(ResultFailure), Rewards: []*protobuf.DROPINFO{}}
	if payload.GetType() != OneStepClaimType {
		return client.SendMessage(64008, response)
	}

	groups, templates, err := loadFleetTechConfigs()
	if err != nil {
		return 0, 64008, err
	}
	if err := client.Commander.Load(); err != nil {
		return 0, 64008, err
	}

	var rewards []*protobuf.DROPINFO
	err = orm.WithPGXTx(context.Background(), func(tx pgx.Tx) error {
		return withFleetTechStateTx(context.Background(), tx, client.Commander.CommanderID, func(state *orm.CommanderFleetTechState) error {
			mergedRewards := map[string]*protobuf.DROPINFO{}
			for i := range state.Groups {
				groupState := &state.Groups[i]
				group, ok := groups[groupState.GroupID]
				if !ok {
					continue
				}
				completeIndex := fleetTechIndexOfTech(group, groupState.EffectTechID)
				if completeIndex < 0 {
					continue
				}
				rewardedIndex := fleetTechIndexOfTech(group, groupState.RewardedTechID)
				if rewardedIndex >= completeIndex {
					continue
				}
				for idx := rewardedIndex + 1; idx <= completeIndex; idx++ {
					techID := group.Techs[idx]
					template, ok := templates[techID]
					if !ok {
						return errFleetTechValidation
					}
					fleetTechMergeDropList(mergedRewards, fleetTechClaimDrops(template))
				}
				groupState.RewardedTechID = groupState.EffectTechID
			}
			rewards = fleetTechDropMapToSlice(mergedRewards)
			if len(rewards) == 0 {
				return nil
			}
			return applyDropsTx(context.Background(), tx, client, rewards)
		})
	})
	if err != nil {
		if errors.Is(err, errFleetTechValidation) {
			return client.SendMessage(64008, response)
		}
		return 0, 64008, err
	}

	response.Result = proto.Uint32(ResultSuccess)
	response.Rewards = rewards
	return client.SendMessage(64008, response)
}

func SetFleetTechAttrAddition(buffer *[]byte, client *connection.Client) (int, int, error) {
	payload := &protobuf.CS_64009{}
	if err := proto.Unmarshal(*buffer, payload); err != nil {
		return 0, 64010, err
	}

	response := &protobuf.SC_64010{Result: proto.Uint32(ResultFailure)}
	groups, templates, err := loadFleetTechConfigs()
	if err != nil {
		return 0, 64010, err
	}

	err = orm.WithPGXTx(context.Background(), func(tx pgx.Tx) error {
		return withFleetTechStateTx(context.Background(), tx, client.Commander.CommanderID, func(state *orm.CommanderFleetTechState) error {
			maxAdditions := fleetTechBuildMaxAdditions(state, groups, templates)
			normalized, ok := fleetTechNormalizeOverrides(payload.GetTechsetList(), maxAdditions)
			if !ok {
				return errFleetTechValidation
			}
			state.SetAttrOverrides(normalized)
			return nil
		})
	})
	if err != nil {
		if errors.Is(err, errFleetTechValidation) {
			return client.SendMessage(64010, response)
		}
		return 0, 64010, err
	}

	response.Result = proto.Uint32(ResultSuccess)
	return client.SendMessage(64010, response)
}
