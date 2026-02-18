package answer

import (
	"testing"

	"github.com/ggmolly/belfast/internal/protobuf"
	"google.golang.org/protobuf/proto"
)

func TestActivityBossPageUpdateSuccess(t *testing.T) {
	client := setupConfigTest(t)
	seedConfigEntry(t, "ShareCfg/activity_template.json", "9001", `{"id":9001,"type":52,"config_id":301}`)
	seedConfigEntry(t, activityEventWorldBossCategory, "301", `{"id":301,"boss_id":[8101,8102],"stage_hp":[15000,13000]}`)

	request := protobuf.CS_26031{ActId: proto.Uint32(9001)}
	data, err := proto.Marshal(&request)
	if err != nil {
		t.Fatalf("marshal request failed: %v", err)
	}

	if _, _, err := ActivityBossPageUpdate(&data, client); err != nil {
		t.Fatalf("ActivityBossPageUpdate failed: %v", err)
	}

	var response protobuf.SC_26032
	decodeResponse(t, client, &response)
	if response.GetResult() != 0 {
		t.Fatalf("expected result 0, got %d", response.GetResult())
	}
	if response.GetBossHp() != 15000 {
		t.Fatalf("expected boss hp 15000, got %d", response.GetBossHp())
	}
	if response.GetDeath() != 0 {
		t.Fatalf("expected death 0, got %d", response.GetDeath())
	}
	if len(response.GetMilestones()) != 0 {
		t.Fatalf("expected empty milestones, got %d entries", len(response.GetMilestones()))
	}
}

func TestActivityBossPageUpdateInvalidActivity(t *testing.T) {
	client := setupConfigTest(t)
	request := protobuf.CS_26031{ActId: proto.Uint32(9999)}
	data, err := proto.Marshal(&request)
	if err != nil {
		t.Fatalf("marshal request failed: %v", err)
	}

	if _, _, err := ActivityBossPageUpdate(&data, client); err != nil {
		t.Fatalf("ActivityBossPageUpdate invalid failed: %v", err)
	}

	var response protobuf.SC_26032
	decodeResponse(t, client, &response)
	if response.GetResult() == 0 {
		t.Fatalf("expected non-zero result for invalid activity")
	}
}

func TestActivityBossPageUpdateWrongActivityType(t *testing.T) {
	client := setupConfigTest(t)
	seedConfigEntry(t, "ShareCfg/activity_template.json", "9001", `{"id":9001,"type":37,"config_id":301}`)

	request := protobuf.CS_26031{ActId: proto.Uint32(9001)}
	data, err := proto.Marshal(&request)
	if err != nil {
		t.Fatalf("marshal request failed: %v", err)
	}

	if _, _, err := ActivityBossPageUpdate(&data, client); err != nil {
		t.Fatalf("ActivityBossPageUpdate wrong type failed: %v", err)
	}

	var response protobuf.SC_26032
	decodeResponse(t, client, &response)
	if response.GetResult() == 0 {
		t.Fatalf("expected non-zero result for wrong activity type")
	}
}

func TestActivityBossPageUpdateMalformedBossConfig(t *testing.T) {
	client := setupConfigTest(t)
	seedConfigEntry(t, "ShareCfg/activity_template.json", "9001", `{"id":9001,"type":52,"config_id":301}`)
	seedConfigEntry(t, activityEventWorldBossCategory, "301", `{"id":301,"boss_id":{},"stage_hp":[15000]}`)

	request := protobuf.CS_26031{ActId: proto.Uint32(9001)}
	data, err := proto.Marshal(&request)
	if err != nil {
		t.Fatalf("marshal request failed: %v", err)
	}

	if _, _, err := ActivityBossPageUpdate(&data, client); err != nil {
		t.Fatalf("ActivityBossPageUpdate malformed config failed: %v", err)
	}

	var response protobuf.SC_26032
	decodeResponse(t, client, &response)
	if response.GetResult() == 0 {
		t.Fatalf("expected non-zero result for malformed boss config")
	}
}

func TestActivityBossPageUpdateDecodeFailure(t *testing.T) {
	client := setupConfigTest(t)
	invalid := []byte{0xff, 0x01}
	_, packetID, err := ActivityBossPageUpdate(&invalid, client)
	if err == nil {
		t.Fatalf("expected unmarshal error")
	}
	if packetID != 26032 {
		t.Fatalf("expected packet id 26032, got %d", packetID)
	}
}

func TestGetBoss4thInfoSuccess(t *testing.T) {
	client := setupConfigTest(t)
	seedConfigEntry(t, "ShareCfg/activity_template.json", "9001", `{"id":9001,"type":52,"config_id":301}`)
	seedConfigEntry(t, activityEventWorldBossCategory, "301", `{"id":301,"boss_id":[8101,8102],"stage_hp":[15000,13000]}`)

	request := protobuf.CS_26081{ActId: proto.Uint32(9001)}
	data, err := proto.Marshal(&request)
	if err != nil {
		t.Fatalf("marshal request failed: %v", err)
	}

	if _, _, err := GetBoss4thInfo(&data, client); err != nil {
		t.Fatalf("GetBoss4thInfo failed: %v", err)
	}

	var response protobuf.SC_26082
	decodeResponse(t, client, &response)
	if response.GetResult() != 0 {
		t.Fatalf("expected result 0, got %d", response.GetResult())
	}
	if len(response.GetBossList()) != 2 {
		t.Fatalf("expected 2 bosses, got %d", len(response.GetBossList()))
	}
	if response.GetBossList()[0].GetId() != 8101 || response.GetBossList()[0].GetBossHp() != 15000 {
		t.Fatalf("unexpected first boss payload")
	}
	if response.GetBossList()[0].GetDeath() != 0 || response.GetBossList()[0].GetHourTraffic() != 0 || response.GetBossList()[0].GetHourOff() != 0 {
		t.Fatalf("expected default zero boss counters")
	}
}

func TestGetBoss4thInfoInvalidActivity(t *testing.T) {
	client := setupConfigTest(t)
	request := protobuf.CS_26081{ActId: proto.Uint32(9999)}
	data, err := proto.Marshal(&request)
	if err != nil {
		t.Fatalf("marshal request failed: %v", err)
	}

	if _, _, err := GetBoss4thInfo(&data, client); err != nil {
		t.Fatalf("GetBoss4thInfo invalid failed: %v", err)
	}

	var response protobuf.SC_26082
	decodeResponse(t, client, &response)
	if response.GetResult() == 0 {
		t.Fatalf("expected non-zero result for invalid activity")
	}
	if len(response.GetBossList()) != 0 {
		t.Fatalf("expected empty boss list on failure, got %d", len(response.GetBossList()))
	}
}

func TestGetBoss4thInfoWrongActivityType(t *testing.T) {
	client := setupConfigTest(t)
	seedConfigEntry(t, "ShareCfg/activity_template.json", "9001", `{"id":9001,"type":37,"config_id":301}`)

	request := protobuf.CS_26081{ActId: proto.Uint32(9001)}
	data, err := proto.Marshal(&request)
	if err != nil {
		t.Fatalf("marshal request failed: %v", err)
	}

	if _, _, err := GetBoss4thInfo(&data, client); err != nil {
		t.Fatalf("GetBoss4thInfo wrong type failed: %v", err)
	}

	var response protobuf.SC_26082
	decodeResponse(t, client, &response)
	if response.GetResult() == 0 {
		t.Fatalf("expected non-zero result for wrong activity type")
	}
}

func TestGetBoss4thInfoMalformedBossConfig(t *testing.T) {
	client := setupConfigTest(t)
	seedConfigEntry(t, "ShareCfg/activity_template.json", "9001", `{"id":9001,"type":52,"config_id":301}`)
	seedConfigEntry(t, activityEventWorldBossCategory, "301", `{"id":301,"boss_id":[8101],"stage_hp":{}}`)

	request := protobuf.CS_26081{ActId: proto.Uint32(9001)}
	data, err := proto.Marshal(&request)
	if err != nil {
		t.Fatalf("marshal request failed: %v", err)
	}

	if _, _, err := GetBoss4thInfo(&data, client); err != nil {
		t.Fatalf("GetBoss4thInfo malformed config failed: %v", err)
	}

	var response protobuf.SC_26082
	decodeResponse(t, client, &response)
	if response.GetResult() == 0 {
		t.Fatalf("expected non-zero result for malformed boss config")
	}
	if len(response.GetBossList()) != 0 {
		t.Fatalf("expected empty boss list for malformed config")
	}
}

func TestGetBoss4thInfoDecodeFailure(t *testing.T) {
	client := setupConfigTest(t)
	invalid := []byte{0xff, 0x01}
	_, packetID, err := GetBoss4thInfo(&invalid, client)
	if err == nil {
		t.Fatalf("expected unmarshal error")
	}
	if packetID != 26082 {
		t.Fatalf("expected packet id 26082, got %d", packetID)
	}
}
