package game

import (
	"google.golang.org/protobuf/proto"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/protobuf"
	"github.com/ggmolly/belfast/internal/protocol"
)

// educateAttrs are child_data 1's attributes: stats, personality, talents.
var educateAttrs = []uint32{101, 102, 103, 104, 201, 202, 203, 301, 302, 303, 304, 305, 306}

// educateShops are the TB shops' goods at the start of a game (shop id,
// goods ids), each in stock once (captured).
var educateShops = []struct {
	id    uint32
	goods []uint32
}{
	{2, []uint32{2, 3, 4, 5, 11}},
	{3, []uint32{24, 22, 25, 23, 21, 26}},
	{4, []uint32{32, 33, 31}},
	{6, []uint32{56, 54, 55, 53, 52, 51}},
	{22, []uint32{1, 13, 12, 14}},
	{23, []uint32{17, 16, 6, 15}},
	{42, []uint32{42, 41, 36, 35, 34}},
	{43, []uint32{45, 47, 44, 46, 43}},
}

// handleEducate answers EducateRequestCommand with a TB (child_data 1) game
// that hasn't started, as the official server sends it to a new commander:
// month 2 week 4 day 7, child_resource defaults for money (20) and mood (50),
// every attribute at 0, favor level 1 with 30 exp, the first three tasks and
// the starting shops, one new game+ already counted.
func (game *Game) handleEducate(client *connection.Client, req *protobuf.CS_27000) error {
	child := &protobuf.CHILD_INFO{
		Tid:                 proto.Uint32(1),
		CurTime:             &protobuf.CHILD_TIME{Month: proto.Uint32(2), Week: proto.Uint32(4), Day: proto.Uint32(7)},
		Mood:                proto.Uint32(50),
		Money:               proto.Uint32(20),
		Favor:               &protobuf.CHILD_FAVOR{Lv: proto.Uint32(1), Exp: proto.Uint32(30)},
		CanTriggerHomeEvent: proto.Uint32(1),
		NewGamePlusCount:    proto.Uint32(1),
	}
	for _, id := range educateAttrs {
		child.Attrs = append(child.Attrs, &protobuf.CHILD_ATTR{Id: proto.Uint32(id), Val: proto.Uint32(0)})
	}
	for _, id := range []uint32{101, 102, 103} {
		child.Tasks = append(child.Tasks, &protobuf.CHILD_TASK{Id: proto.Uint32(id), Progress: proto.Uint32(0)})
	}
	for _, shop := range educateShops {
		data := &protobuf.CHILD_SHOP_DATA{ShopId: proto.Uint32(shop.id)}
		for _, id := range shop.goods {
			data.Goods = append(data.Goods, &protobuf.CHILD_SHOP_GOODS{Id: proto.Uint32(id), Num: proto.Uint32(1)})
		}
		child.Shop = append(child.Shop, data)
	}
	return client.Send(protocol.FillRequired(&protobuf.SC_27001{Result: proto.Uint32(0), Child: child}))
}

// newEducateStarts are each TB character's (child2_data) starting state on
// the official server: attributes, resources and the benefit active from
// round 1.
var newEducateStarts = map[uint32]struct {
	attrs, resources [][2]uint32
	benefit          uint32
}{
	1: {
		attrs:     [][2]uint32{{201, 155}, {104, 0}, {103, 0}, {102, 0}, {101, 0}},
		resources: [][2]uint32{{4, 50}, {3, 0}, {2, 50}, {1, 50}},
		benefit:   10000,
	},
	2: {
		attrs:     [][2]uint32{{305, 145}, {304, 0}, {303, 0}, {302, 0}, {301, 0}},
		resources: [][2]uint32{{306, 5}, {305, 1}, {304, 50}, {303, 0}, {302, 50}, {301, 50}},
		benefit:   10001,
	},
}

func kvList(pairs [][2]uint32) []*protobuf.KVDATA {
	list := make([]*protobuf.KVDATA, len(pairs))
	for i, pair := range pairs {
		list[i] = &protobuf.KVDATA{Key: proto.Uint32(pair[0]), Value: proto.Uint32(pair[1])}
	}
	return list
}

// handleNewEducate answers NewEducateRequestCommand, sent for each
// child2_data entry: a TB game that hasn't started (round 1, easy, nothing
// unlocked), one new game+ counted.
func (game *Game) handleNewEducate(client *connection.Client, req *protobuf.CS_29001) error {
	tb := &protobuf.TBINFO{
		Id:    proto.Uint32(req.GetId()),
		Round: &protobuf.TBROUND{Round: proto.Uint32(1)},
		// NewEducateFSM reads cache[1] unconditionally
		Fsm: &protobuf.TBFSM{Cache: []*protobuf.TBFSMCACHE{{}}},
	}
	if start, ok := newEducateStarts[req.GetId()]; ok {
		tb.Res = &protobuf.TBRES{Attrs: kvList(start.attrs), Resource: kvList(start.resources)}
		tb.Benefit = &protobuf.TBBENEFIT{Actives: []*protobuf.TBBF{{Id: proto.Uint32(start.benefit), Round: proto.Uint32(1)}}}
	}
	return client.Send(protocol.FillRequired(&protobuf.SC_29002{
		Result:    proto.Uint32(0),
		Tb:        tb,
		Permanent: &protobuf.TBPERMANENT{NgPlusCount: proto.Uint32(1)},
	}))
}

// handleEducateEndings answers EducateGetEndingsCommand: no TB ending reached.
func (game *Game) handleEducateEndings(client *connection.Client, req *protobuf.CS_27010) error {
	return client.Send(&protobuf.SC_27011{})
}
