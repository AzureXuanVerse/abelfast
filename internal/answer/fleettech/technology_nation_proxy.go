package fleettech

import (
	"sort"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/orm"
	"github.com/ggmolly/belfast/internal/protobuf"
	"google.golang.org/protobuf/proto"
)

func TechnologyNationProxy(buffer *[]byte, client *connection.Client) (int, int, error) {
	groups, templates, err := loadFleetTechConfigs()
	if err != nil {
		return 0, 64000, err
	}
	state, err := orm.GetOrCreateCommanderFleetTechState(client.Commander.CommanderID)
	if err != nil {
		return 0, 64000, err
	}
	for groupID := range groups {
		state.UpsertGroup(groupID)
	}
	maxAdditions := fleetTechBuildMaxAdditions(state, groups, templates)
	techSetList := fleetTechBuildTechSetList(state, maxAdditions)
	state.SetAttrOverrides(make([]orm.FleetTechAttrOverride, 0, len(techSetList)))
	for _, set := range techSetList {
		state.AttrOverrides = append(state.AttrOverrides, orm.FleetTechAttrOverride{ShipType: set.GetShipType(), AttrType: set.GetAttrType(), SetValue: set.GetSetValue()})
	}
	if err := orm.SaveCommanderFleetTechState(state); err != nil {
		return 0, 64000, err
	}

	response := protobuf.SC_64000{
		TechList:    make([]*protobuf.FLEETTECH, 0, len(groups)),
		TechsetList: techSetList,
	}
	groupIDs := make([]uint32, 0, len(groups))
	for groupID := range groups {
		groupIDs = append(groupIDs, groupID)
	}
	sort.Slice(groupIDs, func(i int, j int) bool {
		return groupIDs[i] < groupIDs[j]
	})
	for _, groupID := range groupIDs {
		group, ok := state.GetGroup(groupID)
		if !ok {
			continue
		}
		response.TechList = append(response.TechList, &protobuf.FLEETTECH{
			GroupId:         proto.Uint32(groupID),
			EffectTechId:    proto.Uint32(group.EffectTechID),
			StudyTechId:     proto.Uint32(group.StudyTechID),
			StudyFinishTime: proto.Uint32(group.StudyFinishTime),
			RewardedTech:    proto.Uint32(group.RewardedTechID),
		})
	}
	return client.SendMessage(64000, &response)
}
