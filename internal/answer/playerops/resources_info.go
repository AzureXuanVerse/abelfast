package playerops

import (
	"encoding/json"
	"time"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/orm"
	"github.com/ggmolly/belfast/internal/protobuf"
	"google.golang.org/protobuf/proto"
)

type classUpgradeTemplate struct {
	Level uint32 `json:"level"`
	Time  uint32 `json:"time"`
}

type navalAcademyShoppingStreetTemplate struct {
	SpecialGoodsNum uint32 `json:"special_goods_num"`
}

func ResourcesInfo(buffer *[]byte, client *connection.Client) (int, int, error) {
	response := protobuf.SC_22001{
		OilWellLevel:       proto.Uint32(1),
		OilWellLvUpTime:    proto.Uint32(0),
		GoldWellLevel:      proto.Uint32(1),
		GoldWellLvUpTime:   proto.Uint32(0),
		ClassLv:            proto.Uint32(1),
		ClassLvUpTime:      proto.Uint32(0),
		SkillClassNum:      proto.Uint32(0),
		DailyFinishBuffCnt: proto.Uint32(0),
		Class: &protobuf.NAVALACADEMY_CLASS{
			Proficiency: proto.Uint32(0),
		},
	}
	runtime, err := loadNavalAcademyRuntimeSnapshot(client.Commander.CommanderID, time.Now().UTC())
	if err != nil {
		return 0, 22001, err
	}
	response.OilWellLevel = proto.Uint32(runtime.OilWellLevel)
	response.OilWellLvUpTime = proto.Uint32(runtime.OilUpgradeCompleteTime)
	response.GoldWellLevel = proto.Uint32(runtime.GoldWellLevel)
	response.GoldWellLvUpTime = proto.Uint32(runtime.GoldUpgradeCompleteTime)
	classEntries, err := orm.ListConfigEntries("ShareCfg/class_upgrade_template.json")
	if err != nil {
		return 0, 22001, err
	}
	if len(classEntries) > 0 {
		var template classUpgradeTemplate
		if err := json.Unmarshal(classEntries[0].Data, &template); err != nil {
			return 0, 22001, err
		}
		response.ClassLv = proto.Uint32(template.Level)
		response.ClassLvUpTime = proto.Uint32(template.Time)
	}
	academyEntries, err := orm.ListConfigEntries("ShareCfg/navalacademy_data_template.json")
	if err != nil {
		return 0, 22001, err
	}
	if len(academyEntries) > 0 {
		response.SkillClassNum = proto.Uint32(uint32(len(academyEntries)))
	}
	shoppingEntries, err := orm.ListConfigEntries("ShareCfg/navalacademy_shoppingstreet_template.json")
	if err != nil {
		return 0, 22001, err
	}
	if len(shoppingEntries) > 0 {
		var template navalAcademyShoppingStreetTemplate
		if err := json.Unmarshal(shoppingEntries[0].Data, &template); err != nil {
			return 0, 22001, err
		}
		response.DailyFinishBuffCnt = proto.Uint32(template.SpecialGoodsNum)
	}
	classes, err := orm.ListCommanderSkillClasses(client.Commander.CommanderID)
	if err != nil {
		return 0, 22001, err
	}
	if len(classes) > 0 {
		response.SkillClassList = make([]*protobuf.SKILL_CLASS, 0, len(classes))
		for _, class := range classes {
			response.SkillClassList = append(response.SkillClassList, &protobuf.SKILL_CLASS{
				RoomId:     proto.Uint32(class.RoomID),
				ShipId:     proto.Uint32(class.ShipID),
				StartTime:  proto.Uint32(class.StartTime),
				FinishTime: proto.Uint32(class.FinishTime),
				SkillPos:   proto.Uint32(class.SkillPos),
				Exp:        proto.Uint32(class.Exp),
			})
		}
	}
	usedQuickFinishes, err := orm.GetCommanderDailyQuickFinishUsed(client.Commander.CommanderID, time.Now().UTC())
	if err != nil {
		return 0, 22001, err
	}
	allowance, err := orm.GetCommanderSkillLearnTimeAllowance(client.Commander.CommanderID, time.Now().UTC())
	if err != nil {
		return 0, 22001, err
	}
	if allowance > 0 {
		if usedQuickFinishes >= allowance {
			response.DailyFinishBuffCnt = proto.Uint32(0)
		} else {
			response.DailyFinishBuffCnt = proto.Uint32(allowance - usedQuickFinishes)
		}
	}
	return client.SendMessage(22001, &response)
}
