package answer

import (
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/orm"
	"github.com/ggmolly/belfast/internal/protobuf"
	"google.golang.org/protobuf/proto"
)

const activityEventWorldBossCategory = "ShareCfg/activity_event_worldboss.json"

type activityEventWorldBossConfig struct {
	ID     uint32          `json:"id"`
	BossID json.RawMessage `json:"boss_id"`
	StageH json.RawMessage `json:"stage_hp"`
}

func ActivityBossPageUpdate(buffer *[]byte, client *connection.Client) (int, int, error) {
	var payload protobuf.CS_26031
	if err := proto.Unmarshal(*buffer, &payload); err != nil {
		return 0, 26032, err
	}

	response := protobuf.SC_26032{
		Result:     proto.Uint32(1),
		BossHp:     proto.Uint32(0),
		Milestones: []uint32{},
		Death:      proto.Uint32(0),
	}

	activity, err := loadActivityTemplate(payload.GetActId())
	if err != nil || activity.Type != activityTypeBossBattleMark2 {
		return client.SendMessage(26032, &response)
	}

	bosses, err := loadActivityBossList(activity)
	if err != nil {
		return client.SendMessage(26032, &response)
	}

	response.Result = proto.Uint32(0)
	if len(bosses) > 0 {
		response.BossHp = proto.Uint32(bosses[0].GetBossHp())
		response.Death = proto.Uint32(bosses[0].GetDeath())
	}

	return client.SendMessage(26032, &response)
}

func GetBoss4thInfo(buffer *[]byte, client *connection.Client) (int, int, error) {
	var payload protobuf.CS_26081
	if err := proto.Unmarshal(*buffer, &payload); err != nil {
		return 0, 26082, err
	}

	response := protobuf.SC_26082{
		Result:   proto.Uint32(1),
		BossList: []*protobuf.BOSS4TH{},
	}

	activity, err := loadActivityTemplate(payload.GetActId())
	if err != nil || activity.Type != activityTypeBossBattleMark2 {
		return client.SendMessage(26082, &response)
	}

	bosses, err := loadActivityBossList(activity)
	if err != nil {
		return client.SendMessage(26082, &response)
	}

	response.Result = proto.Uint32(0)
	response.BossList = bosses
	return client.SendMessage(26082, &response)
}

func loadActivityBossList(activity activityTemplate) ([]*protobuf.BOSS4TH, error) {
	entry, err := orm.GetConfigEntry(activityEventWorldBossCategory, strconv.FormatUint(uint64(activity.ConfigID), 10))
	if err != nil {
		return []*protobuf.BOSS4TH{}, err
	}

	var cfg activityEventWorldBossConfig
	if err := json.Unmarshal(entry.Data, &cfg); err != nil {
		return []*protobuf.BOSS4TH{}, err
	}

	bossIDs, err := parseUint32RawList(cfg.BossID)
	if err != nil {
		return []*protobuf.BOSS4TH{}, err
	}
	stageHP, err := parseUint32RawList(cfg.StageH)
	if err != nil {
		return []*protobuf.BOSS4TH{}, err
	}

	bosses := make([]*protobuf.BOSS4TH, 0, len(bossIDs))
	for idx, bossID := range bossIDs {
		bossHP := uint32(0)
		if idx < len(stageHP) {
			bossHP = stageHP[idx]
		}
		bosses = append(bosses, &protobuf.BOSS4TH{
			Id:          proto.Uint32(bossID),
			BossHp:      proto.Uint32(bossHP),
			Death:       proto.Uint32(0),
			HourTraffic: proto.Uint32(0),
			HourOff:     proto.Uint32(0),
		})
	}

	return bosses, nil
}

func parseUint32RawList(raw json.RawMessage) ([]uint32, error) {
	if len(raw) == 0 {
		return []uint32{}, nil
	}

	var list []uint32
	if err := json.Unmarshal(raw, &list); err == nil {
		return list, nil
	}

	var nested [][]uint32
	if err := json.Unmarshal(raw, &nested); err == nil {
		flat := make([]uint32, 0)
		for _, group := range nested {
			flat = append(flat, group...)
		}
		return flat, nil
	}

	return nil, fmt.Errorf("invalid uint32 list json")
}
