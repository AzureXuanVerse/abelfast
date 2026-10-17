package game

import (
	"google.golang.org/protobuf/proto"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/protobuf"
	"github.com/ggmolly/belfast/internal/protocol"
)

// educateAttrs are child_data 1's attributes: stats, personality, talents.
var educateAttrs = []uint32{101, 102, 103, 104, 201, 202, 203, 301, 302, 303, 304, 305, 306}

// handleEducate answers EducateRequestCommand with a TB game that hasn't
// started: child_data 1 at its first day (week 1 of month 3, day 7, the
// client's own default), child_resource defaults for money (20) and mood
// (50), stage 1 site count (0), and every attribute of the template at its
// starting value (child_data attr_N_list / attr_N_value: all 0).
func (game *Game) handleEducate(client *connection.Client, req *protobuf.CS_27000) error {
	var attrs []*protobuf.CHILD_ATTR
	for _, id := range educateAttrs {
		attrs = append(attrs, &protobuf.CHILD_ATTR{Id: proto.Uint32(id), Val: proto.Uint32(0)})
	}
	return client.Send(&protobuf.SC_27001{
		Result: proto.Uint32(0),
		Child: &protobuf.CHILD_INFO{
			Tid:                     proto.Uint32(1),
			CurTime:                 &protobuf.CHILD_TIME{Month: proto.Uint32(3), Week: proto.Uint32(1), Day: proto.Uint32(7)},
			Mood:                    proto.Uint32(50),
			Money:                   proto.Uint32(20),
			SiteNumber:              proto.Uint32(0),
			Attrs:                   attrs,
			Favor:                   &protobuf.CHILD_FAVOR{Lv: proto.Uint32(1), Exp: proto.Uint32(0)},
			Target:                  proto.Uint32(0),
			UserName:                proto.String(""),
			CanTriggerHomeEvent:     proto.Uint32(0),
			IsEnding:                proto.Uint32(0),
			NewGamePlusCount:        proto.Uint32(0),
			HadTargetStageAward:     proto.Uint32(0),
			HadAdjustment:           proto.Uint32(0),
			IsSpecialSecretaryValid: proto.Uint32(0),
			EndingBuyCount:          proto.Uint32(0),
			MemoryBuyCount:          proto.Uint32(0),
			PolaroidBuyCount:        proto.Uint32(0),
		},
	})
}

// handleNewEducate answers NewEducateRequestCommand, sent for each
// child2_data entry: a TB game that hasn't started (round 1, easy, nothing
// unlocked, never played).
func (game *Game) handleNewEducate(client *connection.Client, req *protobuf.CS_29001) error {
	return client.Send(protocol.FillRequired(&protobuf.SC_29002{
		Result: proto.Uint32(0),
		Tb: &protobuf.TBINFO{
			Id:    proto.Uint32(req.GetId()),
			Round: &protobuf.TBROUND{Round: proto.Uint32(1)},
			// NewEducateFSM reads cache[1] unconditionally
			Fsm: &protobuf.TBFSM{Cache: []*protobuf.TBFSMCACHE{{}}},
		},
	}))
}

// handleEducateEndings answers EducateGetEndingsCommand: no TB ending reached.
func (game *Game) handleEducateEndings(client *connection.Client, req *protobuf.CS_27010) error {
	return client.Send(&protobuf.SC_27011{})
}
