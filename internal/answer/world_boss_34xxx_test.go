package answer

import (
	"testing"
	"time"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/orm"
	"github.com/ggmolly/belfast/internal/protobuf"
	"google.golang.org/protobuf/proto"
)

func worldBossTestClient(commanderID uint32) *connection.Client {
	return &connection.Client{Commander: &orm.Commander{CommanderID: commanderID}}
}

func TestWorldBossActivateAndInfoFlow(t *testing.T) {
	client := worldBossTestClient(1)

	activate := protobuf.CS_34521{TemplateId: proto.Uint32(777)}
	buf, _ := proto.Marshal(&activate)
	if _, _, err := ActivateWorldBoss(&buf, client); err != nil {
		t.Fatalf("activate failed: %v", err)
	}

	var activateResp protobuf.SC_34522
	decodePacketAt(t, client, 0, 34522, &activateResp)
	if activateResp.GetResult() != 0 {
		t.Fatalf("expected activation success")
	}

	client.Buffer.Reset()
	empty := []byte{}
	if _, _, err := WorldBossInfo(&empty, client); err != nil {
		t.Fatalf("world boss info failed: %v", err)
	}
	var info protobuf.SC_34502
	decodePacketAt(t, client, 0, 34502, &info)
	if info.GetSelfBoss().GetTemplateId() != 777 {
		t.Fatalf("expected info self boss template to match activation")
	}
}

func TestWorldBossDamageRankSorted(t *testing.T) {
	client := worldBossTestClient(2)
	state, err := orm.GetOrCreateCommanderWorldBossState(client.Commander.CommanderID)
	if err != nil {
		t.Fatalf("state init failed: %v", err)
	}
	state.SetRankings(17, []orm.WorldBossRankEntry{{CommanderID: 2, Name: "b", Damage: 8}, {CommanderID: 1, Name: "a", Damage: 10}})
	if err := orm.SaveCommanderWorldBossState(state); err != nil {
		t.Fatalf("state save failed: %v", err)
	}

	req := protobuf.CS_34505{BossId: proto.Uint32(17)}
	buf, _ := proto.Marshal(&req)
	if _, _, err := WorldBossDamageRank(&buf, client); err != nil {
		t.Fatalf("rank handler failed: %v", err)
	}

	var resp protobuf.SC_34506
	decodePacketAt(t, client, 0, 34506, &resp)
	if len(resp.GetRankList()) != 2 || resp.GetRankList()[0].GetId() != 1 {
		t.Fatalf("expected rank list sorted by damage desc")
	}
}

func TestWorldBossSupportUpdatesTimer(t *testing.T) {
	client := worldBossTestClient(3)
	state, _ := orm.GetOrCreateCommanderWorldBossState(client.Commander.CommanderID)
	state.SelfBoss = &orm.WorldBossBossState{ID: 1, TemplateID: 7, Lv: 1, Hp: 100, Owner: client.Commander.CommanderID, LastTime: uint32(time.Now().Unix()) + 3600}
	_ = orm.SaveCommanderWorldBossState(state)

	req := protobuf.CS_34509{Type: proto.Uint32(1)}
	buf, _ := proto.Marshal(&req)
	if _, _, err := WorldBossSupport(&buf, client); err != nil {
		t.Fatalf("support failed: %v", err)
	}

	var resp protobuf.SC_34510
	decodePacketAt(t, client, 0, 34510, &resp)
	if resp.GetResult() != 0 {
		t.Fatalf("expected support success")
	}
}

func TestWorldBossStateCheckCodes(t *testing.T) {
	client := worldBossTestClient(4)
	now := uint32(time.Now().Unix())
	state, _ := orm.GetOrCreateCommanderWorldBossState(client.Commander.CommanderID)
	state.SelfBoss = &orm.WorldBossBossState{ID: 5, TemplateID: 5, Lv: 1, Hp: 100, Owner: client.Commander.CommanderID, LastTime: now + 100, FightCount: 10}
	_ = orm.SaveCommanderWorldBossState(state)

	req := protobuf.CS_34515{BossId: proto.Uint32(5), LastTime: proto.Uint32(now + 100)}
	buf, _ := proto.Marshal(&req)
	if _, _, err := WorldBossStateCheck(&buf, client); err != nil {
		t.Fatalf("state check failed: %v", err)
	}
	var resp protobuf.SC_34516
	decodePacketAt(t, client, 0, 34516, &resp)
	if resp.GetResult() != 6 {
		t.Fatalf("expected challenge-limit result code")
	}
}

func TestWorldBossOtherLookupAndFormation(t *testing.T) {
	requester := worldBossTestClient(5)
	requester.Commander.Ships = []orm.OwnedShip{{ID: 9001, OwnerID: 5, ShipID: 202124}}

	requesterState, _ := orm.GetOrCreateCommanderWorldBossState(5)
	requesterState.SelfBoss = &orm.WorldBossBossState{ID: 88, TemplateID: 7, Lv: 1, Hp: 10, Owner: 5, LastTime: uint32(time.Now().Unix()) + 300}
	_ = orm.SaveCommanderWorldBossState(requesterState)

	otherState, _ := orm.GetOrCreateCommanderWorldBossState(6)
	otherState.SelfBoss = &orm.WorldBossBossState{ID: 88, TemplateID: 9, Lv: 1, Hp: 20, Owner: 6, LastTime: uint32(time.Now().Unix()) + 600}
	_ = orm.SaveCommanderWorldBossState(otherState)

	lookup := protobuf.CS_34503{UserIdList: []uint32{6}}
	lookupBuf, _ := proto.Marshal(&lookup)
	if _, _, err := WorldBossOtherBossLookup(&lookupBuf, requester); err != nil {
		t.Fatalf("other lookup failed: %v", err)
	}
	var lookupResp protobuf.SC_34504
	decodePacketAt(t, requester, 0, 34504, &lookupResp)
	if len(lookupResp.GetBossList()) != 1 || lookupResp.GetBossList()[0].GetOwner() != 6 {
		t.Fatalf("expected boss list to include target commander")
	}

	requester.Buffer.Reset()
	formation := protobuf.CS_34519{BossId: proto.Uint32(88), UserId: proto.Uint32(5)}
	formationBuf, _ := proto.Marshal(&formation)
	if _, _, err := WorldBossGetOtherFormation(&formationBuf, requester); err != nil {
		t.Fatalf("formation failed: %v", err)
	}
	var formationResp protobuf.SC_34520
	decodePacketAt(t, requester, 0, 34520, &formationResp)
	if formationResp.GetResult() != 0 || len(formationResp.GetShipList()) == 0 {
		t.Fatalf("expected formation success with ships")
	}
}

func TestWorldBossArchivesAutoBattleStartStop(t *testing.T) {
	client := worldBossTestClient(7)
	state, _ := orm.GetOrCreateCommanderWorldBossState(client.Commander.CommanderID)
	state.SelfBoss = &orm.WorldBossBossState{ID: 55, TemplateID: 7, Lv: 1, Hp: 100, Owner: 7, LastTime: uint32(time.Now().Unix()) + 1800}
	_ = orm.SaveCommanderWorldBossState(state)

	startReq := protobuf.CS_34523{BossId: proto.Uint32(55)}
	startBuf, _ := proto.Marshal(&startReq)
	if _, _, err := WorldBossArchivesAutoBattleStart(&startBuf, client); err != nil {
		t.Fatalf("start failed: %v", err)
	}
	var startResp protobuf.SC_34524
	decodePacketAt(t, client, 0, 34524, &startResp)
	if startResp.GetResult() != 0 || startResp.GetAutoFightFinishTime() == 0 {
		t.Fatalf("expected auto-battle start success")
	}

	client.Buffer.Reset()
	stopReq := protobuf.CS_34525{BossId: proto.Uint32(55)}
	stopBuf, _ := proto.Marshal(&stopReq)
	if _, _, err := WorldBossArchivesStopAutoBattle(&stopBuf, client); err != nil {
		t.Fatalf("stop failed: %v", err)
	}
	var stopResp protobuf.SC_34526
	decodePacketAt(t, client, 0, 34526, &stopResp)
	if stopResp.GetResult() != 0 || stopResp.GetCount() == 0 {
		t.Fatalf("expected auto-battle stop success with settlement")
	}
}

func TestWorldBossSubmitAwardAndOvertimeClear(t *testing.T) {
	client := worldBossTestClient(8)
	state, _ := orm.GetOrCreateCommanderWorldBossState(client.Commander.CommanderID)
	state.SelfBoss = &orm.WorldBossBossState{ID: 6, TemplateID: 8, Lv: 1, Hp: 0, Owner: 8, LastTime: uint32(time.Now().Unix()) + 1000}
	_ = orm.SaveCommanderWorldBossState(state)

	claimReq := protobuf.CS_34511{BossId: proto.Uint32(6)}
	claimBuf, _ := proto.Marshal(&claimReq)
	if _, _, err := WorldBossSubmitAward(&claimBuf, client); err != nil {
		t.Fatalf("submit award failed: %v", err)
	}
	var claimResp protobuf.SC_34512
	decodePacketAt(t, client, 0, 34512, &claimResp)
	if claimResp.GetResult() != 0 || len(claimResp.GetDrops()) == 0 {
		t.Fatalf("expected claim success with drops")
	}

	client.Buffer.Reset()
	overtimeReq := protobuf.CS_34513{Type: proto.Uint32(0)}
	overtimeBuf, _ := proto.Marshal(&overtimeReq)
	if _, _, err := WorldBossOvertimeClear(&overtimeBuf, client); err != nil {
		t.Fatalf("overtime clear failed: %v", err)
	}
	var overtimeResp protobuf.SC_34514
	decodePacketAt(t, client, 0, 34514, &overtimeResp)
	if overtimeResp.GetResult() != 0 {
		t.Fatalf("expected overtime clear success")
	}
}
