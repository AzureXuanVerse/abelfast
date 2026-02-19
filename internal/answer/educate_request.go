package answer

import (
	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/orm"
	"github.com/ggmolly/belfast/internal/protobuf"
	"google.golang.org/protobuf/proto"
)

func EducateRequest(buffer *[]byte, client *connection.Client) (int, int, error) {
	state, err := orm.GetOrCreateLegacyEducateState(client.Commander.CommanderID)
	if err != nil {
		return 0, 27001, err
	}

	attrs := []*protobuf.CHILD_ATTR{
		{Id: proto.Uint32(201), Val: proto.Uint32(state.Attrs[201])},
		{Id: proto.Uint32(202), Val: proto.Uint32(state.Attrs[202])},
		{Id: proto.Uint32(203), Val: proto.Uint32(state.Attrs[203])},
	}
	tasks := make([]*protobuf.CHILD_TASK, 0, len(state.TaskProgress))
	for taskID, progress := range state.TaskProgress {
		tasks = append(tasks, &protobuf.CHILD_TASK{Id: proto.Uint32(taskID), Progress: proto.Uint32(progress)})
	}
	optionRecords := make([]*protobuf.CHILD_OPTION_RECORD, 0, len(state.OptionRecords))
	for optionID, count := range state.OptionRecords {
		optionRecords = append(optionRecords, &protobuf.CHILD_OPTION_RECORD{Id: proto.Uint32(optionID), Count: proto.Uint32(count)})
	}

	response := protobuf.SC_27001{
		Result: proto.Uint32(0),
		Child: &protobuf.CHILD_INFO{
			Tid:        proto.Uint32(1),
			Mood:       proto.Uint32(0),
			Money:      proto.Uint32(0),
			SiteNumber: proto.Uint32(0),
			CurTime: &protobuf.CHILD_TIME{
				Month: proto.Uint32(2),
				Day:   proto.Uint32(7),
				Week:  proto.Uint32(4),
			},
			Favor: &protobuf.CHILD_FAVOR{
				Lv:  proto.Uint32(state.FavorLv),
				Exp: proto.Uint32(state.FavorExp),
			},
			Attrs:                   attrs,
			Items:                   []*protobuf.CHILD_ITEM{},
			PlanHistory:             []*protobuf.CHILD_PLAN_HISTORY{},
			Memorys:                 []uint32{},
			Plans:                   []*protobuf.CHILD_PLAN_CELL{},
			Polaroids:               []*protobuf.CHILD_POLAROID{},
			Target:                  proto.Uint32(state.TargetID),
			Tasks:                   tasks,
			RealizedWish:            []uint32{},
			Buffs:                   []*protobuf.CHILD_BUFF{},
			UserName:                proto.String(state.CallName),
			SpecEvents:              []uint32{},
			CanTriggerHomeEvent:     proto.Uint32(0),
			HomeEvents:              []uint32{},
			DiscountEventId:         []uint32{},
			Shop:                    []*protobuf.CHILD_SHOP_DATA{},
			OptionRecords:           optionRecords,
			FavorAwardHistory:       []uint32{},
			IsEnding:                proto.Uint32(0),
			NewGamePlusCount:        proto.Uint32(0),
			HadTargetStageAward:     proto.Uint32(0),
			HadAdjustment:           proto.Uint32(boolToUint32(state.HadAdjustment)),
			IsSpecialSecretaryValid: proto.Uint32(0),
		},
	}
	return client.SendMessage(27001, &response)
}
