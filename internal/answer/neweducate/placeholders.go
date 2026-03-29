package neweducate

import (
	"github.com/ggmolly/belfast/internal/protobuf"
	"google.golang.org/protobuf/proto"
)

func tbInfoPlaceholder() *protobuf.TBINFO {
	return &protobuf.TBINFO{
		Id: proto.Uint32(0),
		Fsm: &protobuf.TBFSM{
			SystemNo:     proto.Uint32(0),
			CurrentNode:  proto.Uint32(0),
			PriorityFsm:  []*protobuf.TBFSM{},
			TarotSelects: []uint32{},
			Cache: []*protobuf.TBFSMCACHE{{
				CachePlan: []*protobuf.TBFSMCACHEPLAN{{
					CurIndex: proto.Uint32(0),
					Plans:    []*protobuf.KVDATA{},
				}},
				CacheTalent: []*protobuf.TBFSMCACHETALENT{{
					Finished:  proto.Uint32(0),
					Talents:   []uint32{},
					Retalents: []uint32{},
				}},
				CacheSite: []*protobuf.TBFSMCACHESITE{{
					Events:             []uint32{},
					Shops:              []uint32{},
					Buys:               []*protobuf.KVDATA{},
					State:              &protobuf.KVDATA{Key: proto.Uint32(0), Value: proto.Uint32(0)},
					CharacterThisRound: []uint32{},
					RefreshCount:       proto.Uint32(0),
				}},
				CacheChat: []*protobuf.TBFSMCACHECHAT{{
					Finished: proto.Uint32(0),
					Chats:    []uint32{},
				}},
				CacheEnd: []*protobuf.TBFSMCACHEEND{{
					Ends:   []uint32{},
					Select: proto.Uint32(0),
				}},
				CacheMind:    []*protobuf.TBFSMCACHEMIND{{}},
				CacheNin1:    []*protobuf.TBFSMCACHENIN1{},
				CacheAffixUp: []*protobuf.TBFSMCACHEAFFIXUP{},
				CacheTarot:   []*protobuf.TBFSMCACHETAROT{},
				CacheEval:    []*protobuf.TBFSMCACHEEVAL{},
			}},
		},
		Round: &protobuf.TBROUND{Round: proto.Uint32(1), InTemp: proto.Uint32(0), TempRound: proto.Uint32(0)},
		Res: &protobuf.TBRES{
			Attrs:    []*protobuf.KVDATA{},
			Resource: []*protobuf.KVDATA{},
		},
		Talent: &protobuf.TBTALENT{Talents: []uint32{}},
		Plan:   &protobuf.TBPLAN{PlanUpgrade: []uint32{}},
		Site: &protobuf.TBSITE{
			Characters:   []uint32{},
			WorkCounter:  []*protobuf.KVDATA{},
			Works:        []uint32{},
			EventCounter: []*protobuf.KVDATA{},
		},
		Evaluations: []*protobuf.KVDATA{},
		Name:        proto.String(""),
		FavorLv:     proto.Uint32(0),
		Benefit:     &protobuf.TBBENEFIT{Actives: []*protobuf.TBBF{}},
		Difficulty:  proto.Uint32(0),
		EvalFail:    proto.Uint32(0),
		Display:     emptyTBDisplay(),
	}
}

func tbPermanentPlaceholder() *protobuf.TBPERMANENT {
	return &protobuf.TBPERMANENT{
		NgPlusCount:   proto.Uint32(1),
		Polaroids:     []uint32{},
		Endings:       []uint32{},
		ActiveEndings: []uint32{},
		TarotArchive:  []uint32{},
		MaxRound:      proto.Uint32(0),
	}
}

func emptyTBDisplay() *protobuf.TBDISPLAY {
	return &protobuf.TBDISPLAY{
		BenefitDisplay:   []*protobuf.TBDROP{},
		DollarNumDisplay: []*protobuf.TBBENEFITVAL{},
		Counter:          []*protobuf.TBBFCOUNTER{},
	}
}
