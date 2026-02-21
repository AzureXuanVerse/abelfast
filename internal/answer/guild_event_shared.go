package answer

import (
	"encoding/json"
	"strconv"

	"github.com/ggmolly/belfast/internal/orm"
	"github.com/ggmolly/belfast/internal/protobuf"
	"google.golang.org/protobuf/proto"
)

const (
	guildEventResultSuccess             = uint32(0)
	guildEventResultFailure             = uint32(1)
	guildEventResultInsufficientCapital = uint32(2)
	guildEventResultInternal            = uint32(3)
	guildEventResultNoActiveOperation   = uint32(20)
)

type guildOperationTemplateEntry struct {
	Consume          uint32 `json:"consume"`
	UnlockGuildLevel uint32 `json:"unlock_guild_level"`
}

type guildPersonShipPage struct {
	PageID  uint32   `json:"page_id"`
	ShipIDs []uint32 `json:"ship_ids"`
}

func loadGuildOperationTemplate(chapterID uint32) (*guildOperationTemplateEntry, error) {
	entry, err := orm.GetConfigEntry("ShareCfg/guild_operation_template.json", strconv.FormatUint(uint64(chapterID), 10))
	if err != nil {
		return nil, err
	}
	var payload guildOperationTemplateEntry
	if err := json.Unmarshal(entry.Data, &payload); err != nil {
		return nil, err
	}
	return &payload, nil
}

func buildEventBase(event orm.GuildOperationEvent) *protobuf.EVENT_BASE {
	shipInEvent := make([]*protobuf.SHIP_IN_EVENT, 0)
	attrAccList := make([]*protobuf.KEYVALUE_P61, 0)
	attrCountList := make([]*protobuf.KEYVALUE_P61, 0)
	eventNodes := make([]*protobuf.EVENT_NODE, 0)
	personShip := make([]*protobuf.PERSON_SHIP_IN_PAGE, 0)

	_ = json.Unmarshal(event.ShipInEvent, &shipInEvent)
	_ = json.Unmarshal(event.AttrAccList, &attrAccList)
	_ = json.Unmarshal(event.AttrCountList, &attrCountList)
	_ = json.Unmarshal(event.EventNodes, &eventNodes)
	_ = json.Unmarshal(event.PersonShip, &personShip)

	return &protobuf.EVENT_BASE{
		EventId:       proto.Uint32(event.EventTid),
		Position:      proto.Uint32(event.Position),
		StartTime:     proto.Uint32(event.StartTime),
		CompleteTime:  proto.Uint32(event.CompleteTime),
		Shipinevent:   shipInEvent,
		AttrAccList:   attrAccList,
		AttrCountList: attrCountList,
		Eventnodes:    eventNodes,
		Efficiency:    proto.Uint32(event.Efficiency),
		Personship:    personShip,
	}
}

func buildOperationResponse(state *orm.GuildOperationState) *protobuf.CURRENT_OPERATION {
	baseEvents := make([]*protobuf.EVENT_BASE, 0)
	completedEvents := make([]*protobuf.EVENT_BASE_COMPLETED, 0)
	formationTime := make([]*protobuf.KEYVALUE_P61, 0, len(state.Events))
	for _, event := range state.Events {
		if event.Completed {
			completedEvents = append(completedEvents, &protobuf.EVENT_BASE_COMPLETED{
				EventId:  proto.Uint32(event.EventTid),
				Position: proto.Uint32(event.Position),
			})
			continue
		}
		baseEvents = append(baseEvents, buildEventBase(event))
		formationTime = append(formationTime, &protobuf.KEYVALUE_P61{Key: proto.Uint32(event.EventTid), Value: proto.Uint32(event.FormationTime)})
	}

	perfs := make([]*protobuf.EVENT_PERFORMANCE, 0, len(state.Perfs))
	for _, perf := range state.Perfs {
		perfs = append(perfs, &protobuf.EVENT_PERFORMANCE{EventId: proto.Uint32(perf.EventTid), Index: proto.Uint32(perf.Index)})
	}

	return &protobuf.CURRENT_OPERATION{
		OperationId:     proto.Uint32(state.ChapterID),
		StartTime:       proto.Uint32(state.StartTime),
		BaseEvents:      baseEvents,
		BossEvent:       nil,
		Perfs:           perfs,
		FormationTime:   formationTime,
		CompletedEvents: completedEvents,
		DailyCount:      proto.Uint32(0),
		Fleets:          []*protobuf.BOSSEVENTFLEET{},
		JoinTimes:       proto.Uint32(state.JoinTimes),
		IsParticipant:   proto.Uint32(state.IsParticipant),
	}
}
